//go:build linux

// * rebootera-agent: única pieza de Rebootera OS que se ejecuta como root.
// * Escucha en un socket Unix, comprueba con las credenciales del socket que
// * quien llama es el usuario de rebootera-core y ejecuta solo las operaciones
// * de una lista cerrada (ver agentproto).
// ! Nunca ejecuta órdenes arbitrarias ni acepta rutas o argumentos libres.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"rebootera/core/internal/agentproto"
	"rebootera/core/internal/version"
)

// kernelLogLines es el número de líneas del registro del kernel que se devuelven.
const kernelLogLines = 60

// ! Patrones que no deben salir del agente: números de serie de dispositivos
// ! USB y discos aparecen en el registro del kernel y no aportan nada a la UI.
var secretPattern = regexp.MustCompile(`(?i)(serial ?number|serial:|iserial)`)

// agent guarda la configuración del agente.
type agent struct {
	allowedUID uint32
	clientUser string
}

// * main prepara el socket y atiende las conexiones de una en una.
func main() {
	socketPath := flag.String("socket", agentproto.SocketPath, "ruta del socket Unix")
	clientUser := flag.String("client-user", "rebootera-core", "único usuario autorizado a usar el agente")
	onlyVault := flag.Bool("mount-vault", false, "solo monta el Vault del USB de arranque y termina")
	flag.Parse()
	log.SetFlags(0)

	// * Modo de arranque (rebootera-vault.service): monta el Vault y termina.
	if *onlyVault {
		if err := mountVault(*clientUser); err != nil {
			log.Fatalf("vault: %v", err)
		}
		return
	}

	clientAccount, err := user.Lookup(*clientUser)
	if err != nil {
		log.Fatalf("no existe el usuario %q: %v", *clientUser, err)
	}
	uid, _ := strconv.ParseUint(clientAccount.Uid, 10, 32)
	gid, _ := strconv.Atoi(clientAccount.Gid)

	os.Remove(*socketPath)
	listener, err := net.Listen("unix", *socketPath)
	if err != nil {
		log.Fatalf("no se pudo abrir el socket: %v", err)
	}
	// * Solo root y el grupo del usuario cliente pueden conectar.
	if err := os.Chown(*socketPath, 0, gid); err != nil {
		log.Fatalf("no se pudo asignar el grupo del socket: %v", err)
	}
	if err := os.Chmod(*socketPath, 0o660); err != nil {
		log.Fatalf("no se pudieron asignar los permisos del socket: %v", err)
	}

	a := &agent{allowedUID: uint32(uid), clientUser: *clientUser}
	log.Printf("rebootera-agent %s escuchando en %s (cliente autorizado: %s)", version.Version, *socketPath, *clientUser)
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("error al aceptar conexión: %v", err)
			continue
		}
		a.serve(conn.(*net.UnixConn))
	}
}

// * serve atiende una conexión: una petición y una respuesta.
func (a *agent) serve(conn *net.UnixConn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	uid, err := peerUID(conn)
	if err != nil || uid != a.allowedUID {
		log.Printf("conexión rechazada (uid %d)", uid)
		json.NewEncoder(conn).Encode(agentproto.Response{Error: "no autorizado"})
		return
	}

	var req agentproto.Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		json.NewEncoder(conn).Encode(agentproto.Response{Error: "petición no válida"})
		return
	}
	lines, err := a.handle(req)
	if err != nil {
		log.Printf("%s: %v", req.Op, err)
		json.NewEncoder(conn).Encode(agentproto.Response{Error: err.Error()})
		return
	}
	if isStateChanging(req.Op) {
		log.Printf("%s %s: hecho", req.Op, req.Device)
	}
	json.NewEncoder(conn).Encode(agentproto.Response{OK: true, Lines: lines})
}

// * handle ejecuta una operación de la lista cerrada.
func (a *agent) handle(req agentproto.Request) ([]string, error) {
	switch req.Op {
	case agentproto.OpPing:
		return nil, nil
	case agentproto.OpReboot:
		return nil, run("systemctl", "reboot")
	case agentproto.OpPoweroff:
		return nil, run("systemctl", "poweroff")
	case agentproto.OpKernelLog:
		return kernelLog()
	case agentproto.OpMediaMount:
		return nil, mountMedia(req.Device, a.clientUser)
	case agentproto.OpMediaUnmount:
		return nil, unmountMedia(req.Device)
	}
	return nil, fmt.Errorf("operación no admitida: %q", req.Op)
}

// * isStateChanging indica si una operación cambia el sistema y debe quedar
// * registrada. Las consultas periódicas no se registran para no llenar el diario.
func isStateChanging(op string) bool {
	return op != agentproto.OpPing && op != agentproto.OpKernelLog
}

// * kernelLog devuelve las últimas líneas del registro del kernel de este
// * arranque, sin las que contienen números de serie.
func kernelLog() ([]string, error) {
	out, err := exec.Command("journalctl", "--kernel", "--boot", "--no-pager", "--quiet",
		"--output=short-monotonic", "--lines="+strconv.Itoa(kernelLogLines)).Output()
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer el registro del kernel: %w", err)
	}
	lines := []string{}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || secretPattern.MatchString(line) {
			continue
		}
		lines = append(lines, line)
	}
	return lines, nil
}

// * peerUID devuelve el uid del proceso al otro lado del socket.
func peerUID(conn *net.UnixConn) (uint32, error) {
	raw, err := conn.SyscallConn()
	if err != nil {
		return 0, err
	}
	reader := &credReader{}
	if err := raw.Control(reader.read); err != nil {
		return 0, err
	}
	if reader.err != nil {
		return 0, reader.err
	}
	if reader.cred == nil {
		return 0, errors.New("sin credenciales")
	}
	return reader.cred.Uid, nil
}

// credReader lee SO_PEERCRED desde el descriptor del socket.
type credReader struct {
	cred *syscall.Ucred
	err  error
}

// * read se llama con el descriptor del socket y guarda sus credenciales.
func (c *credReader) read(fd uintptr) {
	c.cred, c.err = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
}

// * run ejecuta un programa del sistema con argumentos fijos.
func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s falló: %v: %s", name, err, out)
	}
	return nil
}

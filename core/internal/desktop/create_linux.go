//go:build linux

// * Preparar un disco USB en Linux: Ventoy2Disk.sh (del paquete oficial de Ventoy, verificado) con permisos de
// * administrador que pide polkit (pkexec), y el disco ya con su partición REBOOTERA se monta con udisks.
// ! Ventoy2Disk.sh BORRA el disco. Solo se aceptan discos con conexión USB que no contienen el sistema, y el
// ! dispositivo que llega al script es siempre /dev/<nombre> leído de lsblk (nunca texto de la interfaz).
package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strconv"
	"strings"
	"time"

	"rebootera/core/internal/storage"
)

// devicePath es la forma de un nodo de disco o partición que se acepta: /dev/sdb, /dev/nvme0n1p1, /dev/mmcblk0...
var devicePath = regexp.MustCompile(`^/dev/[a-z0-9]+$`)

// accountName es la forma de un nombre de usuario que se acepta dentro de una ruta de montaje.
var accountName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// lsblkNode es un disco o partición en la salida JSON de lsblk (los campos numéricos pueden venir como texto).
type lsblkNode struct {
	Name      string      `json:"name"`
	Path      string      `json:"path"`
	Type      string      `json:"type"`
	Size      flexNumber  `json:"size"`
	Transport string      `json:"tran"`
	Removable flexNumber  `json:"rm"`
	Model     string      `json:"model"`
	Vendor    string      `json:"vendor"`
	FSType    string      `json:"fstype"`
	Label     string      `json:"label"`
	Mount     string      `json:"mountpoint"`
	Mounts    []string    `json:"mountpoints"`
	MajMin    string      `json:"maj:min"`
	Children  []lsblkNode `json:"children"`
}

// flexNumber lee un número que lsblk escribe como número, como texto ("1") o como booleano (true/false).
type flexNumber uint64

// UnmarshalJSON implementa json.Unmarshaler.
func (n *flexNumber) UnmarshalJSON(data []byte) error {
	text := strings.Trim(strings.TrimSpace(string(data)), `"`)
	switch text {
	case "", "null", "false":
		*n = 0
	case "true":
		*n = 1
	default:
		value, err := strconv.ParseUint(text, 10, 64)
		if err != nil {
			return err
		}
		*n = flexNumber(value)
	}
	return nil
}

// * mountpoints devuelve los puntos de montaje de un nodo (lsblk antiguo da uno solo; el nuevo, una lista).
func (n lsblkNode) mountpoints() []string {
	var found []string
	for _, mount := range append([]string{n.Mount}, n.Mounts...) {
		if mount != "" {
			found = append(found, mount)
		}
	}
	return found
}

// * usbDisksFrom convierte la salida de lsblk en la lista de discos USB donde se puede crear Rebootera. Un disco del
// * sistema (el que tiene montado / o /boot) nunca se incluye. allowLoop deja pasar los dispositivos loop (solo pruebas).
func usbDisksFrom(raw []byte, allowLoop bool) ([]USBDisk, error) {
	var parsed struct {
		Devices []lsblkNode `json:"blockdevices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("no se pudo leer la lista de discos: %w", err)
	}
	disks := []USBDisk{}
	for _, node := range parsed.Devices {
		isUSB := node.Transport == "usb" || (allowLoop && node.Type == "loop")
		if !(node.Type == "disk" || (allowLoop && node.Type == "loop")) || !isUSB || !devicePath.MatchString(node.Path) {
			continue
		}
		if holdsSystem(node) {
			continue
		}
		name := strings.TrimSpace(strings.TrimSpace(node.Vendor) + " " + strings.TrimSpace(node.Model))
		if name == "" {
			name = node.Name
		}
		disk := USBDisk{Number: diskNumber(node.MajMin), Path: node.Path, FriendlyName: name, SizeBytes: uint64(node.Size)}
		for _, part := range node.Children {
			if part.Label == "REBOOTERA" {
				disk.Rebootera = true
			}
			mounts := part.mountpoints()
			volume := USBVolume{Letter: part.Path, Label: part.Label, FileSystem: part.FSType, SizeBytes: uint64(part.Size)}
			if len(mounts) > 0 {
				volume.Letter = mounts[0]
				used, free := storage.FSUsage(mounts[0])
				volume.FreeBytes = free
				disk.UsedBytes += used
			}
			disk.Volumes = append(disk.Volumes, volume)
		}
		disks = append(disks, disk)
	}
	return disks, nil
}

// * holdsSystem indica si el disco o alguna de sus particiones tiene montado el sistema (/) o el arranque (/boot).
func holdsSystem(node lsblkNode) bool {
	for _, mount := range node.mountpoints() {
		if mount == "/" || mount == "/boot" || strings.HasPrefix(mount, "/boot/") || mount == "/usr" || mount == "/home" {
			return true
		}
	}
	for _, child := range node.Children {
		if holdsSystem(child) {
			return true
		}
	}
	return false
}

// * diskNumber da un número estable a un disco a partir de su "mayor:menor" (la interfaz identifica el disco con él).
func diskNumber(majMin string) int {
	major, minor, found := strings.Cut(majMin, ":")
	if !found {
		return 0
	}
	a, errA := strconv.Atoi(strings.TrimSpace(major))
	b, errB := strconv.Atoi(strings.TrimSpace(minor))
	if errA != nil || errB != nil {
		return 0
	}
	return a*1048576 + b
}

// * USBDisks lista los discos USB conectados.
func USBDisks() ([]USBDisk, error) {
	out, err := exec.Command("lsblk", "-J", "-b", "-o", "NAME,PATH,TYPE,SIZE,TRAN,RM,MODEL,VENDOR,FSTYPE,LABEL,MOUNTPOINT,MAJ:MIN").Output()
	if err != nil {
		return nil, errors.New("no se pudieron listar los discos (¿falta lsblk?)")
	}
	// * REBOOTERA_TEST_ALLOW_LOOP solo existe para las pruebas con discos virtuales (loop): nunca lo pone la aplicación.
	return usbDisksFrom(out, os.Getenv("REBOOTERA_TEST_ALLOW_LOOP") == "1")
}

// * isRoot indica si la aplicación ya corre como administrador (entonces no hace falta pedir permisos).
func isRoot() bool {
	return os.Geteuid() == 0
}

// * elevated ejecuta un script de sh como administrador con polkit (pkexec) y devuelve su salida. Los datos van como
// * argumentos posicionales ($1, $2...), nunca dentro del texto del script.
func elevated(script string, args ...string) (string, error) {
	command := append([]string{"/bin/sh", "-c", script, "sh"}, args...)
	if !isRoot() {
		if _, err := exec.LookPath("pkexec"); err != nil {
			return "", errors.New("hace falta pkexec (polkit) para pedir permisos de administrador; instala polkit o ejecuta Rebootera Desktop como root")
		}
		command = append([]string{"pkexec"}, command...)
	}
	out, err := exec.Command(command[0], command[1:]...).CombinedOutput()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && (exit.ExitCode() == 126 || exit.ExitCode() == 127) && !isRoot() {
			return string(out), errors.New("se canceló la petición de permisos de administrador")
		}
		return string(out), fmt.Errorf("%w: %s", err, lastLines(string(out), 4))
	}
	return string(out), nil
}

// linuxInstallScript desmonta lo que haya montado del disco y ejecuta Ventoy2Disk.sh ($1 carpeta de Ventoy, $2 disco).
// Ventoy pregunta dos veces si continuar (las respuestas "y" van por la entrada); -g usa GPT, -s activa Secure Boot y
// -L pone la etiqueta REBOOTERA a la partición de datos.
const linuxInstallScript = `set -e
for part in $(lsblk -nrpo NAME,MOUNTPOINT "$2" | awk 'NF > 1 { print $1 }'); do umount "$part"; done
cd "$1"
printf 'y\ny\n' | ./Ventoy2Disk.sh -I -g -s -L REBOOTERA "$2"`

// * prepareDisk instala Ventoy en el disco, espera a su partición de datos y la monta; devuelve su punto de montaje.
func prepareDisk(j *createJob) (string, error) {
	setPhase("ventoy-download", 0)
	ventoyDir, err := ensureVentoyTree()
	if err != nil {
		return "", err
	}
	if !devicePath.MatchString(j.disk.Path) {
		return "", fmt.Errorf("dispositivo no válido: %q", j.disk.Path)
	}
	setPhase("ventoy-install", 0)
	if _, err := elevated(linuxInstallScript, ventoyDir, j.disk.Path); err != nil {
		return "", fmt.Errorf("Ventoy no pudo preparar el USB: %w", err)
	}
	setPhase("label", 0)
	part, err := waitDataPartition(j.disk.Path)
	if err != nil {
		return "", err
	}
	return mountPartition(part)
}

// * waitDataPartition espera a que el sistema vea la partición REBOOTERA del disco recién preparado. Pregunta por la
// * etiqueta con lsblk y, si el sistema no tiene udev al día (WSL, contenedores), con blkid.
func waitDataPartition(disk string) (string, error) {
	exec.Command("udevadm", "settle").Run()
	last := ""
	for attempt := 0; attempt < 30; attempt++ {
		exec.Command("partprobe", disk).Run()
		out, err := exec.Command("lsblk", "-nrpo", "NAME,TYPE", disk).Output()
		if err == nil {
			for _, line := range strings.Split(string(out), string(rune(10))) {
				fields := strings.Fields(line)
				if len(fields) != 2 || fields[1] != "part" || !devicePath.MatchString(fields[0]) {
					continue
				}
				label, _ := exec.Command("blkid", "-s", "LABEL", "-o", "value", fields[0]).Output()
				last = strings.TrimSpace(string(label))
				if last == "REBOOTERA" {
					return fields[0], nil
				}
			}
		}
		time.Sleep(time.Second)
	}
	return "", fmt.Errorf("el sistema no vio la partición REBOOTERA del nuevo USB (última etiqueta leída: %q)", last)
}

// mountedAt busca el punto de montaje en la respuesta de udisksctl ("Mounted /dev/sdb1 at /run/media/ana/REBOOTERA").
var mountedAt = regexp.MustCompile(`(?m)\bat (/[^\n]*?)\.?\s*$`)

// * mountPartition monta la partición para el usuario: con udisks (sin pedir nada más) o, si no está, con pkexec.
func mountPartition(part string) (string, error) {
	if out, err := exec.Command("udisksctl", "mount", "-b", part).CombinedOutput(); err == nil {
		if match := mountedAt.FindStringSubmatch(string(out)); match != nil {
			return match[1], nil
		}
	} else if strings.Contains(string(out), "already mounted") {
		if match := regexp.MustCompile(`at ` + "`" + `([^']+)'`).FindStringSubmatch(string(out)); match != nil {
			return match[1], nil
		}
	}
	// * Sin udisks se monta en /run/media/<usuario>/REBOOTERA (donde los escritorios montan los USB y Rebootera Desktop los
	// * reconoce). El nombre de usuario se valida porque forma parte de una ruta que crea el administrador.
	account, err := user.Current()
	if err != nil || !accountName.MatchString(account.Username) {
		return "", errors.New("no se pudo saber el usuario para montar el nuevo USB")
	}
	dir := "/run/media/" + account.Username + "/REBOOTERA"
	// * exFAT no guarda propietarios: uid y gid del usuario hacen que pueda escribir sin ser administrador. Si el
	// * kernel no trae exFAT (algunas distribuciones y WSL), se usa el controlador FUSE.
	const mountScript = `mkdir -p "$4" && { mount -o "uid=$1,gid=$2" "$3" "$4" || mount.exfat-fuse -o "uid=$1,gid=$2" "$3" "$4"; }`
	if _, err := elevated(mountScript, strconv.Itoa(os.Getuid()), strconv.Itoa(os.Getgid()), part, dir); err != nil {
		return "", fmt.Errorf("no se pudo montar el nuevo USB: %w", err)
	}
	return dir, nil
}

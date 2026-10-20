// * rebootera-core: servicio sin privilegios de Rebootera OS. Sirve la
// * interfaz y su API en la interfaz de bucle local y delega en
// * rebootera-agent todo lo que necesita permisos de administrador.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"rebootera/core/internal/agentproto"
	"rebootera/core/internal/api"
	"rebootera/core/internal/boot"
	"rebootera/core/internal/library"
	"rebootera/core/internal/transfers"
	"rebootera/core/internal/version"
)

// * main lee las opciones, genera el token del arranque y arranca el servidor.
func main() {
	listen := flag.String("listen", "127.0.0.1:7080", "dirección de escucha (solo bucle local)")
	uiDir := flag.String("ui", "/usr/share/rebootera/ui", "carpeta con los archivos de la interfaz")
	tokenFile := flag.String("token-file", "/run/rebootera-core/ui-token", "archivo donde se publica el token para la ventana kiosco")
	flag.Parse()
	log.SetFlags(0)

	host, port, err := net.SplitHostPort(*listen)
	if err != nil {
		log.Fatalf("dirección de escucha no válida: %v", err)
	}
	// ! Nunca se expone la API fuera del propio equipo.
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		log.Fatalf("rebootera-core solo puede escuchar en una dirección de bucle local, no en %q", host)
	}
	if _, err := os.Stat(filepath.Join(*uiDir, "index.html")); err != nil {
		log.Fatalf("no se encuentra la interfaz en %s: %v", *uiDir, err)
	}

	token, err := newToken()
	if err != nil {
		log.Fatalf("no se pudo generar el token: %v", err)
	}
	if err := writeToken(*tokenFile, token); err != nil {
		log.Fatalf("no se pudo publicar el token: %v", err)
	}

	// * Rebootera ha vuelto a arrancar: si Ventoy tenía preseleccionada otra
	// * imagen (para una instalación), vuelve a ser Rebootera OS la de siempre.
	if changed, err := boot.Restore(); err != nil {
		log.Printf("arranque: no se pudo restaurar la imagen por defecto: %v", err)
	} else if changed {
		log.Printf("arranque: Rebootera OS vuelve a ser la imagen por defecto de Ventoy")
	}

	// * Si se pidió "arrancar el instalador una sola vez", el agente restaura el
	// * orden de arranque original del firmware (UEFI) ahora que se ha vuelto.
	if lines, err := (api.SocketAgent{}).Query(agentproto.OpBootRestore); err != nil {
		log.Printf("arranque: no se pudo restaurar el orden de arranque: %v", err)
	} else if len(lines) > 0 && lines[0] == "restored" {
		log.Printf("arranque: orden de arranque del firmware restaurado")
	}

	// * La distribución de teclado elegida en otro arranque vuelve a aplicarse.
	if layout := api.LoadKeyboard(); layout != "" && layout != agentproto.DefaultKeyboard {
		if _, err := (api.SocketAgent{}).Send(agentproto.Request{Op: agentproto.OpKeyboardSet, Layout: layout}); err != nil {
			log.Printf("teclado: no se pudo aplicar %q: %v", layout, err)
		}
	}

	// * Lo mismo con el puntero y el panel táctil (sensibilidad, toque para clic, desplazamiento).
	if saved := api.LoadInput(); saved != nil && *saved != agentproto.DefaultInput {
		if _, err := (api.SocketAgent{}).Send(agentproto.Request{Op: agentproto.OpInputSet, Input: saved}); err != nil {
			log.Printf("puntero: no se pudieron aplicar los ajustes: %v", err)
		}
	}

	// * Si el USB recuerda alguna red Wi-Fi, se conecta sola en cuanto el adaptador esté listo.
	go api.AutoConnectWifi(api.SocketAgent{})

	lib := library.New()
	queue := transfers.New(lib)
	go queue.Run()

	server := &http.Server{
		Addr:              *listen,
		Handler:           api.New(os.DirFS(*uiDir), token, port, api.SocketAgent{}, api.Deps{Library: lib, Transfers: queue, Mode: api.ModeLive}).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("rebootera-core %s escuchando en http://%s", version.Version, *listen)
	log.Fatal(server.ListenAndServe())
}

// * newToken genera un token aleatorio de 256 bits.
func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// * writeToken guarda el token con permisos 0640 de forma atómica: solo el
// * propio servicio y su grupo (el de la ventana kiosco) pueden leerlo.
func writeToken(path, token string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(token), 0o640); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

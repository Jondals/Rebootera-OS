// * rebootera-desktop: Rebootera Desktop, la aplicación para Windows y Linux
// * que gestiona un USB de Rebootera sin arrancar desde él: biblioteca,
// * descargas oficiales, añadir imágenes del equipo, actualizar Rebootera OS y
// * (en Windows) crear un USB nuevo.
// * Lleva dentro la misma interfaz que Rebootera OS (ui/) y el contenido común
// * de un USB (kit/), que prepara scripts/build-desktop.sh.
// ! Escucha solo en 127.0.0.1, en un puerto aleatorio, con un token por
// ! ejecución: ninguna otra máquina puede usar su API.
package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"flag"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"rebootera/core/internal/api"
	"rebootera/core/internal/desktop"
	"rebootera/core/internal/library"
	"rebootera/core/internal/transfers"
	"rebootera/core/internal/vault"
	"rebootera/core/internal/version"
)

// uiFiles es la interfaz compilada (ui/dist), copiada por build-desktop.sh.
//
//go:embed all:ui
var uiFiles embed.FS

// kitFiles es el contenido común de un USB nuevo (Ventoy, tema, README).
//
//go:embed all:kit
var kitFiles embed.FS

// * main arranca el servidor local y abre la ventana.
func main() {
	// ! -url-file solo sirve para pruebas automáticas: escribe la dirección
	// ! (con su token) en un archivo que elige quien lanza la aplicación.
	urlFile := flag.String("url-file", "", "archivo donde escribir la dirección local (pruebas)")
	noWindow := flag.Bool("no-window", false, "solo el servidor local, sin abrir ninguna ventana (pruebas)")
	route := flag.String("route", "", "sección con la que se abre la interfaz (pruebas)")
	usbRoot := flag.String("usb-root", "", "carpeta que se trata como USB de Rebootera (pruebas)")
	prepare := flag.Bool("prepare-ventoy", false, "solo descarga y verifica Ventoy (no toca ningún disco)")
	flag.Parse()
	setupLog()
	if *prepare {
		dir, err := desktop.PrepareVentoy()
		if err != nil {
			log.Printf("Ventoy: %v", err)
			os.Exit(1)
		}
		log.Printf("Ventoy verificado en %s", dir)
		return
	}
	ui, err := fs.Sub(uiFiles, "ui")
	if err != nil {
		log.Fatal(err)
	}
	kit, err := fs.Sub(kitFiles, "kit")
	if err != nil {
		log.Fatal(err)
	}

	// * Las imágenes solo se guardan en un USB de Rebootera: sin USB no hay dónde descargar y la
	// * interfaz pide conectarlo (nunca se llena el disco del PC en silencio).
	if *usbRoot != "" && desktop.IsRebootera(*usbRoot) {
		vault.Root = *usbRoot
		log.Printf("USB de Rebootera de pruebas en %s", vault.Root)
	} else if drives := desktop.RebooteraDrives(); len(drives) == 1 {
		vault.Root = drives[0].Root
		log.Printf("USB de Rebootera detectado en %s", vault.Root)
	} else {
		vault.Root = filepath.Join(os.TempDir(), "rebootera-no-usb")
		log.Printf("sin USB de Rebootera: se espera a que se conecte uno")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	token := newToken()
	exe, _ := os.Executable()

	lib := library.New()
	queue := transfers.New(lib)
	go queue.Run()
	server := &http.Server{
		Handler: api.New(ui, token, port, nil, api.Deps{
			Library:    lib,
			Transfers:  queue,
			Mode:       api.ModeDesktop,
			Kit:        kit,
			Executable: exe,
		}).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go server.Serve(listener)

	url := "http://127.0.0.1:" + port + "/auth?token=" + token
	if *route != "" {
		url += "&route=" + *route
	}
	log.Printf("Rebootera Desktop %s en http://127.0.0.1:%s", version.Version, port)
	if *urlFile != "" {
		os.WriteFile(*urlFile, []byte(url), 0o600)
	}
	// ! -no-window evita que las pruebas automáticas abran ventanas y roben el foco del equipo.
	if *noWindow {
		select {}
	}
	openWindow(url)
}

// * newToken genera un token aleatorio de 256 bits.
func newToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		log.Fatal(err)
	}
	return hex.EncodeToString(buf)
}

// * dataDir es la carpeta de datos de la aplicación en el equipo (nunca el USB).
func dataDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "Rebootera")
	os.MkdirAll(dir, 0o755)
	return dir
}

// * setupLog escribe el registro en la carpeta de datos de la aplicación.
func setupLog() {
	f, err := os.OpenFile(filepath.Join(dataDir(), "desktop.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err == nil {
		log.SetOutput(f)
	}
	log.SetFlags(log.LstdFlags)
}

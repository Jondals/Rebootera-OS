//go:build linux

// * Distribución del teclado del kiosco. La sesión gráfica la arranca
// * start-kiosk, que lee la distribución de keyboardPath antes de lanzar cage.
// ! Solo se aceptan las distribuciones de la lista cerrada de agentproto: el
// ! valor acaba en una variable de entorno de la sesión y nunca puede ser libre.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rebootera/core/internal/agentproto"
)

// keyboardPath es el archivo que lee start-kiosk (en /run: dura hasta apagar).
const keyboardPath = "/run/rebootera/keyboard"

// restartDelay da tiempo a que Core conteste antes de que la interfaz se cierre.
const restartDelay = time.Second

// * keyboardGet devuelve la distribución en uso.
func keyboardGet() ([]string, error) {
	data, err := os.ReadFile(keyboardPath)
	layout := strings.TrimSpace(string(data))
	if err != nil || !agentproto.ValidKeyboard(layout) {
		layout = agentproto.DefaultKeyboard
	}
	return []string{layout}, nil
}

// * keyboardSet guarda la distribución y la aplica al momento si cambió.
func keyboardSet(layout string) error {
	if !agentproto.ValidKeyboard(layout) {
		return fmt.Errorf("distribución de teclado no admitida: %q", layout)
	}
	current, _ := keyboardGet()
	if err := os.MkdirAll(filepath.Dir(keyboardPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(keyboardPath, []byte(layout+"\n"), 0o644); err != nil {
		return err
	}
	if current[0] == layout {
		return nil
	}
	// * Con labwc el cambio es inmediato (Bloq Despl virtual); si no se pudo, se reinicia la sesión como antes.
	// ! cage ignora SIGTERM: con "systemctl restart" tardaría 90 s. Se mata con SIGKILL y Restart=always del
	// ! servicio lo vuelve a abrir en 2 s.
	go func() {
		if !switchKeyboardLive(layout) {
			time.AfterFunc(restartDelay, killKiosk)
		}
	}()
	return nil
}

// * killKiosk mata la sesión del kiosco para que systemd la reabra.
// ! labwc no se queda en el grupo del servicio (logind lo mueve al ámbito de la sesión), así que "systemctl kill"
// ! lo deja vivo y huérfano agarrando la pantalla: la sesión nueva no podría dibujar (pantalla negra con el ratón).
// ! Por eso también se mata todo lo que sea del usuario del kiosco.
func killKiosk() {
	run("systemctl", "kill", "-s", "KILL", "rebootera-ui.service")
	run("pkill", "-KILL", "-u", "rebootera")
}

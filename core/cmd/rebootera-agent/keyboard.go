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

// * keyboardSet guarda la distribución y reinicia la interfaz si cambió.
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
	// ! cage ignora SIGTERM: con "systemctl restart" tardaría 90 s. Se mata con
	// ! SIGKILL y Restart=always del servicio lo vuelve a abrir en 2 s.
	time.AfterFunc(restartDelay, killKiosk)
	return nil
}

// * killKiosk mata la sesión del kiosco para que systemd la reabra.
func killKiosk() {
	run("systemctl", "kill", "-s", "KILL", "rebootera-ui.service")
}

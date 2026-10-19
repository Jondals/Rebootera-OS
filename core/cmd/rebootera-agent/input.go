//go:build linux

// * Ajustes del puntero y del panel táctil del kiosco. La sesión gráfica la arranca start-kiosk, que lee
// * inputPath al generar la configuración de labwc (sensibilidad, toque para clic, desplazamiento natural).
// ! Solo se aceptan los números y booleanos validados por agentproto: el contenido acaba en un archivo XML
// ! de configuración y nunca puede ser texto libre.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"rebootera/core/internal/agentproto"
)

// inputPath es el archivo que lee start-kiosk (en /run: dura hasta apagar).
const inputPath = "/run/rebootera/input.conf"

// * inputGet devuelve los ajustes en uso: speed=N, tap=0|1 y natural=0|1, una línea cada uno.
func inputGet() ([]string, error) {
	settings := agentproto.DefaultInput
	if data, err := os.ReadFile(inputPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			key, value, found := strings.Cut(strings.TrimSpace(line), "=")
			if !found {
				continue
			}
			switch key {
			case "speed":
				if number, err := strconv.Atoi(value); err == nil && number >= 0 && number <= 100 {
					settings.Speed = number
				}
			case "tap":
				settings.Tap = value == "1"
			case "natural":
				settings.Natural = value == "1"
			}
		}
	}
	return formatInput(settings), nil
}

// * formatInput escribe los ajustes en el formato de inputPath.
func formatInput(settings agentproto.InputSettings) []string {
	return []string{
		"speed=" + strconv.Itoa(settings.Speed),
		"tap=" + boolDigit(settings.Tap),
		"natural=" + boolDigit(settings.Natural),
	}
}

// * boolDigit convierte un booleano en "1" o "0".
func boolDigit(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

// * inputSet valida y guarda los ajustes y reinicia la interfaz si cambiaron.
func inputSet(settings *agentproto.InputSettings) error {
	if settings == nil {
		return fmt.Errorf("faltan los ajustes del puntero")
	}
	if err := settings.Validate(); err != nil {
		return err
	}
	current, _ := inputGet()
	next := formatInput(*settings)
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(inputPath, []byte(strings.Join(next, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	if strings.Join(current, "|") == strings.Join(next, "|") {
		return nil
	}
	// * Igual que con el teclado: SIGKILL a la sesión y Restart=always la reabre con los ajustes nuevos.
	time.AfterFunc(restartDelay, killKiosk)
	return nil
}

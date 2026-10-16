// * Package boot: elige qué imagen arranca Ventoy la próxima vez. Edita
// * ventoy/ventoy.json del Vault cambiando solo VTOY_DEFAULT_IMAGE y
// * VTOY_MENU_TIMEOUT, y respeta el resto de la configuración.
// * Cuando Rebootera OS vuelve a arrancar, Restore deja otra vez Rebootera OS
// * como imagen por defecto.
// ! Ventoy no tiene "arrancar una sola vez": mientras Rebootera no vuelva a
// ! arrancar, el menú de Ventoy seguirá preseleccionando la imagen elegida
// ! (con cuenta atrás, así que siempre se puede elegir otra entrada).
package boot

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"rebootera/core/internal/vault"
)

// Valores de Ventoy que usa Rebootera.
const (
	RebooteraImage  = "/rebootera/rebootera-os.iso"
	defaultTimeout  = "4"
	installTimeout  = "10"
	keyDefaultImage = "VTOY_DEFAULT_IMAGE"
	keyMenuTimeout  = "VTOY_MENU_TIMEOUT"
)

// * configPath es ventoy/ventoy.json dentro del Vault.
func configPath() string {
	return vault.Path("ventoy", "ventoy.json")
}

// * readConfig lee ventoy.json conservando todas sus claves.
func readConfig() (map[string]any, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer la configuración de arranque: %w", err)
	}
	config := map[string]any{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("la configuración de arranque está dañada: %w", err)
	}
	return config, nil
}

// * setControl cambia (o añade) una clave de la lista "control" de Ventoy.
func setControl(config map[string]any, key, value string) {
	list, _ := config["control"].([]any)
	for _, item := range list {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if _, exists := entry[key]; exists {
			entry[key] = value
			return
		}
	}
	config["control"] = append(list, map[string]any{key: value})
}

// * getControl devuelve el valor de una clave de "control".
func getControl(config map[string]any, key string) string {
	list, _ := config["control"].([]any)
	for _, item := range list {
		if entry, ok := item.(map[string]any); ok {
			if value, ok := entry[key].(string); ok {
				return value
			}
		}
	}
	return ""
}

// * writeConfig guarda ventoy.json con una copia del anterior.
func writeConfig(config map[string]any) error {
	if data, err := os.ReadFile(configPath()); err == nil {
		os.WriteFile(configPath()+".bak", data, 0o640)
	}
	return vault.WriteJSON(configPath(), config)
}

// * SetNext hace que Ventoy preseleccione la imagen rel (por ejemplo
// * "isos/linux/debian.iso") en el próximo arranque.
func SetNext(rel string) error {
	if !vault.Available() {
		return vault.ErrUnavailable
	}
	full, err := vault.Resolve(vault.IsosDir(), strings.TrimPrefix(rel, "isos/"))
	if err != nil {
		return err
	}
	if info, err := os.Stat(full); err != nil || !info.Mode().IsRegular() {
		return errors.New("la imagen no existe en la biblioteca")
	}
	config, err := readConfig()
	if err != nil {
		return err
	}
	ventoyPath := "/" + filepath.ToSlash(strings.TrimPrefix(full, vault.Root+string(filepath.Separator)))
	setControl(config, keyDefaultImage, ventoyPath)
	setControl(config, keyMenuTimeout, installTimeout)
	return writeConfig(config)
}

// * Restore vuelve a dejar Rebootera OS como imagen por defecto. Devuelve
// * true si había otra imagen preseleccionada.
func Restore() (bool, error) {
	if !vault.Available() {
		return false, nil
	}
	config, err := readConfig()
	if err != nil {
		return false, err
	}
	if getControl(config, keyDefaultImage) == RebooteraImage && getControl(config, keyMenuTimeout) == defaultTimeout {
		return false, nil
	}
	setControl(config, keyDefaultImage, RebooteraImage)
	setControl(config, keyMenuTimeout, defaultTimeout)
	return true, writeConfig(config)
}

// * Next devuelve la imagen que Ventoy preselecciona ahora mismo.
func Next() string {
	config, err := readConfig()
	if err != nil {
		return ""
	}
	return getControl(config, keyDefaultImage)
}

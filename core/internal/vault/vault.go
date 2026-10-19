// * Package vault: acceso de Core al Vault del USB de Rebootera, montado por
// * el agente en /run/rebootera/vault. Define sus carpetas, guarda los ajustes
// * persistentes y escribe archivos de estado de forma atómica.
// ! Todas las rutas que llegan de la interfaz pasan por Resolve, que impide
// ! salir de la carpeta permitida (por ejemplo con "..").
package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Root es donde el agente monta el Vault. Se puede cambiar en las pruebas.
var Root = "/run/rebootera/vault"

// ErrUnavailable indica que esta sesión no tiene Vault (medio de solo lectura).
var ErrUnavailable = errors.New("no hay Vault de Rebootera en esta sesión")

// ErrOutside indica una ruta que intenta salir de la carpeta permitida.
var ErrOutside = errors.New("ruta fuera del Vault")

// * Available indica si hay un Vault montado y utilizable.
func Available() bool {
	info, err := os.Stat(filepath.Join(Root, "rebootera"))
	return err == nil && info.IsDir()
}

// * EnsureLayout crea las carpetas mínimas de un Vault (estado e imágenes por
// * categoría) en root. Se usa para la biblioteca local del equipo.
func EnsureLayout(root string) error {
	for _, dir := range []string{
		filepath.Join("rebootera", "state"),
		filepath.Join("isos", "windows"),
		filepath.Join("isos", "linux"),
		filepath.Join("isos", "tools"),
	} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// * Path devuelve la ruta absoluta de una ruta relativa al Vault.
func Path(parts ...string) string {
	return filepath.Join(append([]string{Root}, parts...)...)
}

// * StateDir es la carpeta de estado de Rebootera (ajustes, biblioteca, descargas).
func StateDir() string {
	return Path("rebootera", "state")
}

// * IsosDir es la carpeta de la biblioteca de imágenes.
func IsosDir() string {
	return Path("isos")
}

// * PartialDir guarda las descargas sin terminar; Ventoy la ignora.
func PartialDir() string {
	return Path("isos", ".partial")
}

// * Resolve convierte una ruta relativa recibida de la interfaz en absoluta,
// * comprobando que queda dentro de base (por ejemplo, la carpeta isos/).
func Resolve(base, relative string) (string, error) {
	if !Available() {
		return "", ErrUnavailable
	}
	clean := filepath.Clean("/" + filepath.ToSlash(relative))
	full := filepath.Join(base, clean)
	rel, err := filepath.Rel(base, full)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", ErrOutside
	}
	return full, nil
}

// * WriteJSON guarda un valor como JSON de forma atómica (archivo temporal y
// * renombrado) para que un corte de corriente no deje un archivo a medias.
func WriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// * ReadJSON lee un archivo JSON; si no existe deja value sin cambios.
func ReadJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}

// Settings son las preferencias que sobreviven al reinicio.
type Settings struct {
	Language string `json:"language"`
	Accent   string `json:"accent"`
	// Theme es el tema visual: "obsidian" (oscuro), "ivory" (claro) o "sapphire" (azul oscuro).
	Theme        string `json:"theme"`
	Intro        bool   `json:"intro"`
	ReduceMotion bool   `json:"reduceMotion"`
}

var settingsMu sync.Mutex

// * LoadSettings lee los ajustes guardados en el Vault.
func LoadSettings() (Settings, error) {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	settings := Settings{Language: "en", Accent: "aurora", Theme: "obsidian", Intro: true}
	if !Available() {
		return settings, ErrUnavailable
	}
	err := ReadJSON(filepath.Join(StateDir(), "settings.json"), &settings)
	return settings, err
}

// * SaveSettings valida y guarda los ajustes en el Vault.
func SaveSettings(settings Settings) error {
	if settings.Language != "en" && settings.Language != "es" {
		return fmt.Errorf("idioma no válido: %q", settings.Language)
	}
	if settings.Accent != "aurora" && settings.Accent != "nebula" {
		return fmt.Errorf("acento no válido: %q", settings.Accent)
	}
	// * Los temas de la 0.9 (sky, night, forest) pasan al oscuro por defecto.
	switch settings.Theme {
	case "", "sky", "night", "forest":
		settings.Theme = "obsidian"
	case "obsidian", "ivory", "sapphire":
	default:
		return fmt.Errorf("tema no válido: %q", settings.Theme)
	}
	settingsMu.Lock()
	defer settingsMu.Unlock()
	if !Available() {
		return ErrUnavailable
	}
	return WriteJSON(filepath.Join(StateDir(), "settings.json"), settings)
}

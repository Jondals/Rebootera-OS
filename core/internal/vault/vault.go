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
	"time"
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
	// Theme es el tema visual, todos oscuros: amoled (por defecto), obsidian, sapphire, emerald, amethyst, ember, orange o graphite.
	Theme string `json:"theme"`
	// Background controla el fondo animado: "auto" (se apaga solo en equipos modestos), "on" u "off".
	Background string `json:"background"`
	// Scale es el tamaño de la interfaz en porcentaje (90 a 130).
	Scale int `json:"scale"`
	// Blur es el desenfoque del fondo animado: "off", "soft", "medium" o "strong".
	Blur         string `json:"blur"`
	Intro        bool   `json:"intro"`
	ReduceMotion bool   `json:"reduceMotion"`
}

var settingsMu sync.Mutex

// * LoadSettings lee los ajustes guardados en el Vault.
func LoadSettings() (Settings, error) {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	settings := Settings{Language: "en", Theme: "amoled", Background: "auto", Scale: 100, Blur: "soft", Intro: true}
	if !Available() {
		return settings, ErrUnavailable
	}
	err := ReadJSON(filepath.Join(StateDir(), "settings.json"), &settings)
	return settings, err
}

// * SaveSettings valida y guarda los ajustes en el Vault.
func SaveSettings(settings Settings) error {
	settings, err := ValidateSettings(settings)
	if err != nil {
		return err
	}
	settingsMu.Lock()
	defer settingsMu.Unlock()
	if !Available() {
		return ErrUnavailable
	}
	return WriteJSON(filepath.Join(StateDir(), "settings.json"), settings)
}

// * ValidateSettings completa los valores vacíos con los de por defecto y comprueba que los demás son válidos.
func ValidateSettings(settings Settings) (Settings, error) {
	if settings.Language != "en" && settings.Language != "es" {
		return settings, fmt.Errorf("idioma no válido: %q", settings.Language)
	}
	switch settings.Background {
	case "":
		settings.Background = "auto"
	case "auto", "on", "off":
	default:
		return settings, fmt.Errorf("fondo no válido: %q", settings.Background)
	}
	switch settings.Blur {
	case "":
		settings.Blur = "soft"
	case "off", "soft", "medium", "strong":
	default:
		return settings, fmt.Errorf("desenfoque no válido: %q", settings.Blur)
	}
	if settings.Scale == 0 {
		settings.Scale = 100
	}
	if settings.Scale < 90 || settings.Scale > 130 {
		return settings, fmt.Errorf("tamaño de interfaz no válido: %d", settings.Scale)
	}
	// * Los temas de la 0.9 (sky, night, forest) y el claro (ivory) pasan al oscuro por defecto.
	switch settings.Theme {
	case "", "sky", "night", "forest", "ivory":
		settings.Theme = "amoled"
	case "obsidian", "sapphire", "emerald", "amethyst", "ember", "orange", "graphite", "amoled":
	default:
		return settings, fmt.Errorf("tema no válido: %q", settings.Theme)
	}
	return settings, nil
}

// LocalPath es el archivo de ajustes de Rebootera Desktop en el PC (la lleva el escritorio; vacío en Rebootera OS).
// Guarda el tema y el resto de preferencias aunque no haya un USB conectado, y sirve para llevarlas al USB cuando se conecte.
var LocalPath string

// * LoadLocalSettings lee los ajustes guardados en el PC y cuándo se guardaron. ok es false si no hay ninguno.
func LoadLocalSettings() (settings Settings, saved time.Time, ok bool) {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	if LocalPath == "" {
		return Settings{}, time.Time{}, false
	}
	settings = Settings{Language: "en", Theme: "amoled", Background: "auto", Scale: 100, Blur: "soft", Intro: true}
	info, err := os.Stat(LocalPath)
	if err != nil || ReadJSON(LocalPath, &settings) != nil {
		return Settings{}, time.Time{}, false
	}
	checked, err := ValidateSettings(settings)
	if err != nil {
		return Settings{}, time.Time{}, false
	}
	return checked, info.ModTime(), true
}

// * SaveLocalSettings valida y guarda los ajustes en el PC.
func SaveLocalSettings(settings Settings) error {
	settings, err := ValidateSettings(settings)
	if err != nil {
		return err
	}
	settingsMu.Lock()
	defer settingsMu.Unlock()
	if LocalPath == "" {
		return ErrUnavailable
	}
	if err := os.MkdirAll(filepath.Dir(LocalPath), 0o755); err != nil {
		return err
	}
	return WriteJSON(LocalPath, settings)
}

// * VaultSettingsTime devuelve cuándo se guardaron los ajustes del Vault (cero si no hay).
func VaultSettingsTime() time.Time {
	if !Available() {
		return time.Time{}
	}
	info, err := os.Stat(filepath.Join(StateDir(), "settings.json"))
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

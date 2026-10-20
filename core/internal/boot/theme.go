// * Colores del menú de arranque de Ventoy según el tema elegido en la interfaz: el menú del USB usa el mismo fondo
// * y el mismo acento que Rebootera Desktop y Rebootera OS. Solo se tocan los colores de ventoy/theme/theme.txt
// * (el resto del tema, las fuentes y las imágenes de los marcos, queda igual).
package boot

import (
	"os"
	"regexp"

	"rebootera/core/internal/vault"
)

// palette son los colores del menú de un tema: fondo, texto, texto de la entrada elegida y acento.
type palette struct {
	bg, text, selected, accent string
}

// palettes tiene los ocho temas de la interfaz (los mismos valores que ui/src/styles/lux.css).
var palettes = map[string]palette{
	"amoled":   {"#000000", "#8fb0b8", "#e8f4f7", "#4fd8ee"},
	"obsidian": {"#0b0d11", "#a39e8f", "#ece7db", "#d8ae62"},
	"sapphire": {"#070b17", "#97a3c4", "#e6ebf8", "#8eaaff"},
	"emerald":  {"#060d0b", "#93b3a6", "#e4f2ec", "#5fd3a0"},
	"amethyst": {"#0b0814", "#a398c4", "#ece6fa", "#b79cff"},
	"ember":    {"#0f0807", "#b9998d", "#f4e6e0", "#ff9a6b"},
	"orange":   {"#0d0905", "#b9a28a", "#f6ece0", "#ff8a1f"},
	"graphite": {"#0d0e10", "#9ca2ab", "#eceef1", "#e8ebf0"},
}

var (
	desktopColor  = regexp.MustCompile(`(?m)^(desktop-color:\s*)"#[0-9a-fA-F]{6}"`)
	itemColor     = regexp.MustCompile(`(?m)^(\s*item_color\s*=\s*)"#[0-9a-fA-F]{6}"`)
	selectedColor = regexp.MustCompile(`(?m)^(\s*selected_item_color\s*=\s*)"#[0-9a-fA-F]{6}"`)
	labelColor    = regexp.MustCompile(`(?m)^(\s*color\s*=\s*)"#[0-9a-fA-F]{6}"`)
)

// * ApplyTheme pinta el menú de Ventoy del USB con los colores de un tema. Si el USB no tiene tema de Ventoy o el
// * tema no existe, no hace nada: es solo estética y nunca debe fallar al guardar los ajustes.
func ApplyTheme(theme string) {
	colors, ok := palettes[theme]
	if !ok || !vault.Available() {
		return
	}
	path := vault.Path("ventoy", "theme", "theme.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	text := string(data)
	text = desktopColor.ReplaceAllString(text, `${1}"`+colors.bg+`"`)
	text = itemColor.ReplaceAllString(text, `${1}"`+colors.text+`"`)
	text = selectedColor.ReplaceAllString(text, `${1}"`+colors.selected+`"`)
	text = labelColor.ReplaceAllString(text, `${1}"`+colors.accent+`"`)
	if text == string(data) {
		return
	}
	temporary := path + ".tmp"
	if os.WriteFile(temporary, []byte(text), 0o644) == nil {
		_ = os.Rename(temporary, path)
	}
}

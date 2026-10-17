package desktop

import (
	"os"
	"path/filepath"

	"rebootera/core/internal/vault"
)

// * LocalLibrary devuelve (y prepara) la carpeta "Rebootera" del usuario donde
// * Rebootera Desktop guarda las imágenes descargadas cuando no hay un USB de
// * Rebootera elegido. Así descargar sistemas funciona siempre.
func LocalLibrary() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	root := filepath.Join(home, "Rebootera")
	if err := vault.EnsureLayout(root); err != nil {
		return "", err
	}
	return root, nil
}

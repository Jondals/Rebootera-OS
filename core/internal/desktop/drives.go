// * Package desktop: funciones de Rebootera Desktop, la aplicación que
// * gestiona un USB de Rebootera desde Windows o Linux sin arrancar desde él.
// * Detecta los USB de Rebootera conectados (por el archivo del sistema que
// * contienen) y las unidades USB donde se podría crear uno nuevo.
package desktop

import (
	"os"
	"path/filepath"
	"strings"
)

// marker es el archivo que identifica un USB de Rebootera.
const marker = "rebootera/rebootera-os.iso"

// Drive es una unidad montada en el sistema anfitrión.
type Drive struct {
	Root       string `json:"root"`
	Label      string `json:"label"`
	FileSystem string `json:"fileSystem"`
	SizeBytes  uint64 `json:"sizeBytes"`
	FreeBytes  uint64 `json:"freeBytes"`
	Removable  bool   `json:"removable"`
	Rebootera  bool   `json:"rebootera"`
	Version    string `json:"version"`
}

// * IsRebootera indica si una carpeta raíz es un USB de Rebootera.
func IsRebootera(root string) bool {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(marker)))
	return err == nil && info.Mode().IsRegular()
}

// * readVersion lee la versión de Rebootera OS guardada en el USB.
func readVersion(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "rebootera", "VERSION"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// * RebooteraDrives devuelve solo las unidades que son USB de Rebootera.
func RebooteraDrives() []Drive {
	result := []Drive{}
	for _, drive := range Drives() {
		if drive.Rebootera {
			result = append(result, drive)
		}
	}
	return result
}

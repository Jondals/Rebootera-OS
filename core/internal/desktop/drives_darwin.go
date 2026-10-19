//go:build darwin

// * Detección de unidades en macOS: cada carpeta de /Volumes es una unidad montada (el disco del sistema aparece
// * ahí como un enlace a "/", que se ignora).
package desktop

import (
	"os"
	"path/filepath"

	"rebootera/core/internal/storage"
)

// * Drives devuelve las unidades montadas en /Volumes, salvo el disco del sistema.
func Drives() []Drive {
	entries, err := os.ReadDir("/Volumes")
	if err != nil {
		return []Drive{}
	}
	drives := []Drive{}
	for _, entry := range entries {
		point := filepath.Join("/Volumes", entry.Name())
		info, err := os.Lstat(point)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		used, free := storage.FSUsage(point)
		drive := Drive{Root: point, Label: entry.Name(), SizeBytes: used + free, FreeBytes: free, Removable: true, Rebootera: IsRebootera(point)}
		if drive.Rebootera {
			drive.Version = readVersion(point)
		}
		drives = append(drives, drive)
	}
	return drives
}

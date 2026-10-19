//go:build linux

// * Detección de unidades en Linux: puntos de montaje de /proc/self/mountinfo
// * bajo /media, /run/media y /mnt (donde los escritorios montan los USB).
package desktop

import (
	"os"
	"strings"

	"rebootera/core/internal/storage"
)

// * mountPrefixes son las carpetas donde los escritorios montan unidades externas.
var mountPrefixes = []string{"/media/", "/run/media/", "/mnt/"}

// * Drives devuelve las unidades montadas en las carpetas habituales.
func Drives() []Drive {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return []Drive{}
	}
	drives := []Drive{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		point := strings.ReplaceAll(fields[4], `\040`, " ")
		if !hasMountPrefix(point) {
			continue
		}
		sep := 6
		for sep < len(fields) && fields[sep] != "-" {
			sep++
		}
		fsType := ""
		if sep+1 < len(fields) {
			fsType = fields[sep+1]
		}
		used, free := storage.FSUsage(point)
		label := point[strings.LastIndex(point, "/")+1:]
		drive := Drive{Root: point, Label: label, FileSystem: fsType, SizeBytes: used + free, FreeBytes: free, Removable: true, Rebootera: IsRebootera(point)}
		if drive.Rebootera {
			drive.Version = readVersion(point)
		}
		drives = append(drives, drive)
	}
	return drives
}

// * hasMountPrefix indica si un punto de montaje está en una carpeta de unidades externas.
func hasMountPrefix(point string) bool {
	for _, prefix := range mountPrefixes {
		if strings.HasPrefix(point, prefix) {
			return true
		}
	}
	return false
}

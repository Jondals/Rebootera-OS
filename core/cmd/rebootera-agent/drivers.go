//go:build linux

// * Búsqueda automática de drivers: al arrancar Rebootera mira las particiones del equipo y abre en solo lectura
// * las que pueden traer drivers (cualquier NTFS, exFAT, FAT o ext4 que no esté en uso). Las que no tienen
// * ninguna carpeta de drivers se vuelven a cerrar enseguida; las que sí, quedan abiertas en /run/rebootera/media
// * para que Core lea sus INF. Nunca se escribe nada: es el mismo montaje de solo lectura de media.go.
package main

import (
	"os"
	"path/filepath"
	"strings"
)

// maxDriverCandidates limita cuántas particiones se prueban (un equipo con decenas de discos no se bloquea).
const maxDriverCandidates = 24

// driverFolders son las carpetas que delatan una partición con drivers (se buscan sin distinguir mayúsculas).
var driverFolders = []string{"drivers", "driver", "swsetup", "oem", "windows"}

// * scanDriverPartitions abre las particiones candidatas y devuelve los nombres de las que se quedan abiertas.
func scanDriverPartitions(clientUser string) ([]string, error) {
	entries, err := os.ReadDir("/sys/class/block")
	if err != nil {
		return nil, err
	}
	var kept []string
	tried := 0
	for _, entry := range entries {
		name := entry.Name()
		if !partitionPattern.MatchString(name) || strings.HasPrefix(name, "sr") {
			continue
		}
		target := filepath.Join(mediaRoot, name)
		if isMountPoint(target) {
			if hasDriverFolder(target) {
				kept = append(kept, name)
			}
			continue
		}
		if tried++; tried > maxDriverCandidates {
			break
		}
		// * Si ya está en uso (el USB de Rebootera, el medio de arranque) o no se puede abrir, se salta.
		if mountMedia("/dev/"+name, clientUser) != nil {
			continue
		}
		if hasDriverFolder(target) {
			kept = append(kept, name)
			continue
		}
		unmountMedia("/dev/" + name)
	}
	return kept, nil
}

// * hasDriverFolder indica si la raíz de una partición abierta tiene alguna carpeta que suela traer drivers.
func hasDriverFolder(root string) bool {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		for _, folder := range driverFolders {
			if strings.EqualFold(entry.Name(), folder) {
				return true
			}
		}
	}
	return false
}

//go:build linux

// * Búsqueda automática de drivers: al arrancar Rebootera mira las particiones del equipo y abre en solo lectura
// * las que pueden traer drivers (cualquier NTFS, exFAT, FAT o ext4 que no esté en uso). Las que no tienen
// * ninguna carpeta de drivers se vuelven a cerrar enseguida; las que sí, quedan abiertas en /run/rebootera/media
// * para que Core lea sus INF. Nunca se escribe nada: es el mismo montaje de solo lectura de media.go.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"rebootera/core/internal/drivers"
)

// maxDriverCandidates limita cuántas particiones se prueban (un equipo con decenas de discos no se bloquea).
const maxDriverCandidates = 24

// driverFolders son las carpetas que delatan una partición con drivers (se buscan sin distinguir mayúsculas).
var driverFolders = []string{"drivers", "driver", "swsetup", "oem", "windows"}

// reportPath es el informe de la búsqueda que lee Core (drivers.ReportPath).
var reportPath = drivers.ReportPath

// * labelOf devuelve la etiqueta de una partición: la que udev publicó en by-label y, si no, la que lee blkid del propio
// * disco. Así una partición "DRIVERS" se encuentra aunque udev todavía no haya terminado con ella.
func labelOf(name string, labelled map[string]string) string {
	if label, ok := labelled[name]; ok {
		return label
	}
	out, err := exec.Command("blkid", "-o", "value", "-s", "LABEL", "/dev/"+name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// * scanDriverPartitions abre las particiones candidatas y devuelve los nombres de las que se quedan abiertas. Anota en un
// * informe (reportPath) las particiones de drivers que encontró, abiertas o no, para que Core diga qué pasó con cada una.
func scanDriverPartitions(clientUser string) ([]string, error) {
	entries, err := os.ReadDir("/sys/class/block")
	if err != nil {
		return nil, err
	}
	var kept []string
	var report []string
	tried := 0
	// * Una partición cuya etiqueta dice "DRIVERS" (o parecido) se queda abierta aunque no tenga una carpeta de drivers: sus
	// * paquetes están sueltos en la raíz.
	labelled := drivers.LabelledPartitions()
	for _, entry := range entries {
		name := entry.Name()
		if !partitionPattern.MatchString(name) || strings.HasPrefix(name, "sr") {
			continue
		}
		target := filepath.Join(mediaRoot, name)
		if isMountPoint(target) {
			label := labelOf(name, labelled)
			if drivers.IsDriversLabel(label) || hasDriverFolder(target) {
				kept = append(kept, name)
				if drivers.IsDriversLabel(label) {
					report = append(report, name+"\t"+label+"\topen")
				}
			}
			continue
		}
		// * Las particiones que Rebootera ya tiene abiertas (el Vault del USB) no se tocan. Las de drivers que están en el mismo
		// * USB u otro disco, sí: la etiqueta se mira antes de rendirse con una partición que no se puede abrir.
		if tried++; tried > maxDriverCandidates {
			break
		}
		label := labelOf(name, labelled)
		isDrivers := drivers.IsDriversLabel(label)
		if err := mountMedia("/dev/"+name, clientUser); err != nil {
			// * Si ya está en uso (el medio de arranque) o no se puede abrir, se salta; si era de drivers, se anota por qué.
			if isDrivers {
				report = append(report, name+"\t"+label+"\terror: "+err.Error())
			}
			continue
		}
		if isDrivers || hasDriverFolder(target) {
			kept = append(kept, name)
			if isDrivers {
				report = append(report, name+"\t"+label+"\topen")
			}
			continue
		}
		unmountMedia("/dev/" + name)
	}
	writeReport(report)
	return kept, nil
}

// * writeReport guarda el informe de la búsqueda (0644: Core lo lee con otro usuario).
func writeReport(lines []string) {
	text := ""
	if len(lines) > 0 {
		text = strings.Join(lines, "\n") + "\n"
	}
	if err := os.WriteFile(reportPath, []byte(text), 0o644); err != nil {
		os.Remove(reportPath)
	}
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

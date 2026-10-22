// * Orígenes de drivers: de dónde saca Rebootera los paquetes INF sin que el usuario tenga que meterlos. Al
// * arrancar busca solo, en este orden: la carpeta drivers/ del USB de Rebootera, las carpetas de drivers de
// * cualquier partición del equipo (una partición "DRIVERS" de un portátil, una carpeta Drivers o SWSetup de
// * fábrica) y el almacén de drivers de un Windows ya instalado. Todo se lee en solo lectura.
package drivers

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// byLabelDir es donde udev publica un enlace por cada etiqueta de partición (se cambia en las pruebas).
var byLabelDir = "/dev/disk/by-label"

// ReportPath es el archivo donde el agente anota qué particiones de drivers encontró y cuáles no pudo abrir: una línea por
// partición, "nombre<TAB>etiqueta<TAB>estado" con el estado "open" o "error: motivo". Sirve de respaldo cuando udev no
// publicó la etiqueta (by-label) y para decir por qué no se pudo abrir una partición de drivers (se cambia en las pruebas).
var ReportPath = "/run/rebootera/drivers-scan"

// Tipos de origen.
const (
	// KindUSB es la carpeta drivers/ del USB de Rebootera.
	KindUSB = "usb"
	// KindPartition es una carpeta de drivers de otra partición.
	KindPartition = "partition"
	// KindWindows es el almacén de drivers de un Windows instalado.
	KindWindows = "windows"
)

// Source es un sitio donde hay paquetes de drivers.
type Source struct {
	// ID identifica el origen ("vault", "sda3:drivers").
	ID string `json:"id"`
	// Kind es KindUSB, KindPartition o KindWindows.
	Kind string `json:"kind"`
	// Device es la partición ("sda3"), vacío en el USB de Rebootera.
	Device string `json:"device,omitempty"`
	// Folder es la carpeta dentro de la partición ("Drivers", "SWSetup"...).
	Folder string `json:"folder,omitempty"`
	// Root es la carpeta que se recorre en busca de INF.
	Root string `json:"-"`
	// Packages es cuántos paquetes x64 se encontraron.
	Packages int `json:"packages"`
	// Error es el motivo por el que no se pudo abrir una partición de drivers que sí se encontró (vacío si se abrió).
	Error string `json:"error,omitempty"`
}

// folderGuesses son las carpetas de una partición que suelen traer drivers (se buscan sin distinguir mayúsculas).
var folderGuesses = []string{"drivers", "driver", "swsetup", "oem/drivers", "windows/system32/driverstore/filerepository"}

// driversLabel indica si la etiqueta de una partición dice que es de drivers (una partición "DRIVERS" de fábrica).
func driversLabel(label string) bool {
	lower := strings.ToLower(strings.TrimSpace(label))
	return strings.Contains(lower, "driver") || strings.Contains(lower, "controlador") ||
		strings.HasPrefix(lower, "drv") || lower == "swsetup" || lower == "sw_setup"
}

// * IsDriversLabel indica si una etiqueta de partición dice que es de drivers.
func IsDriversLabel(label string) bool {
	return driversLabel(label)
}

// scanLine es una línea del informe del agente.
type scanLine struct {
	name, label, state string
}

// * readReport lee el informe de la última búsqueda del agente (vacío si no hay).
func readReport() []scanLine {
	data, err := os.ReadFile(ReportPath)
	if err != nil {
		return nil
	}
	var lines []scanLine
	for _, raw := range strings.Split(string(data), "\n") {
		fields := strings.SplitN(strings.TrimRight(raw, "\r"), "\t", 3)
		if len(fields) == 3 && fields[0] != "" {
			lines = append(lines, scanLine{name: fields[0], label: fields[1], state: fields[2]})
		}
	}
	return lines
}

// * unescapeLabel deshace el escape de udev en los nombres de by-label (un espacio llega como "\x20").
func unescapeLabel(name string) string {
	var out strings.Builder
	for index := 0; index < len(name); index++ {
		if name[index] == '\\' && index+3 < len(name) && name[index+1] == 'x' {
			if value, err := strconv.ParseUint(name[index+2:index+4], 16, 8); err == nil {
				out.WriteByte(byte(value))
				index += 3
				continue
			}
		}
		out.WriteByte(name[index])
	}
	return out.String()
}

// * LabelledPartitions devuelve las particiones cuya etiqueta dice que son de drivers (nombre de la partición y etiqueta).
func LabelledPartitions() map[string]string {
	return driverPartitions()
}

// * driverPartitions devuelve las particiones cuya etiqueta dice que son de drivers: nombre de la partición ("sda3") y etiqueta.
// * Salen de los enlaces de udev (by-label) y, si udev no los publicó, de lo que anotó el agente al leer la etiqueta él mismo.
func driverPartitions() map[string]string {
	found := map[string]string{}
	for _, line := range readReport() {
		if driversLabel(line.label) {
			found[line.name] = line.label
		}
	}
	entries, err := os.ReadDir(byLabelDir)
	if err != nil {
		return found
	}
	for _, entry := range entries {
		label := unescapeLabel(entry.Name())
		if !driversLabel(label) {
			continue
		}
		target, err := os.Readlink(filepath.Join(byLabelDir, entry.Name()))
		if err != nil {
			continue
		}
		found[filepath.Base(target)] = label
	}
	return found
}

// * lookupFold busca una ruta relativa dentro de root sin distinguir mayúsculas y devuelve la ruta real.
func lookupFold(root, relative string) (string, bool) {
	current := root
	for _, part := range strings.Split(relative, "/") {
		entries, err := os.ReadDir(current)
		if err != nil {
			return "", false
		}
		found := false
		for _, entry := range entries {
			if entry.IsDir() && strings.EqualFold(entry.Name(), part) {
				current = filepath.Join(current, entry.Name())
				found = true
				break
			}
		}
		if !found {
			return "", false
		}
	}
	return current, true
}

// * Discover lista los orígenes de drivers que hay ahora: la carpeta drivers/ del USB (si vaultRoot no está
// * vacío) y las carpetas de drivers de las particiones que el agente tiene abiertas en mediaRoot.
func Discover(vaultRoot, mediaRoot string) []Source {
	var found []Source
	if vaultRoot != "" {
		if info, err := os.Stat(filepath.Join(vaultRoot, "drivers")); err == nil && info.IsDir() {
			found = append(found, Source{ID: "vault", Kind: KindUSB, Root: filepath.Join(vaultRoot, "drivers")})
		}
	}
	entries, err := os.ReadDir(mediaRoot)
	if err != nil {
		return found
	}
	labelled := driverPartitions()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		partition := filepath.Join(mediaRoot, entry.Name())
		// * Una partición etiquetada "DRIVERS" (o similar) es toda ella de drivers: se recorre entera.
		label, isDrivers := labelled[entry.Name()]
		if isDrivers {
			found = append(found, Source{ID: entry.Name() + ":label", Kind: KindPartition, Device: entry.Name(), Folder: label, Root: partition})
		}
		for _, guess := range folderGuesses {
			if isDrivers && !strings.Contains(guess, "driverstore") {
				continue
			}
			real, ok := lookupFold(partition, guess)
			if !ok {
				continue
			}
			kind := KindPartition
			if strings.Contains(guess, "driverstore") {
				kind = KindWindows
			}
			found = append(found, Source{
				ID: entry.Name() + ":" + guess, Kind: kind, Device: entry.Name(), Folder: filepath.Base(real), Root: real,
			})
		}
	}
	return append(found, unreadable(found)...)
}

// * unreadable devuelve las particiones de drivers que se encontraron pero que el agente no pudo abrir (sistema de archivos
// * dañado o desconocido, partición en uso...). Así la interfaz dice por qué no salen en vez de callar.
func unreadable(opened []Source) []Source {
	var failed []Source
	for _, line := range readReport() {
		reason, isError := strings.CutPrefix(line.state, "error:")
		if !isError || !driversLabel(line.label) {
			continue
		}
		duplicated := false
		for _, source := range opened {
			if source.Device == line.name {
				duplicated = true
			}
		}
		if !duplicated {
			failed = append(failed, Source{ID: line.name + ":label", Kind: KindPartition, Device: line.name, Folder: line.label, Error: strings.TrimSpace(reason)})
		}
	}
	return failed
}

// * ScanSources recorre todos los orígenes y devuelve sus paquetes (cada uno con su origen anotado) y los
// * orígenes con su recuento. Los orígenes sin ningún paquete x64 se descartan, salvo una partición etiquetada
// * "DRIVERS": esa se enseña siempre (con 0 paquetes si solo trae instaladores .exe) para que se vea que se encontró.
func ScanSources(sources []Source) ([]*Package, []Source) {
	var packages []*Package
	var used []Source
	for _, source := range sources {
		if source.Root == "" {
			// * Una partición de drivers que no se pudo abrir: se enseña con su motivo.
			used = append(used, source)
			continue
		}
		found := Scan(source.Root)
		if len(found) == 0 {
			if strings.HasSuffix(source.ID, ":label") {
				used = append(used, source)
			}
			continue
		}
		for _, pkg := range found {
			pkg.SourceID = source.ID
			pkg.Root = source.Root
			pkg.Rank = sourceRank(source.Kind)
		}
		source.Packages = len(found)
		packages = append(packages, found...)
		used = append(used, source)
	}
	return packages, used
}

// * sourceRank ordena los orígenes: el USB primero, luego las carpetas de fábrica y al final el almacén de Windows.
func sourceRank(kind string) int {
	switch kind {
	case KindUSB:
		return 0
	case KindPartition:
		return 1
	}
	return 2
}

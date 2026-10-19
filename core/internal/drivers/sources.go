// * Orígenes de drivers: de dónde saca Rebootera los paquetes INF sin que el usuario tenga que meterlos. Al
// * arrancar busca solo, en este orden: la carpeta drivers/ del USB de Rebootera, las carpetas de drivers de
// * cualquier partición del equipo (una partición "DRIVERS" de un portátil, una carpeta Drivers o SWSetup de
// * fábrica) y el almacén de drivers de un Windows ya instalado. Todo se lee en solo lectura.
package drivers

import (
	"os"
	"path/filepath"
	"strings"
)

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
}

// folderGuesses son las carpetas de una partición que suelen traer drivers (se buscan sin distinguir mayúsculas).
var folderGuesses = []string{"drivers", "driver", "swsetup", "oem/drivers", "windows/system32/driverstore/filerepository"}

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
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		partition := filepath.Join(mediaRoot, entry.Name())
		for _, guess := range folderGuesses {
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
	return found
}

// * ScanSources recorre todos los orígenes y devuelve sus paquetes (cada uno con su origen anotado) y los
// * orígenes con su recuento. Los orígenes sin ningún paquete x64 se descartan.
func ScanSources(sources []Source) ([]*Package, []Source) {
	var packages []*Package
	var used []Source
	for _, source := range sources {
		found := Scan(source.Root)
		if len(found) == 0 {
			continue
		}
		for _, pkg := range found {
			pkg.SourceID = source.ID
			pkg.Root = source.Root
		}
		source.Packages = len(found)
		packages = append(packages, found...)
		used = append(used, source)
	}
	return packages, used
}

// * Lectura de las bases de identificadores pci.ids y usb.ids (formato de
// * pciutils/usbutils): fabricantes, dispositivos y, en pci.ids, clases.
package hardware

import (
	"bufio"
	"os"
	"strings"
)

// idDB guarda los nombres de una base de identificadores.
type idDB struct {
	vendors    map[string]string
	devices    map[string]string
	classes    map[string]string
	subclasses map[string]string
}

// * loadIDs lee una base de identificadores. Si el archivo no existe devuelve
// * una base vacía y los nombres quedarán en blanco.
func loadIDs(path string) *idDB {
	db := &idDB{
		vendors:    map[string]string{},
		devices:    map[string]string{},
		classes:    map[string]string{},
		subclasses: map[string]string{},
	}
	f, err := os.Open(path)
	if err != nil {
		return db
	}
	defer f.Close()

	section := "vendors"
	vendor, class := "", ""
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "C "):
			section = "classes"
			id, name := splitID(strings.TrimPrefix(line, "C "))
			class = id
			db.classes[id] = name
		case !strings.HasPrefix(line, "\t"):
			id, name := splitID(line)
			if isHex4(id) {
				section = "vendors"
				vendor = id
				db.vendors[id] = name
			} else {
				// * Otras secciones de usb.ids (HID, idiomas...) no interesan.
				section = "other"
			}
		case strings.HasPrefix(line, "\t\t"):
			// * Subsistemas e interfaces de programación: no se usan.
		case section == "vendors":
			id, name := splitID(strings.TrimPrefix(line, "\t"))
			db.devices[vendor+":"+id] = name
		case section == "classes":
			id, name := splitID(strings.TrimPrefix(line, "\t"))
			db.subclasses[class+id] = name
		}
	}
	return db
}

// * splitID separa "8086  Intel Corporation" en identificador y nombre.
func splitID(line string) (string, string) {
	id, name, _ := strings.Cut(line, " ")
	return strings.ToLower(strings.TrimSpace(id)), strings.TrimSpace(name)
}

// * isHex4 indica si un texto es un identificador de 4 dígitos hexadecimales.
func isHex4(s string) bool {
	if len(s) != 4 {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

// * vendor devuelve el nombre de un fabricante.
func (db *idDB) vendor(id string) string {
	return db.vendors[id]
}

// * device devuelve el nombre de un dispositivo de un fabricante.
func (db *idDB) device(vendor, device string) string {
	return db.devices[vendor+":"+device]
}

// * class devuelve el nombre de la subclase PCI o, si no hay, el de la clase.
func (db *idDB) class(class, subclass string) string {
	if name := db.subclasses[class+subclass]; name != "" {
		return name
	}
	return db.classes[class]
}

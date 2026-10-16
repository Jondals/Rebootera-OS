// * Package hardware: inventario de dispositivos PCI y USB sin privilegios.
// * Para cada dispositivo da sus identificadores, nombres legibles (pci.ids y
// * usb.ids), el controlador del kernel en uso y su Hardware ID con el formato
// * de Windows, que es el que se casa con los archivos INF de drivers.
// * Marca como críticos los dispositivos sin los que no se puede instalar:
// * controladoras de almacenamiento y adaptadores de red.
package hardware

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Device es un dispositivo PCI o USB.
type Device struct {
	Bus         string   `json:"bus"`
	Address     string   `json:"address"`
	VendorID    string   `json:"vendorId"`
	DeviceID    string   `json:"deviceId"`
	Vendor      string   `json:"vendor"`
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	ClassName   string   `json:"className"`
	Driver      string   `json:"driver"`
	HardwareIDs []string `json:"hardwareIds"`
	Critical    bool     `json:"critical"`
}

// Categorías de dispositivo que muestra la interfaz.
const (
	CatStorage   = "storage"
	CatNetwork   = "network"
	CatWireless  = "wireless"
	CatDisplay   = "display"
	CatAudio     = "audio"
	CatBluetooth = "bluetooth"
	CatUSB       = "usb-controller"
	CatInput     = "input"
	CatBridge    = "bridge"
	CatOther     = "other"
)

// Raíz de sysfs y rutas de las bases de identificadores; cambiables en pruebas.
var (
	sysRoot    = "/sys"
	pciIDsPath = "/usr/share/misc/pci.ids"
	usbIDsPath = "/usr/share/misc/usb.ids"
)

// * List devuelve todos los dispositivos PCI y USB del equipo.
func List() []Device {
	pciNames := loadIDs(pciIDsPath)
	usbNames := loadIDs(usbIDsPath)
	devices := append(listPCI(pciNames), listUSB(usbNames)...)
	slices.SortFunc(devices, compareDevices)
	return devices
}

// * compareDevices ordena primero los críticos y después por categoría.
func compareDevices(a, b Device) int {
	if a.Critical != b.Critical {
		if a.Critical {
			return -1
		}
		return 1
	}
	if a.Category != b.Category {
		return strings.Compare(a.Category, b.Category)
	}
	return strings.Compare(a.Address, b.Address)
}

// * listPCI lee /sys/bus/pci/devices.
func listPCI(names *idDB) []Device {
	base := filepath.Join(sysRoot, "bus/pci/devices")
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	devices := []Device{}
	for _, entry := range entries {
		dir := filepath.Join(base, entry.Name())
		vendor := hex4(readTrim(filepath.Join(dir, "vendor")))
		device := hex4(readTrim(filepath.Join(dir, "device")))
		subVendor := hex4(readTrim(filepath.Join(dir, "subsystem_vendor")))
		subDevice := hex4(readTrim(filepath.Join(dir, "subsystem_device")))
		revision := strings.TrimPrefix(readTrim(filepath.Join(dir, "revision")), "0x")
		class := strings.TrimPrefix(readTrim(filepath.Join(dir, "class")), "0x")
		if len(class) < 4 {
			class = fmt.Sprintf("%06s", class)
		}

		d := Device{
			Bus:       "pci",
			Address:   entry.Name(),
			VendorID:  vendor,
			DeviceID:  device,
			Vendor:    names.vendor(vendor),
			Name:      names.device(vendor, device),
			ClassName: names.class(class[:2], class[2:4]),
			Driver:    driverName(dir),
		}
		d.Category = pciCategory(class, dir)
		d.Critical = d.Category == CatStorage || d.Category == CatNetwork || d.Category == CatWireless
		// * Hardware ID de Windows, del más específico al más genérico.
		d.HardwareIDs = []string{
			fmt.Sprintf(`PCI\VEN_%s&DEV_%s&SUBSYS_%s%s&REV_%s`, up(vendor), up(device), up(subDevice), up(subVendor), up(revision)),
			fmt.Sprintf(`PCI\VEN_%s&DEV_%s&SUBSYS_%s%s`, up(vendor), up(device), up(subDevice), up(subVendor)),
			fmt.Sprintf(`PCI\VEN_%s&DEV_%s&REV_%s`, up(vendor), up(device), up(revision)),
			fmt.Sprintf(`PCI\VEN_%s&DEV_%s`, up(vendor), up(device)),
		}
		devices = append(devices, d)
	}
	return devices
}

// * pciCategory traduce la clase PCI a una categoría de la interfaz.
func pciCategory(class, dir string) string {
	switch {
	case strings.HasPrefix(class, "01"):
		return CatStorage
	case strings.HasPrefix(class, "02"):
		if isWireless(dir) {
			return CatWireless
		}
		return CatNetwork
	case strings.HasPrefix(class, "0d"):
		return CatWireless
	case strings.HasPrefix(class, "03"):
		return CatDisplay
	case strings.HasPrefix(class, "0403"), strings.HasPrefix(class, "0401"):
		return CatAudio
	case strings.HasPrefix(class, "0c03"):
		return CatUSB
	case strings.HasPrefix(class, "06"):
		return CatBridge
	}
	return CatOther
}

// * isWireless indica si una tarjeta de red PCI tiene una interfaz Wi-Fi.
func isWireless(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "ieee80211")); err == nil {
		return true
	}
	ifaces, _ := os.ReadDir(filepath.Join(dir, "net"))
	for _, iface := range ifaces {
		if _, err := os.Stat(filepath.Join(dir, "net", iface.Name(), "wireless")); err == nil {
			return true
		}
	}
	return false
}

// * listUSB lee /sys/bus/usb/devices (solo dispositivos, no interfaces ni hubs raíz).
func listUSB(names *idDB) []Device {
	base := filepath.Join(sysRoot, "bus/usb/devices")
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	devices := []Device{}
	for _, entry := range entries {
		name := entry.Name()
		dir := filepath.Join(base, name)
		vendor := hex4(readTrim(filepath.Join(dir, "idVendor")))
		product := hex4(readTrim(filepath.Join(dir, "idProduct")))
		if vendor == "" || strings.HasPrefix(name, "usb") {
			continue
		}
		class := readTrim(filepath.Join(dir, "bDeviceClass"))
		if class == "09" {
			continue
		}
		d := Device{
			Bus:      "usb",
			Address:  name,
			VendorID: vendor,
			DeviceID: product,
			Vendor:   firstNonEmpty(names.vendor(vendor), readTrim(filepath.Join(dir, "manufacturer"))),
			Name:     firstNonEmpty(readTrim(filepath.Join(dir, "product")), names.device(vendor, product)),
		}
		d.Category, d.Driver = usbInterfaces(dir, name)
		if d.Category == CatOther && class == "e0" {
			d.Category = CatBluetooth
		}
		d.ClassName = d.Category
		d.Critical = d.Category == CatNetwork || d.Category == CatWireless
		d.HardwareIDs = []string{
			fmt.Sprintf(`USB\VID_%s&PID_%s&REV_%s`, up(vendor), up(product), up(hex4(readTrim(filepath.Join(dir, "bcdDevice"))))),
			fmt.Sprintf(`USB\VID_%s&PID_%s`, up(vendor), up(product)),
		}
		devices = append(devices, d)
	}
	return devices
}

// * usbInterfaces deduce la categoría y el controlador de un dispositivo USB a
// * partir de sus interfaces (por ejemplo "1-2:1.0").
func usbInterfaces(dir, name string) (string, string) {
	entries, _ := os.ReadDir(dir)
	category := CatOther
	drivers := []string{}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), name+":") {
			continue
		}
		ifDir := filepath.Join(dir, entry.Name())
		if drv := driverName(ifDir); drv != "" && !slices.Contains(drivers, drv) {
			drivers = append(drivers, drv)
		}
		switch readTrim(filepath.Join(ifDir, "bInterfaceClass")) {
		case "08":
			category = CatStorage
		case "03":
			category = CatInput
		case "01":
			category = CatAudio
		case "02", "0a":
			category = CatNetwork
		case "e0":
			category = CatBluetooth
		case "ff":
			if isWireless(ifDir) {
				category = CatWireless
			}
		}
		if _, err := os.Stat(filepath.Join(ifDir, "net")); err == nil {
			category = CatNetwork
			if isWireless(ifDir) {
				category = CatWireless
			}
		}
	}
	return category, strings.Join(drivers, ", ")
}

// * driverName devuelve el controlador del kernel enlazado a un dispositivo.
func driverName(dir string) string {
	target, err := os.Readlink(filepath.Join(dir, "driver"))
	if err != nil {
		return ""
	}
	return filepath.Base(target)
}

// * hex4 normaliza un identificador hexadecimal a 4 dígitos en minúsculas.
func hex4(s string) string {
	s = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(s), "0x"))
	if s == "" {
		return ""
	}
	if v, err := strconv.ParseUint(s, 16, 32); err == nil {
		return fmt.Sprintf("%04x", v)
	}
	return s
}

// * up pasa a mayúsculas, como en los Hardware ID de Windows.
func up(s string) string {
	return strings.ToUpper(s)
}

// * readTrim devuelve el contenido de un archivo sin espacios alrededor.
func readTrim(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// * firstNonEmpty devuelve el primer texto no vacío.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

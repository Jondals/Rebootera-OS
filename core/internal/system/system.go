// * Package system: información básica del equipo en el que corre Rebootera
// * OS: fabricante, modelo, firmware, Secure Boot, TPM, procesador, memoria,
// * tiempo encendido y redes con su tráfico.
// ! Solo lee archivos accesibles sin privilegios. El número de serie del
// ! equipo no se lee a propósito para que no acabe en registros ni capturas.
package system

import (
	"bufio"
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Info es el resumen del equipo que se envía a la interfaz.
type Info struct {
	Vendor        string  `json:"vendor"`
	Model         string  `json:"model"`
	BiosVendor    string  `json:"biosVendor"`
	BiosVersion   string  `json:"biosVersion"`
	Firmware      string  `json:"firmware"`
	SecureBoot    string  `json:"secureBoot"`
	TPM           string  `json:"tpm"`
	CPU           string  `json:"cpu"`
	CPUThreads    int     `json:"cpuThreads"`
	MemoryBytes   uint64  `json:"memoryBytes"`
	Kernel        string  `json:"kernel"`
	Hostname      string  `json:"hostname"`
	UptimeSeconds int64   `json:"uptimeSeconds"`
	Time          string  `json:"time"`
	Interfaces    []NetIf `json:"interfaces"`
	IsVirtual     bool    `json:"isVirtual"`
}

// NetIf describe una interfaz de red y sus contadores de tráfico.
type NetIf struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	State     string   `json:"state"`
	MAC       string   `json:"mac"`
	Addresses []string `json:"addresses"`
	SpeedMbps int      `json:"speedMbps"`
	RxBytes   uint64   `json:"rxBytes"`
	TxBytes   uint64   `json:"txBytes"`
}

// Valores posibles de Info.SecureBoot.
const (
	SecureBootOn      = "on"
	SecureBootOff     = "off"
	SecureBootUnknown = "unknown"
	SecureBootNA      = "n/a"
)

// Raíces de sysfs y procfs; se pueden cambiar en las pruebas.
var (
	sysRoot  = "/sys"
	procRoot = "/proc"
)

// * secureBootVar es la variable EFI global (EFI_GLOBAL_VARIABLE) que indica
// * si Secure Boot está activo.
const secureBootVar = "SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c"

// * Collect lee la información del equipo. Nunca falla: los datos que no se
// * puedan leer quedan vacíos.
func Collect() Info {
	info := Info{
		Vendor:      readTrim(filepath.Join(sysRoot, "class/dmi/id/sys_vendor")),
		Model:       readTrim(filepath.Join(sysRoot, "class/dmi/id/product_name")),
		BiosVendor:  readTrim(filepath.Join(sysRoot, "class/dmi/id/bios_vendor")),
		BiosVersion: readTrim(filepath.Join(sysRoot, "class/dmi/id/bios_version")),
		Kernel:      readTrim(filepath.Join(procRoot, "sys/kernel/osrelease")),
		CPUThreads:  runtime.NumCPU(),
		Time:        time.Now().Format(time.RFC3339),
	}
	if info.Model == "" || isPlaceholder(info.Model) {
		boardModel := readTrim(filepath.Join(sysRoot, "class/dmi/id/board_name"))
		if boardModel != "" {
			info.Model = boardModel
		}
	}
	info.Hostname, _ = os.Hostname()
	info.Firmware, info.SecureBoot = firmwareMode()
	info.TPM = tpmVersion()
	info.CPU = cpuModel()
	info.MemoryBytes = memTotal()
	info.UptimeSeconds = uptime()
	info.Interfaces = interfaces()
	info.IsVirtual = isVirtual(info.Vendor, info.Model)
	return info
}

// * readTrim devuelve el contenido de un archivo sin espacios alrededor, o
// * una cadena vacía si no se puede leer.
func readTrim(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// * isPlaceholder detecta los textos de relleno que algunos fabricantes dejan
// * en la DMI en lugar del modelo real.
func isPlaceholder(s string) bool {
	switch strings.ToLower(s) {
	case "to be filled by o.e.m.", "default string", "system product name", "not applicable":
		return true
	}
	return false
}

// * firmwareMode indica si se arrancó en UEFI o BIOS y el estado de Secure Boot.
func firmwareMode() (string, string) {
	if _, err := os.Stat(filepath.Join(sysRoot, "firmware/efi")); err != nil {
		return "bios", SecureBootNA
	}
	data, err := os.ReadFile(filepath.Join(sysRoot, "firmware/efi/efivars", secureBootVar))
	// * Según la especificación UEFI, si la variable no existe el firmware no
	// * admite Secure Boot, que equivale a tenerlo desactivado.
	if errors.Is(err, fs.ErrNotExist) {
		return "uefi", SecureBootOff
	}
	// * Los 4 primeros bytes son los atributos de la variable; el quinto es el valor.
	if err != nil || len(data) < 5 {
		return "uefi", SecureBootUnknown
	}
	if data[4] == 1 {
		return "uefi", SecureBootOn
	}
	return "uefi", SecureBootOff
}

// * tpmVersion devuelve "2.0", "1.2" o "" si no hay TPM.
func tpmVersion() string {
	major := readTrim(filepath.Join(sysRoot, "class/tpm/tpm0/tpm_version_major"))
	switch major {
	case "2":
		return "2.0"
	case "1":
		return "1.2"
	}
	return ""
}

// * cpuModel devuelve el nombre comercial del procesador.
func cpuModel() string {
	f, err := os.Open(filepath.Join(procRoot, "cpuinfo"))
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		key, value, found := strings.Cut(scanner.Text(), ":")
		if found && strings.TrimSpace(key) == "model name" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// * memTotal devuelve la memoria total en bytes según /proc/meminfo.
func memTotal() uint64 {
	f, err := os.Open(filepath.Join(procRoot, "meminfo"))
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "MemTotal:" {
			kib, err := strconv.ParseUint(fields[1], 10, 64)
			if err == nil {
				return kib * 1024
			}
		}
	}
	return 0
}

// * uptime devuelve los segundos desde el arranque.
func uptime() int64 {
	fields := strings.Fields(readTrim(filepath.Join(procRoot, "uptime")))
	if len(fields) == 0 {
		return 0
	}
	seconds, _ := strconv.ParseFloat(fields[0], 64)
	return int64(seconds)
}

// * interfaces lista las interfaces de red físicas con su estado, direcciones,
// * velocidad de enlace y bytes transferidos.
func interfaces() []NetIf {
	list, err := net.Interfaces()
	if err != nil {
		return nil
	}
	result := []NetIf{}
	for _, iface := range list {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		base := filepath.Join(sysRoot, "class/net", iface.Name)
		// * Solo interfaces con dispositivo físico detrás (descarta puentes y túneles).
		if _, err := os.Stat(filepath.Join(base, "device")); err != nil {
			continue
		}
		item := NetIf{
			Name:      iface.Name,
			Kind:      "ethernet",
			State:     readTrim(filepath.Join(base, "operstate")),
			MAC:       iface.HardwareAddr.String(),
			Addresses: []string{},
		}
		item.SpeedMbps, _ = strconv.Atoi(readTrim(filepath.Join(base, "speed")))
		if item.SpeedMbps < 0 {
			item.SpeedMbps = 0
		}
		item.RxBytes, _ = strconv.ParseUint(readTrim(filepath.Join(base, "statistics/rx_bytes")), 10, 64)
		item.TxBytes, _ = strconv.ParseUint(readTrim(filepath.Join(base, "statistics/tx_bytes")), 10, 64)
		if _, err := os.Stat(filepath.Join(base, "wireless")); err == nil {
			item.Kind = "wifi"
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				item.Addresses = append(item.Addresses, ipnet.IP.String())
			}
		}
		result = append(result, item)
	}
	slices.SortFunc(result, compareNetIf)
	return result
}

// * compareNetIf ordena las interfaces por nombre.
func compareNetIf(a, b NetIf) int {
	return strings.Compare(a.Name, b.Name)
}

// * isVirtual detecta las máquinas virtuales más comunes por la DMI.
func isVirtual(vendor, model string) bool {
	text := strings.ToLower(vendor + " " + model)
	for _, mark := range []string{"qemu", "virtualbox", "vmware", "kvm", "bochs", "microsoft corporation virtual", "xen", "parallels"} {
		if strings.Contains(text, mark) {
			return true
		}
	}
	return false
}

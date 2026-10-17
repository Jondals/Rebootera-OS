// * Package storage: inventario de discos y particiones sin privilegios.
// * Lee /sys/block, la base de datos de udev (/run/udev/data) y
// * /proc/self/mountinfo. Clasifica cada disco (interno, extraíble, medio de
// * arranque de Rebootera) y cada partición (EFI, recuperación, datos...).
// ! Este paquete nunca escribe en un disco. "Protected" indica qué discos no
// ! deben modificarse sin un plan confirmado por el usuario (Safe Disk Guard).
package storage

import (
	"bufio"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Disk es un dispositivo de bloques completo.
type Disk struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Model     string `json:"model"`
	Vendor    string `json:"vendor"`
	SizeBytes uint64 `json:"sizeBytes"`
	// Sectors es el tamaño del disco en sectores de 512 bytes.
	Sectors    uint64      `json:"sectors"`
	Transport  string      `json:"transport"`
	Removable  bool        `json:"removable"`
	Rotational bool        `json:"rotational"`
	ReadOnly   bool        `json:"readOnly"`
	Role       string      `json:"role"`
	Protected  bool        `json:"protected"`
	PartTable  string      `json:"partTable"`
	Partitions []Partition `json:"partitions"`
}

// Partition es una partición de un disco.
type Partition struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Number    int    `json:"number"`
	SizeBytes uint64 `json:"sizeBytes"`
	// StartSector y Sectors sitúan la partición en el disco (sectores de 512 bytes).
	StartSector uint64 `json:"startSector"`
	Sectors     uint64 `json:"sectors"`
	// TypeCode es el tipo de partición de la tabla (GUID en GPT, 0x83... en MBR), en minúsculas.
	TypeCode   string `json:"typeCode"`
	FSType     string `json:"fsType"`
	Label      string `json:"label"`
	Role       string `json:"role"`
	MountPoint string `json:"mountPoint"`
	UsedBytes  uint64 `json:"usedBytes"`
	FreeBytes  uint64 `json:"freeBytes"`
}

// Funciones de un disco.
const (
	RoleInternal  = "internal"
	RoleRemovable = "removable"
	RoleOptical   = "optical"
	RoleLiveMedia = "live-media"
)

// Funciones de una partición.
const (
	PartEFI       = "efi"
	PartMSR       = "msr"
	PartRecovery  = "recovery"
	PartWindows   = "windows"
	PartLinux     = "linux"
	PartSwap      = "swap"
	PartVault     = "vault"
	PartBootTools = "boot-tools"
	PartData      = "data"
	PartUnknown   = "unknown"
)

// * GUID de tipos de partición GPT conocidos (en minúsculas, como los da udev).
var partTypeRoles = map[string]string{
	"c12a7328-f81f-11d2-ba4b-00a0c93ec93b": PartEFI,
	"e3c9e316-0b5c-4db8-817d-f92df00215ae": PartMSR,
	"de94bba4-06d1-4d40-a16a-bfd50179d6ac": PartRecovery,
	"0657fd6d-a4ab-43c4-84e5-0933c84b4f4f": PartSwap,
	"0fc63daf-8483-4772-8e79-3d69d8477de4": PartLinux,
	"4f68bce3-e8cd-4db1-96e7-fbcaf984b709": PartLinux,
}

// Raíces de sysfs, udev y procfs; se pueden cambiar en las pruebas.
var (
	sysRoot  = "/sys"
	udevRoot = "/run/udev/data"
	procRoot = "/proc"
)

// liveMediumMount es donde live-boot monta el medio de arranque de Rebootera.
const liveMediumMount = "/run/live/medium"

// mount es una entrada de /proc/self/mountinfo.
type mount struct {
	majMin     string
	mountPoint string
}

// * List devuelve todos los discos del equipo con sus particiones.
func List() []Disk {
	mounts := resolveMounts(readMounts())
	liveDev := ""
	for _, m := range mounts {
		if m.mountPoint == liveMediumMount {
			liveDev = m.majMin
		}
	}

	entries, err := os.ReadDir(filepath.Join(sysRoot, "block"))
	if err != nil {
		return []Disk{}
	}
	disks := []Disk{}
	for _, entry := range entries {
		name := entry.Name()
		if skipDevice(name) {
			continue
		}
		disk := readDisk(name, mounts)
		if disk.SizeBytes == 0 && disk.Transport != "optical" {
			continue
		}
		if liveDev != "" && containsDevice(name, liveDev) {
			disk.Role = RoleLiveMedia
		}
		// * Safe Disk Guard: los discos internos y el propio medio de
		// * Rebootera nunca se tratan como destino libre.
		disk.Protected = disk.Role == RoleInternal || disk.Role == RoleLiveMedia
		disks = append(disks, disk)
	}
	slices.SortFunc(disks, compareDisks)
	return disks
}

// * skipDevice descarta dispositivos virtuales que no son discos del usuario.
func skipDevice(name string) bool {
	for _, prefix := range []string{"loop", "ram", "zram", "dm-", "md", "nbd", "fd"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// * compareDisks ordena primero el medio de arranque, luego internos y extraíbles.
func compareDisks(a, b Disk) int {
	order := map[string]int{RoleLiveMedia: 0, RoleInternal: 1, RoleRemovable: 2, RoleOptical: 3}
	if order[a.Role] != order[b.Role] {
		return order[a.Role] - order[b.Role]
	}
	return strings.Compare(a.Name, b.Name)
}

// * readDisk lee un disco y sus particiones.
func readDisk(name string, mounts []mount) Disk {
	base := filepath.Join(sysRoot, "block", name)
	props := udevProps(readTrim(filepath.Join(base, "dev")))
	disk := Disk{
		Name:       name,
		Path:       "/dev/" + name,
		Model:      firstNonEmpty(readTrim(filepath.Join(base, "device/model")), props["ID_MODEL"]),
		Vendor:     firstNonEmpty(readTrim(filepath.Join(base, "device/vendor")), props["ID_VENDOR"]),
		SizeBytes:  readUint(filepath.Join(base, "size")) * 512,
		Sectors:    readUint(filepath.Join(base, "size")),
		Removable:  readTrim(filepath.Join(base, "removable")) == "1",
		Rotational: readTrim(filepath.Join(base, "queue/rotational")) == "1",
		ReadOnly:   readTrim(filepath.Join(base, "ro")) == "1",
		PartTable:  props["ID_PART_TABLE_TYPE"],
		Partitions: []Partition{},
	}
	disk.Model = strings.ReplaceAll(disk.Model, "_", " ")
	disk.Transport = transport(name, base, props)

	switch {
	case disk.Transport == "optical":
		disk.Role = RoleOptical
	case disk.Removable || disk.Transport == "usb" || disk.Transport == "sd":
		disk.Role = RoleRemovable
	default:
		disk.Role = RoleInternal
	}

	entries, _ := os.ReadDir(base)
	for _, entry := range entries {
		if _, err := os.Stat(filepath.Join(base, entry.Name(), "partition")); err == nil {
			disk.Partitions = append(disk.Partitions, readPartition(base, entry.Name(), mounts))
		}
	}
	slices.SortFunc(disk.Partitions, comparePartitions)
	return disk
}

// * comparePartitions ordena las particiones por número.
func comparePartitions(a, b Partition) int {
	return a.Number - b.Number
}

// * transport averigua el tipo de conexión del disco a partir de su ruta en
// * sysfs y de udev.
func transport(name, base string, props map[string]string) string {
	real, _ := filepath.EvalSymlinks(base)
	switch {
	case strings.HasPrefix(name, "sr"):
		return "optical"
	case strings.HasPrefix(name, "nvme"):
		return "nvme"
	case strings.HasPrefix(name, "mmcblk"):
		return "sd"
	case strings.Contains(real, "/usb"):
		return "usb"
	case strings.HasPrefix(name, "vd") || strings.Contains(real, "/virtio"):
		return "virtio"
	case props["ID_BUS"] == "ata" || strings.Contains(real, "/ata"):
		return "sata"
	case props["ID_BUS"] == "scsi":
		return "scsi"
	}
	return "unknown"
}

// * readPartition lee una partición, su sistema de archivos y su uso si está
// * montada.
func readPartition(diskBase, name string, mounts []mount) Partition {
	base := filepath.Join(diskBase, name)
	majMin := readTrim(filepath.Join(base, "dev"))
	props := udevProps(majMin)
	number, _ := strconv.Atoi(readTrim(filepath.Join(base, "partition")))
	part := Partition{
		Name:        name,
		Path:        "/dev/" + name,
		Number:      number,
		SizeBytes:   readUint(filepath.Join(base, "size")) * 512,
		StartSector: readUint(filepath.Join(base, "start")),
		Sectors:     readUint(filepath.Join(base, "size")),
		TypeCode:    strings.ToLower(props["ID_PART_ENTRY_TYPE"]),
		FSType:      props["ID_FS_TYPE"],
		Label:       firstNonEmpty(props["ID_FS_LABEL"], props["ID_PART_ENTRY_NAME"]),
	}
	part.Role = partitionRole(props)
	// * Con Ventoy, el medio live (la ISO mapeada) y el Vault están en la misma
	// * partición física: se prefiere cualquier montaje que no sea el medio live.
	for _, m := range mounts {
		if m.majMin != majMin {
			continue
		}
		if part.MountPoint == "" || part.MountPoint == liveMediumMount {
			part.MountPoint = m.mountPoint
		}
	}
	if part.MountPoint != "" && part.MountPoint != liveMediumMount {
		part.UsedBytes, part.FreeBytes = FSUsage(part.MountPoint)
	}
	return part
}

// * partitionRole clasifica una partición por su tipo GPT/MBR, su sistema de
// * archivos y su etiqueta.
func partitionRole(props map[string]string) string {
	partType := strings.ToLower(props["ID_PART_ENTRY_TYPE"])
	if role, ok := partTypeRoles[partType]; ok {
		return role
	}
	label := props["ID_FS_LABEL"]
	switch {
	case label == "VTOYEFI":
		return PartBootTools
	case label == "REBOOTERA" || label == "Ventoy":
		return PartVault
	case partType == "0xef":
		return PartEFI
	case partType == "0x27":
		return PartRecovery
	}
	switch props["ID_FS_TYPE"] {
	case "ntfs":
		return PartWindows
	case "ext4", "ext3", "ext2", "btrfs", "xfs":
		return PartLinux
	case "swap":
		return PartSwap
	case "vfat", "exfat":
		return PartData
	}
	return PartUnknown
}

// * containsDevice indica si el dispositivo majMin es el disco name o una de
// * sus particiones.
func containsDevice(name, majMin string) bool {
	base := filepath.Join(sysRoot, "block", name)
	if readTrim(filepath.Join(base, "dev")) == majMin {
		return true
	}
	entries, _ := os.ReadDir(base)
	for _, entry := range entries {
		if readTrim(filepath.Join(base, entry.Name(), "dev")) == majMin {
			return true
		}
	}
	return false
}

// * udevProps lee las propiedades "E:CLAVE=valor" de udev de un dispositivo.
// ! Se descartan los números de serie: no los necesita la interfaz.
func udevProps(majMin string) map[string]string {
	props := map[string]string{}
	if majMin == "" {
		return props
	}
	f, err := os.Open(filepath.Join(udevRoot, "b"+majMin))
	if err != nil {
		return props
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line, ok := strings.CutPrefix(scanner.Text(), "E:")
		if !ok {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if found && !strings.Contains(key, "SERIAL") && !strings.Contains(key, "WWN") {
			props[key] = value
		}
	}
	return props
}

// * readMounts devuelve los montajes del sistema con su dispositivo.
func readMounts() []mount {
	f, err := os.Open(filepath.Join(procRoot, "self/mountinfo"))
	if err != nil {
		return nil
	}
	defer f.Close()
	mounts := []mount{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 5 {
			mounts = append(mounts, mount{majMin: fields[2], mountPoint: unescapeMount(fields[4])})
		}
	}
	return mounts
}

// * resolveMounts traduce los montajes hechos sobre dispositivos device-mapper
// * de un solo dispositivo (como los de Ventoy: la ISO mapeada en "ventoy" y la
// * partición completa en "VentoyPart") al "mayor:menor" de la partición física
// * que tienen debajo, para que el medio de arranque y el Vault se reconozcan.
func resolveMounts(mounts []mount) []mount {
	result := make([]mount, 0, len(mounts))
	for _, m := range mounts {
		m.majMin = physicalMajMin(m.majMin, 4)
		result = append(result, m)
	}
	return result
}

// * physicalMajMin baja por los dispositivos dm hasta la partición física.
// * depth limita la recursión por si hubiera un ciclo inesperado.
func physicalMajMin(majMin string, depth int) string {
	if depth == 0 {
		return majMin
	}
	link, err := os.Readlink(filepath.Join(sysRoot, "dev/block", majMin))
	if err != nil || !strings.HasPrefix(filepath.Base(link), "dm-") {
		return majMin
	}
	slaves, err := os.ReadDir(filepath.Join(sysRoot, "block", filepath.Base(link), "slaves"))
	if err != nil || len(slaves) != 1 {
		return majMin
	}
	slave := readTrim(filepath.Join(sysRoot, "block", filepath.Base(link), "slaves", slaves[0].Name(), "dev"))
	if slave == "" {
		return majMin
	}
	return physicalMajMin(slave, depth-1)
}

// * unescapeMount deshace el escapado octal de mountinfo (por ejemplo \040).
func unescapeMount(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// * readTrim devuelve el contenido de un archivo sin espacios alrededor.
func readTrim(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// * readUint lee un número entero sin signo de un archivo.
func readUint(path string) uint64 {
	value, _ := strconv.ParseUint(readTrim(path), 10, 64)
	return value
}

// * firstNonEmpty devuelve el primer texto no vacío.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

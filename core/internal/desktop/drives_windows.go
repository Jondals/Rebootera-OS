//go:build windows

// * Detección de unidades en Windows con la API Win32 (sin privilegios):
// * letras de unidad, tipo (extraíble o fija), etiqueta, sistema de archivos
// * y espacio.
package desktop

import (
	"syscall"
	"unsafe"

	"rebootera/core/internal/storage"
)

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procGetLogicalDrives      = kernel32.NewProc("GetLogicalDrives")
	procGetDriveTypeW         = kernel32.NewProc("GetDriveTypeW")
	procGetVolumeInformationW = kernel32.NewProc("GetVolumeInformationW")
	procSetErrorMode          = kernel32.NewProc("SetErrorMode")
)

// Tipos de unidad de GetDriveTypeW.
const (
	driveRemovable = 2
	driveFixed     = 3
)

// semFailCriticalErrors evita los diálogos de "inserte un disco" en lectores vacíos.
const semFailCriticalErrors = 0x0001

// * Drives devuelve las unidades extraíbles y fijas con letra.
func Drives() []Drive {
	procSetErrorMode.Call(semFailCriticalErrors)
	mask, _, _ := procGetLogicalDrives.Call()
	drives := []Drive{}
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		rootPtr, _ := syscall.UTF16PtrFromString(root)
		kind, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(rootPtr)))
		if kind != driveRemovable && kind != driveFixed {
			continue
		}
		label, fs, ok := volumeInfo(rootPtr)
		if !ok {
			continue
		}
		used, free := storage.FSUsage(root)
		drive := Drive{
			Root:       root,
			Label:      label,
			FileSystem: fs,
			SizeBytes:  used + free,
			FreeBytes:  free,
			Removable:  kind == driveRemovable,
			Rebootera:  IsRebootera(root),
		}
		if drive.Rebootera {
			drive.Version = readVersion(root)
		}
		drives = append(drives, drive)
	}
	return drives
}

// * volumeInfo lee la etiqueta y el sistema de archivos de una unidad.
func volumeInfo(root *uint16) (string, string, bool) {
	var label, fs [261]uint16
	var serial, maxLen, flags uint32
	ok, _, _ := procGetVolumeInformationW.Call(
		uintptr(unsafe.Pointer(root)),
		uintptr(unsafe.Pointer(&label[0])), uintptr(len(label)),
		uintptr(unsafe.Pointer(&serial)), uintptr(unsafe.Pointer(&maxLen)), uintptr(unsafe.Pointer(&flags)),
		uintptr(unsafe.Pointer(&fs[0])), uintptr(len(fs)),
	)
	if ok == 0 {
		return "", "", false
	}
	return syscall.UTF16ToString(label[:]), syscall.UTF16ToString(fs[:]), true
}

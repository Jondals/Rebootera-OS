//go:build windows

// * Uso de una unidad en Windows (GetDiskFreeSpaceExW).
package storage

import (
	"syscall"
	"unsafe"
)

var procGetDiskFreeSpaceEx = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")

// * FSUsage devuelve los bytes usados y libres de la unidad que contiene path.
// * Devuelve ceros si no se puede consultar.
func FSUsage(path string) (used, free uint64) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0
	}
	var freeAvail, total, totalFree uint64
	ok, _, _ := procGetDiskFreeSpaceEx.Call(uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&freeAvail)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&totalFree)))
	if ok == 0 {
		return 0, 0
	}
	return total - totalFree, freeAvail
}

//go:build windows

// * Diálogos nativos de Windows para elegir varias imágenes o una carpeta de
// * drivers (la página web no puede conocer rutas de archivos por seguridad).
package desktop

import (
	"syscall"
	"unsafe"
)

var (
	procSHBrowseForFolderW   = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDListW = shell32.NewProc("SHGetPathFromIDListW")
	ole32                    = syscall.NewLazyDLL("ole32.dll")
	procCoTaskMemFree        = ole32.NewProc("CoTaskMemFree")
)

// Opciones extra del diálogo "Abrir" para elegir varios archivos.
const ofnAllowMultiselect = 0x00000200

// * PickFiles abre el diálogo "Abrir" con selección múltiple y devuelve las
// * rutas elegidas ([] si se cancela).
func PickFiles(title string) ([]string, error) {
	filter := utf16Filter("Disk images (*.iso;*.img;*.wim;*.vhd;*.vhdx)", "*.iso;*.img;*.wim;*.vhd;*.vhdx")
	// * Con varias selecciones el resultado es: carpeta\0archivo1\0archivo2\0\0.
	buffer := make([]uint16, 64*1024)
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	ofn := openFileName{
		filter:  &filter[0],
		file:    &buffer[0],
		maxFile: uint32(len(buffer)),
		title:   titlePtr,
		flags:   ofnFileMustExist | ofnPathMustExist | ofnNoChangeDir | ofnExplorer | ofnAllowMultiselect,
	}
	ofn.structSize = uint32(unsafe.Sizeof(ofn))
	ok, _, _ := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if ok == 0 {
		return []string{}, nil
	}
	return splitMultiSelect(buffer), nil
}

// * splitMultiSelect interpreta el resultado del diálogo: un solo archivo viene
// * con su ruta completa; varios, con la carpeta primero y los nombres después.
func splitMultiSelect(buffer []uint16) []string {
	parts := []string{}
	start := 0
	for i, unit := range buffer {
		if unit != 0 {
			continue
		}
		if i == start {
			break
		}
		parts = append(parts, syscall.UTF16ToString(buffer[start:i]))
		start = i + 1
	}
	if len(parts) <= 1 {
		return parts
	}
	paths := make([]string, 0, len(parts)-1)
	for _, name := range parts[1:] {
		paths = append(paths, parts[0]+`\`+name)
	}
	return paths
}

// browseInfo es la estructura BROWSEINFOW de shell32.
type browseInfo struct {
	owner       uintptr
	root        uintptr
	displayName *uint16
	title       *uint16
	flags       uint32
	callback    uintptr
	param       uintptr
	image       int32
}

// Opciones de SHBrowseForFolder: solo carpetas del sistema de archivos, con el diálogo moderno.
const (
	bifReturnOnlyFSDirs = 0x00000001
	bifNewDialogStyle   = 0x00000040
)

// * PickFolder abre el diálogo de elegir carpeta y devuelve su ruta ("" si se cancela).
func PickFolder(title string) (string, error) {
	display := make([]uint16, 260)
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	info := browseInfo{displayName: &display[0], title: titlePtr, flags: bifReturnOnlyFSDirs | bifNewDialogStyle}
	pidl, _, _ := procSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(&info)))
	if pidl == 0 {
		return "", nil
	}
	defer procCoTaskMemFree.Call(pidl)
	path := make([]uint16, 32768)
	ok, _, _ := procSHGetPathFromIDListW.Call(pidl, uintptr(unsafe.Pointer(&path[0])))
	if ok == 0 {
		return "", nil
	}
	return syscall.UTF16ToString(path), nil
}

//go:build windows

// * Herramientas de Windows para Rebootera Desktop:
// *   - PickFile: diálogo nativo "Abrir" para elegir una imagen (la página web
// *     no puede conocer rutas de archivos por seguridad).
// *   - USBDisks: discos físicos USB (Get-Disk), candidatos para crear un USB.
// *   - RunElevated: ejecuta un programa pidiendo permisos de administrador (UAC)
// *     y espera a que termine.
// ! Crear un USB de Rebootera borra el disco elegido: lo hace Ventoy2Disk con
// ! permisos de administrador y solo tras la confirmación de la interfaz.
package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

var (
	comdlg32             = syscall.NewLazyDLL("comdlg32.dll")
	procGetOpenFileNameW = comdlg32.NewProc("GetOpenFileNameW")
	shell32              = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteExW  = shell32.NewProc("ShellExecuteExW")
	procWaitForSingle    = kernel32.NewProc("WaitForSingleObject")
	procGetExitCode      = kernel32.NewProc("GetExitCodeProcess")
	procCloseHandle      = kernel32.NewProc("CloseHandle")
)

// openFileName es la estructura OPENFILENAMEW de comdlg32.
type openFileName struct {
	structSize    uint32
	owner         uintptr
	instance      uintptr
	filter        *uint16
	customFilter  *uint16
	maxCustFilter uint32
	filterIndex   uint32
	file          *uint16
	maxFile       uint32
	fileTitle     *uint16
	maxFileTitle  uint32
	initialDir    *uint16
	title         *uint16
	flags         uint32
	fileOffset    uint16
	fileExtension uint16
	defExt        *uint16
	custData      uintptr
	hook          uintptr
	templateName  *uint16
	reserved      uintptr
	reserved2     uint32
	flagsEx       uint32
}

// Opciones del diálogo: el archivo debe existir y no se cambia de carpeta.
const (
	ofnFileMustExist = 0x00001000
	ofnPathMustExist = 0x00000800
	ofnNoChangeDir   = 0x00000008
	ofnExplorer      = 0x00080000
)

// * PickFile abre el diálogo nativo "Abrir" filtrado por imágenes de disco.
// * Devuelve "" si el usuario lo cancela.
func PickFile(title string) (string, error) {
	filter := utf16Filter("Disk images (*.iso;*.img;*.wim;*.vhd;*.vhdx)", "*.iso;*.img;*.wim;*.vhd;*.vhdx")
	buffer := make([]uint16, 4096)
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	ofn := openFileName{
		filter:  &filter[0],
		file:    &buffer[0],
		maxFile: uint32(len(buffer)),
		title:   titlePtr,
		flags:   ofnFileMustExist | ofnPathMustExist | ofnNoChangeDir | ofnExplorer,
	}
	ofn.structSize = uint32(unsafe.Sizeof(ofn))
	ok, _, _ := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if ok == 0 {
		return "", nil
	}
	return syscall.UTF16ToString(buffer), nil
}

// * utf16Filter construye el filtro "nombre\0patrón\0\0" que pide la API.
func utf16Filter(name, pattern string) []uint16 {
	text := name + "\x00" + pattern + "\x00\x00"
	result := make([]uint16, 0, len(text))
	for _, r := range text {
		result = append(result, uint16(r))
	}
	return result
}

// USBDisk es un disco físico conectado por USB.
type USBDisk struct {
	Number       int         `json:"number"`
	FriendlyName string      `json:"friendlyName"`
	SizeBytes    uint64      `json:"sizeBytes"`
	IsSystem     bool        `json:"isSystem"`
	IsBoot       bool        `json:"isBoot"`
	Volumes      []USBVolume `json:"volumes"`
	UsedBytes    uint64      `json:"usedBytes"`
	Rebootera    bool        `json:"rebootera"`
}

// USBVolume es un volumen con letra dentro de un disco USB.
type USBVolume struct {
	Letter    string `json:"letter"`
	Label     string `json:"label"`
	SizeBytes uint64 `json:"sizeBytes"`
	FreeBytes uint64 `json:"freeBytes"`
}

// * usbDisksScript lista los discos USB con sus volúmenes en JSON. Get-Disk,
// * Get-Partition y Get-Volume no necesitan permisos de administrador para leer.
const usbDisksScript = `$ErrorActionPreference = 'SilentlyContinue'
$result = @(Get-Disk | Where-Object BusType -eq 'USB' | ForEach-Object {
  $disk = $_
  $volumes = @(Get-Partition -DiskNumber $disk.Number | Where-Object DriveLetter | ForEach-Object {
    $v = Get-Volume -DriveLetter $_.DriveLetter
    [pscustomobject]@{ Letter = [string]$_.DriveLetter; Label = [string]$v.FileSystemLabel; Size = [uint64]$v.Size; Free = [uint64]$v.SizeRemaining }
  })
  [pscustomobject]@{ Number = $disk.Number; FriendlyName = $disk.FriendlyName; Size = [uint64]$disk.Size; IsSystem = $disk.IsSystem; IsBoot = $disk.IsBoot; Volumes = $volumes }
})
ConvertTo-Json -InputObject $result -Compress -Depth 4`

// * USBDisks lista los discos USB con sus volúmenes y los datos que contienen,
// * para que la interfaz pueda advertir antes de borrar un disco con datos.
// ! Nunca se ofrecen discos de sistema o de arranque de Windows.
func USBDisks() ([]USBDisk, error) {
	// * Solo para capturas de pantalla de pruebas: no lista ningún disco del equipo.
	if os.Getenv("REBOOTERA_TEST_NO_DISKS") != "" {
		return []USBDisk{}, nil
	}
	out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", usbDisksScript).Output()
	if err != nil {
		return nil, fmt.Errorf("no se pudieron listar los discos: %w", err)
	}
	text := strings.TrimSpace(string(out))
	if text == "" || text == "null" {
		return []USBDisk{}, nil
	}
	var raw []struct {
		Number       int
		FriendlyName string
		Size         uint64
		IsSystem     bool
		IsBoot       bool
		Volumes      []struct {
			Letter string
			Label  string
			Size   uint64
			Free   uint64
		}
	}
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, fmt.Errorf("respuesta de Get-Disk no válida: %w", err)
	}
	disks := []USBDisk{}
	for _, d := range raw {
		if d.IsSystem || d.IsBoot {
			continue
		}
		disk := USBDisk{Number: d.Number, FriendlyName: d.FriendlyName, SizeBytes: d.Size, Volumes: []USBVolume{}}
		for _, v := range d.Volumes {
			disk.Volumes = append(disk.Volumes, USBVolume{Letter: v.Letter, Label: v.Label, SizeBytes: v.Size, FreeBytes: v.Free})
			if v.Size > v.Free {
				disk.UsedBytes += v.Size - v.Free
			}
			if IsRebootera(v.Letter + `:\`) {
				disk.Rebootera = true
			}
		}
		disks = append(disks, disk)
	}
	return disks, nil
}

// shellExecuteInfo es la estructura SHELLEXECUTEINFOW.
type shellExecuteInfo struct {
	size          uint32
	mask          uint32
	hwnd          uintptr
	verb          *uint16
	file          *uint16
	parameters    *uint16
	directory     *uint16
	show          int32
	instApp       uintptr
	idList        uintptr
	class         *uint16
	keyClass      uintptr
	hotKey        uint32
	iconOrMonitor uintptr
	process       syscall.Handle
}

// Valores de ShellExecuteEx.
const (
	seeMaskNoCloseProcess = 0x00000040
	swHide                = 0
	infinite              = 0xFFFFFFFF
)

// * RunElevated ejecuta un programa como administrador (aparece el aviso de
// * UAC), espera a que termine y devuelve su código de salida.
func RunElevated(program, parameters, directory string) (uint32, error) {
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(program)
	params, _ := syscall.UTF16PtrFromString(parameters)
	dir, _ := syscall.UTF16PtrFromString(directory)
	info := shellExecuteInfo{mask: seeMaskNoCloseProcess, verb: verb, file: file, parameters: params, directory: dir, show: swHide}
	info.size = uint32(unsafe.Sizeof(info))
	ok, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return 0, fmt.Errorf("no se pudo ejecutar como administrador: %v", callErr)
	}
	if info.process == 0 {
		return 0, errors.New("Windows no devolvió el proceso")
	}
	defer procCloseHandle.Call(uintptr(info.process))
	procWaitForSingle.Call(uintptr(info.process), infinite)
	var code uint32
	procGetExitCode.Call(uintptr(info.process), uintptr(unsafe.Pointer(&code)))
	return code, nil
}

//go:build !windows

// * En Linux, crear un USB se hace con scripts/make-usb.sh (necesita root y
// * herramientas como losetup); la aplicación de escritorio lo indica.
package desktop

import (
	"errors"
	"io/fs"
)

// CreateStatus es el estado de una creación en curso.
type CreateStatus struct {
	Running  bool    `json:"running"`
	Phase    string  `json:"phase"`
	Progress float64 `json:"progress"`
	Error    string  `json:"error,omitempty"`
	Done     bool    `json:"done"`
	Drive    string  `json:"drive,omitempty"`
}

// USBDisk es un disco físico conectado por USB.
type USBDisk struct {
	Number       int    `json:"number"`
	FriendlyName string `json:"friendlyName"`
	SizeBytes    uint64 `json:"sizeBytes"`
}

// ErrUnsupported indica una función que solo existe en Windows.
var ErrUnsupported = errors.New("solo disponible en Windows; en Linux usa scripts/make-usb.sh")

// * Status devuelve un estado vacío.
func Status() CreateStatus {
	return CreateStatus{}
}

// * Create no está disponible fuera de Windows.
func Create(int, string, fs.FS, string) error {
	return ErrUnsupported
}

// * USBDisks no está disponible fuera de Windows.
func USBDisks() ([]USBDisk, error) {
	return nil, ErrUnsupported
}

// * PickFile no está disponible fuera de Windows (se escribe la ruta).
func PickFile(string) (string, error) {
	return "", ErrUnsupported
}

// * PrepareVentoy no está disponible fuera de Windows.
func PrepareVentoy() (string, error) {
	return "", ErrUnsupported
}

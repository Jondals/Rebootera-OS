//go:build !windows

// * Fuera de Windows, crear un USB desde la aplicación no está disponible: en Linux se hace con
// * scripts/usb/make-usb.sh (necesita root y herramientas como losetup) y la aplicación lo indica.
package desktop

import (
	"errors"
	"io/fs"
)

// ErrUnsupported indica una función que solo existe en Windows.
var ErrUnsupported = errors.New("crear el USB desde la aplicación solo está disponible en Windows; en Linux usa scripts/usb/make-usb.sh")

// * Create no está disponible fuera de Windows.
func Create(int, string, fs.FS, string, Extras, bool) error {
	return ErrUnsupported
}

// * USBDisks no está disponible fuera de Windows.
func USBDisks() ([]USBDisk, error) {
	return nil, ErrUnsupported
}

// * PrepareVentoy no está disponible fuera de Windows.
func PrepareVentoy() (string, error) {
	return "", ErrUnsupported
}

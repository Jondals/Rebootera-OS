//go:build !windows && !linux && !darwin

// * En sistemas distintos de Windows, Linux y macOS (por ejemplo FreeBSD), crear un USB desde la aplicación no está
// * disponible y la aplicación lo indica.
package desktop

import (
	"errors"
	"io/fs"
)

// ErrUnsupported indica una función que solo existe en Windows, Linux y macOS.
var ErrUnsupported = errors.New("crear el USB desde la aplicación solo está disponible en Windows, Linux y macOS")

// * Create no está disponible en este sistema.
func Create(int, string, fs.FS, string, Extras, bool) error {
	return ErrUnsupported
}

// * USBDisks no está disponible en este sistema.
func USBDisks() ([]USBDisk, error) {
	return nil, ErrUnsupported
}

// * PrepareVentoy no está disponible en este sistema.
func PrepareVentoy() (string, error) {
	return "", ErrUnsupported
}

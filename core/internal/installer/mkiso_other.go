//go:build !linux && !windows

// * Creación de una ISO desde la carpeta de un instalador en sistemas sin herramienta propia (macOS): no disponible.
package installer

import (
	"context"
	"errors"
)

// * MakeISO no está disponible en macOS: crear una ISO arrancable de Windows (BIOS y UEFI, con archivos de más
// * de 4 GiB) necesita herramientas que ese sistema no trae. Se puede crear la ISO en Windows o Linux, o añadir
// * la carpeta del instalador con otro programa.
func MakeISO(context.Context, Info, string, Reporter) error {
	return errors.New("crear una ISO desde una carpeta no está disponible en macOS")
}

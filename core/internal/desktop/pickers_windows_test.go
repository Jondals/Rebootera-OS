//go:build windows

// * Pruebas de la interpretación del resultado del diálogo "Abrir" con varias selecciones.
package desktop

import (
	"slices"
	"syscall"
	"testing"
)

// * encode construye el búfer del diálogo: cadenas separadas por NUL y terminadas por doble NUL.
func encode(parts ...string) []uint16 {
	buffer := []uint16{}
	for _, part := range parts {
		encoded, _ := syscall.UTF16FromString(part)
		buffer = append(buffer, encoded...)
	}
	return append(buffer, 0, 0, 0)
}

// * TestSplitMultiSelect cubre un solo archivo, varios y ninguno.
func TestSplitMultiSelect(t *testing.T) {
	one := splitMultiSelect(encode(`D:\ISOs\W11.iso`))
	if !slices.Equal(one, []string{`D:\ISOs\W11.iso`}) {
		t.Fatalf("uno: %v", one)
	}
	many := splitMultiSelect(encode(`D:\ISOs`, "W11.iso", "Debian.iso"))
	if !slices.Equal(many, []string{`D:\ISOs\W11.iso`, `D:\ISOs\Debian.iso`}) {
		t.Fatalf("varios: %v", many)
	}
	if none := splitMultiSelect(encode()); len(none) != 0 {
		t.Fatalf("ninguno: %v", none)
	}
}

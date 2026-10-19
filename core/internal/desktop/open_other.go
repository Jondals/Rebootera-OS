//go:build !windows

// * Abrir una dirección web con el navegador del escritorio en Linux y macOS.
package desktop

import (
	"os/exec"
	"runtime"
)

// * OpenURL abre una dirección web con el navegador predeterminado del escritorio ("open" en macOS, "xdg-open"
// * en Linux).
func OpenURL(url string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", url).Start()
	}
	return exec.Command("xdg-open", url).Start()
}

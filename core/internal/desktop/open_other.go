//go:build !windows

package desktop

import "os/exec"

// * OpenURL abre una dirección web con el navegador predeterminado del escritorio.
func OpenURL(url string) error {
	return exec.Command("xdg-open", url).Start()
}

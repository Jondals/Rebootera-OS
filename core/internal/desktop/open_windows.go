//go:build windows

// * Abrir una dirección web con el navegador del escritorio en Windows.
package desktop

import "os/exec"

// * OpenURL abre una dirección web con el navegador predeterminado de Windows, sin ventanas de consola.
func OpenURL(url string) error {
	return quiet(exec.Command("rundll32", "url.dll,FileProtocolHandler", url)).Start()
}

//go:build windows

package desktop

import (
	"os/exec"
	"syscall"
)

// createNoWindow impide que Windows abra una consola para el proceso hijo.
const createNoWindow = 0x08000000

// * quiet devuelve la orden con la ventana oculta. Rebootera Desktop no tiene consola,
// * así que sin esto cada PowerShell que lanza abre un instante una ventana negra.
func quiet(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return cmd
}

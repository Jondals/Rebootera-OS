//go:build !windows

// * Diálogos nativos de Linux y macOS para elegir imágenes y carpetas (la página web no puede conocer rutas de
// * archivos por seguridad). En macOS se usa AppleScript (osascript, que viene con el sistema); en Linux, zenity o
// * kdialog, lo que haya instalado. Si no hay ninguno, la interfaz recibe un error claro y se puede escribir la ruta.
package desktop

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

// ErrNoPicker indica que no hay un programa de diálogos instalado (en Linux: zenity o kdialog).
var ErrNoPicker = errors.New("no hay un selector de archivos instalado (zenity o kdialog)")

// pickKind es lo que se quiere elegir.
type pickKind int

const (
	pickOneFile pickKind = iota
	pickManyFiles
	pickOneFolder
)

// * PickFile abre un diálogo para elegir una imagen y devuelve su ruta ("" si se cancela).
func PickFile(title string) (string, error) {
	paths, err := runPicker(pickOneFile, title)
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

// * PickFiles abre un diálogo con selección múltiple y devuelve las rutas elegidas ([] si se cancela).
func PickFiles(title string) ([]string, error) {
	return runPicker(pickManyFiles, title)
}

// * PickFolder abre un diálogo para elegir una carpeta y devuelve su ruta ("" si se cancela).
func PickFolder(title string) (string, error) {
	paths, err := runPicker(pickOneFolder, title)
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

// * runPicker lanza el selector del sistema y devuelve las rutas, una por línea de su salida.
func runPicker(kind pickKind, title string) ([]string, error) {
	command, err := pickerCommand(kind, title)
	if err != nil {
		return nil, err
	}
	out, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			// * Cancelar el diálogo termina con un código distinto de cero: no es un error.
			return []string{}, nil
		}
		return nil, err
	}
	paths := []string{}
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	return paths, nil
}

// * pickerCommand arma la orden del selector según el sistema.
func pickerCommand(kind pickKind, title string) (*exec.Cmd, error) {
	if runtime.GOOS == "darwin" {
		return exec.Command("osascript", "-e", appleScript(kind, title)), nil
	}
	if path, err := exec.LookPath("zenity"); err == nil {
		args := []string{"--file-selection", "--title=" + title}
		switch kind {
		case pickManyFiles:
			args = append(args, "--multiple", "--separator=\n")
		case pickOneFolder:
			args = append(args, "--directory")
		}
		return exec.Command(path, args...), nil
	}
	if path, err := exec.LookPath("kdialog"); err == nil {
		switch kind {
		case pickManyFiles:
			return exec.Command(path, "--title", title, "--multiple", "--separate-output", "--getopenfilename", ".", "*.iso *.img *.wim *.vhd *.vhdx"), nil
		case pickOneFolder:
			return exec.Command(path, "--title", title, "--getexistingdirectory", "."), nil
		}
		return exec.Command(path, "--title", title, "--getopenfilename", ".", "*.iso *.img *.wim *.vhd *.vhdx"), nil
	}
	return nil, ErrNoPicker
}

// * appleScript devuelve el programa de AppleScript que pide una o varias rutas, una por línea.
func appleScript(kind pickKind, title string) string {
	prompt := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(title)
	switch kind {
	case pickManyFiles:
		return `set picked to choose file with prompt "` + prompt + `" with multiple selections allowed
set out to ""
repeat with item_ in picked
set out to out & POSIX path of item_ & linefeed
end repeat
return out`
	case pickOneFolder:
		return `POSIX path of (choose folder with prompt "` + prompt + `")`
	}
	return `POSIX path of (choose file with prompt "` + prompt + `")`
}

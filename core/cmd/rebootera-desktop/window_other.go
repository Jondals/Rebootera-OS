//go:build !windows

// * En Linux y macOS, Rebootera Desktop abre la interfaz en una ventana propia: un navegador basado en Chromium
// * (Chrome, Chromium, Edge, Brave) lanzado en "modo aplicación", sin barra de direcciones ni pestañas, con un
// * perfil aparte para que no toque el del usuario. Si no hay ninguno, usa el navegador predeterminado y la
// * aplicación sigue en marcha hasta Ctrl+C. Con la ventana propia, cerrarla cierra la aplicación.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// * appBrowsers son los navegadores que se prueban, por sistema, para abrir la ventana en modo aplicación.
func appBrowsers() []string {
	if runtime.GOOS == "darwin" {
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		}
	}
	return []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge", "brave-browser"}
}

// * findAppBrowser devuelve la ruta del primer navegador de la lista que esté instalado ("" si no hay).
func findAppBrowser() string {
	for _, name := range appBrowsers() {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

// * openWindow abre la ventana de la aplicación y espera a que se cierre.
func openWindow(url string) {
	fmt.Println("Rebootera Desktop:", url)
	if path := findAppBrowser(); path != "" {
		profile := filepath.Join(os.TempDir(), "rebootera-desktop-profile")
		command := exec.Command(path, "--app="+url, "--user-data-dir="+profile, "--no-first-run", "--no-default-browser-check")
		if command.Run() == nil {
			return
		}
	}
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	exec.Command(opener, url).Start()
	select {}
}

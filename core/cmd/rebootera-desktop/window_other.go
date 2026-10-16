//go:build !windows

// * En Linux, Rebootera Desktop abre la interfaz en el navegador predeterminado,
// * escribe la dirección en la terminal (por si el navegador no se abre solo)
// * y sigue en marcha hasta que se cierra con Ctrl+C.
package main

import (
	"fmt"
	"os/exec"
)

// * openWindow abre el navegador y espera indefinidamente.
func openWindow(url string) {
	fmt.Println("Rebootera Desktop:", url)
	exec.Command("xdg-open", url).Start()
	select {}
}

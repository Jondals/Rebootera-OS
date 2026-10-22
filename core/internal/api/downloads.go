// * Descargas del navegador de Rebootera OS: la sesión gráfica las deja en DownloadsDir (una carpeta de grupo, ver
// * tmpfiles.d) y Core las pasa al USB, a drivers/Descargas, donde las lee el asistente de drivers.
package api

import (
	"log"
	"time"

	"rebootera/core/internal/drivers"
	"rebootera/core/internal/vault"
)

// DownloadsDir es donde el navegador guarda lo que se descarga (se puede cambiar en las pruebas).
var DownloadsDir = "/run/rebootera/downloads"

// downloadsEvery es cada cuánto se mira la carpeta de descargas.
const downloadsEvery = 4 * time.Second

// * WatchDownloads pasa al USB lo que descarga el navegador. Sin USB (Vault) no hace nada: las descargas esperan.
func WatchDownloads() {
	for {
		time.Sleep(downloadsEvery)
		if !vault.Available() {
			continue
		}
		imported, err := drivers.ImportDownloads(DownloadsDir, vault.Path("drivers"))
		if len(imported) > 0 {
			log.Printf("descargas: %d archivo(s) pasados al USB (drivers/Descargas)", len(imported))
		}
		if err != nil {
			log.Printf("descargas: %v", err)
		}
	}
}

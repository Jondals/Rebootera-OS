// * Actualizar un USB de Rebootera que ya existe, sin borrarlo: instala una
// * versión nueva de Rebootera OS (o conserva la actual) y añade imágenes y
// * drivers. Es la otra mitad de "Crear USB": la interfaz tiene una sola opción
// * y, según el USB elegido, borra y crea uno nuevo o actualiza este.
package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"rebootera/core/internal/release"
)

// Origen de Rebootera OS al actualizar un USB existente.
const (
	SystemKeep   = "keep"
	SystemLatest = "latest"
	SystemFile   = "file"
)

// ApplyRequest describe una actualización de un USB de Rebootera.
type ApplyRequest struct {
	Root   string `json:"root"`
	System string `json:"system"`
	ISO    string `json:"iso"`
	Extras
}

// * Apply actualiza el USB en segundo plano. Comprueba todo antes de tocarlo.
func Apply(request ApplyRequest) error {
	if !IsRebootera(request.Root) {
		return errors.New("esa unidad no es un USB de Rebootera")
	}
	switch request.System {
	case SystemKeep, SystemLatest:
	case SystemFile:
		info, err := os.Stat(request.ISO)
		if err != nil || !info.Mode().IsRegular() || !strings.EqualFold(filepath.Ext(request.ISO), ".iso") {
			return errors.New("elige una imagen .iso de Rebootera OS que exista")
		}
	default:
		return errors.New("origen de Rebootera OS no válido")
	}
	if request.System == SystemKeep && request.Extras.Empty() {
		return errors.New("no hay nada que actualizar: elige una versión de Rebootera OS o añade algo")
	}
	if err := request.Extras.Validate(); err != nil {
		return err
	}
	if err := begin(); err != nil {
		return err
	}
	go applySteps(request)
	return nil
}

// * applySteps ejecuta la actualización e informa del resultado.
func applySteps(request ApplyRequest) {
	iso := request.ISO
	// ! La descarga va antes de tocar el USB: si falla, no se cambia nada.
	if request.System == SystemLatest {
		setPhase("download", 0)
		path, err := release.Fetch(release.CacheDir(), downloadProgress)
		if err != nil {
			finish("", err)
			return
		}
		iso = path
	}
	if request.System != SystemKeep {
		setPhase("copy-system", 0)
		if err := InstallSystem(request.Root, iso, systemProgress); err != nil {
			finish("", err)
			return
		}
	}
	finish(request.Root, ApplyExtras(request.Root, request.Extras))
}

// * downloadProgress informa del avance al descargar Rebootera OS.
func downloadProgress(fraction float64) {
	setPhase("download", fraction)
}

// * systemProgress informa del avance al copiar Rebootera OS.
func systemProgress(fraction float64) {
	setPhase("copy-system", fraction)
}

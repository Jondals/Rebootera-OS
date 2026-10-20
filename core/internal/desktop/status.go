// * Estado compartido de la creación o actualización de un USB: fase, progreso y error, que la interfaz consulta
// * cada segundo.
package desktop

import (
	"errors"
	"sync"

	"rebootera/core/internal/release"
)

// CreateStatus es el estado de una creación o actualización de USB en curso.
type CreateStatus struct {
	Running  bool    `json:"running"`
	Phase    string  `json:"phase"`
	Progress float64 `json:"progress"`
	// Item es el archivo que se está copiando (imágenes y drivers), o "".
	Item  string `json:"item,omitempty"`
	Error string `json:"error,omitempty"`
	Done  bool   `json:"done"`
	Drive string `json:"drive,omitempty"`
	// Paused indica que la descarga de Rebootera OS está en pausa; Cancelled, que se canceló (no es un error).
	Paused    bool `json:"paused,omitempty"`
	Cancelled bool `json:"cancelled,omitempty"`
}

var (
	createMu     sync.Mutex
	createStatus CreateStatus
	// control pausa y cancela la descarga de la operación en marcha.
	control *release.Control
)

// errNotDownloading se devuelve al pausar o cancelar cuando ya no se está descargando.
var errNotDownloading = errors.New("solo se puede pausar o cancelar mientras se descarga Rebootera OS")

// * Status devuelve el estado de la creación o actualización.
func Status() CreateStatus {
	createMu.Lock()
	defer createMu.Unlock()
	return createStatus
}

// * setPhase actualiza la fase y el progreso.
func setPhase(phase string, progress float64) {
	setItem(phase, "", progress)
}

// * setItem actualiza la fase, el archivo que se copia y el progreso.
func setItem(phase, item string, progress float64) {
	createMu.Lock()
	createStatus.Phase = phase
	createStatus.Item = item
	createStatus.Progress = progress
	createMu.Unlock()
}

// * begin marca el inicio de una operación y falla si ya hay una en marcha.
func begin() error {
	createMu.Lock()
	defer createMu.Unlock()
	if createStatus.Running {
		return errBusy
	}
	createStatus = CreateStatus{Running: true, Phase: "prepare"}
	control = release.NewControl()
	return nil
}

// * Pause pausa la descarga de Rebootera OS conservando lo descargado.
// ! Solo en la fase de descarga: después se escribe en el USB y eso no se interrumpe a medias.
func Pause() error {
	createMu.Lock()
	defer createMu.Unlock()
	if !createStatus.Running || createStatus.Phase != "download" {
		return errNotDownloading
	}
	control.Pause()
	createStatus.Paused = true
	return nil
}

// * Resume continúa la descarga pausada.
func Resume() error {
	createMu.Lock()
	defer createMu.Unlock()
	if !createStatus.Running || createStatus.Phase != "download" {
		return errNotDownloading
	}
	control.Resume()
	createStatus.Paused = false
	return nil
}

// * Cancel cancela la operación durante la descarga y borra lo descargado; el USB no se toca.
func Cancel() error {
	createMu.Lock()
	defer createMu.Unlock()
	if !createStatus.Running || createStatus.Phase != "download" {
		return errNotDownloading
	}
	control.Cancel()
	return nil
}

// * downloadCancelled indica si se pidió cancelar: las operaciones lo comprueban al acabar de descargar,
// * antes de tocar el USB.
func downloadCancelled() error {
	createMu.Lock()
	defer createMu.Unlock()
	if control.Cancelled() {
		return release.ErrCancelled
	}
	return nil
}

// * finish deja el estado final de la operación.
func finish(drive string, err error) {
	createMu.Lock()
	defer createMu.Unlock()
	createStatus.Running = false
	createStatus.Drive = drive
	createStatus.Item = ""
	createStatus.Paused = false
	if errors.Is(err, release.ErrCancelled) {
		createStatus.Cancelled = true
		return
	}
	if err != nil {
		createStatus.Error = err.Error()
		return
	}
	createStatus.Done = true
	createStatus.Progress = 1
}

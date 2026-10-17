package desktop

import "sync"

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
}

var (
	createMu     sync.Mutex
	createStatus CreateStatus
)

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
	return nil
}

// * finish deja el estado final de la operación.
func finish(drive string, err error) {
	createMu.Lock()
	defer createMu.Unlock()
	createStatus.Running = false
	createStatus.Drive = drive
	createStatus.Item = ""
	if err != nil {
		createStatus.Error = err.Error()
		return
	}
	createStatus.Done = true
	createStatus.Progress = 1
}

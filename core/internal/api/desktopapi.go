// * Endpoints de Rebootera Desktop (la aplicación para Windows y Linux que
// * gestiona un USB de Rebootera sin arrancar desde él). Solo se registran en
// * modo escritorio: unidades, elección del USB, añadir imágenes con el
// * diálogo nativo, actualizar Rebootera OS y crear un USB nuevo (Windows).
package api

import (
	"errors"
	"log"
	"net/http"
	"runtime"
	"strconv"
	"sync"

	"rebootera/core/internal/desktop"
	"rebootera/core/internal/vault"
)

// Modos de funcionamiento de Core.
const (
	ModeLive    = "live"
	ModeDesktop = "desktop"
)

// updateState es el estado de "Actualizar Rebootera OS".
type updateState struct {
	mu       sync.Mutex
	Running  bool    `json:"running"`
	Progress float64 `json:"progress"`
	Error    string  `json:"error,omitempty"`
	Done     bool    `json:"done"`
}

var update updateState

// * handleMode indica a la interfaz en qué modo y sistema corre Core.
func (s *Server) handleMode(w http.ResponseWriter, r *http.Request) {
	selected := ""
	if s.deps.Mode == ModeDesktop && vault.Available() {
		selected = vault.Root
	}
	writeJSON(w, http.StatusOK, map[string]string{"mode": s.mode(), "platform": runtime.GOOS, "usb": selected})
}

// * mode devuelve el modo con "live" por defecto.
func (s *Server) mode() string {
	if s.deps.Mode == "" {
		return ModeLive
	}
	return s.deps.Mode
}

// * registerDesktop añade los endpoints del modo escritorio.
func (s *Server) registerDesktop(mux *http.ServeMux) {
	mux.Handle("GET /api/desktop/drives", s.requireSession(http.HandlerFunc(s.handleDrives)))
	mux.Handle("GET /api/desktop/disks", s.requireSession(http.HandlerFunc(s.handleUSBDisks)))
	mux.Handle("GET /api/desktop/update", s.requireSession(http.HandlerFunc(s.handleUpdateStatus)))
	mux.Handle("GET /api/desktop/create", s.requireSession(http.HandlerFunc(s.handleCreateStatus)))
	post := map[string]http.HandlerFunc{
		"/api/desktop/select": s.handleSelectDrive,
		"/api/desktop/pick":   s.handlePick,
		"/api/desktop/add":    s.handleAddImage,
		"/api/desktop/update": s.handleUpdate,
		"/api/desktop/create": s.handleCreate,
	}
	for path, handler := range post {
		mux.Handle("POST "+path, s.requireSession(s.requireCSRF(handler)))
	}
}

// * handleDrives lista las unidades del equipo marcando los USB de Rebootera.
func (s *Server) handleDrives(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, desktop.Drives())
}

// rootRequest es el cuerpo de POST /api/desktop/select.
type rootRequest struct {
	Root string `json:"root"`
}

// * handleSelectDrive elige el USB de Rebootera con el que se trabaja.
func (s *Server) handleSelectDrive(w http.ResponseWriter, r *http.Request) {
	var req rootRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	if !desktop.IsRebootera(req.Root) {
		fail(w, errors.New("esa unidad no es un USB de Rebootera"))
		return
	}
	if err := s.deps.Transfers.Reset(); err != nil {
		fail(w, err)
		return
	}
	if err := s.deps.Library.Reset(); err != nil {
		fail(w, err)
		return
	}
	vault.Root = req.Root
	log.Printf("escritorio: USB de Rebootera elegido en %s", req.Root)
	writeJSON(w, http.StatusOK, map[string]string{"usb": req.Root})
}

// pickRequest es el cuerpo de POST /api/desktop/pick.
type pickRequest struct {
	Title string `json:"title"`
}

// * handlePick abre el diálogo nativo para elegir una imagen.
func (s *Server) handlePick(w http.ResponseWriter, r *http.Request) {
	var req pickRequest
	decode(r, &req)
	path, err := desktop.PickFile(req.Title)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": path})
}

// addRequest es el cuerpo de POST /api/desktop/add.
type addRequest struct {
	Path     string `json:"path"`
	Category string `json:"category"`
}

// * handleAddImage copia una imagen del equipo a la biblioteca del USB.
func (s *Server) handleAddImage(w http.ResponseWriter, r *http.Request) {
	var req addRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	job, err := s.deps.Transfers.Import(req.Path, req.Category)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

// pathOnlyRequest es el cuerpo de las peticiones con una ruta local.
type pathOnlyRequest struct {
	Path string `json:"path"`
}

// * handleUpdate instala una versión nueva de Rebootera OS en el USB elegido.
func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	var req pathOnlyRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	if !vault.Available() {
		fail(w, vault.ErrUnavailable)
		return
	}
	update.mu.Lock()
	if update.Running {
		update.mu.Unlock()
		fail(w, errors.New("ya se está actualizando"))
		return
	}
	update.Running, update.Done, update.Error, update.Progress = true, false, "", 0
	update.mu.Unlock()
	job := &updateJob{root: vault.Root, iso: req.Path}
	go job.run()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "running"})
}

// updateJob copia la ISO nueva al USB en segundo plano.
type updateJob struct {
	root string
	iso  string
}

// * run ejecuta la actualización y deja el estado final.
func (j *updateJob) run() {
	err := desktop.InstallSystem(j.root, j.iso, updateProgress)
	update.mu.Lock()
	defer update.mu.Unlock()
	update.Running = false
	if err != nil {
		update.Error = err.Error()
		log.Printf("escritorio: la actualización falló: %v", err)
		return
	}
	update.Done = true
	update.Progress = 1
	log.Printf("escritorio: Rebootera OS actualizado en %s", j.root)
}

// * updateProgress guarda el avance de la copia.
func updateProgress(fraction float64) {
	update.mu.Lock()
	update.Progress = fraction
	update.mu.Unlock()
}

// * handleUpdateStatus devuelve el estado de la actualización.
func (s *Server) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	update.mu.Lock()
	defer update.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"running": update.Running, "progress": update.Progress, "error": update.Error, "done": update.Done})
}

// * handleUSBDisks lista los discos USB donde se puede crear un USB de Rebootera.
func (s *Server) handleUSBDisks(w http.ResponseWriter, r *http.Request) {
	disks, err := desktop.USBDisks()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, disks)
}

// createRequest es el cuerpo de POST /api/desktop/create.
type createRequest struct {
	Disk    int    `json:"disk"`
	ISO     string `json:"iso"`
	Confirm string `json:"confirm"`
}

// * handleCreate crea un USB de Rebootera nuevo (borra el disco elegido).
// ! Exige que la interfaz repita el número de disco en "confirm".
func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	if req.Confirm != "ERASE DISK "+strconv.Itoa(req.Disk) {
		fail(w, errors.New("falta la confirmación explícita del disco que se va a borrar"))
		return
	}
	if err := desktop.Create(req.Disk, req.ISO, s.deps.Kit, s.deps.Executable); err != nil {
		fail(w, err)
		return
	}
	log.Printf("escritorio: creando un USB de Rebootera en el disco %d", req.Disk)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "running"})
}

// * handleCreateStatus devuelve el estado de la creación.
func (s *Server) handleCreateStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, desktop.Status())
}

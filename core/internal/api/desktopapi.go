// * Endpoints de Rebootera Desktop (la aplicación para Windows y Linux que
// * gestiona un USB de Rebootera sin arrancar desde él). Solo se registran en
// * modo escritorio: unidades, elección del USB, diálogos nativos, crear un USB
// * nuevo (Windows) o actualizar uno existente, y los drivers del USB.
package api

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"strconv"

	"rebootera/core/internal/catalog"
	"rebootera/core/internal/desktop"
	"rebootera/core/internal/drivers"
	"rebootera/core/internal/vault"
)

// Modos de funcionamiento de Core.
const (
	ModeLive    = "live"
	ModeDesktop = "desktop"
)

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
	mux.Handle("GET /api/desktop/create", s.requireSession(http.HandlerFunc(s.handleCreateStatus)))
	mux.Handle("GET /api/desktop/drivers", s.requireSession(http.HandlerFunc(s.handleDesktopDrivers)))
	post := map[string]http.HandlerFunc{
		"/api/desktop/select":         s.handleSelectDrive,
		"/api/desktop/pick":           s.handlePick,
		"/api/desktop/open":           s.handleOpenURL,
		"/api/desktop/pick-files":     s.handlePickFiles,
		"/api/desktop/pick-folder":    s.handlePickFolder,
		"/api/desktop/add":            s.handleAddImage,
		"/api/desktop/apply":          s.handleApply,
		"/api/desktop/create":         s.handleCreate,
		"/api/desktop/drivers/add":    s.handleDriversAdd,
		"/api/desktop/drivers/remove": s.handleDriversRemove,
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
	SyncSettings()
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

// * handlePickFiles abre el diálogo nativo para elegir varias imágenes.
func (s *Server) handlePickFiles(w http.ResponseWriter, r *http.Request) {
	var req pickRequest
	decode(r, &req)
	paths, err := desktop.PickFiles(req.Title)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]string{"paths": paths})
}

// * handlePickFolder abre el diálogo nativo para elegir una carpeta de drivers.
func (s *Server) handlePickFolder(w http.ResponseWriter, r *http.Request) {
	var req pickRequest
	decode(r, &req)
	path, err := desktop.PickFolder(req.Title)
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

// * handleApply actualiza un USB de Rebootera que ya existe (sin borrarlo).
func (s *Server) handleApply(w http.ResponseWriter, r *http.Request) {
	var req desktop.ApplyRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	if err := desktop.Apply(req); err != nil {
		fail(w, err)
		return
	}
	log.Printf("escritorio: actualizando el USB de Rebootera %s", req.Root)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "running"})
}

// * handleUSBDisks lista los discos USB donde se puede crear un USB de Rebootera. Sin parámetros usa la lista
// * guardada hace pocos segundos (sale al instante); con ?fresh=1 vuelve a leer los discos.
func (s *Server) handleUSBDisks(w http.ResponseWriter, r *http.Request) {
	disks, err := desktop.CachedUSBDisks(r.URL.Query().Get("fresh") == "1")
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
	// Mode es "erase" (borra el disco, por defecto) o "keep" (añade Rebootera sin borrar).
	Mode string `json:"mode"`
	// Extras son las imágenes y carpetas de drivers que salen ya dentro del USB.
	desktop.Extras
}

// * handleCreate crea un USB de Rebootera nuevo (borra el disco elegido).
// ! Exige que la interfaz repita el número de disco en "confirm".
func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	keep := req.Mode == "keep"
	want := "ERASE DISK " + strconv.Itoa(req.Disk)
	if keep {
		want = "ADD TO DISK " + strconv.Itoa(req.Disk)
	}
	if req.Confirm != want {
		fail(w, errors.New("falta la confirmación explícita del disco que se va a modificar"))
		return
	}
	if err := desktop.Create(req.Disk, req.ISO, s.deps.Kit, s.deps.Executable, req.Extras, keep); err != nil {
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

// driversView es la respuesta de GET /api/desktop/drivers.
type driversView struct {
	// Available indica que hay un USB (o biblioteca local) con carpeta drivers/.
	Available bool `json:"available"`
	// Root es la unidad o carpeta donde están los drivers.
	Root     string             `json:"root"`
	Packages []*drivers.Package `json:"packages"`
}

// * handleDesktopDrivers lista los paquetes de drivers que hay en el USB elegido.
func (s *Server) handleDesktopDrivers(w http.ResponseWriter, r *http.Request) {
	view := driversView{Packages: []*drivers.Package{}}
	if vault.Available() {
		view.Available, view.Root = true, vault.Root
		view.Packages = append(view.Packages, drivers.Scan(driversRoot())...)
	}
	writeJSON(w, http.StatusOK, view)
}

// * handleDriversAdd copia una carpeta de drivers del equipo al USB elegido.
func (s *Server) handleDriversAdd(w http.ResponseWriter, r *http.Request) {
	var req pathOnlyRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	if !vault.Available() {
		fail(w, vault.ErrUnavailable)
		return
	}
	if err := desktop.AddDrivers(vault.Root, req.Path); err != nil {
		fail(w, err)
		return
	}
	log.Printf("escritorio: drivers añadidos desde %s", req.Path)
	s.handleDesktopDrivers(w, r)
}

// nameRequest es el cuerpo de POST /api/desktop/drivers/remove.
type nameRequest struct {
	Name string `json:"name"`
}

// * handleDriversRemove borra una carpeta de drivers del USB elegido.
func (s *Server) handleDriversRemove(w http.ResponseWriter, r *http.Request) {
	var req nameRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	if !vault.Available() {
		fail(w, vault.ErrUnavailable)
		return
	}
	if err := desktop.RemoveDrivers(vault.Root, req.Name); err != nil {
		fail(w, err)
		return
	}
	s.handleDesktopDrivers(w, r)
}

// openRequest es el cuerpo de POST /api/desktop/open.
type openRequest struct {
	URL string `json:"url"`
}

// * handleOpenURL abre en el navegador la página del fabricante de un sistema del catálogo.
// ! Solo se abren direcciones HTTPS de las páginas del propio catálogo: la interfaz no puede lanzar
// ! cualquier dirección.
func (s *Server) handleOpenURL(w http.ResponseWriter, r *http.Request) {
	var req openRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	parsed, err := url.Parse(req.URL)
	if err != nil || parsed.Scheme != "https" || !catalogHost(parsed.Host) {
		fail(w, errors.New("esa dirección no es una página del catálogo"))
		return
	}
	if err := desktop.OpenURL(parsed.String()); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"opened": parsed.String()})
}

// * catalogHost indica si un servidor es el de la página de alguna entrada o familia del catálogo.
func catalogHost(host string) bool {
	entries, err := catalog.Entries()
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if page, err := url.Parse(entry.Homepage); err == nil && page.Host == host {
			return true
		}
	}
	return false
}

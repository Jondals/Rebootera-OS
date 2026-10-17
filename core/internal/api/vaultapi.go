// * Endpoints del Vault: ajustes persistentes, catálogo, transferencias
// * (descargas e importaciones), biblioteca, medios externos y elección del
// * próximo arranque. Todos los cambios exigen sesión y cabecera anti-CSRF.
// * Los errores llevan un código ("code") que la interfaz traduce.
package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"rebootera/core/internal/agentproto"
	"rebootera/core/internal/boot"
	"rebootera/core/internal/catalog"
	"rebootera/core/internal/library"
	"rebootera/core/internal/storage"
	"rebootera/core/internal/transfers"
	"rebootera/core/internal/vault"
)

// MediaRoot es donde el agente monta los medios externos.
var MediaRoot = "/run/rebootera/media"

// * maxBrowseEntries limita lo que se lista de una carpeta de un medio externo.
const maxBrowseEntries = 500

// Códigos de error de la API.
const (
	codeNoVault  = "no-vault"
	codeInvalid  = "invalid"
	codeConflict = "conflict"
	codeManual   = "manual"
	codeAgent    = "agent"
	codeFailed   = "failed"
)

// Deps son los servicios y la configuración de Core.
type Deps struct {
	Library   *library.Library
	Transfers *transfers.Manager
	// Mode es "live" (Rebootera OS) o "desktop" (Rebootera Desktop).
	Mode string
	// Kit es el contenido común de un USB nuevo (solo en el escritorio).
	Kit fs.FS
	// Executable es la ruta de Rebootera Desktop, que se copia a los USB nuevos.
	Executable string
}

// * registerVault añade los endpoints del Vault al enrutador.
func (s *Server) registerVault(mux *http.ServeMux) {
	get := map[string]http.HandlerFunc{
		"/api/vault":     s.handleVault,
		"/api/settings":  s.handleGetSettings,
		"/api/catalog":   s.handleCatalog,
		"/api/transfers": s.handleTransfers,
		"/api/library":   s.handleLibrary,
	}
	post := map[string]http.HandlerFunc{
		"/api/settings":           s.handleSaveSettings,
		"/api/catalog/resolve":    s.handleResolve,
		"/api/transfers/download": s.handleDownload,
		"/api/transfers/import":   s.handleImport,
		"/api/transfers/pause":    s.handleTransferAction,
		"/api/transfers/resume":   s.handleTransferAction,
		"/api/transfers/cancel":   s.handleTransferAction,
		"/api/transfers/clear":    s.handleTransferClear,
		"/api/library/verify":     s.handleVerify,
		"/api/library/delete":     s.handleDelete,
	}
	// * Abrir discos del equipo y arrancar instaladores solo tiene sentido
	// * dentro de Rebootera OS, donde existe el agente.
	if s.mode() == ModeLive {
		get["/api/media/browse"] = s.handleBrowse
		post["/api/media/mount"] = s.handleMediaMount
		post["/api/media/unmount"] = s.handleMediaMount
		post["/api/boot/install"] = s.handleBootInstall
		get["/api/drivers/plan"] = s.handleDriverPlan
	}
	for path, handler := range get {
		mux.Handle("GET "+path, s.requireSession(handler))
	}
	for path, handler := range post {
		mux.Handle("POST "+path, s.requireSession(s.requireCSRF(handler)))
	}
}

// * decode lee el cuerpo JSON de una petición.
func decode(r *http.Request, value any) error {
	return json.NewDecoder(r.Body).Decode(value)
}

// * fail responde con el código y mensaje adecuados para un error.
func fail(w http.ResponseWriter, err error) {
	status, code := http.StatusBadRequest, codeFailed
	switch {
	case errors.Is(err, vault.ErrUnavailable):
		status, code = http.StatusConflict, codeNoVault
	case errors.Is(err, vault.ErrOutside):
		status, code = http.StatusBadRequest, codeInvalid
	case errors.Is(err, catalog.ErrManual):
		status, code = http.StatusConflict, codeManual
	case strings.Contains(err.Error(), "ya está"):
		status, code = http.StatusConflict, codeConflict
	}
	writeJSON(w, status, map[string]string{"error": err.Error(), "code": code})
}

// vaultStatus es la respuesta de GET /api/vault.
type vaultStatus struct {
	Available bool   `json:"available"`
	UsedBytes uint64 `json:"usedBytes"`
	FreeBytes uint64 `json:"freeBytes"`
	NextBoot  string `json:"nextBoot"`
}

// * handleVault informa de si hay Vault y de su espacio.
func (s *Server) handleVault(w http.ResponseWriter, r *http.Request) {
	status := vaultStatus{Available: vault.Available()}
	if status.Available {
		status.UsedBytes, status.FreeBytes = storage.FSUsage(vault.Root)
		status.NextBoot = boot.Next()
	}
	writeJSON(w, http.StatusOK, status)
}

// * handleGetSettings devuelve los ajustes guardados en el Vault.
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := vault.LoadSettings()
	if err != nil && !errors.Is(err, vault.ErrUnavailable) {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"persistent": err == nil, "settings": settings})
}

// * handleSaveSettings guarda los ajustes en el Vault.
func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var settings vault.Settings
	if err := decode(r, &settings); err != nil {
		fail(w, err)
		return
	}
	if err := vault.SaveSettings(settings); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// catalogItem es una entrada del catálogo con su estado en la biblioteca.
type catalogItem struct {
	catalog.Entry
	InLibrary bool `json:"inLibrary"`
}

// * handleCatalog devuelve el catálogo marcando lo que ya está en la biblioteca.
func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	entries, err := catalog.Entries()
	if err != nil {
		fail(w, err)
		return
	}
	sources := map[string]bool{}
	if items, err := s.deps.Library.List(); err == nil {
		for _, item := range items {
			sources[item.Source] = true
		}
	}
	result := make([]catalogItem, 0, len(entries))
	for _, entry := range entries {
		result = append(result, catalogItem{Entry: entry, InLibrary: sources[entry.ID]})
	}
	writeJSON(w, http.StatusOK, result)
}

// idRequest es el cuerpo de las peticiones que solo llevan un id.
type idRequest struct {
	ID string `json:"id"`
}

// * handleResolve averigua la versión y el tamaño actuales de una entrada.
func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	var req idRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	entry, err := catalog.Find(req.ID)
	if err != nil {
		fail(w, err)
		return
	}
	resolved, err := catalog.Resolve(entry)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resolved)
}

// * handleTransfers devuelve la cola de transferencias.
func (s *Server) handleTransfers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.deps.Transfers.List())
}

// * handleDownload añade una descarga del catálogo a la cola.
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	var req idRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	job, err := s.deps.Transfers.Download(req.ID)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

// importRequest es el cuerpo de POST /api/transfers/import.
type importRequest struct {
	Device   string `json:"device"`
	Path     string `json:"path"`
	Category string `json:"category"`
}

// * handleImport copia una imagen de un medio montado a la biblioteca.
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	var req importRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	source, err := mediaPath(req.Device, req.Path)
	if err != nil {
		fail(w, err)
		return
	}
	job, err := s.deps.Transfers.Import(source, req.Category)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

// * handleTransferAction pausa, reanuda o cancela según la ruta.
func (s *Server) handleTransferAction(w http.ResponseWriter, r *http.Request) {
	var req idRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	var err error
	switch filepath.Base(r.URL.Path) {
	case "pause":
		err = s.deps.Transfers.Pause(req.ID)
	case "resume":
		err = s.deps.Transfers.Resume(req.ID)
	case "cancel":
		err = s.deps.Transfers.Cancel(req.ID)
	}
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// * handleTransferClear quita de la lista las transferencias terminadas.
func (s *Server) handleTransferClear(w http.ResponseWriter, r *http.Request) {
	s.deps.Transfers.Clear()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// * handleLibrary lista la biblioteca con la compatibilidad de arranque de
// * las imágenes que vienen del catálogo.
func (s *Server) handleLibrary(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Library.List()
	if err != nil {
		fail(w, err)
		return
	}
	for i := range items {
		if entry, err := catalog.Find(items[i].Source); err == nil {
			items[i].BootStatus = entry.Boot.Status
		}
	}
	writeJSON(w, http.StatusOK, items)
}

// pathRequest es el cuerpo de las peticiones sobre una imagen.
type pathRequest struct {
	Path string `json:"path"`
}

// * handleVerify calcula el hash de una imagen en segundo plano.
func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	var req pathRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	if err := s.deps.Library.Verify(req.Path); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "hashing"})
}

// * handleDelete borra una imagen de la biblioteca.
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	var req pathRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	if err := s.deps.Library.Delete(req.Path); err != nil {
		fail(w, err)
		return
	}
	log.Printf("biblioteca: %s borrada desde la interfaz", req.Path)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// deviceRequest es el cuerpo de las peticiones sobre un medio externo.
type deviceRequest struct {
	Device string `json:"device"`
}

// * handleMediaMount monta o desmonta (según la ruta) un medio en solo lectura.
func (s *Server) handleMediaMount(w http.ResponseWriter, r *http.Request) {
	var req deviceRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	op := agentproto.OpMediaMount
	if strings.HasSuffix(r.URL.Path, "/unmount") {
		op = agentproto.OpMediaUnmount
	}
	if _, err := s.agent.Send(agentproto.Request{Op: op, Device: req.Device}); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error(), "code": codeAgent})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// browseEntry es un elemento de una carpeta de un medio externo.
type browseEntry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Dir       bool   `json:"dir"`
	SizeBytes int64  `json:"sizeBytes"`
}

// * imageExtensions son los archivos que se pueden importar.
var imageExtensions = []string{".iso", ".img", ".wim", ".vhd", ".vhdx", ".efi"}

// * handleBrowse lista carpetas e imágenes de un medio montado.
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	dir, err := mediaPath(r.URL.Query().Get("device"), r.URL.Query().Get("dir"))
	if err != nil {
		fail(w, err)
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		fail(w, err)
		return
	}
	base := filepath.Join(MediaRoot, filepath.Base(r.URL.Query().Get("device")))
	result := []browseEntry{}
	for _, entry := range entries {
		if len(result) >= maxBrowseEntries {
			break
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "$") {
			continue
		}
		full := filepath.Join(dir, name)
		rel, _ := filepath.Rel(base, full)
		item := browseEntry{Name: name, Path: filepath.ToSlash(rel), Dir: entry.IsDir()}
		if !item.Dir {
			if !slices.Contains(imageExtensions, strings.ToLower(filepath.Ext(name))) {
				continue
			}
			if info, err := entry.Info(); err == nil {
				item.SizeBytes = info.Size()
			}
		}
		result = append(result, item)
	}
	slices.SortFunc(result, compareBrowse)
	writeJSON(w, http.StatusOK, result)
}

// * compareBrowse ordena carpetas primero y luego por nombre.
func compareBrowse(a, b browseEntry) int {
	if a.Dir != b.Dir {
		if a.Dir {
			return -1
		}
		return 1
	}
	return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
}

// * mediaPath resuelve una ruta dentro de un medio montado sin salir de él.
func mediaPath(device, rel string) (string, error) {
	name := filepath.Base(strings.TrimPrefix(device, "/dev/"))
	if name == "." || name == "/" || name == "" {
		return "", vault.ErrOutside
	}
	base := filepath.Join(MediaRoot, name)
	if _, err := os.Stat(base); err != nil {
		return "", errors.New("ese medio no está abierto")
	}
	full := filepath.Join(base, filepath.Clean("/"+rel))
	if full != base && !strings.HasPrefix(full, base+string(filepath.Separator)) {
		return "", vault.ErrOutside
	}
	return full, nil
}

// * handleBootInstall preselecciona una imagen en Ventoy y reinicia el equipo.
func (s *Server) handleBootInstall(w http.ResponseWriter, r *http.Request) {
	var req installRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	items, err := s.deps.Library.List()
	if err != nil {
		fail(w, err)
		return
	}
	var chosen library.Item
	// ! Nunca se arranca una imagen dañada, a medio comprobar o que se sabe
	// ! que no arranca desde el menú del USB.
	for _, item := range items {
		if item.Path != req.Path {
			continue
		}
		chosen = item
		if item.State == library.StateMismatch || item.State == library.StateHashing {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "image not verified", "code": "unsafe-image"})
			return
		}
		if entry, err := catalog.Find(item.Source); err == nil && entry.Boot.Status == "incompatible" {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "image cannot boot from the menu", "code": "incompatible-image"})
			return
		}
	}
	if err := s.agent.Call(agentproto.OpPing); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error(), "code": codeAgent})
		return
	}
	prep, code, message := prepareInstall(chosen, req)
	if code != "" {
		status := http.StatusBadRequest
		if code == codeNoDrivers {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": message, "code": code})
		return
	}
	if err := boot.Prepare(req.Path, prep); err != nil {
		fail(w, err)
		return
	}
	bootOrder := "unchanged"
	if req.BootOnce {
		lines, err := s.agent.Query(agentproto.OpBootOnce)
		if err != nil {
			// * Sin permiso para el firmware no se instala a medias: se avisa y no se reinicia.
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error(), "code": codeBootOrder})
			return
		}
		if len(lines) > 0 {
			bootOrder = lines[0]
		}
	}
	log.Printf("arranque: %s preseleccionada en Ventoy (orden de arranque: %s); reiniciando", req.Path, bootOrder)
	task := &delayedOp{op: agentproto.OpReboot, agent: s.agent}
	time.AfterFunc(powerDelay, task.run)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "rebooting", "bootOrder": bootOrder})
}

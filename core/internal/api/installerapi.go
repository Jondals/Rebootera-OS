// * Endpoints de las carpetas de instalador: cualquier instalación hecha con cualquier herramienta (Rufus, Ventoy,
// * una carpeta extraída). Inspeccionar la carpeta y convertirla en una ISO arrancable dentro de la biblioteca.
package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"rebootera/core/internal/desktop"
	"rebootera/core/internal/installer"
)

// installerRequest es el cuerpo de las peticiones sobre una carpeta de instalador.
// En Rebootera OS la carpeta está en un medio abierto (device + path); en el escritorio es una ruta local.
type installerRequest struct {
	Device string `json:"device"`
	Path   string `json:"path"`
}

// * installerFolder resuelve la carpeta indicada por la petición.
func (s *Server) installerFolder(req installerRequest) (string, error) {
	if req.Device != "" {
		return mediaPath(req.Device, req.Path)
	}
	if s.mode() == ModeDesktop && filepath.IsAbs(req.Path) {
		return filepath.Clean(req.Path), nil
	}
	return "", errors.New("carpeta no válida")
}

// * handleInspect dice si una carpeta es un instalador de Windows y cuánto ocupa.
func (s *Server) handleInspect(w http.ResponseWriter, r *http.Request) {
	var req installerRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	folder, err := s.installerFolder(req)
	if err != nil {
		fail(w, err)
		return
	}
	info, err := installer.Measure(installer.Detect(folder))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// * handleMakeISO añade a la cola la creación de una ISO a partir de la carpeta de un instalador.
func (s *Server) handleMakeISO(w http.ResponseWriter, r *http.Request) {
	var req installerRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	folder, err := s.installerFolder(req)
	if err != nil {
		fail(w, err)
		return
	}
	job, err := s.deps.Transfers.MakeISO(folder)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

// foundInstaller es un instalador en carpetas encontrado en un disco.
type foundInstaller struct {
	installer.Info
	// Drive es el disco donde está (letra o nombre del medio) y Device/Rel lo identifican en Rebootera OS.
	Drive  string `json:"drive"`
	Device string `json:"device,omitempty"`
	Rel    string `json:"rel,omitempty"`
}

// * handleFoundInstallers busca instaladores en carpetas (los que crea Rufus) en los discos del equipo:
// * en la raíz de cada unidad y en sus carpetas de primer nivel. Solo lee.
func (s *Server) handleFoundInstallers(w http.ResponseWriter, r *http.Request) {
	found := []foundInstaller{}
	if s.mode() == ModeDesktop {
		for _, drive := range desktop.Drives() {
			if drive.Rebootera {
				continue
			}
			found = scanRoot(found, drive.Root, drive.Root, "", "")
		}
	} else {
		media, _ := os.ReadDir(MediaRoot)
		for _, entry := range media {
			if entry.IsDir() {
				found = scanRoot(found, filepath.Join(MediaRoot, entry.Name()), entry.Name(), entry.Name(), "")
			}
		}
	}
	writeJSON(w, http.StatusOK, found)
}

// * scanRoot añade los instaladores de la raíz y de sus carpetas de primer nivel.
func scanRoot(found []foundInstaller, root, drive, device, rel string) []foundInstaller {
	found = appendIfInstaller(found, root, drive, device, rel)
	entries, err := os.ReadDir(root)
	if err != nil {
		return found
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "$") {
			continue
		}
		found = appendIfInstaller(found, filepath.Join(root, name), drive, device, name)
	}
	return found
}

// * appendIfInstaller mide y añade la carpeta si es un instalador de Windows con algún modo de arranque.
func appendIfInstaller(found []foundInstaller, folder, drive, device, rel string) []foundInstaller {
	info := installer.Detect(folder)
	if info.Kind == "" {
		return found
	}
	if measured, err := installer.Measure(info); err == nil {
		info = measured
	}
	return append(found, foundInstaller{Info: info, Drive: drive, Device: device, Rel: rel})
}

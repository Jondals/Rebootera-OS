// * Endpoints de las descargas desde un enlace: inspeccionar qué imágenes hay detrás de una dirección https y
// * poner en cola la que elige el usuario. La consulta se repite en Core: nunca se confía en el tamaño ni en las sumas
// * que diga el cliente.
package api

import (
	"net/http"

	"rebootera/core/internal/catalog"
)

// linkRequest es el cuerpo de POST /api/transfers/url.
type linkRequest struct {
	URL      string `json:"url"`
	Category string `json:"category"`
}

// * handleInspectLink dice qué archivos de imagen hay detrás de la dirección que escribió el usuario.
func (s *Server) handleInspectLink(w http.ResponseWriter, r *http.Request) {
	candidates, err := catalog.InspectLink(r.URL.Query().Get("url"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, candidates)
}

// * handleDownloadLink añade a la cola la descarga de un archivo de imagen indicado con un enlace.
func (s *Server) handleDownloadLink(w http.ResponseWriter, r *http.Request) {
	var req linkRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	job, err := s.deps.Transfers.DownloadLink(req.URL, req.Category)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

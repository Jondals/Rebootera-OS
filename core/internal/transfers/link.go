// * Descarga de una imagen indicada con un enlace (directo o de un elemento del Internet Archive): se consulta el
// * enlace, se elige el archivo y se encola como una descarga más, sin comprobar si la fuente no publica suma.
package transfers

import (
	"errors"
	"strings"

	"rebootera/core/internal/catalog"
	"rebootera/core/internal/vault"
)

// * DownloadLink añade a la cola la descarga de un archivo de imagen indicado por el usuario con un enlace
// * (directo o de un elemento del Internet Archive). Se vuelve a consultar el enlace para tomar el tamaño y la
// * suma de la fuente, nunca del cliente. Sin suma SHA-256 oficial, la imagen entra en la biblioteca sin comprobar.
func (m *Manager) DownloadLink(link, category string) (Job, error) {
	if !vault.Available() {
		return Job{}, vault.ErrUnavailable
	}
	candidates, err := catalog.InspectLink(link)
	if err != nil {
		return Job{}, err
	}
	var pick *catalog.Candidate
	for i := range candidates {
		if candidates[i].URL == link || len(candidates) == 1 {
			pick = &candidates[i]
			break
		}
	}
	if pick == nil {
		return Job{}, errors.New("el enlace no indica un único archivo")
	}
	switch category {
	case "windows", "linux", "tools":
	default:
		category = "tools"
	}
	return m.enqueue(&Job{
		Kind:       KindDownload,
		Name:       strings.TrimSuffix(pick.Name, ".iso"),
		CatalogID:  "link",
		URL:        pick.URL,
		FileName:   pick.Name,
		Category:   category,
		SHA1:       pick.SHA1,
		TotalBytes: pick.SizeBytes,
	})
}

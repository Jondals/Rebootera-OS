// * Descargas por enlace: el usuario pega una dirección (un archivo directo o la página de un elemento del
// * Internet Archive) y se le ofrecen los archivos de imagen que hay detrás. Rebootera no elige ni recomienda
// * qué enlace pegar: solo descarga al USB lo que se le indica, por HTTPS y con la suma que haya publicada.
package catalog

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
)

// imageExtensions son las extensiones de imagen que se aceptan al descargar por enlace.
var imageExtensions = []string{".iso", ".img", ".wim", ".esd", ".vhd", ".vhdx", ".img.xz", ".img.gz"}

// Candidate es un archivo de imagen que se puede descargar por enlace.
type Candidate struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	SizeBytes int64  `json:"sizeBytes"`
	// SHA1 es la suma que publica el Internet Archive; vacía en un enlace directo.
	SHA1 string `json:"sha1,omitempty"`
}

// * isImageName indica si un nombre de archivo tiene extensión de imagen.
func isImageName(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range imageExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// * archiveItem extrae el identificador de un elemento de una dirección del Internet Archive
// * (https://archive.org/details/ID o https://archive.org/download/ID/...), o "" si no lo es.
func archiveItem(parsed *url.URL) string {
	if parsed.Host != "archive.org" && parsed.Host != "www.archive.org" {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) >= 2 && (parts[0] == "details" || parts[0] == "download") {
		return parts[1]
	}
	return ""
}

// * InspectLink dice qué archivos de imagen hay detrás de una dirección: los de un elemento del Internet
// * Archive (con su tamaño y SHA-1) o el archivo directo (con su tamaño, si el servidor lo da).
func InspectLink(raw string) ([]Candidate, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return nil, errors.New("la dirección no es válida")
	}
	if parsed.Scheme != "https" {
		return nil, errors.New("solo se admiten direcciones HTTPS")
	}
	if id := archiveItem(parsed); id != "" {
		return inspectArchive(id)
	}
	return inspectDirect(parsed)
}

// * inspectArchive lista los archivos de imagen originales de un elemento del Internet Archive con su suma.
func inspectArchive(id string) ([]Candidate, error) {
	files, err := archiveFiles(id)
	if err != nil {
		return nil, err
	}
	var found []Candidate
	for _, file := range files {
		if file.Source != "original" || !isImageName(file.Name) {
			continue
		}
		found = append(found, archiveCandidate(id, file))
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("el elemento %s no tiene archivos de imagen (%s)", id, strings.Join(imageExtensions, ", "))
	}
	return found, nil
}

// * inspectDirect pregunta al servidor por un archivo directo (HEAD) y comprueba que sea una imagen.
func inspectDirect(parsed *url.URL) ([]Candidate, error) {
	name := path.Base(parsed.Path)
	if !isImageName(name) {
		return nil, fmt.Errorf("el enlace no termina en una imagen (%s)", strings.Join(imageExtensions, ", "))
	}
	req, err := http.NewRequest("HEAD", parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("el servidor respondió %d", resp.StatusCode)
	}
	if resp.Request.URL.Scheme != "https" {
		return nil, errors.New("el enlace redirige a una conexión sin cifrar")
	}
	return []Candidate{{Name: name, URL: resp.Request.URL.String(), SizeBytes: max(resp.ContentLength, 0)}}, nil
}

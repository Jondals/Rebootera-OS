package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
)

// versionName es lo que se acepta como versión leída de un manifiesto: sin barras ni nada que cambie la dirección.
var versionName = regexp.MustCompile(`^[0-9][0-9A-Za-z._-]{0,30}$`)

// * resolveManifest sirve a los proyectos que publican un JSON con la última versión ("version") y guardan la imagen y su
// * suma SHA-256 en una carpeta con esa versión (TrueNAS): Index es el JSON, Link la dirección de la imagen con {version}
// * donde va la versión y SumSuffix lo que se añade a esa dirección para encontrar la suma.
func resolveManifest(r Resolver) (Resolved, error) {
	if r.SumSuffix == "" || !strings.Contains(r.Link, "{version}") {
		return Resolved{}, errors.New("el resolvedor manifest necesita link con {version} y sumSuffix")
	}
	text, err := fetchText(r.Index)
	if err != nil {
		return Resolved{}, err
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(text), &manifest); err != nil || !versionName.MatchString(manifest.Version) {
		return Resolved{}, fmt.Errorf("%s: no trae una versión válida", r.Index)
	}
	link := strings.ReplaceAll(r.Link, "{version}", manifest.Version)
	if !strings.HasPrefix(link, "https://") {
		return Resolved{}, fmt.Errorf("%s: la imagen no está en una conexión cifrada", link)
	}
	sumText, err := fetchText(link + r.SumSuffix)
	if err != nil {
		return Resolved{}, err
	}
	fields := strings.Fields(sumText)
	if len(fields) == 0 || !isSHA256(fields[0]) {
		return Resolved{}, fmt.Errorf("%s: la suma publicada no es un SHA-256", link+r.SumSuffix)
	}
	name := path.Base(link)
	if len(fields) > 1 && strings.TrimPrefix(fields[1], "*") != name {
		return Resolved{}, fmt.Errorf("%s: la suma es de otro archivo (%s)", link+r.SumSuffix, fields[1])
	}
	return Resolved{URL: link, FileName: name, SHA256: strings.ToLower(fields[0])}, nil
}

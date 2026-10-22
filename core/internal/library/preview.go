// * Vista previa animada (un GIF) que el usuario le pone a una imagen de la biblioteca. Va en un archivo aparte dentro
// * del estado del Vault y no en library.json: la interfaz lee la lista cada pocos segundos y un GIF de varios MB no
// * debe viajar en cada lectura. La imagen del disco no se toca.
package library

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"rebootera/core/internal/vault"
)

// MaxPreview es el tamaño máximo (en bytes) de un GIF de vista previa.
const MaxPreview = 6 << 20

// gifPrefix es el comienzo de un GIF como data URL.
const gifPrefix = "data:image/gif;base64,"

// * previewFile es el archivo del GIF de una imagen (el nombre sale de un hash: nunca de texto de la interfaz).
func previewFile(rel string) string {
	sum := sha256.Sum256([]byte(rel))
	return filepath.Join(vault.StateDir(), "previews", hex.EncodeToString(sum[:8])+".gif")
}

// * previewURL es la dirección desde la que la interfaz pide el GIF; el número cambia al sustituirlo para no usar uno viejo.
func previewURL(rel string, version int64) string {
	return "/api/library/preview?path=" + url.QueryEscape(rel) + "&v=" + strconv.FormatInt(version, 10)
}

// * SetPreview guarda (o, con un texto vacío, quita) el GIF de vista previa de una imagen de la biblioteca.
// * El GIF llega como data URL base64 y se comprueba su tamaño y su cabecera.
func (l *Library) SetPreview(rel, dataURL string) error {
	var data []byte
	if dataURL != "" {
		encoded, ok := strings.CutPrefix(dataURL, gifPrefix)
		if !ok || !base64Only(encoded) {
			return errors.New("la vista previa no es un GIF válido")
		}
		var err error
		if data, err = base64.StdEncoding.DecodeString(encoded); err != nil {
			return errors.New("la vista previa no es un GIF válido")
		}
		if len(data) > MaxPreview {
			return errors.New("el GIF es demasiado grande (máximo 6 MB)")
		}
		if !bytes.HasPrefix(data, []byte("GIF87a")) && !bytes.HasPrefix(data, []byte("GIF89a")) {
			return errors.New("la vista previa no es un GIF válido")
		}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.load()
	m, ok := l.db[rel]
	if !ok {
		return errors.New("esa imagen no está en la biblioteca")
	}
	path := previewFile(rel)
	if data == nil {
		os.Remove(path)
		m.Preview = 0
	} else {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return err
		}
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, data, 0o640); err != nil {
			return err
		}
		if err := os.Rename(tmp, path); err != nil {
			return err
		}
		m.Preview = time.Now().UnixNano()
	}
	l.db[rel] = m
	l.invalidate()
	return l.save()
}

// * PreviewPath devuelve el archivo del GIF de una imagen, o un error si no tiene.
func (l *Library) PreviewPath(rel string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.load()
	if l.db[rel].Preview == 0 {
		return "", errors.New("esa imagen no tiene vista previa")
	}
	return previewFile(rel), nil
}

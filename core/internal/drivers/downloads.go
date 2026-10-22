// * Lo que el navegador de Rebootera OS descarga (drivers de la web del fabricante) pasa solo al USB: los .zip se
// * descomprimen en drivers/Descargas/<nombre>/ para que Rebootera lea sus INF como los demás, y el resto de archivos
// * (instaladores .exe, .msi, .cab...) se guardan tal cual para usarlos luego en Windows. La carpeta de descargas la
// * escribe la sesión gráfica (otro usuario) y la vacía Core: nunca se ejecuta nada de lo que hay en ella.
// ! Al descomprimir se rechaza cualquier ruta que salga de la carpeta de destino (zip slip), los enlaces simbólicos
// ! y los archivos que declaran un tamaño desmedido (bombas zip).
package drivers

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Límites de lo que se acepta al descomprimir un .zip de drivers.
const (
	// maxZipEntries es cuántos archivos como mucho trae un paquete de drivers.
	maxZipEntries = 20000
	// maxZipBytes es lo máximo que se descomprime de un .zip (2 GiB).
	maxZipBytes = 2 << 30
	// settleTime es lo que debe llevar sin cambiar un archivo para darlo por descargado.
	settleTime = 4 * time.Second
)

// downloadsFolder es la carpeta del USB donde quedan las descargas.
const downloadsFolder = "Descargas"

// unfinished son los sufijos de las descargas a medias (no se tocan hasta que terminan).
var unfinished = []string{".wkdownload", ".part", ".crdownload", ".tmp"}

// * ImportDownloads pasa al USB lo que hay en la carpeta de descargas: from es esa carpeta y to, la carpeta drivers/ del USB.
// * Devuelve los nombres de lo importado. Lo que está a medias se deja para después.
func ImportDownloads(from, to string) ([]string, error) {
	entries, err := os.ReadDir(from)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var imported []string
	var failures []string
	for _, entry := range entries {
		if !entry.Type().IsRegular() || isUnfinished(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil || time.Since(info.ModTime()) < settleTime {
			continue
		}
		source := filepath.Join(from, entry.Name())
		if err := importOne(source, entry.Name(), to); err != nil {
			failures = append(failures, entry.Name()+": "+err.Error())
		} else {
			imported = append(imported, entry.Name())
		}
		// * Un archivo que no se puede usar se quita igual: si no, se intentaría cada pocos segundos para siempre.
		os.Remove(source)
	}
	if len(failures) > 0 {
		return imported, fmt.Errorf("no se pudo importar: %s", strings.Join(failures, "; "))
	}
	return imported, nil
}

// * isUnfinished indica si el nombre es de una descarga a medias.
func isUnfinished(name string) bool {
	lower := strings.ToLower(name)
	for _, suffix := range unfinished {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// * importOne importa un archivo: un .zip se descomprime y cualquier otro se copia entero.
func importOne(source, name, to string) error {
	base := filepath.Join(to, downloadsFolder)
	if strings.EqualFold(filepath.Ext(name), ".zip") {
		folder := uniquePath(base, strings.TrimSuffix(name, filepath.Ext(name)))
		if err := unzip(source, folder); err != nil {
			os.RemoveAll(folder)
			return err
		}
		return nil
	}
	return copyDownload(source, uniquePath(base, name))
}

// * uniquePath devuelve base/name, o base/name (2), (3)... si ya existe: una descarga nueva nunca pisa otra.
func uniquePath(base, name string) string {
	candidate := filepath.Join(base, name)
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for n := 2; ; n++ {
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
		candidate = filepath.Join(base, fmt.Sprintf("%s (%d)%s", stem, n, ext))
	}
}

// * copyDownload copia un archivo normal a otro sitio (creando las carpetas que falten).
func copyDownload(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(target)
		return err
	}
	return out.Close()
}

// * unzip descomprime un .zip en la carpeta de destino con los límites de arriba.
func unzip(source, folder string) error {
	reader, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer reader.Close()
	if len(reader.File) > maxZipEntries {
		return errors.New("el .zip trae demasiados archivos")
	}
	var total uint64
	for _, file := range reader.File {
		total += file.UncompressedSize64
	}
	if total > maxZipBytes {
		return errors.New("el .zip es demasiado grande al descomprimirlo")
	}
	if err := os.MkdirAll(folder, 0o750); err != nil {
		return err
	}
	var written int64
	for _, file := range reader.File {
		target, err := safeJoin(folder, file.Name)
		if err != nil {
			return err
		}
		mode := file.Mode()
		switch {
		case mode.IsDir() || strings.HasSuffix(file.Name, "/"):
			if err := os.MkdirAll(target, 0o750); err != nil {
				return err
			}
		case mode.IsRegular():
			n, err := extractFile(file, target)
			if err != nil {
				return err
			}
			written += n
			if written > maxZipBytes {
				return errors.New("el .zip es demasiado grande al descomprimirlo")
			}
		default:
			// * Enlaces simbólicos y archivos especiales: nunca se crean.
		}
	}
	return nil
}

// * extractFile escribe un archivo del .zip sin pasar del tamaño máximo (aunque la cabecera mienta).
func extractFile(file *zip.File, target string) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return 0, err
	}
	in, err := file.Open()
	if err != nil {
		return 0, err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(out, io.LimitReader(in, maxZipBytes+1))
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	return n, err
}

// * safeJoin une una ruta del .zip con la carpeta de destino y rechaza lo que salga de ella.
func safeJoin(folder, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(name, "\\", "/")))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("ruta no permitida en el .zip: %q", name)
	}
	return filepath.Join(folder, clean), nil
}

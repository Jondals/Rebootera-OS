// * Contenido común de un USB de Rebootera (carpetas, configuración de Ventoy,
// * tema y README) y actualización de Rebootera OS en un USB existente.
// * Lo usan la creación de USB (Windows) y "Actualizar Rebootera OS".
package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// * usbFolders son las carpetas de un USB de Rebootera recién creado.
var usbFolders = []string{"rebootera/state", "isos/windows", "isos/linux", "isos/tools", "drivers", "tools", "files", "ventoy/theme"}

// * ignoredFolders llevan .ventoyignore para que Ventoy no busque imágenes en ellas.
var ignoredFolders = []string{"drivers", "tools", "files", "rebootera/state"}

// * WriteKit crea las carpetas y copia la configuración de Ventoy, el tema y
// * el README desde kit (ventoy/ventoy.json, ventoy/theme/*, README.txt).
func WriteKit(root string, kit fs.FS) error {
	for _, dir := range usbFolders {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0o755); err != nil {
			return err
		}
	}
	for _, dir := range ignoredFolders {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(dir), ".ventoyignore"), nil, 0o644); err != nil {
			return err
		}
	}
	copier := &kitCopier{root: root, kit: kit}
	return fs.WalkDir(kit, ".", copier.visit)
}

// kitCopier copia los archivos del kit al USB.
type kitCopier struct {
	root string
	kit  fs.FS
}

// * visit copia cada archivo del kit a su ruta en el USB.
func (c *kitCopier) visit(path string, entry fs.DirEntry, err error) error {
	if err != nil || entry.IsDir() {
		return err
	}
	data, err := fs.ReadFile(c.kit, path)
	if err != nil {
		return err
	}
	target := filepath.Join(c.root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o644)
}

// Progress recibe el avance de una copia larga (0 a 1).
type Progress func(fraction float64)

// * InstallSystem copia la ISO de Rebootera OS al USB conservando la versión
// * anterior como rebootera-os.previous.iso (entrada de rescate en el menú).
// ! La copia se escribe primero como .new y se comprueba su hash antes de
// ! sustituir nada: un corte a mitad no deja el USB sin sistema.
func InstallSystem(root, isoPath string, progress Progress) error {
	if !strings.EqualFold(filepath.Ext(isoPath), ".iso") {
		return errors.New("el archivo de Rebootera OS debe ser una imagen .iso")
	}
	dir := filepath.Join(root, "rebootera")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	current := filepath.Join(dir, "rebootera-os.iso")
	previous := filepath.Join(dir, "rebootera-os.previous.iso")
	staged := current + ".new"

	sourceHash, err := copyWithHash(isoPath, staged, progress)
	if err != nil {
		os.Remove(staged)
		return err
	}
	// * Se relee la copia del USB: así se detecta un USB que escribe mal.
	copiedHash, err := hashFile(staged)
	if err != nil || copiedHash != sourceHash {
		os.Remove(staged)
		return errors.New("la copia en el USB no coincide con el original: el USB podría estar dañado")
	}
	if _, err := os.Stat(current); err == nil {
		os.Remove(previous)
		if err := os.Rename(current, previous); err != nil {
			return err
		}
	}
	if err := os.Rename(staged, current); err != nil {
		return err
	}
	os.WriteFile(current+".sha256", []byte(sourceHash+"  rebootera-os.iso\n"), 0o644)
	return os.WriteFile(filepath.Join(dir, "VERSION"), []byte(filepath.Base(isoPath)+"\n"), 0o644)
}

// * copyWithHash copia un archivo calculando su SHA-256 e informando del avance.
func copyWithHash(src, dst string, progress Progress) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return "", err
	}
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	hasher := sha256.New()
	counter := &progressCounter{total: info.Size(), report: progress}
	if _, err := io.Copy(io.MultiWriter(out, hasher, counter), in); err != nil {
		out.Close()
		return "", fmt.Errorf("no se pudo copiar al USB: %w", err)
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// * hashFile calcula el SHA-256 de un archivo.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// progressCounter cuenta bytes copiados e informa del avance.
type progressCounter struct {
	total  int64
	done   int64
	report Progress
}

// * Write implementa io.Writer.
func (p *progressCounter) Write(b []byte) (int, error) {
	p.done += int64(len(b))
	if p.report != nil && p.total > 0 {
		p.report(float64(p.done) / float64(p.total))
	}
	return len(b), nil
}

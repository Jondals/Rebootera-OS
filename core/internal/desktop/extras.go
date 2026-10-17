// * Lo que se mete en el USB "del tirón" junto con Rebootera OS: imágenes
// * (por ejemplo una ISO de Windows 11 LTSC) y carpetas de drivers. Sirve igual
// * para un USB recién creado que para uno de Rebootera que ya existe, y no
// * toca nada de lo que el USB ya tenga.
// ! Todo se valida ANTES de borrar o modificar el USB: una ruta que no existe
// ! o una carpeta sin drivers se rechaza sin haber hecho nada.
package desktop

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"rebootera/core/internal/drivers"
)

// errBusy se devuelve si ya hay una creación o actualización en marcha.
var errBusy = errors.New("ya hay un USB en preparación")

// Image es una imagen del equipo que se copia a la carpeta isos/<categoría>.
type Image struct {
	Path     string `json:"path"`
	Category string `json:"category"`
}

// Extras son las imágenes y carpetas de drivers que se añaden al USB.
type Extras struct {
	Images     []Image  `json:"images"`
	DriverDirs []string `json:"driverDirs"`
}

// imageCategories son las carpetas de isos/ donde se pueden dejar imágenes.
var imageCategories = []string{"windows", "linux", "tools"}

// imageExtensions son los tipos de imagen que admite el menú de arranque.
var imageExtensions = []string{".iso", ".img", ".wim", ".vhd", ".vhdx"}

// skippedInDrivers no se copia de una carpeta de drivers: imágenes de disco que no son
// drivers (alguien elige una carpeta de Descargas) e instaladores que Rebootera nunca ejecuta.
var skippedInDrivers = append([]string{".exe", ".msi", ".zip", ".7z", ".rar"}, imageExtensions...)

// * copyable indica si un archivo de una carpeta de drivers se copia al USB.
func copyable(name string) bool {
	return !slices.Contains(skippedInDrivers, strings.ToLower(filepath.Ext(name)))
}

// * Empty indica que no hay nada que añadir.
func (e Extras) Empty() bool {
	return len(e.Images) == 0 && len(e.DriverDirs) == 0
}

// * Validate comprueba que cada imagen y cada carpeta de drivers sirven.
func (e Extras) Validate() error {
	seen := map[string]bool{}
	for _, image := range e.Images {
		name := filepath.Base(image.Path)
		if !slices.Contains(imageCategories, image.Category) {
			return fmt.Errorf("categoría no válida para %s: %q", name, image.Category)
		}
		if !slices.Contains(imageExtensions, strings.ToLower(filepath.Ext(name))) {
			return fmt.Errorf("%s no es una imagen de disco (.iso, .img, .wim, .vhd, .vhdx)", name)
		}
		info, err := os.Stat(image.Path)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("no se encuentra la imagen %s", image.Path)
		}
		key := strings.ToLower(image.Category + "/" + name)
		if seen[key] {
			return fmt.Errorf("%s está repetida", name)
		}
		seen[key] = true
	}
	folders := map[string]bool{}
	for _, dir := range e.DriverDirs {
		if err := ValidateDriverDir(dir); err != nil {
			return err
		}
		key := strings.ToLower(filepath.Base(dir))
		if folders[key] {
			return fmt.Errorf("hay dos carpetas de drivers con el nombre %s", filepath.Base(dir))
		}
		folders[key] = true
	}
	return nil
}

// * ValidateDriverDir exige una carpeta con al menos un paquete de drivers x64.
func ValidateDriverDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("no se encuentra la carpeta %s", dir)
	}
	if len(drivers.Scan(dir)) == 0 {
		return fmt.Errorf("la carpeta %s no tiene ningún paquete de drivers (.inf) para x64", filepath.Base(dir))
	}
	return nil
}

// * totalBytes suma el tamaño de lo que se va a copiar para repartir el avance.
func (e Extras) totalBytes() int64 {
	var total int64
	for _, image := range e.Images {
		if info, err := os.Stat(image.Path); err == nil {
			total += info.Size()
		}
	}
	counter := &sizeCounter{}
	for _, dir := range e.DriverDirs {
		filepath.WalkDir(dir, counter.visit)
	}
	return total + counter.total
}

// sizeCounter suma el tamaño de los archivos normales de un árbol.
type sizeCounter struct {
	total int64
}

// * visit suma un archivo (los errores de lectura se ignoran: solo es una estimación).
func (s *sizeCounter) visit(path string, entry fs.DirEntry, err error) error {
	if err == nil && entry.Type().IsRegular() && copyable(path) {
		if info, infoErr := entry.Info(); infoErr == nil {
			s.total += info.Size()
		}
	}
	return nil
}

// extrasCopier copia las imágenes y los drivers repartiendo el avance.
type extrasCopier struct {
	root  string
	total int64
	done  int64
	phase string
	item  string
}

// * ApplyExtras copia imágenes y drivers a root (la raíz de un USB de Rebootera).
func ApplyExtras(root string, extras Extras) error {
	if extras.Empty() {
		return nil
	}
	copier := &extrasCopier{root: root, total: max(extras.totalBytes(), 1)}
	for _, image := range extras.Images {
		copier.phase, copier.item = "copy-images", filepath.Base(image.Path)
		target := filepath.Join(root, "isos", image.Category, copier.item)
		if err := copier.file(image.Path, target); err != nil {
			return err
		}
	}
	for _, dir := range extras.DriverDirs {
		copier.phase = "copy-drivers"
		if err := copier.tree(dir, filepath.Join(root, "drivers", filepath.Base(dir))); err != nil {
			return err
		}
	}
	return nil
}

// * file copia un archivo a un .part y lo renombra al terminar, de modo que un
// * corte nunca deja una imagen a medias con su nombre definitivo.
func (c *extrasCopier) file(source, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	partial := target + ".part"
	out, err := os.Create(partial)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, io.TeeReader(in, c))
	if err == nil {
		err = out.Sync()
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		if written, statErr := os.Stat(partial); statErr != nil || written.Size() != info.Size() {
			err = errors.New("el tamaño de la copia no coincide con el original")
		}
	}
	if err != nil {
		os.Remove(partial)
		return fmt.Errorf("no se pudo copiar %s al USB: %w", filepath.Base(source), err)
	}
	os.Remove(target)
	return os.Rename(partial, target)
}

// * tree copia una carpeta de drivers entera (solo archivos normales).
func (c *extrasCopier) tree(source, target string) error {
	walk := &treeCopy{copier: c, source: source, target: target}
	return filepath.WalkDir(source, walk.visit)
}

// treeCopy copia cada entrada de un árbol de carpetas.
type treeCopy struct {
	copier         *extrasCopier
	source, target string
}

// * visit crea la carpeta o copia el archivo equivalente en el destino.
func (w *treeCopy) visit(path string, entry fs.DirEntry, err error) error {
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(w.source, path)
	if err != nil {
		return err
	}
	if entry.IsDir() {
		return os.MkdirAll(filepath.Join(w.target, rel), 0o755)
	}
	if !entry.Type().IsRegular() || !copyable(path) {
		return nil
	}
	w.copier.item = filepath.ToSlash(filepath.Join(filepath.Base(w.source), rel))
	return w.copier.file(path, filepath.Join(w.target, rel))
}

// * Write cuenta los bytes copiados e informa del avance (io.Writer del TeeReader).
func (c *extrasCopier) Write(b []byte) (int, error) {
	c.done += int64(len(b))
	setItem(c.phase, c.item, min(float64(c.done)/float64(c.total), 1))
	return len(b), nil
}

// * AddDrivers copia una carpeta de drivers al USB elegido (Asistente de drivers).
func AddDrivers(root, dir string) error {
	if err := ValidateDriverDir(dir); err != nil {
		return err
	}
	copier := &extrasCopier{root: root, total: max(Extras{DriverDirs: []string{dir}}.totalBytes(), 1), phase: "copy-drivers"}
	return copier.tree(dir, filepath.Join(root, "drivers", filepath.Base(dir)))
}

// * RemoveDrivers borra una carpeta de drivers de primer nivel del USB.
// ! Solo acepta el nombre de una carpeta directamente dentro de drivers/.
func RemoveDrivers(root, name string) error {
	if name == "" || name != filepath.Base(name) || name == "." || name == ".." {
		return errors.New("nombre de carpeta no válido")
	}
	target := filepath.Join(root, "drivers", name)
	info, err := os.Lstat(target)
	if err != nil || !info.IsDir() {
		return errors.New("esa carpeta de drivers no existe")
	}
	return os.RemoveAll(target)
}

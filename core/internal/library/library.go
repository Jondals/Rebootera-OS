// * Package library: biblioteca de imágenes del Vault (carpeta isos/). Lista
// * las imágenes arrancables, guarda sus metadatos (hash, origen, estado de
// * verificación) en rebootera/state/library.json y calcula sumas SHA-256 en
// * segundo plano con progreso.
// * Estados de verificación:
// *   unchecked  todavía no se ha calculado el hash
// *   hashing    calculándolo ahora
// *   verified   el hash coincide con el publicado por la fuente oficial
// *   imported   hash calculado, pero su procedencia no está verificada
// *   mismatch   el hash NO coincide con el esperado: la imagen está dañada
// ! Una imagen "mismatch" o incompleta nunca se ofrece para instalar.
package library

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"rebootera/core/internal/vault"
)

// Estados de verificación.
const (
	StateUnchecked = "unchecked"
	StateHashing   = "hashing"
	StateVerified  = "verified"
	StateImported  = "imported"
	StateMismatch  = "mismatch"
)

// * Extensiones que Ventoy puede arrancar y su tipo.
var kinds = map[string]string{".iso": "iso", ".img": "img", ".wim": "wim", ".vhd": "vhd", ".vhdx": "vhd", ".efi": "efi"}

// Item es una imagen de la biblioteca.
type Item struct {
	Path      string  `json:"path"`
	Name      string  `json:"name"`
	Category  string  `json:"category"`
	Kind      string  `json:"kind"`
	SizeBytes int64   `json:"sizeBytes"`
	Modified  string  `json:"modified"`
	SHA256    string  `json:"sha256"`
	Expected  string  `json:"expected,omitempty"`
	State     string  `json:"state"`
	Source    string  `json:"source"`
	Progress  float64 `json:"progress"`
	// BootStatus es la compatibilidad de arranque conocida si la imagen viene
	// del catálogo ("verified", "incompatible", "untested"); la rellena la API.
	BootStatus string `json:"bootStatus,omitempty"`
}

// meta es lo que se guarda de cada imagen entre arranques.
type meta struct {
	SizeBytes int64  `json:"sizeBytes"`
	ModUnix   int64  `json:"modUnix"`
	SHA256    string `json:"sha256"`
	Expected  string `json:"expected"`
	State     string `json:"state"`
	Source    string `json:"source"`
}

// Library guarda los metadatos y el progreso de los cálculos de hash.
type Library struct {
	mu       sync.Mutex
	db       map[string]meta
	progress map[string]float64
	loaded   bool
}

// * New crea una biblioteca vacía; los metadatos se cargan al primer uso.
func New() *Library {
	return &Library{db: map[string]meta{}, progress: map[string]float64{}}
}

// * Reset olvida los metadatos cargados (al cambiar de USB en el escritorio).
func (l *Library) Reset() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, m := range l.db {
		if m.State == StateHashing {
			return errors.New("espera a que termine la comprobación en curso")
		}
	}
	l.db = map[string]meta{}
	l.progress = map[string]float64{}
	l.loaded = false
	return nil
}

// * dbPath es el archivo de metadatos dentro del Vault.
func dbPath() string {
	return filepath.Join(vault.StateDir(), "library.json")
}

// * load lee los metadatos guardados (una vez por sesión).
func (l *Library) load() {
	if l.loaded {
		return
	}
	vault.ReadJSON(dbPath(), &l.db)
	if l.db == nil {
		l.db = map[string]meta{}
	}
	l.loaded = true
}

// * save guarda los metadatos (con el bloqueo ya tomado).
func (l *Library) save() error {
	return vault.WriteJSON(dbPath(), l.db)
}

// * List recorre isos/ y devuelve las imágenes con su estado.
func (l *Library) List() ([]Item, error) {
	if !vault.Available() {
		return nil, vault.ErrUnavailable
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.load()

	root := vault.IsosDir()
	items := []Item{}
	seen := map[string]bool{}
	walker := &scan{root: root, lib: l, items: &items, seen: seen}
	if err := filepath.WalkDir(root, walker.visit); err != nil {
		return nil, err
	}
	// * Se olvidan los metadatos de imágenes borradas desde fuera (por ejemplo, en Windows).
	changed := false
	for path := range l.db {
		if !seen[path] {
			delete(l.db, path)
			changed = true
		}
	}
	if changed {
		l.save()
	}
	slices.SortFunc(items, compareItems)
	return items, nil
}

// scan acumula imágenes mientras se recorre la carpeta.
type scan struct {
	root  string
	lib   *Library
	items *[]Item
	seen  map[string]bool
}

// * visit se llama con cada archivo y carpeta de isos/.
func (s *scan) visit(path string, entry fs.DirEntry, err error) error {
	if err != nil {
		return nil
	}
	name := entry.Name()
	if entry.IsDir() {
		// * Las descargas a medias (.partial) y las carpetas ocultas no son imágenes.
		if path != s.root && strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}
		return nil
	}
	kind, ok := kinds[strings.ToLower(filepath.Ext(name))]
	if !ok || strings.HasPrefix(name, ".") {
		return nil
	}
	info, err := entry.Info()
	if err != nil {
		return nil
	}
	rel, _ := filepath.Rel(vault.Root, path)
	rel = filepath.ToSlash(rel)
	s.seen[rel] = true

	m, known := s.lib.db[rel]
	// * Si el archivo cambió (tamaño o fecha), su hash anterior ya no vale.
	if !known || m.SizeBytes != info.Size() || m.ModUnix != info.ModTime().Unix() {
		source := "manual"
		if known {
			source = m.Source
		}
		m = meta{SizeBytes: info.Size(), ModUnix: info.ModTime().Unix(), State: StateUnchecked, Source: source, Expected: m.Expected}
		s.lib.db[rel] = m
		s.lib.save()
	}
	item := Item{
		Path:      rel,
		Name:      name,
		Category:  category(rel),
		Kind:      kind,
		SizeBytes: info.Size(),
		Modified:  info.ModTime().UTC().Format(time.RFC3339),
		SHA256:    m.SHA256,
		Expected:  m.Expected,
		State:     m.State,
		Source:    m.Source,
		Progress:  s.lib.progress[rel],
	}
	*s.items = append(*s.items, item)
	return nil
}

// * category deduce la categoría de la carpeta (isos/windows/... -> windows).
func category(rel string) string {
	parts := strings.Split(rel, "/")
	if len(parts) >= 3 {
		switch parts[1] {
		case "windows", "linux", "tools":
			return parts[1]
		}
	}
	return "other"
}

// * compareItems ordena por categoría y nombre.
func compareItems(a, b Item) int {
	if a.Category != b.Category {
		return strings.Compare(a.Category, b.Category)
	}
	return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
}

// * Register anota una imagen recién descargada o importada con su hash.
// * expected es el hash oficial (vacío si la procedencia no se puede verificar).
func (l *Library) Register(rel, sha, expected, source string) error {
	full := vault.Path(rel)
	info, err := os.Stat(full)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.load()
	l.db[rel] = meta{
		SizeBytes: info.Size(),
		ModUnix:   info.ModTime().Unix(),
		SHA256:    sha,
		Expected:  expected,
		State:     stateFor(sha, expected),
		Source:    source,
	}
	return l.save()
}

// * stateFor decide el estado de verificación a partir de los dos hashes.
func stateFor(sha, expected string) string {
	switch {
	case sha == "":
		return StateUnchecked
	case expected == "":
		return StateImported
	case strings.EqualFold(sha, expected):
		return StateVerified
	}
	return StateMismatch
}

// * Verify calcula en segundo plano el hash de una imagen.
func (l *Library) Verify(rel string) error {
	full, err := vault.Resolve(vault.IsosDir(), strings.TrimPrefix(rel, "isos/"))
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.load()
	m, ok := l.db[rel]
	if !ok {
		return fmt.Errorf("la imagen %s no está en la biblioteca", rel)
	}
	if m.State == StateHashing {
		return nil
	}
	m.State = StateHashing
	l.db[rel] = m
	l.progress[rel] = 0
	job := &hashJob{lib: l, rel: rel, full: full}
	go job.run()
	return nil
}

// hashJob calcula el hash de una imagen y guarda el resultado.
type hashJob struct {
	lib  *Library
	rel  string
	full string
}

// * run lee la imagen entera actualizando el progreso.
func (j *hashJob) run() {
	sha, err := HashFile(j.full, j.setProgress)
	j.lib.mu.Lock()
	defer j.lib.mu.Unlock()
	m := j.lib.db[j.rel]
	delete(j.lib.progress, j.rel)
	if err != nil {
		m.State = StateUnchecked
	} else {
		m.SHA256 = sha
		m.State = stateFor(sha, m.Expected)
	}
	j.lib.db[j.rel] = m
	j.lib.save()
}

// * setProgress guarda el avance del cálculo (0 a 1).
func (j *hashJob) setProgress(fraction float64) {
	j.lib.mu.Lock()
	j.lib.progress[j.rel] = fraction
	j.lib.mu.Unlock()
}

// * Delete borra una imagen de la biblioteca.
// ! Solo se pueden borrar archivos dentro de isos/.
func (l *Library) Delete(rel string) error {
	full, err := vault.Resolve(vault.IsosDir(), strings.TrimPrefix(rel, "isos/"))
	if err != nil {
		return err
	}
	if _, ok := kinds[strings.ToLower(filepath.Ext(full))]; !ok {
		return errors.New("solo se pueden borrar imágenes de la biblioteca")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.load()
	if l.db[rel].State == StateHashing {
		return errors.New("espera a que termine la comprobación de esta imagen")
	}
	if err := os.Remove(full); err != nil {
		return err
	}
	delete(l.db, rel)
	return l.save()
}

// * HashFile calcula el SHA-256 de un archivo e informa del progreso.
func HashFile(path string, onProgress func(float64)) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	hasher := sha256.New()
	counter := &progressWriter{total: info.Size(), report: onProgress}
	if _, err := io.Copy(io.MultiWriter(hasher, counter), f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// progressWriter cuenta bytes y avisa del progreso cada 1 %.
type progressWriter struct {
	total  int64
	done   int64
	last   float64
	report func(float64)
}

// * Write implementa io.Writer contando los bytes.
func (p *progressWriter) Write(b []byte) (int, error) {
	p.done += int64(len(b))
	if p.report != nil && p.total > 0 {
		fraction := float64(p.done) / float64(p.total)
		if fraction-p.last >= 0.01 || fraction >= 1 {
			p.last = fraction
			p.report(fraction)
		}
	}
	return len(b), nil
}

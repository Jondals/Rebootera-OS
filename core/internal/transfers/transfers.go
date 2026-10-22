// * Package transfers: cola de transferencias hacia la biblioteca del Vault.
// * Dos tipos de trabajo comparten la cola:
// *   download  descarga desde la fuente oficial resuelta por el catálogo.
// *             Se puede pausar y reanudar (cabecera Range), incluso tras un
// *             reinicio, y el SHA-256 se calcula mientras se descarga.
// *   import    copia una imagen desde otro disco montado en solo lectura.
// * Los datos se escriben en isos/.partial/ y solo pasan a la biblioteca al
// * terminar y comprobar su hash.
// ! Una imagen incompleta o con un hash distinto del oficial nunca llega a la
// ! biblioteca: se borra y el trabajo queda como fallido con el motivo.
package transfers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"rebootera/core/internal/catalog"
	"rebootera/core/internal/fastcopy"
	"rebootera/core/internal/library"
	"rebootera/core/internal/storage"
	"rebootera/core/internal/vault"
)

// Estados de un trabajo.
const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusVerifying = "verifying"
	StatusPaused    = "paused"
	StatusDone      = "done"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// Tipos de trabajo.
const (
	KindDownload = "download"
	KindImport   = "import"
	KindMakeISO  = "iso"
)

// * spaceMargin es el espacio que se deja siempre libre en el Vault (64 MB).
const spaceMargin = 64 << 20

// Códigos de error que la interfaz traduce.
const (
	CodeNetwork    = "network"
	CodeSpace      = "space"
	CodeChecksum   = "checksum"
	CodeSignature  = "signature"
	CodeSource     = "source"
	CodeWrite      = "write"
	CodeIncomplete = "incomplete"
)

// codedError es un error con un código que la interfaz puede traducir.
type codedError struct {
	code string
	err  error
}

// * Error implementa error.
func (e *codedError) Error() string {
	return e.err.Error()
}

// * coded envuelve un error con su código.
func coded(code string, err error) error {
	return &codedError{code: code, err: err}
}

// * codeOf extrae el código de un error, o "failed" si no tiene.
func codeOf(err error) string {
	var c *codedError
	if errors.As(err, &c) {
		return c.code
	}
	return "failed"
}

// Job es una transferencia en la cola.
type Job struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Name       string    `json:"name"`
	CatalogID  string    `json:"catalogId,omitempty"`
	URL        string    `json:"url,omitempty"`
	SourcePath string    `json:"sourcePath,omitempty"`
	FileName   string    `json:"fileName"`
	Category   string    `json:"category"`
	SHA256     string    `json:"sha256,omitempty"`
	SHA1       string    `json:"sha1,omitempty"`
	SignedBy   string    `json:"signedBy,omitempty"`
	TotalBytes int64     `json:"totalBytes"`
	DoneBytes  int64     `json:"doneBytes"`
	SpeedBps   int64     `json:"speedBps"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
	ErrorCode  string    `json:"errorCode,omitempty"`
	Created    time.Time `json:"created"`
	// * retryAt es cuándo se vuelve a intentar sola una descarga que falló por la red (no se guarda: al arrancar se reintenta ya).
	retryAt time.Time
}

// * networkRetry es lo que se espera entre reintentos automáticos de una descarga sin red.
const networkRetry = 20 * time.Second

// Manager gestiona la cola y su único trabajador.
type Manager struct {
	mu      sync.Mutex
	jobs    []*Job
	lib     *library.Library
	client  *http.Client
	wake    chan struct{}
	cancel  context.CancelFunc
	current string
	stop    string
	loaded  bool
	// Ajustes de ancho de banda y el limitador que reparte la velocidad entre las conexiones.
	setMu          sync.Mutex
	settings       Settings
	settingsLoaded bool
	limiter        limiter
}

// * New crea el gestor; el trabajador se arranca con Run.
func New(lib *library.Library) *Manager {
	return &Manager{
		lib:    lib,
		client: &http.Client{Timeout: 0, Transport: &http.Transport{ResponseHeaderTimeout: 30 * time.Second}},
		wake:   make(chan struct{}, 1),
	}
}

// * statePath es el archivo de la cola dentro del Vault.
func statePath() string {
	return filepath.Join(vault.StateDir(), "transfers.json")
}

// * load lee la cola guardada. Los trabajos que estaban en marcha cuando se
// * apagó el equipo vuelven a la cola para reanudarse.
func (m *Manager) load() {
	if m.loaded || !vault.Available() {
		return
	}
	vault.ReadJSON(statePath(), &m.jobs)
	for _, job := range m.jobs {
		// * Lo que estaba a medias (también lo que se cortó por falta de red) sigue solo al volver a abrir.
		if job.Status == StatusRunning || job.Status == StatusVerifying || (job.Status == StatusFailed && job.ErrorCode == CodeNetwork) {
			job.Status = StatusQueued
			job.Error = ""
			job.ErrorCode = ""
		}
		job.SpeedBps = 0
	}
	m.loaded = true
}

// * save guarda la cola (con el bloqueo tomado).
func (m *Manager) save() {
	if !vault.Available() {
		return
	}
	if err := vault.WriteJSON(statePath(), m.jobs); err != nil {
		log.Printf("transferencias: no se pudo guardar la cola: %v", err)
	}
}

// * List devuelve una copia de la cola.
func (m *Manager) List() []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.load()
	list := make([]Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		list = append(list, *job)
	}
	return list
}

// * Busy indica si hay alguna transferencia en marcha o esperando turno (no cuenta las pausadas ni las fallidas).
func (m *Manager) Busy() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.load()
	for _, job := range m.jobs {
		if job.Status == StatusRunning || job.Status == StatusVerifying || job.Status == StatusQueued {
			return true
		}
	}
	return false
}

// * Download añade a la cola la descarga de una entrada del catálogo.
func (m *Manager) Download(entryID string) (Job, error) {
	if !vault.Available() {
		return Job{}, vault.ErrUnavailable
	}
	entry, err := catalog.Find(entryID)
	if err != nil {
		return Job{}, err
	}
	resolved, err := catalog.Resolve(entry)
	if err != nil {
		return Job{}, err
	}
	job := &Job{
		Kind:       KindDownload,
		Name:       entry.Name + " " + entry.Version,
		CatalogID:  entry.ID,
		URL:        resolved.URL,
		FileName:   resolved.FileName,
		Category:   entry.Category,
		SHA256:     resolved.SHA256,
		SHA1:       resolved.SHA1,
		SignedBy:   resolved.SignedBy,
		TotalBytes: resolved.SizeBytes,
	}
	return m.enqueue(job)
}

// * Import añade a la cola la copia de una imagen desde un disco montado.
func (m *Manager) Import(sourcePath, category string) (Job, error) {
	if !vault.Available() {
		return Job{}, vault.ErrUnavailable
	}
	info, err := os.Stat(sourcePath)
	if err != nil || !info.Mode().IsRegular() {
		return Job{}, fmt.Errorf("no se puede leer %s", sourcePath)
	}
	if category != "windows" && category != "linux" && category != "tools" {
		category = "other"
	}
	job := &Job{
		Kind:       KindImport,
		Name:       filepath.Base(sourcePath),
		SourcePath: sourcePath,
		FileName:   filepath.Base(sourcePath),
		Category:   category,
		TotalBytes: info.Size(),
	}
	return m.enqueue(job)
}

// * enqueue valida y añade un trabajo, y despierta al trabajador.
func (m *Manager) enqueue(job *Job) (Job, error) {
	if strings.ContainsAny(job.FileName, `/\`) || strings.HasPrefix(job.FileName, ".") || job.FileName == "" {
		return Job{}, fmt.Errorf("nombre de archivo no válido: %q", job.FileName)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.load()
	for _, other := range m.jobs {
		active := other.Status == StatusQueued || other.Status == StatusRunning || other.Status == StatusPaused || other.Status == StatusVerifying
		if active && other.FileName == job.FileName {
			return Job{}, fmt.Errorf("%s ya está en la cola", job.FileName)
		}
	}
	if _, err := os.Stat(finalPath(job)); err == nil {
		return Job{}, fmt.Errorf("%s ya está en la biblioteca", job.FileName)
	}
	job.ID = strconv.FormatInt(time.Now().UnixNano(), 36)
	job.Status = StatusQueued
	job.Created = time.Now().UTC()
	m.jobs = append(m.jobs, job)
	m.save()
	m.signal()
	return *job, nil
}

// * signal despierta al trabajador sin bloquear.
func (m *Manager) signal() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// * Pause detiene un trabajo conservando lo descargado.
func (m *Manager) Pause(id string) error {
	return m.interrupt(id, StatusPaused)
}

// * Cancel detiene un trabajo y borra lo descargado.
func (m *Manager) Cancel(id string) error {
	return m.interrupt(id, StatusCancelled)
}

// * interrupt pone un trabajo en pausa o lo cancela.
func (m *Manager) interrupt(id, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.find(id)
	if job == nil {
		return errors.New("transferencia desconocida")
	}
	if job.Status == StatusDone {
		return errors.New("la transferencia ya ha terminado")
	}
	if m.current == id && m.cancel != nil {
		m.stop = status
		m.cancel()
		return nil
	}
	job.Status = status
	job.SpeedBps = 0
	if status == StatusCancelled {
		removePartial(job)
	}
	m.save()
	return nil
}

// * Resume vuelve a poner en cola un trabajo pausado o fallido.
func (m *Manager) Resume(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.find(id)
	if job == nil {
		return errors.New("transferencia desconocida")
	}
	if job.Status != StatusPaused && job.Status != StatusFailed {
		return nil
	}
	job.Status = StatusQueued
	job.Error = ""
	job.ErrorCode = ""
	m.save()
	m.signal()
	return nil
}

// * Reset olvida la cola cargada (al cambiar de USB en el escritorio). No se
// * permite con una transferencia en marcha.
func (m *Manager) Reset() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current != "" {
		return errors.New("espera a que termine la transferencia en curso")
	}
	m.jobs = nil
	m.loaded = false
	return nil
}

// * Clear quita de la lista los trabajos terminados, fallidos o cancelados.
func (m *Manager) Clear() {
	m.ClearFinished(true)
}

// * ClearFinished quita de la lista los trabajos terminados o cancelados y, si failed es true, también los fallidos.
// * La interfaz lo llama sola con failed=false: un fallo se queda a la vista para poder reintentarlo.
func (m *Manager) ClearFinished(failed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs = slices.DeleteFunc(m.jobs, func(job *Job) bool {
		return isFinished(job) && (failed || job.Status != StatusFailed)
	})
	m.save()
}

// * isFinished indica si un trabajo ya no está activo.
func isFinished(job *Job) bool {
	return job.Status == StatusDone || job.Status == StatusFailed || job.Status == StatusCancelled
}

// * find busca un trabajo por id (con el bloqueo tomado).
func (m *Manager) find(id string) *Job {
	for _, job := range m.jobs {
		if job.ID == id {
			return job
		}
	}
	return nil
}

// * Run es el trabajador: procesa la cola de uno en uno para siempre.
func (m *Manager) Run() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		job := m.next()
		if job == nil {
			select {
			case <-m.wake:
			case <-ticker.C:
			}
			continue
		}
		m.process(job)
	}
}

// * next toma el primer trabajo en cola y lo marca en marcha.
func (m *Manager) next() *Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.load()
	now := time.Now()
	for _, job := range m.jobs {
		// * Una descarga cortada por la red se reanuda sola donde se quedó en cuanto vuelve la conexión.
		if job.Status == StatusFailed && job.ErrorCode == CodeNetwork && !job.retryAt.IsZero() && now.After(job.retryAt) {
			job.Status = StatusQueued
			job.Error = ""
			job.ErrorCode = ""
		}
	}
	for _, job := range m.jobs {
		if job.Status == StatusQueued {
			job.Status = StatusRunning
			job.Error = ""
			m.current = job.ID
			m.stop = ""
			m.save()
			return job
		}
	}
	return nil
}

// * process ejecuta un trabajo y deja su estado final.
func (m *Manager) process(job *Job) {
	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.cancel = cancel
	m.mu.Unlock()

	sha, err := m.transfer(ctx, job)
	// * Los enlaces temporales (Windows) caducan: si la fuente rechaza el enlace guardado, se pide uno nuevo y se continúa.
	if err != nil && ctx.Err() == nil && job.Kind == KindDownload && job.CatalogID != "" && codeOf(err) == CodeSource {
		if _, ok := m.refreshLink(job); ok {
			sha, err = m.transfer(ctx, job)
		}
	}
	cancel()

	m.mu.Lock()
	stop := m.stop
	m.current = ""
	m.cancel = nil
	job.SpeedBps = 0
	if stop == "" && err == nil {
		job.Status = StatusVerifying
		m.save()
	}
	m.mu.Unlock()

	// * La comprobación final y el movimiento a la biblioteca van sin bloqueo:
	// * Register usa su propio bloqueo y la interfaz sigue viendo la cola.
	if stop == "" && err == nil {
		err = m.finish(job, sha)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case stop != "":
		job.Status = stop
		if stop == StatusCancelled {
			removePartial(job)
		}
	case err != nil:
		job.Status = StatusFailed
		job.Error = err.Error()
		job.ErrorCode = codeOf(err)
		if job.ErrorCode == CodeNetwork {
			job.retryAt = time.Now().Add(networkRetry)
		}
		log.Printf("transferencias: %s falló: %v", job.FileName, err)
	default:
		job.Status = StatusDone
		log.Printf("transferencias: %s guardado en la biblioteca", job.FileName)
	}
	m.save()
}

// * refreshLink vuelve a pedir al catálogo el enlace de una descarga. Devuelve true si es distinto del que tenía.
func (m *Manager) refreshLink(job *Job) (string, bool) {
	entry, err := catalog.Find(job.CatalogID)
	if err != nil {
		return "", false
	}
	resolved, err := catalog.Resolve(entry)
	if err != nil || resolved.URL == "" || resolved.URL == job.URL {
		return "", false
	}
	m.mu.Lock()
	job.URL = resolved.URL
	m.save()
	m.mu.Unlock()
	return resolved.URL, true
}

// * finish comprueba el hash y mueve el archivo a la biblioteca.
func (m *Manager) finish(job *Job, sha string) error {
	partial := partialPath(job)
	if job.Kind == KindDownload && !(job.SHA256 == "" || strings.EqualFold(sha, job.SHA256)) {
		os.Remove(partial)
		return coded(CodeChecksum, fmt.Errorf("el SHA-256 no coincide con el oficial (descargado %s…, oficial %s…): la imagen se ha borrado", sha[:12], job.SHA256[:12]))
	}
	// * El Internet Archive solo publica SHA-1: se calcula con una pasada más y se compara.
	if job.Kind == KindDownload && job.SHA1 != "" {
		if got, err := sha1File(partial); err != nil || !strings.EqualFold(got, job.SHA1) {
			os.Remove(partial)
			return coded(CodeChecksum, fmt.Errorf("el SHA-1 no coincide con el que publica el Internet Archive: la imagen se ha borrado"))
		}
	}
	// * Si el proyecto firma cada ISO (Arch, Alpine), la firma OpenPGP se comprueba
	// * con la clave fijada; una ISO con firma incorrecta se borra como una con SHA-256 malo.
	if job.Kind == KindDownload {
		if entry, err := catalog.Find(job.CatalogID); err == nil && entry.Resolver.ISOSignature != nil {
			signer, err := catalog.VerifyISO(partial, job.URL, *entry.Resolver.ISOSignature)
			if err != nil {
				os.Remove(partial)
				return coded(CodeSignature, fmt.Errorf("la firma GPG de la imagen no es válida: %v: la imagen se ha borrado", err))
			}
			job.SignedBy = signer
		}
	}
	final := finalPath(job)
	if err := os.MkdirAll(filepath.Dir(final), 0o750); err != nil {
		return err
	}
	if err := os.Rename(partial, final); err != nil {
		return err
	}
	rel, _ := filepath.Rel(vault.Root, final)
	source := "import"
	switch job.Kind {
	case KindDownload:
		source = job.CatalogID
	case KindMakeISO:
		source = "installer"
	}
	return m.lib.Register(filepath.ToSlash(rel), sha, job.SHA256, source)
}

// * removePartial borra lo descargado de una transferencia y su mapa de bloques.
func removePartial(job *Job) {
	os.Remove(partialPath(job))
	os.Remove(partialPath(job) + ".map")
}

// * partialPath es donde se escribe una transferencia sin terminar.
func partialPath(job *Job) string {
	return filepath.Join(vault.PartialDir(), job.FileName+".part")
}

// * finalPath es donde queda la imagen en la biblioteca.
func finalPath(job *Job) string {
	return vault.Path("isos", job.Category, job.FileName)
}

// * transfer copia los datos al archivo parcial y devuelve su SHA-256.
func (m *Manager) transfer(ctx context.Context, job *Job) (string, error) {
	if err := os.MkdirAll(vault.PartialDir(), 0o750); err != nil {
		return "", err
	}
	os.WriteFile(filepath.Join(vault.PartialDir(), ".ventoyignore"), nil, 0o640)
	// * Un instalador en carpetas se empaqueta en una ISO nueva (ver makeiso.go): no se reanuda, se rehace.
	if job.Kind == KindMakeISO {
		return m.makeISO(ctx, job)
	}
	partial := partialPath(job)
	file, err := os.OpenFile(partial, os.O_CREATE|os.O_RDWR, 0o640)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// * Con varias conexiones el archivo se descarga por trozos (ver parallel.go).
	if job.Kind == KindDownload {
		if sum, done, err := m.tryParallel(ctx, job, file); done {
			return sum, err
		}
	}

	// * Al reanudar, el hash se pone al día leyendo lo ya descargado.
	hasher := sha256.New()
	offset, err := io.Copy(hasher, file)
	if err != nil {
		return "", err
	}
	if job.Kind == KindImport {
		return m.copyImport(ctx, job, file, hasher, offset)
	}
	return m.download(ctx, job, file, hasher, offset)
}

// * download descarga desde job.URL continuando en offset.
func (m *Manager) download(ctx context.Context, job *Job, file *os.File, hasher hash.Hash, offset int64) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", job.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", catalog.UserAgent)
	if offset > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return "", coded(CodeNetwork, fmt.Errorf("no se pudo conectar con la fuente: %w", err))
	}
	defer resp.Body.Close()
	if resp.Request.URL.Scheme != "https" {
		return "", coded(CodeSource, errors.New("la fuente redirigió a una conexión sin cifrar"))
	}
	switch resp.StatusCode {
	case http.StatusPartialContent:
	case http.StatusOK:
		// * El servidor no admite reanudar: se empieza de cero.
		if offset > 0 {
			if err := file.Truncate(0); err != nil {
				return "", err
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				return "", err
			}
			hasher.Reset()
			offset = 0
		}
	case http.StatusRequestedRangeNotSatisfiable:
		// * El archivo ya estaba completo.
		return hex.EncodeToString(hasher.Sum(nil)), nil
	default:
		return "", coded(CodeSource, fmt.Errorf("la fuente respondió %d", resp.StatusCode))
	}
	total := int64(0)
	if resp.ContentLength > 0 {
		total = offset + resp.ContentLength
		m.setTotal(job, total)
	}
	if err := checkSpace(total - offset); err != nil {
		return "", err
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return "", err
	}
	return m.pump(ctx, job, resp.Body, file, hasher, offset, total)
}

// * copyImport copia una imagen desde un disco montado continuando en offset.
func (m *Manager) copyImport(ctx context.Context, job *Job, file *os.File, hasher hash.Hash, offset int64) (string, error) {
	source, err := os.Open(job.SourcePath)
	if err != nil {
		return "", coded(CodeSource, fmt.Errorf("no se puede leer la imagen de origen: %w", err))
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return "", err
	}
	if err := checkSpace(info.Size() - offset); err != nil {
		return "", err
	}
	if _, err := source.Seek(offset, io.SeekStart); err != nil {
		return "", err
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return "", err
	}
	return m.pump(ctx, job, source, file, hasher, offset, info.Size())
}

// * pump copia datos en tubería (lee y escribe a la vez) actualizando el
// * progreso, la velocidad y el hash.
func (m *Manager) pump(ctx context.Context, job *Job, src io.Reader, dst *os.File, hasher hash.Hash, offset, total int64) (string, error) {
	now := time.Now()
	visitor := &pumpVisitor{m: m, job: job, dst: dst, hasher: hasher, done: offset, lastBytes: offset, lastTick: now, lastSave: now}
	_, err := fastcopy.Copy(ctx, dst, src, visitor)
	if err != nil {
		dst.Sync()
		var write *fastcopy.WriteError
		var read *fastcopy.ReadError
		switch {
		case ctx.Err() != nil:
			return "", ctx.Err()
		case errors.As(err, &write):
			return "", coded(CodeWrite, fmt.Errorf("no se pudo escribir en el USB: %w", write.Err))
		case errors.As(err, &read):
			return "", coded(CodeNetwork, fmt.Errorf("la transferencia se cortó: %w", read.Err))
		}
		return "", err
	}
	if err := dst.Sync(); err != nil {
		return "", err
	}
	if total > 0 && visitor.done != total {
		return "", coded(CodeIncomplete, fmt.Errorf("transferencia incompleta: %d de %d bytes", visitor.done, total))
	}
	m.setProgress(job, visitor.done, 0, true)
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// pumpVisitor calcula el hash y anota el avance de cada trozo ya escrito.
type pumpVisitor struct {
	m         *Manager
	job       *Job
	dst       *os.File
	hasher    hash.Hash
	done      int64
	lastBytes int64
	lastTick  time.Time
	lastSave  time.Time
}

// * Visit suma el trozo al hash y, cada segundo, publica el progreso; cada 5 s también guarda la cola.
func (v *pumpVisitor) Visit(chunk []byte) {
	v.hasher.Write(chunk)
	v.done += int64(len(chunk))
	// * Límite de velocidad: si se va más deprisa de lo permitido se espera; el resto de la tubería se
	// * llena y la lectura se frena sola.
	if wait := v.m.limiter.take(int64(len(chunk)), v.m.Settings().MaxBytesPerSecond); wait > 0 {
		time.Sleep(wait)
	}
	now := time.Now()
	if now.Sub(v.lastTick) < time.Second {
		return
	}
	speed := int64(float64(v.done-v.lastBytes) / now.Sub(v.lastTick).Seconds())
	save := now.Sub(v.lastSave) >= 5*time.Second
	v.m.setProgress(v.job, v.done, speed, save)
	if save {
		v.dst.Sync()
		v.lastSave = now
	}
	v.lastTick, v.lastBytes = now, v.done
}

// * setTotal anota el tamaño total conocido.
func (m *Manager) setTotal(job *Job, total int64) {
	m.mu.Lock()
	job.TotalBytes = total
	m.mu.Unlock()
}

// * setProgress anota el avance y, si se pide, guarda la cola.
func (m *Manager) setProgress(job *Job, done, speed int64, persist bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job.DoneBytes = done
	job.SpeedBps = speed
	if persist {
		m.save()
	}
}

// * checkSpace comprueba que cabe lo que falta por transferir.
func checkSpace(remaining int64) error {
	_, free := storage.FSUsage(vault.Root)
	if remaining > 0 && free > 0 && uint64(remaining)+spaceMargin > free {
		return coded(CodeSpace, fmt.Errorf("no hay espacio suficiente en el USB: faltan %d MB", (uint64(remaining)+spaceMargin-free)>>20))
	}
	return nil
}

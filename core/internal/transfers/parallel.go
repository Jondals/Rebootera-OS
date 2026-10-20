// * Descarga con varias conexiones a la vez. Muchos servidores limitan la velocidad de cada
// * conexión: pidiendo trozos distintos del archivo (cabecera Range) por varias conexiones se
// * aprovecha todo el ancho de banda. Los trozos se escriben en su sitio del archivo parcial; el
// * SHA-256 se calcula en orden a medida que los bloques están completos, y un archivo auxiliar
// * (.map) apunta qué bloques ya están descargados para poder reanudar tras una pausa o un corte.
// ! Si el servidor no admite Range, el archivo es pequeño o ya hay una descarga a medias de una sola
// ! conexión, se usa la descarga normal.
package transfers

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"rebootera/core/internal/catalog"
)

// Parámetros de la descarga por trozos (variables para poder ajustarlos en las pruebas).
var (
	// parallelBlock es el tamaño de cada bloque que se anota como descargado (1 MiB).
	parallelBlock int64 = 1 << 20
	// parallelChunkBlocks es cuántos bloques seguidos pide cada petición (8 MiB).
	parallelChunkBlocks = 8
	// parallelMin es el tamaño mínimo para repartir un archivo entre conexiones.
	parallelMin int64 = 32 << 20
)

// blockMap lleva la cuenta de qué bloques están descargados y cuáles reparte a las conexiones.
type blockMap struct {
	mu      sync.Mutex
	cond    *sync.Cond
	done    []bool
	claimed []bool
	count   int
	failed  error
}

// * newBlockMap crea el mapa de n bloques, todos pendientes.
func newBlockMap(n int) *blockMap {
	b := &blockMap{done: make([]bool, n), claimed: make([]bool, n)}
	b.cond = sync.NewCond(&b.mu)
	return b
}

// * claim reserva el siguiente tramo de bloques pendientes (hasta parallelChunkBlocks seguidos).
func (b *blockMap) claim() (first, n int, ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range b.done {
		if b.done[i] || b.claimed[i] {
			continue
		}
		first = i
		for i < len(b.done) && n < parallelChunkBlocks && !b.done[i] && !b.claimed[i] {
			b.claimed[i] = true
			n++
			i++
		}
		return first, n, true
	}
	return 0, 0, false
}

// * finish anota un tramo como descargado y avisa a quien espera esos bloques.
func (b *blockMap) finish(first, n int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := first; i < first+n; i++ {
		if !b.done[i] {
			b.done[i] = true
			b.count++
		}
	}
	b.cond.Broadcast()
}

// * release devuelve un tramo que no se pudo descargar para que otra conexión lo pida.
func (b *blockMap) release(first, n int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := first; i < first+n; i++ {
		b.claimed[i] = false
	}
}

// * fail anota el primer error y despierta a quien espere.
func (b *blockMap) fail(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.failed == nil {
		b.failed = err
	}
	b.cond.Broadcast()
}

// * wait espera a que el bloque k esté descargado, a un fallo o a que se cancele el contexto.
func (b *blockMap) wait(ctx context.Context, k int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for !b.done[k] {
		if b.failed != nil {
			return b.failed
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		b.cond.Wait()
	}
	return nil
}

// * downloaded devuelve cuántos bloques hay descargados.
func (b *blockMap) downloaded() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.count
}

// * snapshot serializa los bloques descargados (un bit por bloque).
func (b *blockMap) snapshot() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	bits := make([]byte, (len(b.done)+7)/8)
	for i, d := range b.done {
		if d {
			bits[i/8] |= 1 << (i % 8)
		}
	}
	return base64.StdEncoding.EncodeToString(bits)
}

// * restore carga los bloques descargados guardados (false si no encajan).
func (b *blockMap) restore(encoded string) bool {
	bits, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(bits) != (len(b.done)+7)/8 {
		return false
	}
	for i := range b.done {
		if bits[i/8]&(1<<(i%8)) != 0 {
			b.done[i] = true
			b.count++
		}
	}
	return true
}

// savedMap es lo que se guarda en el archivo .map.
type savedMap struct {
	Total  int64  `json:"total"`
	URL    string `json:"url"`
	Block  int64  `json:"block"`
	Blocks string `json:"blocks"`
}

// * probeRange pregunta al servidor si admite Range y cuánto mide el archivo.
func (m *Manager) probeRange(ctx context.Context, url string) (int64, bool) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, false
	}
	req.Header.Set("User-Agent", catalog.UserAgent)
	req.Header.Set("Range", "bytes=0-0")
	resp, err := m.client.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 16))
	if resp.StatusCode != http.StatusPartialContent || resp.Request.URL.Scheme != "https" {
		return 0, false
	}
	_, total, ok := strings.Cut(resp.Header.Get("Content-Range"), "/")
	size, err := strconv.ParseInt(total, 10, 64)
	if !ok || err != nil || size <= 0 {
		return 0, false
	}
	return size, true
}

// * tryParallel intenta la descarga por trozos. done es false si no procede y hay que descargar de una
// * sola conexión; si procede, devuelve el resultado completo.
func (m *Manager) tryParallel(ctx context.Context, job *Job, file *os.File) (sum string, done bool, err error) {
	connections := m.Settings().Connections
	mapPath := partialPath(job) + ".map"
	_, mapErr := os.Stat(mapPath)
	resuming := mapErr == nil
	info, statErr := file.Stat()
	if statErr != nil || (!resuming && (connections < 2 || info.Size() > 0)) {
		return "", false, nil
	}
	total, ok := m.probeRange(ctx, job.URL)
	if !ok || (!resuming && total < parallelMin) {
		if resuming {
			// * El servidor ya no admite Range o cambió: se empieza de cero con una conexión.
			os.Remove(mapPath)
			file.Truncate(0)
		}
		return "", false, nil
	}
	sum, err = m.downloadParallel(ctx, job, file, total, connections, mapPath, resuming)
	return sum, true, err
}

// * downloadParallel descarga el archivo entero repartido entre varias conexiones.
func (m *Manager) downloadParallel(ctx context.Context, job *Job, file *os.File, total int64, connections int, mapPath string, resuming bool) (string, error) {
	blocks := int((total + parallelBlock - 1) / parallelBlock)
	bm := newBlockMap(blocks)
	restored := false
	if resuming {
		var saved savedMap
		if data, err := os.ReadFile(mapPath); err == nil && json.Unmarshal(data, &saved) == nil &&
			saved.Total == total && sameFile(saved.URL, job.URL) && saved.Block == parallelBlock {
			restored = bm.restore(saved.Blocks)
		}
		if !restored {
			bm = newBlockMap(blocks)
		}
	}
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !restored || info.Size() != total {
		if err := checkSpace(total); err != nil {
			return "", err
		}
		if err := file.Truncate(0); err != nil {
			return "", err
		}
		if err := file.Truncate(total); err != nil {
			return "", coded(CodeWrite, fmt.Errorf("no se pudo reservar el espacio en el USB: %w", err))
		}
		bm = newBlockMap(blocks)
	}
	m.setTotal(job, total)

	parent := ctx
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var workers sync.WaitGroup
	for i := 0; i < max(connections, 1); i++ {
		workers.Add(1)
		go m.parallelWorker(ctx, cancel, &workers, job, file, bm, total)
	}
	stopTicker := make(chan struct{})
	tickerDone := make(chan struct{})
	go m.parallelProgress(job, file, bm, total, mapPath, stopTicker, tickerDone)

	sum, hashErr := m.hashInOrder(ctx, file, bm, total)
	if hashErr != nil {
		bm.fail(hashErr)
	}
	cancel()
	workers.Wait()
	close(stopTicker)
	<-tickerDone
	m.saveMap(job, file, bm, total, mapPath)

	if hashErr != nil {
		bm.mu.Lock()
		failure := bm.failed
		bm.mu.Unlock()
		switch {
		case failure != nil && failure != hashErr:
			return "", failure
		case parent.Err() != nil:
			return "", parent.Err()
		case failure != nil:
			return "", failure
		}
		return "", hashErr
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	os.Remove(mapPath)
	m.setProgress(job, total, 0, true)
	return sum, nil
}

// * hashInOrder calcula el SHA-256 leyendo del archivo cada bloque en cuanto está descargado.
func (m *Manager) hashInOrder(ctx context.Context, file *os.File, bm *blockMap, total int64) (string, error) {
	hasher := sha256.New()
	buffer := make([]byte, parallelBlock)
	for k := range bm.done {
		if err := bm.wait(ctx, k); err != nil {
			return "", err
		}
		start := int64(k) * parallelBlock
		size := min(parallelBlock, total-start)
		if _, err := file.ReadAt(buffer[:size], start); err != nil {
			return "", fmt.Errorf("no se pudo leer lo descargado: %w", err)
		}
		hasher.Write(buffer[:size])
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// * parallelWorker pide trozos hasta que no quedan, reintentando cada uno.
func (m *Manager) parallelWorker(ctx context.Context, cancel context.CancelFunc, workers *sync.WaitGroup, job *Job, file *os.File, bm *blockMap, total int64) {
	defer workers.Done()
	for ctx.Err() == nil {
		first, n, ok := bm.claim()
		if !ok {
			return
		}
		start := int64(first) * parallelBlock
		end := min(total, int64(first+n)*parallelBlock) - 1
		var err error
		for attempt := 1; attempt <= 3 && ctx.Err() == nil; attempt++ {
			if err = m.fetchRange(ctx, job.URL, file, start, end); err == nil {
				break
			}
			time.Sleep(time.Duration(attempt) * 700 * time.Millisecond)
		}
		if err != nil || ctx.Err() != nil {
			bm.release(first, n)
			if err != nil && ctx.Err() == nil {
				bm.fail(coded(CodeNetwork, fmt.Errorf("la transferencia se cortó: %w", err)))
				cancel()
			}
			return
		}
		bm.finish(first, n)
	}
}

// * fetchRange descarga los bytes [start, end] y los escribe en su sitio del archivo.
func (m *Manager) fetchRange(ctx context.Context, url string, file *os.File, start, end int64) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", catalog.UserAgent)
	req.Header.Set("Range", "bytes="+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10))
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("el servidor respondió %d a un Range", resp.StatusCode)
	}
	if resp.Request.URL.Scheme != "https" {
		return errors.New("la fuente redirigió a una conexión sin cifrar")
	}
	writer := &rangeWriter{ctx: ctx, m: m, file: file, offset: start}
	written, err := io.CopyBuffer(writer, struct{ io.Reader }{resp.Body}, make([]byte, 256<<10))
	if err != nil {
		return err
	}
	if written != end-start+1 {
		return fmt.Errorf("el trozo llegó incompleto (%d de %d bytes)", written, end-start+1)
	}
	return nil
}

// rangeWriter escribe en su sitio del archivo y respeta el límite de velocidad.
type rangeWriter struct {
	ctx    context.Context
	m      *Manager
	file   *os.File
	offset int64
}

// * Write respeta el límite de velocidad compartido y escribe en la posición que toca.
func (w *rangeWriter) Write(p []byte) (int, error) {
	if wait := w.m.limiter.take(int64(len(p)), w.m.Settings().MaxBytesPerSecond); wait > 0 {
		select {
		case <-time.After(wait):
		case <-w.ctx.Done():
			return 0, w.ctx.Err()
		}
	}
	n, err := w.file.WriteAt(p, w.offset)
	w.offset += int64(n)
	return n, err
}

// * parallelProgress publica el avance y la velocidad cada segundo y guarda el mapa cada 5 s.
func (m *Manager) parallelProgress(job *Job, file *os.File, bm *blockMap, total int64, mapPath string, stop, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	last := min(int64(bm.downloaded())*parallelBlock, total)
	lastSave := time.Now()
	for {
		select {
		case <-stop:
			return
		case now := <-ticker.C:
			current := min(int64(bm.downloaded())*parallelBlock, total)
			save := now.Sub(lastSave) >= 5*time.Second
			m.setProgress(job, current, current-last, save)
			last = current
			if save {
				m.saveMap(job, file, bm, total, mapPath)
				lastSave = now
			}
		}
	}
}

// * saveMap vacía el archivo y guarda qué bloques están descargados.
func (m *Manager) saveMap(job *Job, file *os.File, bm *blockMap, total int64, mapPath string) {
	file.Sync()
	data, err := json.Marshal(savedMap{Total: total, URL: job.URL, Block: parallelBlock, Blocks: bm.snapshot()})
	if err == nil {
		os.WriteFile(mapPath, data, 0o640)
	}
}

// * sameFile dice si dos enlaces apuntan al mismo archivo aunque cambie la parte temporal (los enlaces de Windows llevan una
// * firma que caduca y se renueva, pero el archivo y su tamaño son los mismos).
func sameFile(a, b string) bool {
	left, errLeft := url.Parse(a)
	right, errRight := url.Parse(b)
	if errLeft != nil || errRight != nil {
		return a == b
	}
	return left.Host == right.Host && left.Path == right.Path
}

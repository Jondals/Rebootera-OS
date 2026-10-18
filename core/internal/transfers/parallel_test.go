// * Pruebas de la descarga con varias conexiones y del límite de velocidad.
package transfers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"rebootera/core/internal/library"
	"rebootera/core/internal/testutil"
	"rebootera/core/internal/vault"
)

// slowBody sirve un archivo grande con Range y espera un poco en cada escritura, como un servidor que
// limita la velocidad de cada conexión.
type slowBody struct {
	data   []byte
	pause  time.Duration
	mu     sync.Mutex
	ranges []string
}

// slowWriter retrasa cada escritura.
type slowWriter struct {
	http.ResponseWriter
	pause time.Duration
}

// Write espera y escribe.
func (w *slowWriter) Write(p []byte) (int, error) {
	time.Sleep(w.pause)
	return w.ResponseWriter.Write(p)
}

// * ServeHTTP sirve los datos con soporte de Range y la espera por escritura.
func (s *slowBody) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.ranges = append(s.ranges, r.Header.Get("Range"))
	s.mu.Unlock()
	http.ServeContent(&slowWriter{ResponseWriter: w, pause: s.pause}, r, "big.iso", time.Unix(0, 0), bytes.NewReader(s.data))
}

// * bigSetup prepara un Vault falso, un servidor lento con 6 MiB y un gestor con trozos pequeños.
func bigSetup(t *testing.T, pause time.Duration) (*Manager, *slowBody, *httptest.Server, string) {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(root+"/rebootera/state", 0o755)
	t.Cleanup(testutil.Keep(&vault.Root).Restore)
	vault.Root = root
	t.Cleanup(testutil.Keep(&parallelMin).Restore)
	t.Cleanup(testutil.Keep(&parallelChunkBlocks).Restore)
	parallelMin = 1 << 20
	parallelChunkBlocks = 1
	data := make([]byte, 6<<20+12345)
	rand.New(rand.NewSource(7)).Read(data)
	body := &slowBody{data: data, pause: pause}
	server := httptest.NewTLSServer(body)
	t.Cleanup(server.Close)
	m := New(library.New())
	m.client = server.Client()
	sum := sha256.Sum256(data)
	return m, body, server, hex.EncodeToString(sum[:])
}

// * download pone en cola y procesa una descarga con las conexiones indicadas; devuelve el trabajo y lo que tardó.
func download(t *testing.T, m *Manager, server *httptest.Server, name, sum string, connections int) (Job, time.Duration) {
	t.Helper()
	if err := m.SetSettings(Settings{Connections: connections}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.enqueue(&Job{Kind: KindDownload, Name: name, CatalogID: name, URL: server.URL + "/big.iso", FileName: name + ".iso", Category: "linux", SHA256: sum}); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	return runOne(t, m), time.Since(start)
}

// * TestDescargaPorTrozos comprueba que varias conexiones dan el archivo exacto y que son más rápidas.
func TestDescargaPorTrozos(t *testing.T) {
	m, body, server, sum := bigSetup(t, 12*time.Millisecond)
	single, singleTime := download(t, m, server, "una", sum, 1)
	if single.Status != StatusDone {
		t.Fatalf("una conexión: %+v", single)
	}
	body.mu.Lock()
	body.ranges = nil
	body.mu.Unlock()
	multi, multiTime := download(t, m, server, "cuatro", sum, 4)
	if multi.Status != StatusDone || multi.DoneBytes != int64(len(body.data)) {
		t.Fatalf("cuatro conexiones: %+v", multi)
	}
	body.mu.Lock()
	chunks := 0
	for _, r := range body.ranges {
		if strings.HasPrefix(r, "bytes=") && r != "bytes=0-0" {
			chunks++
		}
	}
	body.mu.Unlock()
	if chunks < 6 {
		t.Fatalf("debería haber pedido varios trozos, pidió %d: %v", chunks, body.ranges)
	}
	if multiTime > singleTime*7/10 {
		t.Fatalf("cuatro conexiones (%v) deberían ser claramente más rápidas que una (%v)", multiTime, singleTime)
	}
	items, _ := m.lib.List()
	if len(items) != 2 || items[0].State != library.StateVerified {
		t.Fatalf("biblioteca: %+v", items)
	}
	if _, err := os.Stat(partialPath(&multi) + ".map"); !os.IsNotExist(err) {
		t.Fatal("el mapa de bloques debería haberse borrado")
	}
}

// * TestDescargaPorTrozosReanuda parte de un archivo a medias con su mapa y comprueba que solo se piden los trozos que faltan.
func TestDescargaPorTrozosReanuda(t *testing.T) {
	m, body, server, sum := bigSetup(t, 0)
	job := &Job{Kind: KindDownload, Name: "x", CatalogID: "x", URL: server.URL + "/big.iso", FileName: "reanuda.iso", Category: "linux", SHA256: sum}
	os.MkdirAll(vault.PartialDir(), 0o755)
	// * Los tres primeros bloques ya están descargados.
	partial := make([]byte, len(body.data))
	copy(partial, body.data[:3<<20])
	if err := os.WriteFile(partialPath(job), partial, 0o640); err != nil {
		t.Fatal(err)
	}
	blocks := (len(body.data) + int(parallelBlock) - 1) / int(parallelBlock)
	bm := newBlockMap(blocks)
	bm.finish(0, 3)
	data, _ := json.Marshal(savedMap{Total: int64(len(body.data)), URL: job.URL, Block: parallelBlock, Blocks: bm.snapshot()})
	os.WriteFile(partialPath(job)+".map", data, 0o640)
	if _, err := m.enqueue(job); err != nil {
		t.Fatal(err)
	}
	m.SetSettings(Settings{Connections: 3})
	done := runOne(t, m)
	if done.Status != StatusDone {
		t.Fatalf("trabajo: %+v", done)
	}
	for _, r := range body.ranges {
		if r == "bytes=0-1048575" || r == "bytes=1048576-2097151" || r == "bytes=2097152-3145727" {
			t.Fatalf("no debería volver a pedir bloques ya descargados: %v", body.ranges)
		}
	}
}

// * TestDescargaPorTrozosHashMalo rechaza una imagen cuyo hash no coincide.
func TestDescargaPorTrozosHashMalo(t *testing.T) {
	m, _, server, _ := bigSetup(t, 0)
	job, _ := download(t, m, server, "mala", strings.Repeat("0", 64), 4)
	if job.Status != StatusFailed || job.ErrorCode != CodeChecksum {
		t.Fatalf("debería fallar por el hash: %+v", job)
	}
	if _, err := os.Stat(partialPath(&job)); !os.IsNotExist(err) {
		t.Fatal("el archivo parcial debería haberse borrado")
	}
}

// plainBody sirve el archivo entero sin admitir Range.
type plainBody struct{ data []byte }

// * ServeHTTP responde siempre con el archivo completo.
func (p *plainBody) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Length", strconv.Itoa(len(p.data)))
	w.Write(p.data)
}

// * TestSinRangeUsaUnaConexion: un servidor que no admite Range sigue funcionando con la descarga normal.
func TestSinRangeUsaUnaConexion(t *testing.T) {
	m, body, _, sum := bigSetup(t, 0)
	plain := httptest.NewTLSServer(&plainBody{data: body.data})
	defer plain.Close()
	m.client = plain.Client()
	job, _ := download(t, m, plain, "plano", sum, 4)
	if job.Status != StatusDone {
		t.Fatalf("trabajo: %+v", job)
	}
}

// * TestAjustes valida los límites y que se guardan en el Vault.
func TestAjustes(t *testing.T) {
	m, _, _, _ := bigSetup(t, 0)
	if m.Settings().Connections != defaultConnections || m.Settings().MaxBytesPerSecond != 0 {
		t.Fatalf("valores por defecto: %+v", m.Settings())
	}
	for _, bad := range []Settings{{Connections: 0}, {Connections: 9}, {Connections: 2, MaxBytesPerSecond: -1}, {Connections: 2, MaxBytesPerSecond: maxRate + 1}} {
		if err := m.SetSettings(bad); err == nil {
			t.Fatalf("%+v debería rechazarse", bad)
		}
	}
	if err := m.SetSettings(Settings{Connections: 6, MaxBytesPerSecond: 5 << 20}); err != nil {
		t.Fatal(err)
	}
	again := New(library.New())
	if got := again.Settings(); got.Connections != 6 || got.MaxBytesPerSecond != 5<<20 {
		t.Fatalf("no se guardaron: %+v", got)
	}
}

// * TestLimitador comprueba la cubeta de fichas con horas inventadas.
func TestLimitador(t *testing.T) {
	var l limiter
	start := time.Unix(1000, 0)
	const rate = 1 << 20
	if wait := l.takeAt(start, 100, 0); wait != 0 {
		t.Fatal("sin límite no se espera")
	}
	// * La primera ráfaga de un segundo pasa sin esperar.
	if wait := l.takeAt(start, rate, rate); wait != 0 {
		t.Fatalf("la ráfaga inicial no debería esperar: %v", wait)
	}
	// * Pasado eso, cada MiB cuesta un segundo.
	if wait := l.takeAt(start, rate, rate); wait < 990*time.Millisecond || wait > 1010*time.Millisecond {
		t.Fatalf("un MiB más debería esperar ~1 s: %v", wait)
	}
	// * Una pausa larga recupera fichas, pero nunca más de un segundo de ráfaga.
	if wait := l.takeAt(start.Add(time.Hour), rate, rate); wait != 0 {
		t.Fatalf("tras una pausa se puede gastar un segundo: %v", wait)
	}
	if wait := l.takeAt(start.Add(time.Hour), rate, rate); wait < 990*time.Millisecond {
		t.Fatalf("la ráfaga no se acumula: %v", wait)
	}
}

// * Pruebas de la descarga de la última ISO con un servidor HTTPS local.
package release

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"rebootera/core/internal/testutil"
)

// fakeRelease publica una "release" falsa: la ISO y su suma.
type fakeRelease struct {
	iso  []byte
	hash string
}

// * ServeHTTP sirve la ISO y su archivo .sha256.
func (f *fakeRelease) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/" + ISOName + ".sha256":
		w.Write([]byte(f.hash + "  " + ISOName + "\n"))
	case "/" + ISOName:
		http.ServeContent(w, r, ISOName, time.Time{}, bytes.NewReader(f.iso))
	default:
		http.NotFound(w, r)
	}
}

// * publish levanta el servidor y apunta el paquete a él.
func publish(t *testing.T, iso []byte, wrongHash bool) {
	t.Helper()
	sum := sha256.Sum256(iso)
	hash := hex.EncodeToString(sum[:])
	if wrongHash {
		hash = strings.Repeat("0", 64)
	}
	srv := httptest.NewTLSServer(&fakeRelease{iso: iso, hash: hash})
	t.Cleanup(srv.Close)
	t.Cleanup(testutil.Keep(&BaseURL).Restore)
	t.Cleanup(testutil.Keep(&Client).Restore)
	BaseURL = srv.URL + "/"
	Client = srv.Client()
}

// * TestFetch descarga, verifica y reutiliza la copia ya descargada.
func TestFetch(t *testing.T) {
	iso := []byte(strings.Repeat("rebootera", 100000))
	publish(t, iso, false)
	dir := t.TempDir()
	progress := &testutil.Recorder{}
	path, err := Fetch(dir, progress.Record, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(iso) || progress.Last != 1 || filepath.Base(path) != ISOName {
		t.Fatalf("descarga: %q progreso %v", path, progress.Last)
	}
	if _, err := os.Stat(path + ".part"); err == nil {
		t.Fatal("no debe quedar el archivo parcial")
	}
	if again, err := Fetch(dir, nil, nil); err != nil || again != path {
		t.Fatalf("reutilizar: %v", err)
	}
}

// * TestFetchHashIncorrecto borra la descarga cuyo SHA-256 no coincide.
func TestFetchHashIncorrecto(t *testing.T) {
	publish(t, []byte("contenido"), true)
	dir := t.TempDir()
	if _, err := Fetch(dir, nil, nil); err == nil || !strings.Contains(err.Error(), "no coincide") {
		t.Fatalf("debería rechazar una suma incorrecta: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("no debe quedar nada tras una descarga rechazada: %v", entries)
	}
}

// * TestSinRelease explica que todavía no hay versión publicada.
func TestSinRelease(t *testing.T) {
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	t.Cleanup(testutil.Keep(&BaseURL).Restore)
	t.Cleanup(testutil.Keep(&Client).Restore)
	BaseURL, Client = srv.URL+"/", srv.Client()
	if _, err := Fetch(t.TempDir(), nil, nil); err == nil || !strings.Contains(err.Error(), "todavía no hay") {
		t.Fatalf("mensaje sin release: %v", err)
	}
}

// * TestSoloHTTPS rechaza descargas sin cifrar.
func TestSoloHTTPS(t *testing.T) {
	t.Cleanup(testutil.Keep(&BaseURL).Restore)
	BaseURL = "http://example.com/"
	if _, err := Fetch(t.TempDir(), nil, nil); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("debería exigir HTTPS: %v", err)
	}
}

// * TestFetchToReanuda continúa un archivo parcial en vez de empezar de cero y deja el resultado exacto.
func TestFetchToReanuda(t *testing.T) {
	iso := []byte(strings.Repeat("reanudar", 50000))
	publish(t, iso, false)
	final := filepath.Join(t.TempDir(), "rebootera-os.iso.new")
	if err := os.WriteFile(final+".part", iso[:123457], 0o644); err != nil {
		t.Fatal(err)
	}
	sum, err := FetchTo(final, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(final)
	want := sha256.Sum256(iso)
	if string(got) != string(iso) || sum != hex.EncodeToString(want[:]) {
		t.Fatalf("reanudar: %d bytes, suma %s", len(got), sum)
	}
	// * Un parcial corrupto no pasa la comprobación: se borra y el siguiente intento empieza limpio.
	bad := append([]byte("x"), iso[1:2000]...)
	if err := os.WriteFile(final+".part", bad, 0o644); err != nil {
		t.Fatal(err)
	}
	os.Remove(final)
	if _, err := FetchTo(final, nil, nil); err == nil {
		t.Fatal("un parcial corrupto debería fallar")
	}
	if _, err := os.Stat(final + ".part"); err == nil {
		t.Fatal("el parcial corrupto debería haberse borrado")
	}
	if _, err := FetchTo(final, nil, nil); err != nil {
		t.Fatalf("tras borrar el parcial debe descargar entero: %v", err)
	}
}

// slowRelease sirve la ISO a trozos pequeños y despacio, para poder pausar y cancelar a mitad.
type slowRelease struct {
	fakeRelease
	started chan struct{}
	once    sync.Once
}

// * ServeHTTP sirve la ISO despacio y avisa cuando empieza a enviar datos.
func (s *slowRelease) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/"+ISOName {
		s.fakeRelease.ServeHTTP(w, r)
		return
	}
	s.once.Do(func() { close(s.started) })
	http.ServeContent(&slowWriter{ResponseWriter: w, r: r}, r, ISOName, time.Time{}, bytes.NewReader(s.iso))
}

// slowWriter escribe de 64 KiB en 64 KiB con una pausa, y se rinde si el cliente se va.
type slowWriter struct {
	http.ResponseWriter
	r *http.Request
}

// * Write trocea la escritura y espera entre trozos.
func (w *slowWriter) Write(p []byte) (int, error) {
	written := 0
	for written < len(p) {
		end := min(written+64<<10, len(p))
		n, err := w.ResponseWriter.Write(p[written:end])
		written += n
		if err != nil {
			return written, err
		}
		w.ResponseWriter.(http.Flusher).Flush()
		select {
		case <-w.r.Context().Done():
			return written, w.r.Context().Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
	return written, nil
}

// * publishSlow publica una release lenta y devuelve su aviso de inicio.
func publishSlow(t *testing.T, iso []byte) chan struct{} {
	t.Helper()
	sum := sha256.Sum256(iso)
	slow := &slowRelease{fakeRelease: fakeRelease{iso: iso, hash: hex.EncodeToString(sum[:])}, started: make(chan struct{})}
	srv := httptest.NewTLSServer(slow)
	t.Cleanup(srv.Close)
	t.Cleanup(testutil.Keep(&BaseURL).Restore)
	t.Cleanup(testutil.Keep(&Client).Restore)
	BaseURL, Client = srv.URL+"/", srv.Client()
	return slow.started
}

// * waitFor espera a que se cumpla una condición.
func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("se agotó la espera de: %s", what)
}

// * TestFetchPausaYReanuda: pausada, la descarga se detiene sin avanzar; reanudada, termina con el hash exacto.
func TestFetchPausaYReanuda(t *testing.T) {
	iso := []byte(strings.Repeat("pausa", 200000))
	started := publishSlow(t, iso)
	final := filepath.Join(t.TempDir(), "rebootera-os.iso.new")
	ctl := NewControl()
	var last atomic.Int64
	progress := func(fraction float64) { last.Store(int64(fraction * 1e6)) }
	type result struct {
		sum string
		err error
	}
	finished := make(chan result, 1)
	go func() {
		sum, err := FetchTo(final, progress, ctl)
		finished <- result{sum, err}
	}()
	<-started
	waitFor(t, "algo de progreso", func() bool { return last.Load() > 0 })
	ctl.Pause()
	time.Sleep(200 * time.Millisecond)
	frozen := last.Load()
	time.Sleep(300 * time.Millisecond)
	if last.Load() != frozen {
		t.Fatalf("en pausa no debe avanzar: %d -> %d", frozen, last.Load())
	}
	select {
	case r := <-finished:
		t.Fatalf("no debe terminar en pausa: %v", r.err)
	default:
	}
	if _, err := os.Stat(final + ".part"); err != nil {
		t.Fatalf("en pausa se conserva lo descargado: %v", err)
	}
	ctl.Resume()
	r := <-finished
	if r.err != nil {
		t.Fatal(r.err)
	}
	got, _ := os.ReadFile(final)
	want := sha256.Sum256(iso)
	if string(got) != string(iso) || r.sum != hex.EncodeToString(want[:]) {
		t.Fatalf("tras pausar y reanudar: %d bytes, suma %s", len(got), r.sum)
	}
}

// * TestFetchCancela: cancelar, incluso en pausa, devuelve ErrCancelled y borra lo descargado.
func TestFetchCancela(t *testing.T) {
	for _, paused := range []bool{false, true} {
		iso := []byte(strings.Repeat("cancela", 200000))
		started := publishSlow(t, iso)
		final := filepath.Join(t.TempDir(), "rebootera-os.iso.new")
		ctl := NewControl()
		finished := make(chan error, 1)
		go func() {
			_, err := FetchTo(final, nil, ctl)
			finished <- err
		}()
		<-started
		time.Sleep(50 * time.Millisecond)
		if paused {
			ctl.Pause()
			time.Sleep(100 * time.Millisecond)
		}
		ctl.Cancel()
		select {
		case err := <-finished:
			if !errors.Is(err, ErrCancelled) {
				t.Fatalf("pausada=%v: debería devolver ErrCancelled: %v", paused, err)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("pausada=%v: la cancelación no terminó", paused)
		}
		if _, err := os.Stat(final + ".part"); err == nil {
			t.Fatalf("pausada=%v: cancelar debe borrar lo descargado", paused)
		}
	}
}

// * Pruebas de la descarga de la última ISO con un servidor HTTPS local.
package release

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	path, err := Fetch(dir, progress.Record)
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
	if again, err := Fetch(dir, nil); err != nil || again != path {
		t.Fatalf("reutilizar: %v", err)
	}
}

// * TestFetchHashIncorrecto borra la descarga cuyo SHA-256 no coincide.
func TestFetchHashIncorrecto(t *testing.T) {
	publish(t, []byte("contenido"), true)
	dir := t.TempDir()
	if _, err := Fetch(dir, nil); err == nil || !strings.Contains(err.Error(), "no coincide") {
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
	if _, err := Fetch(t.TempDir(), nil); err == nil || !strings.Contains(err.Error(), "todavía no hay") {
		t.Fatalf("mensaje sin release: %v", err)
	}
}

// * TestSoloHTTPS rechaza descargas sin cifrar.
func TestSoloHTTPS(t *testing.T) {
	t.Cleanup(testutil.Keep(&BaseURL).Restore)
	BaseURL = "http://example.com/"
	if _, err := Fetch(t.TempDir(), nil); err == nil || !strings.Contains(err.Error(), "HTTPS") {
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
	sum, err := FetchTo(final, nil)
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
	if _, err := FetchTo(final, nil); err == nil {
		t.Fatal("un parcial corrupto debería fallar")
	}
	if _, err := os.Stat(final + ".part"); err == nil {
		t.Fatal("el parcial corrupto debería haberse borrado")
	}
	if _, err := FetchTo(final, nil); err != nil {
		t.Fatalf("tras borrar el parcial debe descargar entero: %v", err)
	}
}

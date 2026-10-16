// * Pruebas de la cola de transferencias contra un servidor HTTPS local que
// * admite reanudación (Range), con un Vault falso en una carpeta temporal.
package transfers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"rebootera/core/internal/library"
	"rebootera/core/internal/testutil"
	"rebootera/core/internal/vault"
)

// payload es la "imagen" que sirve el servidor de pruebas (3 MB).
var payload = bytes.Repeat([]byte("rebootera-iso-"), 3<<20/14)

// * payloadHash devuelve el SHA-256 de payload.
func payloadHash() string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// rangeLog guarda las cabeceras Range recibidas.
type rangeLog struct {
	ranges []string
}

// * ServeHTTP sirve payload con soporte de Range.
func (r *rangeLog) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.ranges = append(r.ranges, req.Header.Get("Range"))
	http.ServeContent(w, req, "test.iso", time.Unix(0, 0), bytes.NewReader(payload))
}

// * setup crea un Vault falso, un servidor y un gestor que confía en él.
func setup(t *testing.T) (*Manager, *library.Library, *httptest.Server, *rangeLog) {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "rebootera", "state"), 0o755)
	t.Cleanup(testutil.Keep(&vault.Root).Restore)
	vault.Root = root

	log := &rangeLog{}
	server := httptest.NewTLSServer(log)
	t.Cleanup(server.Close)
	lib := library.New()
	m := New(lib)
	m.client = server.Client()
	return m, lib, server, log
}

// * runOne procesa el siguiente trabajo de la cola.
func runOne(t *testing.T, m *Manager) Job {
	t.Helper()
	job := m.next()
	if job == nil {
		t.Fatal("no había trabajo en cola")
	}
	m.process(job)
	return *job
}

// * TestDescargaVerificada comprueba el camino completo hasta la biblioteca.
func TestDescargaVerificada(t *testing.T) {
	m, lib, server, _ := setup(t)
	_, err := m.enqueue(&Job{Kind: KindDownload, Name: "Test", CatalogID: "test", URL: server.URL + "/test.iso", FileName: "test.iso", Category: "linux", SHA256: payloadHash()})
	if err != nil {
		t.Fatal(err)
	}
	job := runOne(t, m)
	if job.Status != StatusDone || job.DoneBytes != int64(len(payload)) {
		t.Fatalf("trabajo: %+v", job)
	}
	items, _ := lib.List()
	if len(items) != 1 || items[0].State != library.StateVerified || items[0].Path != "isos/linux/test.iso" || items[0].Source != "test" {
		t.Fatalf("biblioteca: %+v", items)
	}
	if _, err := os.Stat(partialPath(&job)); !os.IsNotExist(err) {
		t.Fatal("el archivo parcial debería haber desaparecido")
	}
}

// * TestHashIncorrecto comprueba que una imagen alterada nunca llega a la biblioteca.
func TestHashIncorrecto(t *testing.T) {
	m, lib, server, _ := setup(t)
	m.enqueue(&Job{Kind: KindDownload, Name: "Test", URL: server.URL + "/test.iso", FileName: "bad.iso", Category: "linux", SHA256: strings.Repeat("0", 64)})
	job := runOne(t, m)
	if job.Status != StatusFailed || job.ErrorCode != CodeChecksum {
		t.Fatalf("debería fallar por el hash: %+v", job)
	}
	items, _ := lib.List()
	if len(items) != 0 {
		t.Fatalf("la biblioteca debería estar vacía: %+v", items)
	}
	if _, err := os.Stat(partialPath(&job)); !os.IsNotExist(err) {
		t.Fatal("el archivo parcial debería haberse borrado")
	}
}

// * TestReanudar comprueba que una descarga a medias continúa con Range.
func TestReanudar(t *testing.T) {
	m, lib, server, log := setup(t)
	job := &Job{Kind: KindDownload, Name: "Test", URL: server.URL + "/test.iso", FileName: "resume.iso", Category: "tools", SHA256: payloadHash()}
	os.MkdirAll(vault.PartialDir(), 0o755)
	half := len(payload) / 2
	os.WriteFile(partialPath(job), payload[:half], 0o640)
	if _, err := m.enqueue(job); err != nil {
		t.Fatal(err)
	}
	done := runOne(t, m)
	if done.Status != StatusDone {
		t.Fatalf("trabajo: %+v", done)
	}
	if len(log.ranges) != 1 || log.ranges[0] != "bytes="+strconv.Itoa(half)+"-" {
		t.Fatalf("esperaba una petición Range desde %d, obtuve %v", half, log.ranges)
	}
	items, _ := lib.List()
	if len(items) != 1 || items[0].State != library.StateVerified {
		t.Fatalf("biblioteca: %+v", items)
	}
}

// * TestImportar copia una imagen local y la marca como importada (sin verificar).
func TestImportar(t *testing.T) {
	m, lib, _, _ := setup(t)
	source := filepath.Join(t.TempDir(), "mine.iso")
	os.WriteFile(source, payload, 0o644)
	if _, err := m.Import(source, "windows"); err != nil {
		t.Fatal(err)
	}
	job := runOne(t, m)
	if job.Status != StatusDone {
		t.Fatalf("trabajo: %+v", job)
	}
	items, _ := lib.List()
	if len(items) != 1 || items[0].State != library.StateImported || items[0].SHA256 != payloadHash() || items[0].Category != "windows" {
		t.Fatalf("biblioteca: %+v", items)
	}
}

// * TestNombresPeligrosos rechaza nombres que salen de la carpeta.
func TestNombresPeligrosos(t *testing.T) {
	m, _, _, _ := setup(t)
	for _, name := range []string{"../evil.iso", "a/b.iso", `a\b.iso`, ".hidden.iso", ""} {
		if _, err := m.enqueue(&Job{Kind: KindDownload, FileName: name, Category: "linux"}); err == nil {
			t.Fatalf("%q debería rechazarse", name)
		}
	}
}

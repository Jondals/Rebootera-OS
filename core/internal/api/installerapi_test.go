// * Pruebas de los instaladores en carpetas, las familias del catálogo y el ancho de banda.
package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rebootera/core/internal/testutil"
	"rebootera/core/internal/vault"
)

// * fakeInstaller crea en dir una carpeta con el aspecto de un Windows extraído por Rufus.
func fakeInstaller(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{"bootmgr", "Boot/etfsboot.com", "EFI/Microsoft/Boot/efisys.bin", "sources/boot.wim", "sources/install.wim"} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("datos de prueba"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// postJSON hace una petición POST con sesión y cabecera anti-CSRF.
func postJSON(h http.Handler, target, body string) *httptest.ResponseRecorder {
	headers := map[string]string{"Content-Type": "application/json", csrfHeader: "1"}
	return do(h, "POST", target, "127.0.0.1:7080", true, body, headers)
}

// * TestInstaladoresEnCarpetas recorre buscar, inspeccionar y encolar la ISO, y rechaza lo que no es un instalador.
func TestInstaladoresEnCarpetas(t *testing.T) {
	media := t.TempDir()
	t.Cleanup(testutil.Keep(&MediaRoot).Restore)
	MediaRoot = media
	fakeInstaller(t, filepath.Join(media, "sdb1", "Win11"))
	os.MkdirAll(filepath.Join(media, "sdb1", "fotos"), 0o755)
	t.Cleanup(testutil.Keep(&vault.Root).Restore)
	vault.Root = t.TempDir()
	os.MkdirAll(filepath.Join(vault.Root, "rebootera", "state"), 0o755)
	os.MkdirAll(filepath.Join(vault.Root, "rebootera", "isos"), 0o755)
	h := newTestServer(t, &fakeAgent{})

	rec := do(h, "GET", "/api/installer/found", "127.0.0.1:7080", true, "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"rel":"Win11"`) || strings.Contains(rec.Body.String(), "fotos") {
		t.Fatalf("buscar: %d %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(h, "/api/installer/inspect", `{"device":"sdb1","path":"Win11"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"kind":"windows"`) || !strings.Contains(rec.Body.String(), `"biosBoot":"Boot/etfsboot.com"`) {
		t.Fatalf("inspeccionar: %d %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(h, "/api/transfers/iso", `{"device":"sdb1","path":"Win11"}`)
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"kind":"iso"`) || !strings.Contains(rec.Body.String(), `"fileName":"Win11.iso"`) {
		t.Fatalf("crear ISO: %d %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(h, "/api/transfers/iso", `{"device":"sdb1","path":"Win11"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("repetida debería dar 409: %d %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(h, "/api/transfers/iso", `{"device":"sdb1","path":"fotos"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("una carpeta cualquiera no es un instalador: %d %s", rec.Code, rec.Body.String())
	}
	for _, body := range []string{`{"device":"sdb1","path":"../../etc"}`, `{"path":"/etc"}`, `{"device":"","path":"Win11"}`} {
		if rec = postJSON(h, "/api/transfers/iso", body); rec.Code == http.StatusAccepted {
			t.Fatalf("%s no debería aceptarse", body)
		}
	}
}

// * TestFamiliasYAnchoDeBanda comprueba las rutas nuevas del Centro de descargas.
func TestFamiliasYAnchoDeBanda(t *testing.T) {
	t.Cleanup(testutil.Keep(&vault.Root).Restore)
	vault.Root = t.TempDir()
	os.MkdirAll(filepath.Join(vault.Root, "rebootera", "state"), 0o755)
	h := newTestServer(t, &fakeAgent{})
	rec := do(h, "GET", "/api/catalog/families", "127.0.0.1:7080", true, "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"ubuntu"`) {
		t.Fatalf("familias: %d %.200s", rec.Code, rec.Body.String())
	}
	rec = postJSON(h, "/api/transfers/settings", `{"connections":6,"maxBytesPerSecond":5242880}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("guardar ajustes: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(h, "GET", "/api/transfers/settings", "127.0.0.1:7080", true, "", nil)
	if !strings.Contains(rec.Body.String(), `"connections":6`) || !strings.Contains(rec.Body.String(), `5242880`) {
		t.Fatalf("leer ajustes: %s", rec.Body.String())
	}
	if rec = postJSON(h, "/api/transfers/settings", `{"connections":99}`); rec.Code == http.StatusOK {
		t.Fatal("99 conexiones deberían rechazarse")
	}
}

// * Pruebas del servidor de la interfaz.
package api

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"rebootera/core/internal/testutil"
	"rebootera/core/internal/vault"
)

// * TestUIIndexSinVault comprueba que, sin Vault, la página principal se sirve tal cual y que el resto de archivos también.
func TestUIIndexSinVault(t *testing.T) {
	server := &Server{ui: fstest.MapFS{
		"index.html": {Data: []byte(`<html lang="en" data-theme="amoled"><body>hola</body></html>`)},
		"app.js":     {Data: []byte("console.log(1)")},
	}}
	handler := server.uiHandler()

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest("GET", "/", nil))
	if page.Code != 200 || !strings.Contains(page.Body.String(), `data-theme="amoled"`) {
		t.Fatalf("la página debería servirse igual: %d %s", page.Code, page.Body.String())
	}
	if page.Header().Get("Cache-Control") != "no-cache" {
		t.Fatal("la página principal debe revalidarse siempre: el tema puede cambiar")
	}

	script := httptest.NewRecorder()
	handler.ServeHTTP(script, httptest.NewRequest("GET", "/app.js", nil))
	if script.Code != 200 || script.Body.String() != "console.log(1)" {
		t.Fatalf("los demás archivos se sirven tal cual: %d %q", script.Code, script.Body.String())
	}
}

// * TestUITemaGuardadoEnElPC comprueba, con Rebootera Desktop sin USB, que el tema elegido se guarda en el PC, que la página
// * principal ya sale con él (y marcada como guardada) y que al elegir un USB los ajustes más recientes se ponen de acuerdo.
func TestUITemaGuardadoEnElPC(t *testing.T) {
	t.Cleanup(testutil.Keep(&vault.Root).Restore)
	t.Cleanup(testutil.Keep(&vault.LocalPath).Restore)
	vault.Root = filepath.Join(t.TempDir(), "sin-usb")
	vault.LocalPath = filepath.Join(t.TempDir(), "settings.json")
	server := &Server{deps: Deps{Mode: ModeDesktop}, ui: fstest.MapFS{
		"index.html": {Data: []byte(`<html lang="en" data-theme="amoled"><body></body></html>`)},
	}}

	// * Sin nada guardado, la página queda como está.
	page := httptest.NewRecorder()
	server.uiHandler().ServeHTTP(page, httptest.NewRequest("GET", "/", nil))
	if strings.Contains(page.Body.String(), "data-saved") {
		t.Fatalf("sin ajustes guardados no debe marcarse: %s", page.Body.String())
	}

	// * Se guarda el tema por la API (sin USB: solo en el PC).
	save := httptest.NewRecorder()
	body := strings.NewReader(`{"language":"es","theme":"orange","background":"auto","scale":100,"blur":"soft","intro":true}`)
	server.handleSaveSettings(save, httptest.NewRequest("POST", "/api/settings", body))
	if save.Code != 200 {
		t.Fatalf("guardar sin USB debe funcionar en el escritorio: %d %s", save.Code, save.Body.String())
	}
	page = httptest.NewRecorder()
	server.uiHandler().ServeHTTP(page, httptest.NewRequest("GET", "/", nil))
	want := `<html lang="es" data-theme="orange" data-saved="1"`
	if !strings.Contains(page.Body.String(), want) {
		t.Fatalf("la página debería salir con el tema guardado:\n%s", page.Body.String())
	}
	get := httptest.NewRecorder()
	server.handleGetSettings(get, httptest.NewRequest("GET", "/api/settings", nil))
	if !strings.Contains(get.Body.String(), `"persistent":true`) || !strings.Contains(get.Body.String(), `"theme":"orange"`) {
		t.Fatalf("la API debe devolver el tema guardado: %s", get.Body.String())
	}

	// * Al conectar un USB sin ajustes, se le llevan los del PC.
	usb := t.TempDir()
	if err := os.MkdirAll(filepath.Join(usb, "rebootera", "state"), 0o755); err != nil {
		t.Fatal(err)
	}
	vault.Root = usb
	SyncSettings()
	saved, err := vault.LoadSettings()
	if err != nil || saved.Theme != "orange" || saved.Language != "es" {
		t.Fatalf("el USB debería recibir el tema del PC: %+v %v", saved, err)
	}
}

// * TestGzip comprueba que el texto se comprime cuando el navegador lo acepta y que lo demás sale tal cual.
func TestGzip(t *testing.T) {
	handler := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bin" {
			w.Header().Set("Content-Type", "application/octet-stream")
		} else {
			w.Header().Set("Content-Type", "text/javascript")
		}
		w.Write([]byte(strings.Repeat("hola ", 200)))
	}))
	zipped := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/app.js", nil)
	request.Header.Set("Accept-Encoding", "gzip, br")
	handler.ServeHTTP(zipped, request)
	if zipped.Header().Get("Content-Encoding") != "gzip" || zipped.Body.Len() >= 1000 {
		t.Fatalf("debería comprimirse: %v %d bytes", zipped.Header(), zipped.Body.Len())
	}
	reader, err := gzip.NewReader(zipped.Body)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(reader)
	if string(data) != strings.Repeat("hola ", 200) {
		t.Fatal("lo descomprimido no coincide")
	}
	plain := httptest.NewRecorder()
	handler.ServeHTTP(plain, httptest.NewRequest("GET", "/app.js", nil))
	if plain.Header().Get("Content-Encoding") != "" || plain.Body.Len() != 1000 {
		t.Fatal("sin Accept-Encoding no se comprime")
	}
	binary := httptest.NewRecorder()
	request = httptest.NewRequest("GET", "/bin", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	handler.ServeHTTP(binary, request)
	if binary.Header().Get("Content-Encoding") != "" {
		t.Fatal("un tipo binario no se comprime")
	}
}

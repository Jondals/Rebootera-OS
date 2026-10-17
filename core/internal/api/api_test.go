// * Pruebas de las protecciones de la API: Host, sesión, CSRF y energía.
package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rebootera/core/internal/agentproto"
	"rebootera/core/internal/library"
	"rebootera/core/internal/testutil"
	"rebootera/core/internal/transfers"
	"rebootera/core/internal/vault"
)

// fakeAgent simula el agente y registra las operaciones recibidas.
type fakeAgent struct {
	ops []string
	// replies fija la respuesta de una operación; sent guarda las peticiones; errs fuerza errores por operación.
	replies map[string][]string
	sent    []agentproto.Request
	errs    map[string]error
	fail    bool
}

// * Call simula una operación del agente.
func (f *fakeAgent) Call(op string) error {
	f.ops = append(f.ops, op)
	if f.fail {
		return os.ErrDeadlineExceeded
	}
	return nil
}

// * Query simula una consulta al agente que devuelve texto.
func (f *fakeAgent) Query(op string) ([]string, error) {
	if err := f.Call(op); err != nil {
		return nil, err
	}
	if lines, ok := f.replies[op]; ok {
		return lines, nil
	}
	return []string{"[    0.000000] Linux version de prueba"}, nil
}

// * Send simula una petición con dispositivo.
func (f *fakeAgent) Send(request agentproto.Request) ([]string, error) {
	f.sent = append(f.sent, request)
	if err, ok := f.errs[request.Op]; ok {
		return nil, err
	}
	return nil, f.Call(request.Op)
}

// * newTestServer crea un servidor con una interfaz mínima.
func newTestServer(t *testing.T, agent *fakeAgent) http.Handler {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>ui</h1>"), 0o644)
	lib := library.New()
	return New(os.DirFS(dir), "secreto", "7080", agent, Deps{Library: lib, Transfers: transfers.New(lib)}).Handler()
}

// do ejecuta una petición contra el manejador.
func do(h http.Handler, method, target, host string, cookie bool, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = host
	if cookie {
		req.AddCookie(&http.Cookie{Name: cookieName, Value: "secreto"})
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestHostRechazado comprueba la defensa frente a DNS rebinding.
func TestHostRechazado(t *testing.T) {
	h := newTestServer(t, &fakeAgent{})
	rec := do(h, "GET", "/healthz", "evil.example:7080", false, "", nil)
	if rec.Code != http.StatusMisdirectedRequest {
		t.Fatalf("esperaba 421, obtuve %d", rec.Code)
	}
}

// TestSinSesion comprueba que la interfaz y la API exigen la cookie.
func TestSinSesion(t *testing.T) {
	h := newTestServer(t, &fakeAgent{})
	for _, path := range []string{"/", "/api/system", "/api/version"} {
		rec := do(h, "GET", path, "127.0.0.1:7080", false, "", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: esperaba 401, obtuve %d", path, rec.Code)
		}
	}
}

// TestAuth comprueba el canje del token por la cookie.
func TestAuth(t *testing.T) {
	h := newTestServer(t, &fakeAgent{})
	if rec := do(h, "GET", "/auth?token=malo", "127.0.0.1:7080", false, "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("token malo: esperaba 401, obtuve %d", rec.Code)
	}
	rec := do(h, "GET", "/auth?token=secreto", "localhost:7080", false, "", nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperaba 303, obtuve %d", rec.Code)
	}
	setCookie := rec.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "HttpOnly") || !strings.Contains(setCookie, "SameSite=Strict") {
		t.Fatalf("cookie sin protecciones: %s", setCookie)
	}
}

// TestConSesion comprueba que con la cookie se sirve la interfaz y la CSP.
func TestConSesion(t *testing.T) {
	h := newTestServer(t, &fakeAgent{})
	rec := do(h, "GET", "/", "127.0.0.1:7080", true, "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ui") {
		t.Fatalf("esperaba la interfaz, obtuve %d %q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Security-Policy"), "default-src 'self'") {
		t.Fatal("falta la CSP")
	}
	for _, path := range []string{"/api/system", "/api/storage", "/api/hardware", "/api/sensors", "/api/logs/kernel"} {
		if rec := do(h, "GET", path, "127.0.0.1:7080", true, "", nil); rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", path, rec.Code)
		}
	}
}

// TestPowerCSRF comprueba que sin la cabecera propia no se ejecuta nada.
func TestPowerCSRF(t *testing.T) {
	agent := &fakeAgent{}
	h := newTestServer(t, agent)
	rec := do(h, "POST", "/api/power", "127.0.0.1:7080", true, `{"action":"reboot"}`, map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusForbidden || len(agent.ops) != 0 {
		t.Fatalf("esperaba 403 sin llamar al agente, obtuve %d %v", rec.Code, agent.ops)
	}
}

// TestPowerValido comprueba el camino correcto y la acción desconocida.
func TestPowerValido(t *testing.T) {
	agent := &fakeAgent{}
	h := newTestServer(t, agent)
	headers := map[string]string{"Content-Type": "application/json", csrfHeader: "1"}
	rec := do(h, "POST", "/api/power", "127.0.0.1:7080", true, `{"action":"formatear"}`, headers)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("acción desconocida: esperaba 400, obtuve %d", rec.Code)
	}
	rec = do(h, "POST", "/api/power", "127.0.0.1:7080", true, `{"action":"reboot"}`, headers)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("esperaba 202, obtuve %d", rec.Code)
	}
	if len(agent.ops) != 1 || agent.ops[0] != agentproto.OpPing {
		t.Fatalf("esperaba un ping previo, obtuve %v", agent.ops)
	}
}

// TestPowerAgenteCaido comprueba el error comprensible si el agente no responde.
func TestPowerAgenteCaido(t *testing.T) {
	h := newTestServer(t, &fakeAgent{fail: true})
	headers := map[string]string{"Content-Type": "application/json", csrfHeader: "1"}
	rec := do(h, "POST", "/api/power", "127.0.0.1:7080", true, `{"action":"poweroff"}`, headers)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("esperaba 503, obtuve %d", rec.Code)
	}
}

// * TestSinVault comprueba las respuestas cuando la sesión no tiene USB de Rebootera.
func TestSinVault(t *testing.T) {
	// * Ruta inexistente: la prueba no depende de que el equipo tenga un Vault.
	t.Cleanup(testutil.Keep(&vault.Root).Restore)
	vault.Root = filepath.Join(t.TempDir(), "sin-vault")
	h := newTestServer(t, &fakeAgent{})
	rec := do(h, "GET", "/api/vault", "127.0.0.1:7080", true, "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"available":false`) {
		t.Fatalf("/api/vault: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(h, "GET", "/api/library", "127.0.0.1:7080", true, "", nil)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"code":"no-vault"`) {
		t.Fatalf("/api/library: %d %s", rec.Code, rec.Body.String())
	}
	headers := map[string]string{"Content-Type": "application/json", csrfHeader: "1"}
	rec = do(h, "POST", "/api/library/delete", "127.0.0.1:7080", true, `{"path":"isos/a.iso"}`, headers)
	if rec.Code != http.StatusConflict {
		t.Fatalf("borrar sin Vault: %d", rec.Code)
	}
	rec = do(h, "POST", "/api/library/delete", "127.0.0.1:7080", true, `{"path":"isos/a.iso"}`, map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("borrar sin cabecera anti-CSRF debería dar 403, dio %d", rec.Code)
	}
}

// * TestRutasDeMedios impide salir de la carpeta del medio montado.
func TestRutasDeMedios(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "sdb1", "isos"), 0o755)
	t.Cleanup(testutil.Keep(&MediaRoot).Restore)
	MediaRoot = root
	if got, err := mediaPath("/dev/sdb1", "isos"); err != nil || got != filepath.Join(root, "sdb1", "isos") {
		t.Fatalf("ruta válida: %q %v", got, err)
	}
	for _, bad := range []string{"../../etc", "isos/../../x"} {
		got, err := mediaPath("sdb1", bad)
		if err == nil && !strings.HasPrefix(got, filepath.Join(root, "sdb1")) {
			t.Fatalf("%q salió del medio: %q", bad, got)
		}
	}
	// * Un nombre de dispositivo con ".." se reduce a su último componente.
	if got, _ := mediaPath("../sdb1", ""); got != filepath.Join(root, "sdb1") {
		t.Fatalf("dispositivo con ..: %q", got)
	}
}

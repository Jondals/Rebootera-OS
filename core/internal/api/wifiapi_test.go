// * Pruebas de la API de Wi-Fi y de la preparación de instaladores.
package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rebootera/core/internal/agentproto"
	"rebootera/core/internal/testutil"
	"rebootera/core/internal/vault"
)

var jsonHeaders = map[string]string{"Content-Type": "application/json", csrfHeader: "1"}

// * TestWifiEstadoYBusqueda comprueba que se interpreta lo que devuelve el agente.
func TestWifiEstadoYBusqueda(t *testing.T) {
	agent := &fakeAgent{replies: map[string][]string{
		agentproto.OpWifiStatus: {"radio:enabled", "enp0s3:ethernet:connected:Wired", "wlan0:wifi:disconnected:"},
		agentproto.OpWifiScan:   {"*:Casa:80:WPA2", ":Cafe:50:--", ":Oficina:70:WPA2 802.1X"},
	}}
	h := newTestServer(t, agent)
	rec := do(h, "GET", "/api/wifi", "127.0.0.1:7080", true, "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"adapter":true`) || !strings.Contains(rec.Body.String(), `"device":"wlan0"`) || !strings.Contains(rec.Body.String(), `"enabled":true`) {
		t.Fatalf("estado: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(h, "POST", "/api/wifi/scan", "127.0.0.1:7080", true, "{}", jsonHeaders)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ssid":"Casa"`) || !strings.Contains(body, `"security":"enterprise"`) || !strings.Contains(body, `"security":"open"`) {
		t.Fatalf("búsqueda: %d %s", rec.Code, body)
	}
	// * Buscar redes cambia el estado de la radio: exige la cabecera anti-CSRF.
	rec = do(h, "POST", "/api/wifi/scan", "127.0.0.1:7080", true, "{}", map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("sin cabecera anti-CSRF debería dar 403, dio %d", rec.Code)
	}
}

// * TestWifiConectar comprueba la validación previa y el paso de datos al agente.
func TestWifiConectar(t *testing.T) {
	agent := &fakeAgent{}
	h := newTestServer(t, agent)
	bad := []string{
		`{"ssid":"","password":"12345678"}`,
		`{"ssid":"` + strings.Repeat("a", 40) + `","password":"12345678"}`,
		`{"ssid":"Casa","password":"` + strings.Repeat("a", 70) + `"}`,
		`{"ssid":"Casa","password":"a\nb"}`,
	}
	for _, body := range bad {
		rec := do(h, "POST", "/api/wifi/connect", "127.0.0.1:7080", true, body, jsonHeaders)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"wifi-invalid"`) {
			t.Fatalf("%s: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	if len(agent.sent) != 0 {
		t.Fatalf("no debe llamar al agente con datos inválidos: %v", agent.sent)
	}
	rec := do(h, "POST", "/api/wifi/connect", "127.0.0.1:7080", true, `{"ssid":"Casa","password":"clavesegura","hidden":true}`, jsonHeaders)
	if rec.Code != http.StatusOK || len(agent.sent) != 1 {
		t.Fatalf("conectar: %d %s", rec.Code, rec.Body.String())
	}
	got := agent.sent[0]
	if got.Op != agentproto.OpWifiConnect || got.SSID != "Casa" || got.Password != "clavesegura" || !got.Hidden {
		t.Fatalf("petición al agente: %+v", got)
	}
	if strings.Contains(rec.Body.String(), "clavesegura") {
		t.Fatal("la respuesta no debe repetir la contraseña")
	}
}

// * TestWifiErrores comprueba que los errores del agente se traducen a códigos.
func TestWifiErrores(t *testing.T) {
	cases := map[string]string{
		"este equipo no tiene un adaptador Wi-Fi":                codeWifiNoAdapter,
		"no se encuentra esa red; vuelve a buscar redes":         codeWifiNotFound,
		"la contraseña debe tener entre 8 y 63 caracteres":       codeWifiInvalid,
		"no se pudo conectar: revisa la contraseña o acércate":   codeWifiFailed,
		"no se pudo contactar con el agente: connection refused": codeAgent,
	}
	for message, want := range cases {
		agent := &fakeAgent{errs: map[string]error{agentproto.OpWifiConnect: errors.New(message)}}
		h := newTestServer(t, agent)
		rec := do(h, "POST", "/api/wifi/connect", "127.0.0.1:7080", true, `{"ssid":"Casa","password":"clavesegura"}`, jsonHeaders)
		if !strings.Contains(rec.Body.String(), `"code":"`+want+`"`) {
			t.Fatalf("%q: esperaba %s, obtuve %s", message, want, rec.Body.String())
		}
	}
}

// * fakeInstallVault prepara un Vault con una imagen de Linux y otra de Windows.
func fakeInstallVault(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"rebootera/state", "ventoy", "isos/linux", "isos/windows", "drivers"} {
		os.MkdirAll(filepath.Join(root, dir), 0o755)
	}
	data, err := os.ReadFile("../../../os/usb/ventoy/ventoy.json")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "ventoy", "ventoy.json"), data, 0o644)
	os.WriteFile(filepath.Join(root, "isos", "linux", "ubuntu.iso"), []byte("iso"), 0o644)
	os.WriteFile(filepath.Join(root, "isos", "linux", "debian-13.7.0-amd64-netinst.iso"), []byte("iso"), 0o644)
	os.WriteFile(filepath.Join(root, "isos", "windows", "Win11.iso"), []byte("iso"), 0o644)
	t.Cleanup(testutil.Keep(&vault.Root).Restore)
	vault.Root = root
}

// * TestInstalarCombinaciones comprueba qué se admite en cada tipo de imagen.
func TestInstalarCombinaciones(t *testing.T) {
	fakeInstallVault(t)
	agent := &fakeAgent{}
	h := newTestServer(t, agent)
	// * Respuestas automáticas en Ubuntu: no se admiten.
	r := postInstall(h, `{"path":"isos/linux/ubuntu.iso","unattended":{"language":"es"}}`)
	if r.code != http.StatusBadRequest || !strings.Contains(r.body, "unattended-unsupported") {
		t.Fatalf("Ubuntu: %d %s", r.code, r.body)
	}
	// * Drivers y atajo de Windows 11 solo en Windows.
	r = postInstall(h, `{"path":"isos/linux/debian-13.7.0-amd64-netinst.iso","drivers":true}`)
	if r.code != http.StatusBadRequest || !strings.Contains(r.body, "drivers-unsupported") {
		t.Fatalf("drivers en Linux: %d %s", r.code, r.body)
	}
	r = postInstall(h, `{"path":"isos/linux/debian-13.7.0-amd64-netinst.iso","winBypass":true}`)
	if r.code != http.StatusBadRequest {
		t.Fatalf("atajo de Windows en Linux: %d %s", r.code, r.body)
	}
	// * Drivers sin ningún paquete que case: error claro, sin tocar el arranque.
	r = postInstall(h, `{"path":"isos/windows/Win11.iso","drivers":true}`)
	if r.code != http.StatusConflict || !strings.Contains(r.body, "no-drivers") {
		t.Fatalf("sin drivers: %d %s", r.code, r.body)
	}
	// * Opciones inválidas.
	r = postInstall(h, `{"path":"isos/windows/Win11.iso","unattended":{"language":"es","userName":"root","password":"x"}}`)
	if r.code != http.StatusBadRequest || !strings.Contains(r.body, `"code":"invalid"`) {
		t.Fatalf("usuario root: %d %s", r.code, r.body)
	}
	if got := bootNext(t); !strings.HasPrefix(got, "/rebootera/") {
		t.Fatalf("un intento rechazado no debe cambiar el arranque: %s", got)
	}
	// * Windows con respuestas: se escribe el autounattend y se reinicia.
	r = postInstall(h, `{"path":"isos/windows/Win11.iso","winBypass":true,"unattended":{"language":"es","timeZone":"Europe/Madrid","userName":"ana","password":"clave"}}`)
	if r.code != http.StatusAccepted {
		t.Fatalf("Windows con respuestas: %d %s", r.code, r.body)
	}
	answers, err := os.ReadFile(filepath.Join(vault.Root, "ventoy", "rebootera", "autounattend.xml"))
	if err != nil || !strings.Contains(string(answers), "Romance Standard Time") {
		t.Fatalf("autounattend.xml: %v", err)
	}
	if got := bootNext(t); got != "/isos/windows/Win11.iso" {
		t.Fatalf("imagen preseleccionada: %s", got)
	}
}

// * postInstall pide instalar una imagen y devuelve el código y el cuerpo.
func postInstall(h http.Handler, body string) *httpRec {
	rec := do(h, "POST", "/api/boot/install", "127.0.0.1:7080", true, body, jsonHeaders)
	return &httpRec{code: rec.Code, body: rec.Body.String()}
}

// httpRec guarda el código y el cuerpo de una respuesta.
type httpRec struct {
	code int
	body string
}

// * bootNext lee la imagen que Ventoy arrancará la próxima vez.
func bootNext(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(vault.Root, "ventoy", "ventoy.json"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	i := strings.Index(text, `"VTOY_DEFAULT_IMAGE"`)
	if i < 0 {
		t.Fatal("falta VTOY_DEFAULT_IMAGE")
	}
	rest := text[i+len(`"VTOY_DEFAULT_IMAGE"`):]
	start := strings.Index(rest, `"`)
	end := strings.Index(rest[start+1:], `"`)
	return rest[start+1 : start+1+end]
}

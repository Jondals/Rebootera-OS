// * Pruebas de la API del teclado.
package api

import (
	"net/http"
	"strings"
	"testing"

	"rebootera/core/internal/agentproto"
)

// * TestTecladoConsultaYCambio comprueba la lista cerrada y que se pide al agente.
func TestTecladoConsultaYCambio(t *testing.T) {
	agent := &fakeAgent{replies: map[string][]string{agentproto.OpKeyboardGet: {"de"}}}
	h := newTestServer(t, agent)
	rec := do(h, "GET", "/api/keyboard", "127.0.0.1:7080", true, "", nil)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"layout":"de"`) || !strings.Contains(body, `"pt"`) {
		t.Fatalf("consulta: %d %s", rec.Code, body)
	}
	rec = do(h, "POST", "/api/keyboard", "127.0.0.1:7080", true, `{"layout":"fr"}`, jsonHeaders)
	if rec.Code != http.StatusOK || len(agent.sent) != 1 || agent.sent[0].Op != agentproto.OpKeyboardSet || agent.sent[0].Layout != "fr" {
		t.Fatalf("cambio: %d %s %v", rec.Code, rec.Body.String(), agent.sent)
	}
	// * Una distribución fuera de la lista no llega al agente.
	rec = do(h, "POST", "/api/keyboard", "127.0.0.1:7080", true, `{"layout":"es;rm"}`, jsonHeaders)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), codeKeyboardInvalid) || len(agent.sent) != 1 {
		t.Fatalf("no válida: %d %s", rec.Code, rec.Body.String())
	}
	// * Cambiar el teclado exige la cabecera anti-CSRF.
	rec = do(h, "POST", "/api/keyboard", "127.0.0.1:7080", true, `{"layout":"us"}`, map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("sin cabecera anti-CSRF debería dar 403, dio %d", rec.Code)
	}
}

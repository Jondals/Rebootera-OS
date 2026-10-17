// * Endpoints de la distribución del teclado. Core no puede tocar la sesión del
// * kiosco (no tiene privilegios): le pide el cambio al agente, que reinicia la
// * interfaz con la distribución nueva. La elección se guarda en el Vault.
package api

import (
	"errors"
	"net/http"
	"path/filepath"

	"rebootera/core/internal/agentproto"
	"rebootera/core/internal/vault"
)

// codeKeyboardInvalid es el código de error de una distribución desconocida.
const codeKeyboardInvalid = "keyboard-invalid"

// keyboardFile guarda la distribución elegida dentro del estado del Vault.
const keyboardFile = "keyboard.json"

// keyboardState es lo que se guarda y lo que devuelve GET /api/keyboard.
type keyboardState struct {
	Layout string `json:"layout"`
}

// * registerKeyboard añade los endpoints del teclado (solo en Rebootera OS).
func (s *Server) registerKeyboard(mux *http.ServeMux) {
	mux.Handle("GET /api/keyboard", s.requireSession(http.HandlerFunc(s.handleKeyboardGet)))
	mux.Handle("POST /api/keyboard", s.requireSession(s.requireCSRF(http.HandlerFunc(s.handleKeyboardSet))))
}

// * LoadKeyboard devuelve la distribución guardada ("" si no hay ninguna).
func LoadKeyboard() string {
	if !vault.Available() {
		return ""
	}
	var state keyboardState
	if err := vault.ReadJSON(filepath.Join(vault.StateDir(), keyboardFile), &state); err != nil {
		return ""
	}
	if !agentproto.ValidKeyboard(state.Layout) {
		return ""
	}
	return state.Layout
}

// * handleKeyboardGet devuelve la distribución actual y las disponibles.
func (s *Server) handleKeyboardGet(w http.ResponseWriter, r *http.Request) {
	layout := agentproto.DefaultKeyboard
	if lines, err := s.agent.Query(agentproto.OpKeyboardGet); err == nil && len(lines) > 0 && agentproto.ValidKeyboard(lines[0]) {
		layout = lines[0]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"layout":     layout,
		"layouts":    agentproto.KeyboardLayouts,
		"persistent": vault.Available(),
	})
}

// * handleKeyboardSet cambia la distribución. La interfaz se reinicia un
// * instante después para poder contestar antes de que se cierre.
func (s *Server) handleKeyboardSet(w http.ResponseWriter, r *http.Request) {
	var req keyboardState
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	if !agentproto.ValidKeyboard(req.Layout) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "distribución de teclado no válida", "code": codeKeyboardInvalid})
		return
	}
	if _, err := s.agent.Send(agentproto.Request{Op: agentproto.OpKeyboardSet, Layout: req.Layout}); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error(), "code": codeAgent})
		return
	}
	// * Guardar es un extra: sin Vault el cambio vale hasta apagar.
	if err := vault.WriteJSON(filepath.Join(vault.StateDir(), keyboardFile), req); err != nil && !errors.Is(err, vault.ErrUnavailable) {
		writeJSON(w, http.StatusOK, map[string]any{"layout": req.Layout, "saved": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"layout": req.Layout, "saved": vault.Available()})
}

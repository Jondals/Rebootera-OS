// * Endpoints del puntero y del panel táctil del kiosco (sensibilidad, toque para clic, desplazamiento natural).
// * Core no puede tocar la sesión gráfica: le pide el cambio al agente, que reinicia la interfaz con los ajustes
// * nuevos. La elección se guarda en el Vault para aplicarla en el siguiente arranque.
package api

import (
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"rebootera/core/internal/agentproto"
	"rebootera/core/internal/vault"
)

// inputFile guarda los ajustes del puntero dentro del estado del Vault.
const inputFile = "input.json"

// * registerInput añade los endpoints del puntero (solo en Rebootera OS).
func (s *Server) registerInput(mux *http.ServeMux) {
	mux.Handle("GET /api/input", s.requireSession(http.HandlerFunc(s.handleInputGet)))
	mux.Handle("POST /api/input", s.requireSession(s.requireCSRF(http.HandlerFunc(s.handleInputSet))))
}

// * LoadInput devuelve los ajustes del puntero guardados, o nil si no hay ninguno válido.
func LoadInput() *agentproto.InputSettings {
	if !vault.Available() {
		return nil
	}
	var saved agentproto.InputSettings
	if err := vault.ReadJSON(filepath.Join(vault.StateDir(), inputFile), &saved); err != nil || saved.Validate() != nil {
		return nil
	}
	return &saved
}

// * parseInput lee las líneas speed=N, tap=0|1 y natural=0|1 que devuelve el agente.
func parseInput(lines []string) agentproto.InputSettings {
	settings := agentproto.DefaultInput
	for _, line := range lines {
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		switch key {
		case "speed":
			if number, err := strconv.Atoi(value); err == nil {
				settings.Speed = number
			}
		case "tap":
			settings.Tap = value == "1"
		case "natural":
			settings.Natural = value == "1"
		}
	}
	return settings
}

// * handleInputGet devuelve los ajustes del puntero en uso.
func (s *Server) handleInputGet(w http.ResponseWriter, r *http.Request) {
	settings := agentproto.DefaultInput
	if lines, err := s.agent.Query(agentproto.OpInputGet); err == nil {
		settings = parseInput(lines)
	}
	writeJSON(w, http.StatusOK, map[string]any{"input": settings, "persistent": vault.Available()})
}

// * handleInputSet cambia los ajustes del puntero. La interfaz se reinicia un instante después para poder
// * contestar antes de que se cierre.
func (s *Server) handleInputSet(w http.ResponseWriter, r *http.Request) {
	var req agentproto.InputSettings
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	if err := req.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error(), "code": codeInvalid})
		return
	}
	if _, err := s.agent.Send(agentproto.Request{Op: agentproto.OpInputSet, Input: &req}); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error(), "code": codeAgent})
		return
	}
	// * Guardar es un extra: sin Vault el cambio vale hasta apagar.
	if err := vault.WriteJSON(filepath.Join(vault.StateDir(), inputFile), req); err != nil && !errors.Is(err, vault.ErrUnavailable) {
		writeJSON(w, http.StatusOK, map[string]any{"input": req, "saved": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"input": req, "saved": vault.Available()})
}

// * Endpoints de Wi-Fi: estado del adaptador, búsqueda de redes, conexión y
// * desconexión. Core no toca NetworkManager (no tiene privilegios): pide cada
// * operación al agente. Los errores llevan un código que la interfaz traduce.
// ! La contraseña solo viaja en el cuerpo de la petición local y por el socket
// ! del agente; nunca se registra.
package api

import (
	"net/http"
	"strings"

	"rebootera/core/internal/agentproto"
	"rebootera/core/internal/wifi"
)

// Códigos de error de Wi-Fi.
const (
	codeWifiInvalid   = "wifi-invalid"
	codeWifiNoAdapter = "wifi-no-adapter"
	codeWifiNotFound  = "wifi-not-found"
	codeWifiFailed    = "wifi-failed"
)

// * registerWifi añade los endpoints de Wi-Fi (solo en Rebootera OS).
func (s *Server) registerWifi(mux *http.ServeMux) {
	mux.Handle("GET /api/wifi", s.requireSession(http.HandlerFunc(s.handleWifiStatus)))
	mux.Handle("POST /api/wifi/scan", s.requireSession(s.requireCSRF(http.HandlerFunc(s.handleWifiScan))))
	mux.Handle("POST /api/wifi/connect", s.requireSession(s.requireCSRF(http.HandlerFunc(s.handleWifiConnect))))
	mux.Handle("POST /api/wifi/disconnect", s.requireSession(s.requireCSRF(http.HandlerFunc(s.handleWifiDisconnect))))
}

// wifiStatusResponse es la respuesta de GET /api/wifi.
type wifiStatusResponse struct {
	wifi.Status
	// Enabled indica que el interruptor de radio de Wi-Fi está activo.
	Enabled bool `json:"enabled"`
}

// * handleWifiStatus devuelve el estado del adaptador Wi-Fi.
func (s *Server) handleWifiStatus(w http.ResponseWriter, r *http.Request) {
	lines, err := s.agent.Query(agentproto.OpWifiStatus)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error(), "code": codeAgent})
		return
	}
	response := wifiStatusResponse{Enabled: true}
	for _, line := range lines {
		if strings.HasPrefix(line, "radio:") {
			response.Enabled = strings.TrimPrefix(line, "radio:") == "enabled"
		}
	}
	response.Status = wifi.ParseStatus(strings.Join(lines, "\n"))
	writeJSON(w, http.StatusOK, response)
}

// * handleWifiScan busca redes y devuelve las visibles.
func (s *Server) handleWifiScan(w http.ResponseWriter, r *http.Request) {
	lines, err := s.agent.Query(agentproto.OpWifiScan)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error(), "code": wifiCode(err)})
		return
	}
	writeJSON(w, http.StatusOK, wifi.ParseList(strings.Join(lines, "\n")))
}

// connectRequest es el cuerpo de POST /api/wifi/connect.
type connectRequest struct {
	SSID     string `json:"ssid"`
	Password string `json:"password"`
	Hidden   bool   `json:"hidden"`
}

// * handleWifiConnect conecta a una red. La seguridad la decide el agente con
// * su propio escaneo; aquí solo se descartan los datos imposibles.
func (s *Server) handleWifiConnect(w http.ResponseWriter, r *http.Request) {
	var req connectRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	if err := wifi.ValidateSSID(req.SSID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error(), "code": codeWifiInvalid})
		return
	}
	if len(req.Password) > 64 || strings.ContainsAny(req.Password, "\r\n\x00") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "contraseña no válida", "code": codeWifiInvalid})
		return
	}
	_, err := s.agent.Send(agentproto.Request{Op: agentproto.OpWifiConnect, SSID: req.SSID, Password: req.Password, Hidden: req.Hidden})
	if err != nil {
		status := http.StatusBadGateway
		if wifiCode(err) == codeWifiInvalid {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]string{"error": err.Error(), "code": wifiCode(err)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "connected"})
}

// * handleWifiDisconnect desconecta y olvida la red.
func (s *Server) handleWifiDisconnect(w http.ResponseWriter, r *http.Request) {
	if err := s.agent.Call(agentproto.OpWifiDisconnect); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error(), "code": codeWifiFailed})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "disconnected"})
}

// * wifiCode traduce el error del agente al código de la API.
func wifiCode(err error) string {
	message := err.Error()
	switch {
	case strings.Contains(message, "adaptador"):
		return codeWifiNoAdapter
	case strings.Contains(message, "no se encuentra esa red"):
		return codeWifiNotFound
	case strings.Contains(message, "contraseña") && strings.Contains(message, "debe"),
		strings.Contains(message, "no lleva contraseña"),
		strings.Contains(message, "no se puede configurar"),
		strings.Contains(message, "nombre de la red"):
		return codeWifiInvalid
	case strings.Contains(message, "no se pudo contactar con el agente"):
		return codeAgent
	}
	return codeWifiFailed
}

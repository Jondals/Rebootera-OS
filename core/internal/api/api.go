// * Package api: API HTTP local que usa Rebootera UI y servidor de sus
// * archivos estáticos. Solo escucha en la interfaz de bucle local.
// ! Cada petición pasa por estas protecciones:
// !   - lista de cabeceras Host admitidas (evita ataques de DNS rebinding),
// !   - token por arranque que la ventana kiosco canjea por una cookie,
// !   - cabecera obligatoria en las peticiones que cambian algo (evita CSRF),
// !   - política de seguridad de contenido que solo permite recursos propios.
package api

import (
	"crypto/subtle"
	"encoding/json"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"time"

	"rebootera/core/internal/agentproto"
	"rebootera/core/internal/hardware"
	"rebootera/core/internal/sensors"
	"rebootera/core/internal/storage"
	"rebootera/core/internal/system"
	"rebootera/core/internal/version"
)

// Nombre de la cookie de sesión, cabecera anti-CSRF y límites.
const (
	cookieName   = "rebootera_session"
	csrfHeader   = "X-Rebootera"
	powerDelay   = 1500 * time.Millisecond
	maxBodyBytes = 64 * 1024
)

// Agent es lo que la API necesita del agente privilegiado.
type Agent interface {
	Call(op string) error
	Query(op string) ([]string, error)
	Send(request agentproto.Request) ([]string, error)
}

// SocketAgent habla con el rebootera-agent real por su socket Unix.
type SocketAgent struct{}

// * Call envía una operación al agente real.
func (SocketAgent) Call(op string) error {
	return agentproto.Call(op)
}

// * Query envía una operación al agente real y devuelve sus líneas de texto.
func (SocketAgent) Query(op string) ([]string, error) {
	return agentproto.Query(op)
}

// * Send envía una petición completa (con dispositivo) al agente real.
func (SocketAgent) Send(request agentproto.Request) ([]string, error) {
	return agentproto.Send(request)
}

// Server agrupa la configuración y las dependencias de la API.
type Server struct {
	ui           fs.FS
	token        string
	allowedHosts map[string]bool
	agent        Agent
	deps         Deps
}

// * Las fuentes de la interfaz son woff2: algunos sistemas no conocen su tipo y el navegador las rechazaría.
func init() {
	mime.AddExtensionType(".woff2", "font/woff2")
}

// * New crea el servidor. port es el puerto de escucha, que se usa para
// * construir la lista de cabeceras Host válidas.
// * ui es la carpeta de la interfaz: una carpeta del disco en Rebootera OS o
// * los archivos embebidos en el ejecutable de Rebootera Desktop.
func New(ui fs.FS, token, port string, agent Agent, deps Deps) *Server {
	return &Server{
		ui:    ui,
		token: token,
		allowedHosts: map[string]bool{
			"127.0.0.1:" + port: true,
			"localhost:" + port: true,
		},
		agent: agent,
		deps:  deps,
	}
}

// * Handler devuelve el manejador HTTP completo con todas las protecciones.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /auth", s.handleAuth)
	mux.Handle("GET /api/version", s.requireSession(http.HandlerFunc(s.handleVersion)))
	mux.Handle("GET /api/mode", s.requireSession(http.HandlerFunc(s.handleMode)))
	mux.Handle("GET /api/system", s.requireSession(http.HandlerFunc(s.handleSystem)))
	mux.Handle("GET /api/storage", s.requireSession(http.HandlerFunc(s.handleStorage)))
	mux.Handle("GET /api/hardware", s.requireSession(http.HandlerFunc(s.handleHardware)))
	mux.Handle("GET /api/sensors", s.requireSession(http.HandlerFunc(s.handleSensors)))
	if s.mode() == ModeLive {
		// * Solo Rebootera OS tiene agente: energía y registro del kernel.
		mux.Handle("GET /api/logs/kernel", s.requireSession(http.HandlerFunc(s.handleKernelLog)))
		mux.Handle("POST /api/power", s.requireSession(s.requireCSRF(http.HandlerFunc(s.handlePower))))
		s.registerWifi(mux)
		s.registerKeyboard(mux)
		s.registerInput(mux)
		s.registerPartitions(mux)
	} else {
		s.registerDesktop(mux)
	}
	s.registerVault(mux)
	mux.Handle("GET /", s.requireSession(http.FileServer(http.FS(s.ui))))
	return &guard{server: s, next: mux}
}

// guard aplica a todas las peticiones la comprobación de Host y las
// cabeceras de seguridad.
type guard struct {
	server *Server
	next   http.Handler
}

// * ServeHTTP implementa http.Handler.
func (g *guard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !g.server.allowedHosts[r.Host] {
		http.Error(w, "Host no permitido", http.StatusMisdirectedRequest)
		return
	}
	h := w.Header()
	h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; font-src 'self'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cache-Control", "no-store")
	g.next.ServeHTTP(w, r)
}

// sessionGuard exige la cookie de sesión válida.
type sessionGuard struct {
	server *Server
	next   http.Handler
}

// * requireSession envuelve un manejador para exigir la cookie de sesión.
func (s *Server) requireSession(next http.Handler) http.Handler {
	return &sessionGuard{server: s, next: next}
}

// * ServeHTTP implementa http.Handler.
func (g *sessionGuard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(cookieName)
	if err != nil || !g.server.tokenMatches(cookie.Value) {
		http.Error(w, "Sesión no válida", http.StatusUnauthorized)
		return
	}
	g.next.ServeHTTP(w, r)
}

// csrfGuard exige la cabecera propia en las peticiones que modifican estado.
// * Un sitio externo no puede añadir cabeceras propias sin una petición previa
// * CORS, que este servidor nunca autoriza.
type csrfGuard struct {
	next http.Handler
}

// * requireCSRF envuelve un manejador para exigir la cabecera anti-CSRF.
func (s *Server) requireCSRF(next http.Handler) http.Handler {
	return &csrfGuard{next: next}
}

// * ServeHTTP implementa http.Handler.
func (g *csrfGuard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(csrfHeader) != "1" || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "Petición rechazada", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	g.next.ServeHTTP(w, r)
}

// * tokenMatches compara el token en tiempo constante.
func (s *Server) tokenMatches(candidate string) bool {
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(s.token)) == 1
}

// * handleHealth responde sin autenticación para que la ventana kiosco sepa
// * cuándo está listo el servicio.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte("ok"))
}

// * handleAuth canjea el token por una cookie de sesión y redirige a la interfaz.
func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) {
	if !s.tokenMatches(r.URL.Query().Get("token")) {
		http.Error(w, "Token no válido", http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    s.token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	// * "route" (solo pruebas) abre la interfaz en una sección; solo se admiten letras.
	if route := r.URL.Query().Get("route"); routePattern.MatchString(route) {
		http.Redirect(w, r, "/#/"+route, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// routePattern es lo único que se acepta como nombre de sección en /auth.
var routePattern = regexp.MustCompile(`^[a-z]{1,12}$`)

// * handleVersion devuelve la versión de Rebootera OS.
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": version.Version})
}

// * handleSystem devuelve el resumen del equipo.
func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, system.Collect())
}

// * handleStorage devuelve los discos y particiones (solo lectura).
func (s *Server) handleStorage(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, storage.List())
}

// * handleHardware devuelve el inventario de dispositivos PCI y USB.
func (s *Server) handleHardware(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, hardware.List())
}

// * handleSensors devuelve las temperaturas actuales.
func (s *Server) handleSensors(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, sensors.Temperatures())
}

// * handleKernelLog devuelve las últimas líneas del registro del kernel,
// * que solo puede leer el agente.
func (s *Server) handleKernelLog(w http.ResponseWriter, r *http.Request) {
	lines, err := s.agent.Query(agentproto.OpKernelLog)
	if err != nil {
		log.Printf("registro del kernel: %v", err)
		writeError(w, http.StatusServiceUnavailable, "El servicio del sistema no responde.")
		return
	}
	if lines == nil {
		lines = []string{}
	}
	writeJSON(w, http.StatusOK, map[string][]string{"lines": lines})
}

// powerRequest es el cuerpo de POST /api/power.
type powerRequest struct {
	Action string `json:"action"`
}

// * handlePower reinicia o apaga el equipo. Primero comprueba que el agente
// * responde, contesta a la interfaz y ejecuta la orden poco después para que
// * la respuesta llegue antes de que el sistema se detenga.
func (s *Server) handlePower(w http.ResponseWriter, r *http.Request) {
	var req powerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Petición no válida")
		return
	}
	var op string
	switch req.Action {
	case "reboot":
		op = agentproto.OpReboot
	case "poweroff":
		op = agentproto.OpPoweroff
	default:
		writeError(w, http.StatusBadRequest, "Acción desconocida")
		return
	}
	if err := s.agent.Call(agentproto.OpPing); err != nil {
		log.Printf("energía: el agente no responde: %v", err)
		writeError(w, http.StatusServiceUnavailable, "El servicio del sistema no responde. Vuelve a intentarlo.")
		return
	}
	log.Printf("energía: %s solicitado desde la interfaz", op)
	task := &delayedOp{op: op, agent: s.agent}
	time.AfterFunc(powerDelay, task.run)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

// delayedOp es una operación del agente que se ejecuta más tarde.
type delayedOp struct {
	op    string
	agent Agent
}

// * run ejecuta la operación y registra el error si lo hay.
func (d *delayedOp) run() {
	if err := d.agent.Call(d.op); err != nil {
		log.Printf("energía: %s falló: %v", d.op, err)
	}
}

// * writeJSON escribe una respuesta JSON.
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

// * writeError escribe un error JSON con un mensaje comprensible.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

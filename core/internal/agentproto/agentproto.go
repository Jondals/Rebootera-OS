// * Package agentproto: protocolo entre rebootera-core (sin privilegios) y
// * rebootera-agent (root). Cada conexión al socket Unix transporta una única
// * petición JSON y una única respuesta JSON.
// ! El agente solo acepta las operaciones de esta lista cerrada: añadir una
// ! operación nueva aquí es ampliar lo que puede hacer root.
package agentproto

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"rebootera/core/internal/partman"
)

// SocketPath es la ruta del socket del agente.
const SocketPath = "/run/rebootera-agent/agent.sock"

// Operaciones admitidas por el agente.
const (
	OpPing      = "ping"
	OpReboot    = "power.reboot"
	OpPoweroff  = "power.poweroff"
	OpKernelLog = "logs.kernel"
	// * Montan o desmontan en solo lectura una partición para importar imágenes.
	OpMediaMount   = "media.mount"
	OpMediaUnmount = "media.unmount"
	// * Wi-Fi: estado del adaptador, búsqueda de redes, conexión y desconexión.
	// * Arranque UEFI de una sola vez: USB el último del orden y BootNext; restauración.
	OpBootOnce       = "boot.once"
	OpBootRestore    = "boot.restore"
	OpWifiStatus     = "wifi.status"
	OpWifiScan       = "wifi.scan"
	OpWifiConnect    = "wifi.connect"
	OpWifiDisconnect = "wifi.disconnect"
	// * Teclado del kiosco: consultar la distribución y cambiarla (reinicia la interfaz).
	OpKeyboardGet = "keyboard.get"
	OpKeyboardSet = "keyboard.set"
	// * Gestor de particiones: mínimos de un sistema de archivos, empezar una operación y ver su estado.
	OpPartInfo   = "part.info"
	OpPartApply  = "part.apply"
	OpPartStatus = "part.status"
)

// DefaultKeyboard es la distribución con la que arranca la interfaz.
const DefaultKeyboard = "es"

// KeyboardLayouts es la lista cerrada de distribuciones que acepta el agente.
var KeyboardLayouts = []string{"es", "us"}

// * ValidKeyboard indica si una distribución está en la lista cerrada.
func ValidKeyboard(layout string) bool {
	for _, known := range KeyboardLayouts {
		if known == layout {
			return true
		}
	}
	return false
}

// Request es una petición al agente. Device solo lo usan las operaciones de
// medios y el agente lo valida antes de usarlo.
type Request struct {
	Op     string `json:"op"`
	Device string `json:"device,omitempty"`
	// SSID, Password y Hidden solo los usa wifi.connect. La contraseña nunca se registra.
	SSID     string `json:"ssid,omitempty"`
	Password string `json:"password,omitempty"`
	Hidden   bool   `json:"hidden,omitempty"`
	// Disk y Part (nombre de disco, número de partición) los usa part.info.
	Disk string `json:"disk,omitempty"`
	Part int    `json:"part,omitempty"`
	// Partition y Confirm los usa part.apply: la operación y el texto que escribió el usuario.
	Partition *partman.Op `json:"partition,omitempty"`
	Confirm   string      `json:"confirm,omitempty"`
	// Layout solo lo usa keyboard.set y el agente lo valida contra KeyboardLayouts.
	Layout string `json:"layout,omitempty"`
}

// Response es la respuesta del agente. Lines solo se rellena en las
// operaciones que devuelven texto, como el registro del kernel.
type Response struct {
	OK    bool     `json:"ok"`
	Error string   `json:"error,omitempty"`
	Lines []string `json:"lines,omitempty"`
}

// * Call envía una operación al agente y espera su respuesta.
func Call(op string) error {
	_, err := Send(Request{Op: op})
	return err
}

// * Query envía una operación al agente y devuelve las líneas de texto que
// * acompañan a la respuesta.
func Query(op string) ([]string, error) {
	return Send(Request{Op: op})
}

// * Send envía una petición completa al agente.
func Send(request Request) ([]string, error) {
	conn, err := net.DialTimeout("unix", SocketPath, 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("no se pudo contactar con el agente: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(Deadline(request.Op)))

	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return nil, fmt.Errorf("no se pudo enviar la petición al agente: %w", err)
	}
	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, fmt.Errorf("respuesta del agente no válida: %w", err)
	}
	if !resp.OK {
		return nil, errors.New(resp.Error)
	}
	return resp.Lines, nil
}

// * Deadline es el tiempo máximo de una operación: conectar a una red Wi-Fi
// * puede tardar casi un minuto (autenticación, DHCP); el resto, segundos.
func Deadline(op string) time.Duration {
	switch op {
	case OpWifiConnect:
		return 90 * time.Second
	case OpWifiScan:
		return 45 * time.Second
	// * Calcular los mínimos de un sistema de archivos grande y validar una operación llevan unos segundos.
	case OpPartInfo, OpPartApply:
		return 3 * time.Minute
	}
	return 30 * time.Second
}

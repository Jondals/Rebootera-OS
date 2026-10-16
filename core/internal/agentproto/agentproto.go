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
)

// Request es una petición al agente. Device solo lo usan las operaciones de
// medios y el agente lo valida antes de usarlo.
type Request struct {
	Op     string `json:"op"`
	Device string `json:"device,omitempty"`
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
	conn.SetDeadline(time.Now().Add(30 * time.Second))

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

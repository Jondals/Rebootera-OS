// * Package wifi: lógica pura de Wi-Fi sin privilegios: análisis de la salida
// * de nmcli, validación de nombres y contraseñas y generación del perfil de
// * NetworkManager. El agente (root) es quien ejecuta nmcli y escribe el perfil.
// ! La contraseña nunca va en los argumentos de un programa (se vería en la
// ! lista de procesos): se escribe en un archivo 0600 que solo lee NetworkManager.
package wifi

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Tipos de seguridad que entiende Rebootera.
const (
	SecOpen       = "open"
	SecWPA        = "wpa"        // WPA/WPA2 personal (PSK)
	SecWPA3       = "wpa3"       // WPA3 personal (SAE)
	SecEnterprise = "enterprise" // 802.1X: no se admite
	SecUnsupFmt   = "unsupported"
)

// Network es una red Wi-Fi vista en un escaneo.
type Network struct {
	SSID     string `json:"ssid"`
	Signal   int    `json:"signal"`
	Security string `json:"security"`
	InUse    bool   `json:"inUse"`
}

// ProfileName es el nombre del único perfil que crea Rebootera.
const ProfileName = "rebootera-wifi"

// * splitTerse separa una línea de nmcli --terse: los campos van separados por
// * ':' y dentro de un campo los ':' y '\' se escapan con '\'.
func splitTerse(line string) []string {
	var fields []string
	var current strings.Builder
	escaped := false
	for _, r := range line {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == ':':
			fields = append(fields, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	return append(fields, current.String())
}

// * Classify reduce la columna SECURITY de nmcli a un tipo de Rebootera.
func Classify(security string) string {
	upper := strings.ToUpper(security)
	switch {
	case strings.TrimSpace(security) == "" || strings.TrimSpace(security) == "--":
		return SecOpen
	case strings.Contains(upper, "802.1X"):
		return SecEnterprise
	case strings.Contains(upper, "WEP"), strings.Contains(upper, "OWE"):
		return SecUnsupFmt
	case strings.Contains(upper, "WPA2"), strings.Contains(upper, "WPA1"):
		// * Las redes mixtas WPA2/WPA3 se configuran como WPA2 (PSK), que acepta cualquier adaptador.
		return SecWPA
	case strings.Contains(upper, "WPA3"), strings.Contains(upper, "SAE"):
		return SecWPA3
	case strings.Contains(upper, "WPA"):
		return SecWPA
	}
	return SecUnsupFmt
}

// * ParseList lee la salida de
// *   nmcli -t -f IN-USE,SSID,SIGNAL,SECURITY device wifi list
// * y devuelve una red por nombre (la de más señal), ordenadas de más a menos señal.
func ParseList(output string) []Network {
	best := map[string]Network{}
	for _, line := range strings.Split(output, "\n") {
		fields := splitTerse(strings.TrimRight(line, "\r"))
		if len(fields) < 4 || fields[1] == "" {
			continue
		}
		signal, _ := strconv.Atoi(fields[2])
		n := Network{SSID: fields[1], Signal: signal, Security: Classify(fields[3]), InUse: strings.TrimSpace(fields[0]) == "*"}
		if old, ok := best[n.SSID]; !ok || n.Signal > old.Signal {
			n.InUse = n.InUse || old.InUse
			best[n.SSID] = n
		} else if n.InUse {
			old.InUse = true
			best[n.SSID] = old
		}
	}
	list := make([]Network, 0, len(best))
	for _, n := range best {
		list = append(list, n)
	}
	slices.SortFunc(list, compareNetworks)
	return list
}

// * compareNetworks ordena por señal descendente y luego por nombre.
func compareNetworks(a, b Network) int {
	if a.Signal != b.Signal {
		return b.Signal - a.Signal
	}
	return strings.Compare(a.SSID, b.SSID)
}

// * ValidateSSID comprueba el nombre de una red: 1 a 32 bytes, sin NUL ni saltos de línea.
func ValidateSSID(ssid string) error {
	if len(ssid) == 0 || len(ssid) > 32 {
		return errors.New("el nombre de la red debe tener entre 1 y 32 caracteres")
	}
	if strings.ContainsAny(ssid, "\x00\r\n") {
		return errors.New("el nombre de la red tiene caracteres no válidos")
	}
	return nil
}

// * ValidatePassword comprueba la contraseña según el tipo de red: WPA admite
// * 8 a 63 caracteres ASCII imprimibles o 64 hexadecimales; una red abierta
// * no lleva contraseña.
func ValidatePassword(security, password string) error {
	switch security {
	case SecOpen:
		if password != "" {
			return errors.New("una red abierta no lleva contraseña")
		}
		return nil
	case SecWPA, SecWPA3:
		if len(password) == 64 && isHex(password) {
			if security == SecWPA3 {
				return errors.New("WPA3 no admite claves en hexadecimal")
			}
			return nil
		}
		if len(password) < 8 || len(password) > 63 {
			return errors.New("la contraseña debe tener entre 8 y 63 caracteres")
		}
		for _, r := range password {
			if r < 0x20 || r > 0x7e {
				return errors.New("la contraseña solo puede tener caracteres ASCII imprimibles")
			}
		}
		return nil
	}
	return errors.New("este tipo de red no se puede configurar desde Rebootera")
}

// * isHex indica si un texto es hexadecimal.
func isHex(s string) bool {
	for _, c := range strings.ToLower(s) {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

// * Keyfile genera el perfil de NetworkManager (formato keyfile). El nombre
// * de la red va como lista de bytes decimales, que admite cualquier valor, y
// * la contraseña con las barras invertidas escapadas.
func Keyfile(ssid, security, password, uuid string) (string, error) {
	if err := ValidateSSID(ssid); err != nil {
		return "", err
	}
	if err := ValidatePassword(security, password); err != nil {
		return "", err
	}
	var bytesList strings.Builder
	for _, b := range []byte(ssid) {
		bytesList.WriteString(strconv.Itoa(int(b)) + ";")
	}
	var out strings.Builder
	out.WriteString("[connection]\n")
	fmt.Fprintf(&out, "id=%s\nuuid=%s\ntype=wifi\nautoconnect=true\n\n", ProfileName, uuid)
	out.WriteString("[wifi]\nmode=infrastructure\n")
	fmt.Fprintf(&out, "ssid=%s\n\n", bytesList.String())
	switch security {
	case SecWPA:
		out.WriteString("[wifi-security]\nkey-mgmt=wpa-psk\n")
		fmt.Fprintf(&out, "psk=%s\n\n", strings.ReplaceAll(password, `\`, `\\`))
	case SecWPA3:
		out.WriteString("[wifi-security]\nkey-mgmt=sae\n")
		fmt.Fprintf(&out, "psk=%s\n\n", strings.ReplaceAll(password, `\`, `\\`))
	}
	out.WriteString("[ipv4]\nmethod=auto\n\n[ipv6]\nmethod=auto\naddr-gen-mode=default\n")
	return out.String(), nil
}

// Status es el estado del adaptador Wi-Fi.
type Status struct {
	// Adapter indica que hay al menos un adaptador Wi-Fi.
	Adapter    bool   `json:"adapter"`
	Device     string `json:"device"`
	State      string `json:"state"`
	Connection string `json:"connection"`
	// Blocked indica que rfkill tiene el Wi-Fi bloqueado.
	Blocked bool `json:"blocked"`
}

// * ParseStatus lee la salida de nmcli -t -f DEVICE,TYPE,STATE,CONNECTION device
// * y devuelve el primer adaptador Wi-Fi.
func ParseStatus(output string) Status {
	for _, line := range strings.Split(output, "\n") {
		fields := splitTerse(strings.TrimRight(line, "\r"))
		if len(fields) < 4 || fields[1] != "wifi" {
			continue
		}
		return Status{Adapter: true, Device: fields[0], State: fields[2], Connection: fields[3]}
	}
	return Status{}
}

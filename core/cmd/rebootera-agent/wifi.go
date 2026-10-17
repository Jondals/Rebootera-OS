//go:build linux

// * Operaciones Wi-Fi del agente: estado, búsqueda, conexión y desconexión
// * mediante NetworkManager (nmcli).
// ! Los argumentos de nmcli son fijos. El nombre y la contraseña de la red solo
// ! se escriben en un perfil 0600 de root (nunca en la línea de órdenes). El
// ! tipo de seguridad lo decide el agente con su propio escaneo, no el cliente.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"rebootera/core/internal/wifi"
)

// profilePath es donde NetworkManager lee el perfil que crea Rebootera.
const profilePath = "/etc/NetworkManager/system-connections/" + wifi.ProfileName + ".nmconnection"

// * nmcli ejecuta nmcli con argumentos fijos y un límite de tiempo.
func nmcli(timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "nmcli", args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("nmcli %s: %v: %s", args[0], err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// * lines separa una salida en líneas no vacías.
func lines(out string) []string {
	var result []string
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}
	return result
}

// * wifiStatus devuelve el estado de los dispositivos y del interruptor de radio.
func wifiStatus() ([]string, error) {
	radio, _ := nmcli(10*time.Second, "radio", "wifi")
	out, err := nmcli(10*time.Second, "-t", "-f", "DEVICE,TYPE,STATE,CONNECTION", "device")
	if err != nil {
		return nil, err
	}
	return append([]string{"radio:" + strings.TrimSpace(radio)}, lines(out)...), nil
}

// * wifiScan busca redes y devuelve la lista tal como la da nmcli.
func wifiScan() ([]string, error) {
	var found []string
	// * El primer escaneo tras encender la radio suele salir vacío: se repite
	// * un par de veces antes de dar la lista por vacía.
	for attempt := 0; attempt < 3; attempt++ {
		out, err := nmcli(25*time.Second, "-t", "-f", "IN-USE,SSID,SIGNAL,SECURITY", "device", "wifi", "list", "--rescan", "yes")
		if err != nil {
			return nil, err
		}
		found = lines(out)
		if len(found) > 0 {
			break
		}
		time.Sleep(4 * time.Second)
	}
	return found, nil
}

// * newUUID genera un UUID versión 4 para el perfil.
func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// * securityOf decide el tipo de seguridad de una red con un escaneo propio.
func securityOf(ssid string, hidden, hasPassword bool) (string, error) {
	scan, err := wifiScan()
	if err != nil {
		return "", err
	}
	for _, network := range wifi.ParseList(strings.Join(scan, "\n")) {
		if network.SSID == ssid {
			return network.Security, nil
		}
	}
	if !hidden {
		return "", errors.New("no se encuentra esa red; vuelve a buscar redes")
	}
	// * Una red oculta no aparece en el escaneo: se supone WPA si hay contraseña.
	if hasPassword {
		return wifi.SecWPA, nil
	}
	return wifi.SecOpen, nil
}

// * wifiConnect crea el perfil de la red y la conecta. Si falla, borra el
// * perfil para no recordar una contraseña equivocada.
func wifiConnect(ssid, password string, hidden bool) error {
	if err := wifi.ValidateSSID(ssid); err != nil {
		return err
	}
	status, err := wifiStatus()
	if err != nil {
		return err
	}
	if !wifi.ParseStatus(strings.Join(status, "\n")).Adapter {
		return errors.New("este equipo no tiene un adaptador Wi-Fi")
	}
	nmcli(10*time.Second, "radio", "wifi", "on")
	security, err := securityOf(ssid, hidden, password != "")
	if err != nil {
		return err
	}
	uuid, err := newUUID()
	if err != nil {
		return err
	}
	profile, err := wifi.Keyfile(ssid, security, password, uuid)
	if err != nil {
		return err
	}
	if err := writeProfile(profile); err != nil {
		return err
	}
	if _, err := nmcli(15*time.Second, "connection", "reload"); err != nil {
		removeProfile()
		return err
	}
	if _, err := nmcli(60*time.Second, "--wait", "45", "connection", "up", wifi.ProfileName); err != nil {
		removeProfile()
		return errors.New("no se pudo conectar: revisa la contraseña o acércate al punto de acceso")
	}
	return nil
}

// * writeProfile guarda el perfil con permisos 0600 de forma atómica.
func writeProfile(profile string) error {
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
		return err
	}
	tmp := profilePath + ".tmp"
	if err := os.WriteFile(tmp, []byte(profile), 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, profilePath)
}

// * removeProfile borra el perfil y avisa a NetworkManager.
func removeProfile() {
	os.Remove(profilePath)
	nmcli(15*time.Second, "connection", "reload")
}

// * wifiDisconnect desconecta y olvida la red de Rebootera.
func wifiDisconnect() error {
	nmcli(20*time.Second, "connection", "down", wifi.ProfileName)
	removeProfile()
	return nil
}

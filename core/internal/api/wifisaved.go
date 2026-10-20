package api

import (
	"log"
	"path/filepath"
	"strings"
	"time"

	"rebootera/core/internal/agentproto"
	"rebootera/core/internal/vault"
	"rebootera/core/internal/wifi"
)

// wifiFile guarda las redes recordadas dentro del estado del Vault.
const wifiFile = "wifi.json"

// maxSavedNetworks limita cuántas redes se recuerdan (las más recientes).
const maxSavedNetworks = 8

// savedNetwork es una red Wi-Fi recordada en el USB.
// ! La contraseña queda en el Vault del USB del propio usuario (exFAT no tiene permisos): por eso la interfaz deja
// ! elegir si se recuerda, y "Olvidar" la borra.
type savedNetwork struct {
	SSID     string `json:"ssid"`
	Password string `json:"password"`
	Hidden   bool   `json:"hidden,omitempty"`
}

// * loadSavedWifi lee las redes recordadas (la más reciente primero).
func loadSavedWifi() []savedNetwork {
	if !vault.Available() {
		return nil
	}
	var saved []savedNetwork
	if err := vault.ReadJSON(filepath.Join(vault.StateDir(), wifiFile), &saved); err != nil {
		return nil
	}
	valid := saved[:0]
	for _, network := range saved {
		if wifi.ValidateSSID(network.SSID) == nil && len(network.Password) <= 64 && !strings.ContainsAny(network.Password, "\r\n\x00") {
			valid = append(valid, network)
		}
	}
	return valid
}

// * saveWifi guarda la lista de redes recordadas.
func saveWifi(list []savedNetwork) {
	if !vault.Available() {
		return
	}
	if len(list) > maxSavedNetworks {
		list = list[:maxSavedNetworks]
	}
	if err := vault.WriteJSON(filepath.Join(vault.StateDir(), wifiFile), list); err != nil {
		log.Printf("wifi: no se pudo recordar la red: %v", err)
	}
}

// * rememberWifi pone una red la primera de las recordadas (sustituye a la que tenía el mismo nombre).
func rememberWifi(ssid, password string, hidden bool) {
	list := []savedNetwork{{SSID: ssid, Password: password, Hidden: hidden}}
	for _, network := range loadSavedWifi() {
		if network.SSID != ssid {
			list = append(list, network)
		}
	}
	saveWifi(list)
}

// * forgetWifi olvida todas las redes recordadas.
func forgetWifi() {
	saveWifi([]savedNetwork{})
}

// * pickSaved elige la primera red recordada que se ve en el escaneo (o una oculta recordada).
func pickSaved(saved []savedNetwork, visible []wifi.Network) (savedNetwork, bool) {
	for _, network := range saved {
		if network.Hidden {
			return network, true
		}
		for _, seen := range visible {
			if seen.SSID == network.SSID {
				return network, true
			}
		}
	}
	return savedNetwork{}, false
}

// * AutoConnectWifi conecta sola a una red recordada al arrancar, si no hay ya una conexión Wi-Fi. Espera a que el
// * Vault esté montado y a que aparezca el adaptador; se rinde a los pocos minutos.
func AutoConnectWifi(agent Agent) {
	deadline := time.Now().Add(4 * time.Minute)
	for attempt := 0; time.Now().Before(deadline) && attempt < 6; {
		time.Sleep(6 * time.Second)
		saved := loadSavedWifi()
		if len(saved) == 0 {
			continue
		}
		lines, err := agent.Query(agentproto.OpWifiStatus)
		if err != nil {
			continue
		}
		status := wifi.ParseStatus(strings.Join(lines, "\n"))
		if !status.Adapter {
			continue
		}
		if status.State == "connected" {
			return
		}
		scan, err := agent.Query(agentproto.OpWifiScan)
		if err != nil {
			continue
		}
		network, ok := pickSaved(saved, wifi.ParseList(strings.Join(scan, "\n")))
		if !ok {
			continue
		}
		attempt++
		_, err = agent.Send(agentproto.Request{Op: agentproto.OpWifiConnect, SSID: network.SSID, Password: network.Password, Hidden: network.Hidden})
		if err == nil {
			log.Printf("wifi: conectado a la red recordada %q", network.SSID)
			return
		}
		log.Printf("wifi: no se pudo conectar a %q: %v", network.SSID, err)
	}
}

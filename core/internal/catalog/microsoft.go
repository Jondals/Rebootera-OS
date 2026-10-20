// * Resolvedor "microsoft": pide a la web oficial de descargas de Microsoft el enlace temporal de la ISO de
// * Windows 10/11 en el idioma elegido, igual que hace su página (y herramientas como Rufus/Fido). Rebootera no
// * aloja ni redistribuye nada: la ISO se descarga directamente de los servidores de Microsoft.
// ! Microsoft no publica una suma por la API: la autenticidad la da HTTPS y que el enlace sea de un dominio
// ! de Microsoft (se comprueba). El enlace caduca en unas 24 horas, por eso se pide justo al descargar.
package catalog

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

// msBrowserAgent es el User-Agent de un navegador: la API de Microsoft rechaza los que no lo parecen.
const msBrowserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:128.0) Gecko/20100101 Firefox/128.0"

// Direcciones de la API de Microsoft; se cambian en las pruebas.
var (
	msSessionURL = "https://vlscppe.microsoft.com/tags?org_id=y6jn8c31&session_id="
	msScriptURL  = "https://ov-df.microsoft.com/mdt.js?instanceId=560dc9f3-1aa5-4a2f-b63c-9e18f8d0e175&PageId=si&session_id="
	msAPIURL     = "https://www.microsoft.com/software-download-connector/api/"
)

// msHostSuffix es el dominio al que tienen que pertenecer los enlaces de descarga (se cambia en las pruebas).
var msHostSuffix = ".microsoft.com"

// msProfile es el perfil público de la página de descargas de Microsoft.
const msProfile = "606624d44113"

// msFrame y msTicks sacan de mdt.js la dirección de la segunda llamada de la sesión y el valor de rticks.
var (
	msFrame = regexp.MustCompile(`url:"([^"]+)"`)
	msTicks = regexp.MustCompile(`rticks="\+(\d+)`)
)

// msGet hace una petición GET disfrazada de navegador y devuelve el cuerpo (como mucho 4 MB).
func msGet(rawURL, referer string) ([]byte, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", msBrowserAgent)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("no se pudo hablar con Microsoft: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Microsoft respondió %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 4<<20))
}

// msSession abre una sesión que la API de Microsoft acepta y devuelve su identificador.
func msSession() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	raw[6] = raw[6]&0x0f | 0x40
	raw[8] = raw[8]&0x3f | 0x80
	id := fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:])
	if _, err := msGet(msSessionURL+id, ""); err != nil {
		return "", err
	}
	script, err := msGet(msScriptURL+id, "")
	if err != nil {
		return "", err
	}
	frame := msFrame.FindSubmatch(script)
	ticks := msTicks.FindSubmatch(script)
	if frame == nil || ticks == nil {
		return "", fmt.Errorf("Microsoft ha cambiado su página de descargas (sesión)")
	}
	// * Segunda llamada de la sesión: la web de Microsoft la hace desde un marco oculto.
	link := fmt.Sprintf("%s&mdt=%d&rticks=%s", frame[1], nowMillis(), ticks[1])
	if _, err := msGet(link, ""); err != nil {
		return "", err
	}
	return id, nil
}

// msAPI llama a un punto de la API de descargas y decodifica su JSON.
func msAPI(endpoint string, query url.Values, referer string, into any) error {
	data, err := msGet(msAPIURL+endpoint+"?"+query.Encode(), referer)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, into)
}

// * resolveMicrosoft pide el enlace de la ISO: edición (producto), idioma y arquitectura salen del resolvedor.
func resolveMicrosoft(r Resolver, homepage string) (Resolved, error) {
	session, err := msSession()
	if err != nil {
		return Resolved{}, err
	}
	base := url.Values{
		"profile":          {msProfile},
		"productEditionId": {r.Edition},
		"SKU":              {"undefined"},
		"friendlyFileName": {"undefined"},
		"Locale":           {"en-US"},
		"sessionID":        {session},
	}
	var skus struct {
		Skus []struct{ Id, Language string }
	}
	if err := msAPI("getskuinformationbyproductedition", base, homepage, &skus); err != nil {
		return Resolved{}, err
	}
	sku := ""
	for _, candidate := range skus.Skus {
		if candidate.Language == r.Language {
			sku = candidate.Id
		}
	}
	if sku == "" {
		return Resolved{}, fmt.Errorf("Microsoft no ofrece Windows en %q", r.Language)
	}
	base.Set("productEditionId", "undefined")
	base.Set("SKU", sku)
	var links struct {
		Options []struct {
			Uri          string `json:"Uri"`
			DownloadType int    `json:"DownloadType"`
		} `json:"ProductDownloadOptions"`
		Errors []struct{ Value string }
	}
	if err := msAPI("GetProductDownloadLinksBySku", base, homepage, &links); err != nil {
		return Resolved{}, err
	}
	for _, option := range links.Options {
		// * 1 es x64 (0 es x86 y 2 es ARM64).
		if option.DownloadType != 1 && len(links.Options) > 1 {
			continue
		}
		return msResolved(option.Uri)
	}
	if len(links.Errors) > 0 {
		return Resolved{}, fmt.Errorf("Microsoft rechazó la petición (%s): prueba de nuevo en unos minutos", links.Errors[0].Value)
	}
	return Resolved{}, fmt.Errorf("Microsoft no devolvió ningún enlace")
}

// msResolved comprueba que el enlace es de Microsoft y saca de él el nombre del archivo.
func msResolved(rawURL string) (Resolved, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || !strings.HasSuffix(parsed.Hostname(), msHostSuffix) {
		return Resolved{}, fmt.Errorf("Microsoft devolvió un enlace que no es suyo: se descarta")
	}
	name := path.Base(parsed.Path)
	if !strings.HasSuffix(strings.ToLower(name), ".iso") {
		return Resolved{}, fmt.Errorf("Microsoft devolvió algo que no es una ISO (%s)", name)
	}
	return Resolved{URL: rawURL, FileName: name}, nil
}

// nowMillis es la hora actual en milisegundos desde 1970 (lo que Microsoft espera en la sesión).
func nowMillis() int64 {
	return time.Now().UnixMilli()
}

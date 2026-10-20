// * Servidor de la interfaz: sirve los archivos de la UI y, en la página principal, deja escritos el tema y el
// * idioma guardados en el Vault para que la intro y el primer fotograma ya salgan con ellos (sin parpadear al tema
// * por defecto mientras la interfaz pregunta a la API).
package api

import (
	"bytes"
	"errors"
	"net/http"
	"regexp"
	"time"

	"rebootera/core/internal/boot"
	"rebootera/core/internal/vault"
)

// htmlTag es la etiqueta <html> de index.html, con su idioma y tema por defecto.
var htmlTag = regexp.MustCompile(`<html lang="[a-z]{2}" data-theme="[a-z]+"`)

// hashedAsset reconoce los archivos de JavaScript cuyo nombre lleva una huella de su contenido.
var hashedAsset = regexp.MustCompile(`^/js/[a-z-]+-[A-Za-z0-9]{8}[.]js$`)

// safeWord limita lo que se escribe en la página a letras minúsculas: el archivo de ajustes está en un USB y
// podría haberse editado a mano.
var safeWord = regexp.MustCompile(`^[a-z]{2,12}$`)

// * uiHandler devuelve el manejador de los archivos de la interfaz.
func (s *Server) uiHandler() http.Handler {
	files := http.FileServer(http.FS(s.ui))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			// * Los archivos de JavaScript con huella en el nombre (chunk-AB12CD34.js) no cambian nunca: se guardan para siempre.
			if hashedAsset.MatchString(r.URL.Path) {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
			return
		}
		page, err := readUIFile(s, "index.html")
		if err != nil {
			files.ServeHTTP(w, r)
			return
		}
		// * Sin Vault (sesión de solo lectura) o sin ajustes guardados se queda lo que trae la página.
		// * data-saved le dice a la interfaz que estos valores vienen de los ajustes guardados y mandan sobre el almacenamiento
		// * del navegador (que en Rebootera Desktop se pierde en cada arranque, porque el puerto local cambia).
		settings, persistent, err := s.currentSettings()
		if err == nil && persistent && safeWord.MatchString(settings.Language) && safeWord.MatchString(settings.Theme) {
			tag := []byte(`<html lang="` + settings.Language + `" data-theme="` + settings.Theme + `" data-saved="1"`)
			page = htmlTag.ReplaceAll(page, tag)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(page))
	})
}

// * currentSettings devuelve los ajustes con los que se debe dibujar la interfaz y si se guardan de verdad (persistent). En
// * Rebootera OS salen del Vault del USB; en Rebootera Desktop, del Vault si hay un USB de Rebootera y, si no, del archivo
// * de ajustes del PC. Sin ninguno de los dos se devuelven los de por defecto.
func (s *Server) currentSettings() (settings vault.Settings, persistent bool, err error) {
	settings, err = vault.LoadSettings()
	if err == nil {
		return settings, true, nil
	}
	if !errors.Is(err, vault.ErrUnavailable) {
		return settings, false, err
	}
	if s.mode() == ModeDesktop {
		if local, _, ok := vault.LoadLocalSettings(); ok {
			return local, true, nil
		}
	}
	return settings, false, nil
}

// * SyncSettings pone de acuerdo los ajustes del PC y los del USB de Rebootera cuando se elige uno (Rebootera Desktop): gana
// * el que se guardó más tarde. Así el tema elegido en el escritorio llega al USB y, al arrancar de él, a Rebootera OS; y lo
// * elegido en Rebootera OS vuelve al escritorio. Sin USB o fuera del escritorio, no hace nada.
func SyncSettings() {
	local, localTime, hasLocal := vault.LoadLocalSettings()
	usbTime := vault.VaultSettingsTime()
	if !vault.Available() || !hasLocal && usbTime.IsZero() {
		return
	}
	switch {
	case hasLocal && (usbTime.IsZero() || localTime.After(usbTime)):
		if vault.SaveSettings(local) == nil {
			boot.ApplyTheme(local.Theme)
		}
	case !usbTime.IsZero() && (!hasLocal || usbTime.After(localTime)):
		if usb, err := vault.LoadSettings(); err == nil {
			_ = vault.SaveLocalSettings(usb)
		}
	}
}

// * readUIFile lee un archivo de la interfaz.
func readUIFile(s *Server, name string) ([]byte, error) {
	file, err := s.ui.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(file); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

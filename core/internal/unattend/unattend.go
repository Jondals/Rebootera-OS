// * Package unattend: genera las respuestas de una instalación desatendida
// * parcial: idioma, teclado, zona horaria, cuenta de usuario y, en Windows,
// * la ruta de los drivers inyectados. Nunca configura particiones ni borra
// * discos: el usuario sigue eligiendo siempre dónde se instala el sistema.
// * Windows usa autounattend.xml y Debian usa preseed; ambos se entregan al
// * instalador a través del plugin auto_install de Ventoy.
// ! La contraseña se guarda como hash (Debian) o codificada en base64 (Windows,
// ! formato propio de Microsoft, reversible). Los archivos se borran del USB en
// ! cuanto Rebootera OS vuelve a arrancar.
package unattend

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"
	"text/template"
	"unicode/utf16"
)

// Options son las respuestas que da el usuario en el asistente.
type Options struct {
	// Language es el idioma del sistema instalado ("es", "en", "gb", "de", "fr", "it", "pt").
	Language string `json:"language"`
	// TimeZone es una zona IANA de la lista de este paquete; vacía = que decida el instalador.
	TimeZone string `json:"timeZone"`
	// UserName y Password crean la cuenta; si UserName está vacío no se crea ninguna.
	UserName string `json:"userName"`
	FullName string `json:"fullName"`
	Password string `json:"password"`
	// Drivers indica que hay un archivo de drivers inyectado en el instalador de Windows.
	Drivers bool `json:"drivers"`
	// ReduceTelemetry (solo Windows) desactiva la recogida de datos de diagnóstico y los anuncios al
	// instalar: directivas de DataCollection a 0, servicios DiagTrack y dmwappushservice desactivados,
	// sin contenido sugerido y sin la pantalla de "configuración rápida". En Enterprise, Education y LTSC
	// el nivel 0 ("Security") se respeta; en Home y Pro Windows lo trata como el nivel básico.
	ReduceTelemetry bool `json:"privacy"`
	// DisableBitLocker (solo Windows) evita el cifrado automático del disco (BitLocker / cifrado de dispositivo).
	DisableBitLocker bool `json:"bitlocker"`
	// QoL (solo Windows) quita de serie Copilot, Recall, OneDrive, Widgets, el Outlook nuevo y el inicio rápido.
	QoL bool `json:"qol"`
}

// privacyCommands son las órdenes que se ejecutan en la fase specialize (como SYSTEM, antes del primer
// inicio de sesión) cuando se pide reducir la telemetría.
var privacyCommands = []string{
	`reg add "HKLM\SOFTWARE\Policies\Microsoft\Windows\DataCollection" /v AllowTelemetry /t REG_DWORD /d 0 /f`,
	`reg add "HKLM\SOFTWARE\Policies\Microsoft\Windows\DataCollection" /v DoNotShowFeedbackNotifications /t REG_DWORD /d 1 /f`,
	`reg add "HKLM\SOFTWARE\Policies\Microsoft\Windows\DataCollection" /v AllowDeviceNameInTelemetry /t REG_DWORD /d 0 /f`,
	`reg add "HKLM\SOFTWARE\Policies\Microsoft\Windows\CloudContent" /v DisableWindowsConsumerFeatures /t REG_DWORD /d 1 /f`,
	`reg add "HKLM\SOFTWARE\Policies\Microsoft\Windows\AdvertisingInfo" /v DisabledByGroupPolicy /t REG_DWORD /d 1 /f`,
	`reg add "HKLM\SOFTWARE\Microsoft\Windows\Windows Error Reporting" /v Disabled /t REG_DWORD /d 1 /f`,
	`sc config DiagTrack start= disabled`,
	`sc config dmwappushservice start= disabled`,
}

// bitLockerCommands impiden el cifrado automático de dispositivo al terminar la instalación.
var bitLockerCommands = []string{
	`reg add "HKLM\SYSTEM\CurrentControlSet\Control\BitLocker" /v PreventDeviceEncryption /t REG_DWORD /d 1 /f`,
}

// qolCommands son las mejoras de "calidad de vida": directivas de equipo, así que valen para todos los usuarios.
var qolCommands = []string{
	`reg add "HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsCopilot" /v TurnOffWindowsCopilot /t REG_DWORD /d 1 /f`,
	`reg add "HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsAI" /v DisableAIDataAnalysis /t REG_DWORD /d 1 /f`,
	`reg add "HKLM\SOFTWARE\Policies\Microsoft\Windows\OneDrive" /v DisableFileSyncNGSC /t REG_DWORD /d 1 /f`,
	`reg add "HKLM\SOFTWARE\Policies\Microsoft\Dsh" /v AllowNewsAndInterests /t REG_DWORD /d 0 /f`,
	`reg add "HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Power" /v HiberbootEnabled /t REG_DWORD /d 0 /f`,
	`reg delete "HKLM\SOFTWARE\Microsoft\WindowsUpdate\Orchestrator\UScheduler_Oobe\OutlookUpdate" /f`,
	`reg delete "HKLM\SOFTWARE\Microsoft\WindowsUpdate\Orchestrator\UScheduler_Oobe\DevHomeUpdate" /f`,
}

// commands devuelve las órdenes de la fase specialize que piden las opciones, en este orden.
func (o Options) commands() []string {
	var out []string
	if o.ReduceTelemetry {
		out = append(out, privacyCommands...)
	}
	if o.DisableBitLocker {
		out = append(out, bitLockerCommands...)
	}
	if o.QoL {
		out = append(out, qolCommands...)
	}
	return out
}

// locale agrupa los códigos de un idioma en cada sistema.
type locale struct {
	windows    string // es-ES
	inputs     string // 0c0a:0000040a
	linux      string // es_ES.UTF-8
	keymap     string // es
	debianLang string // es
	country    string // ES (código ISO del país que ofrece el instalador de Debian)
}

var locales = map[string]locale{
	"es": {"es-ES", "0c0a:0000040a", "es_ES.UTF-8", "es", "es", "ES"},
	"en": {"en-US", "0409:00000409", "en_US.UTF-8", "us", "en", "US"},
	"gb": {"en-GB", "0809:00000809", "en_GB.UTF-8", "gb", "en", "GB"},
	"de": {"de-DE", "0407:00000407", "de_DE.UTF-8", "de", "de", "DE"},
	"fr": {"fr-FR", "040c:0000040c", "fr_FR.UTF-8", "fr", "fr", "FR"},
	"it": {"it-IT", "0410:00000410", "it_IT.UTF-8", "it", "it", "IT"},
	"pt": {"pt-PT", "0816:00000816", "pt_PT.UTF-8", "pt", "pt", "PT"},
}

// TimeZones relaciona las zonas IANA que ofrece la interfaz con el nombre de
// zona de Windows. Linux usa directamente el nombre IANA.
var TimeZones = map[string]string{
	"UTC":                            "UTC",
	"Europe/Madrid":                  "Romance Standard Time",
	"Atlantic/Canary":                "GMT Standard Time",
	"Europe/London":                  "GMT Standard Time",
	"Europe/Lisbon":                  "GMT Standard Time",
	"Europe/Paris":                   "Romance Standard Time",
	"Europe/Berlin":                  "W. Europe Standard Time",
	"Europe/Rome":                    "W. Europe Standard Time",
	"Europe/Amsterdam":               "W. Europe Standard Time",
	"Europe/Athens":                  "GTB Standard Time",
	"Europe/Moscow":                  "Russian Standard Time",
	"America/New_York":               "Eastern Standard Time",
	"America/Chicago":                "Central Standard Time",
	"America/Denver":                 "Mountain Standard Time",
	"America/Los_Angeles":            "Pacific Standard Time",
	"America/Mexico_City":            "Central Standard Time (Mexico)",
	"America/Bogota":                 "SA Pacific Standard Time",
	"America/Lima":                   "SA Pacific Standard Time",
	"America/Santiago":               "Pacific SA Standard Time",
	"America/Argentina/Buenos_Aires": "Argentina Standard Time",
	"America/Sao_Paulo":              "E. South America Standard Time",
	"Asia/Tokyo":                     "Tokyo Standard Time",
	"Asia/Shanghai":                  "China Standard Time",
	"Asia/Kolkata":                   "India Standard Time",
	"Australia/Sydney":               "AUS Eastern Standard Time",
}

// userPattern es lo que acepta un nombre de usuario: minúsculas, números,
// guion y guion bajo, empezando por una letra (válido en Windows y Linux).
var userPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

// reservedUsers son nombres que no se pueden usar.
var reservedUsers = map[string]bool{"root": true, "administrator": true, "admin": true, "guest": true, "system": true, "daemon": true}

// * Validate comprueba las opciones antes de generar nada.
func (o Options) Validate() error {
	// * Idioma vacío (solo Windows): no se toca el idioma ni el teclado del instalador.
	if _, ok := locales[o.Language]; !ok && o.Language != "" {
		return fmt.Errorf("idioma no admitido: %q", o.Language)
	}
	if o.TimeZone != "" {
		if _, ok := TimeZones[o.TimeZone]; !ok {
			return fmt.Errorf("zona horaria no admitida: %q", o.TimeZone)
		}
	}
	if o.UserName != "" {
		if !userPattern.MatchString(o.UserName) || reservedUsers[o.UserName] {
			return errors.New("nombre de usuario no válido (minúsculas, números, - y _, empezando por una letra)")
		}
		if o.Password == "" {
			return errors.New("la cuenta de usuario necesita una contraseña")
		}
		if len(o.Password) > 128 || strings.ContainsAny(o.Password, "\r\n\x00") {
			return errors.New("contraseña no válida")
		}
	}
	if len(o.FullName) > 64 || strings.ContainsAny(o.FullName, "\r\n\x00") {
		return errors.New("nombre completo no válido")
	}
	return nil
}

// * Enabled indica si hay algo que responder además del idioma (que siempre se pone).
func (o Options) fullName() string {
	if o.FullName != "" {
		return o.FullName
	}
	return o.UserName
}

// * windowsPassword codifica una contraseña como la espera autounattend.xml:
// * base64 de UTF-16LE de contraseña + "Password", con PlainText=false.
func windowsPassword(password string) string {
	units := utf16.Encode([]rune(password + "Password"))
	raw := make([]byte, 0, len(units)*2)
	for _, u := range units {
		raw = binary.LittleEndian.AppendUint16(raw, u)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

// windowsTemplate es el autounattend.xml. Solo toca idioma, zona horaria,
// drivers y la cuenta local; no incluye DiskConfiguration.
var windowsTemplate = template.Must(template.New("autounattend").Funcs(template.FuncMap{
	"esc": html.EscapeString,
	"inc": plusOne,
}).Parse(`<?xml version="1.0" encoding="utf-8"?>
<unattend xmlns="urn:schemas-microsoft-com:unattend">
  <settings pass="windowsPE">
{{- if .Locale.Windows}}
    <component name="Microsoft-Windows-International-Core-WinPE" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS" xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
      <SetupUILanguage>
        <UILanguage>{{.Locale.Windows}}</UILanguage>
      </SetupUILanguage>
      <InputLocale>{{.Locale.Inputs}}</InputLocale>
      <SystemLocale>{{.Locale.Windows}}</SystemLocale>
      <UILanguage>{{.Locale.Windows}}</UILanguage>
      <UserLocale>{{.Locale.Windows}}</UserLocale>
    </component>
{{- end}}
{{- if .Drivers}}
    <component name="Microsoft-Windows-PnpCustomizationsWinPE" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS" xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
      <DriverPaths>
        <PathAndCredentials wcm:action="add" wcm:keyValue="1">
          <Path>{{.DriverPath}}</Path>
        </PathAndCredentials>
      </DriverPaths>
    </component>
{{- end}}
  </settings>
{{- if .Drivers}}
  <settings pass="offlineServicing">
    <component name="Microsoft-Windows-PnpCustomizationsNonWinPE" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS" xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
      <DriverPaths>
        <PathAndCredentials wcm:action="add" wcm:keyValue="1">
          <Path>{{.DriverPath}}</Path>
        </PathAndCredentials>
      </DriverPaths>
    </component>
  </settings>
{{- end}}
{{- if or .WindowsZone .Commands}}
  <settings pass="specialize">
{{- if .WindowsZone}}
    <component name="Microsoft-Windows-Shell-Setup" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS" xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
      <TimeZone>{{.WindowsZone}}</TimeZone>
    </component>
{{- end}}
{{- if .Commands}}
    <component name="Microsoft-Windows-Deployment" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS" xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
      <RunSynchronous>
{{- range $i, $cmd := .Commands}}
        <RunSynchronousCommand wcm:action="add">
          <Order>{{inc $i}}</Order>
          <Path>{{esc $cmd}}</Path>
        </RunSynchronousCommand>
{{- end}}
      </RunSynchronous>
    </component>
{{- end}}
  </settings>
{{- end}}
  <settings pass="oobeSystem">
{{- if .Locale.Windows}}
    <component name="Microsoft-Windows-International-Core" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS" xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
      <InputLocale>{{.Locale.Inputs}}</InputLocale>
      <SystemLocale>{{.Locale.Windows}}</SystemLocale>
      <UILanguage>{{.Locale.Windows}}</UILanguage>
      <UserLocale>{{.Locale.Windows}}</UserLocale>
    </component>
{{- end}}
{{- if or .UserName .PrivacyCommands}}
    <component name="Microsoft-Windows-Shell-Setup" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS" xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
      <OOBE>
        <HideEULAPage>true</HideEULAPage>
{{- if .UserName}}
        <HideOnlineAccountScreens>true</HideOnlineAccountScreens>
{{- end}}
        <ProtectYourPC>3</ProtectYourPC>
      </OOBE>
{{- if .UserName}}
      <UserAccounts>
        <LocalAccounts>
          <LocalAccount wcm:action="add">
            <Name>{{.UserName}}</Name>
            <DisplayName>{{.DisplayName}}</DisplayName>
            <Group>Administrators</Group>
            <Password>
              <Value>{{.PasswordB64}}</Value>
              <PlainText>false</PlainText>
            </Password>
          </LocalAccount>
        </LocalAccounts>
      </UserAccounts>
{{- end}}
    </component>
{{- end}}
  </settings>
</unattend>
`))

// plusOne suma uno (el Order de las órdenes empieza en 1).
func plusOne(i int) int { return i + 1 }

// * WindowsAnswerFile devuelve el autounattend.xml para las opciones dadas.
func WindowsAnswerFile(o Options) (string, error) {
	if err := o.Validate(); err != nil {
		return "", err
	}
	var privacy []string
	if o.ReduceTelemetry {
		privacy = privacyCommands
	}
	data := map[string]any{
		"PrivacyCommands": privacy,
		"Commands":        o.commands(),
		"Locale":          struct{ Windows, Inputs string }{locales[o.Language].windows, locales[o.Language].inputs},
		"Drivers":         o.Drivers,
		"DriverPath":      `X:\rebootera-drivers`,
		"WindowsZone":     TimeZones[o.TimeZone],
		"UserName":        o.UserName,
		"DisplayName":     html.EscapeString(o.fullName()),
		"PasswordB64":     "",
	}
	if o.UserName != "" {
		data["PasswordB64"] = windowsPassword(o.Password)
	}
	var out bytes.Buffer
	if err := windowsTemplate.Execute(&out, data); err != nil {
		return "", err
	}
	return out.String(), nil
}

// * Preseed devuelve el archivo de respuestas de Debian para las opciones
// * dadas. No incluye particionado: el instalador pregunta siempre el disco.
func Preseed(o Options) (string, error) {
	if err := o.Validate(); err != nil {
		return "", err
	}
	if o.Language == "" {
		return "", errors.New("falta el idioma")
	}
	l := locales[o.Language]
	var out strings.Builder
	out.WriteString("# Generado por Rebootera OS. Solo responde idioma, teclado, zona horaria y cuenta.\n")
	out.WriteString("# El disco de destino lo elige siempre el usuario.\n")
	fmt.Fprintf(&out, "d-i debian-installer/language string %s\n", l.debianLang)
	fmt.Fprintf(&out, "d-i debian-installer/locale string %s\n", l.linux)
	fmt.Fprintf(&out, "d-i debian-installer/country string %s\n", l.country)
	fmt.Fprintf(&out, "d-i keyboard-configuration/xkb-keymap select %s\n", l.keymap)
	if o.TimeZone != "" {
		fmt.Fprintf(&out, "d-i clock-setup/utc boolean true\nd-i time/zone string %s\n", o.TimeZone)
	}
	if o.UserName != "" {
		hash, err := HashPassword(o.Password)
		if err != nil {
			return "", err
		}
		out.WriteString("d-i passwd/root-login boolean false\n")
		fmt.Fprintf(&out, "d-i passwd/user-fullname string %s\n", oneLine(o.fullName()))
		fmt.Fprintf(&out, "d-i passwd/username string %s\n", o.UserName)
		fmt.Fprintf(&out, "d-i passwd/user-password-crypted password %s\n", hash)
	}
	return out.String(), nil
}

// * oneLine deja un texto en una sola línea para un archivo de respuestas.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

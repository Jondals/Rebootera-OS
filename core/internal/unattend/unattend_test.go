// * Pruebas de las respuestas desatendidas y del hash de contraseña.
package unattend

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

// * TestCryptVectors comprueba sha512-crypt con los vectores de prueba del
// * documento de referencia de Drepper.
func TestCryptVectors(t *testing.T) {
	cases := []struct {
		password, salt string
		rounds         int
		want           string
	}{
		{"Hello world!", "saltstring", 5000,
			"$6$saltstring$svn8UoSVapNtMuq1ukKS4tPQd8iKwSMHWjl/O817G3uBnIFNjnQJuesI68u4OTLiBFdcbYEdFCoEOfaS35inz1"},
		{"Hello world!", "saltstringsaltstring", 10000, ""},
	}
	got, err := sha512Crypt(cases[0].password, cases[0].salt, cases[0].rounds)
	if err != nil || got != cases[0].want {
		t.Fatalf("vector 1: %q %v", got, err)
	}
	// * Una sal de más de 16 caracteres se rechaza (el estándar la recorta).
	if _, err := sha512Crypt("x", cases[1].salt, 5000); err == nil {
		t.Fatal("debería rechazar una sal demasiado larga")
	}
	hash, err := HashPassword("secreto")
	if err != nil || !strings.HasPrefix(hash, "$6$") || len(hash) < 100 {
		t.Fatalf("hash: %q %v", hash, err)
	}
	other, _ := HashPassword("secreto")
	if hash == other {
		t.Fatal("la sal debe ser aleatoria")
	}
}

// * wellFormed comprueba que un texto es XML bien formado.
func wellFormed(t *testing.T, text string) {
	t.Helper()
	decoder := xml.NewDecoder(strings.NewReader(text))
	for {
		if _, err := decoder.Token(); err == io.EOF {
			return
		} else if err != nil {
			t.Fatalf("XML mal formado: %v\n%s", err, text)
		}
	}
}

// * TestWindowsAnswerFile comprueba el autounattend.xml.
func TestWindowsAnswerFile(t *testing.T) {
	xmlText, err := WindowsAnswerFile(Options{
		Language: "es", TimeZone: "Europe/Madrid", UserName: "ana", FullName: `Ana <"Test"> & Co`,
		Password: "clave", Drivers: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	wellFormed(t, xmlText)
	for _, want := range []string{
		`es-ES`, `0c0a:0000040a`, `Romance Standard Time`, `X:\rebootera-drivers`,
		`Microsoft-Windows-PnpCustomizationsWinPE`, `Microsoft-Windows-PnpCustomizationsNonWinPE`,
		`<Name>ana</Name>`, `Ana &lt;&#34;Test&#34;&gt; &amp; Co`, `<PlainText>false</PlainText>`,
	} {
		if !strings.Contains(xmlText, want) {
			t.Fatalf("falta %q en el XML:\n%s", want, xmlText)
		}
	}
	if strings.Contains(xmlText, "clave</Value>") {
		t.Fatal("la contraseña no debe ir en claro")
	}
	// * Sin disco: el instalador nunca recibe una configuración de particiones.
	if strings.Contains(xmlText, "DiskConfiguration") || strings.Contains(xmlText, "WillWipeDisk") {
		t.Fatal("el archivo de respuestas no debe tocar los discos")
	}
}

// * TestWindowsMinimal comprueba que sin opciones solo quedan los idiomas.
func TestWindowsMinimal(t *testing.T) {
	xmlText, err := WindowsAnswerFile(Options{Language: "en"})
	if err != nil {
		t.Fatal(err)
	}
	wellFormed(t, xmlText)
	if strings.Contains(xmlText, "UserAccounts") || strings.Contains(xmlText, "DriverPaths") {
		t.Fatalf("no debería crear cuentas ni rutas de drivers:\n%s", xmlText)
	}
	if _, err := WindowsAnswerFile(Options{Language: "xx"}); err == nil {
		t.Fatal("un idioma desconocido debe dar error")
	}
}

// * TestPreseed comprueba el preseed de Debian.
func TestPreseed(t *testing.T) {
	text, err := Preseed(Options{Language: "es", TimeZone: "Europe/Madrid", UserName: "ana", FullName: "Ana", Password: "clave"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"d-i debian-installer/locale string es_ES.UTF-8",
		"d-i debian-installer/country string ES",
		"d-i keyboard-configuration/xkb-keymap select es",
		"d-i time/zone string Europe/Madrid",
		"d-i passwd/username string ana",
		"d-i passwd/root-login boolean false",
		"d-i passwd/user-password-crypted password $6$",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("falta %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "clave\n") || strings.Contains(text, "partman") {
		t.Fatalf("no debe incluir la contraseña en claro ni particionado:\n%s", text)
	}
}

// * TestInvalidUser rechaza nombres de usuario peligrosos o ilegales.
func TestInvalidUser(t *testing.T) {
	for _, name := range []string{"Root", "a b", "x;rm", "", "-a", strings.Repeat("a", 40), "ñu"} {
		if err := (Options{Language: "en", UserName: name, Password: "x"}).Validate(); name != "" && err == nil {
			t.Fatalf("debería rechazar el usuario %q", name)
		}
	}
	if err := (Options{Language: "en", UserName: "ana"}).Validate(); err == nil {
		t.Fatal("un usuario sin contraseña debe rechazarse")
	}
	if err := (Options{Language: "en", UserName: "ana", Password: "x", TimeZone: "No/Existe"}).Validate(); err == nil {
		t.Fatal("una zona horaria desconocida debe rechazarse")
	}
}

// * TestWindowsPrivacy comprueba la reducción de telemetría: órdenes en specialize, sin pantalla de
// * configuración rápida y sin tocar el idioma cuando no se pide.
func TestWindowsPrivacy(t *testing.T) {
	xmlText, err := WindowsAnswerFile(Options{ReduceTelemetry: true})
	if err != nil {
		t.Fatal(err)
	}
	wellFormed(t, xmlText)
	for _, want := range []string{
		`pass="specialize"`, `Microsoft-Windows-Deployment`, `AllowTelemetry /t REG_DWORD /d 0`,
		`sc config DiagTrack start= disabled`, `<Order>1</Order>`, `<Order>8</Order>`, `<ProtectYourPC>3</ProtectYourPC>`,
	} {
		if !strings.Contains(xmlText, want) {
			t.Fatalf("falta %q en el XML:\n%s", want, xmlText)
		}
	}
	if strings.Contains(xmlText, "International-Core") || strings.Contains(xmlText, "UserAccounts") {
		t.Fatalf("sin idioma ni cuenta no debe tocar ninguno de los dos:\n%s", xmlText)
	}
	if strings.Contains(xmlText, "DiskConfiguration") {
		t.Fatal("nunca debe tocar los discos")
	}
	// * Con idioma y cuenta conviven con las órdenes.
	full, err := WindowsAnswerFile(Options{Language: "es", UserName: "ana", Password: "clave", ReduceTelemetry: true})
	if err != nil {
		t.Fatal(err)
	}
	wellFormed(t, full)
	if !strings.Contains(full, "es-ES") || !strings.Contains(full, "<Name>ana</Name>") || !strings.Contains(full, "DiagTrack") {
		t.Fatalf("faltan partes en el XML combinado:\n%s", full)
	}
	// * Sin la opción no aparece nada de telemetría.
	plain, _ := WindowsAnswerFile(Options{Language: "en"})
	if strings.Contains(plain, "DataCollection") {
		t.Fatal("sin la opción no debe haber órdenes de telemetría")
	}
	if _, err := Preseed(Options{}); err == nil {
		t.Fatal("el preseed necesita idioma")
	}
}

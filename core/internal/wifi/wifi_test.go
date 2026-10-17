// * Pruebas del análisis de nmcli, la validación y el perfil de NetworkManager.
package wifi

import (
	"strings"
	"testing"
)

// * TestParseList comprueba escapes, duplicados, ocultas y orden.
func TestParseList(t *testing.T) {
	out := strings.Join([]string{
		`*:Casa\: 5G:81:WPA2`,
		`:Casa\: 5G:40:WPA2`,
		`:Cafe:55:--`,
		`:Oficina:70:WPA2 802.1X`,
		`:Nueva:62:WPA3`,
		`:Mixta:50:WPA2 WPA3`,
		`::90:WPA2`,
		`:Vieja:30:WEP`,
		`basura`,
	}, "\n")
	list := ParseList(out)
	want := []struct {
		ssid, sec string
		inUse     bool
	}{
		{"Casa: 5G", SecWPA, true},
		{"Oficina", SecEnterprise, false},
		{"Nueva", SecWPA3, false},
		{"Cafe", SecOpen, false},
		{"Mixta", SecWPA, false},
		{"Vieja", SecUnsupFmt, false},
	}
	if len(list) != len(want) {
		t.Fatalf("esperaba %d redes, obtuve %+v", len(want), list)
	}
	for i, w := range want {
		if list[i].SSID != w.ssid || list[i].Security != w.sec || list[i].InUse != w.inUse {
			t.Fatalf("red %d: %+v, esperaba %+v", i, list[i], w)
		}
	}
	if list[0].Signal != 81 {
		t.Fatalf("debe quedarse con la señal más fuerte: %d", list[0].Signal)
	}
}

// * TestValidate comprueba nombres y contraseñas.
func TestValidate(t *testing.T) {
	for _, bad := range []string{"", strings.Repeat("a", 33), "a\nb", "a\x00b"} {
		if ValidateSSID(bad) == nil {
			t.Fatalf("SSID %q debería rechazarse", bad)
		}
	}
	if ValidateSSID("-Wi-Fi ñ") != nil {
		t.Fatal("un SSID con guion inicial y acentos es válido")
	}
	cases := []struct {
		sec, pw string
		ok      bool
	}{
		{SecOpen, "", true}, {SecOpen, "x", false},
		{SecWPA, "12345678", true}, {SecWPA, "1234567", false}, {SecWPA, strings.Repeat("a", 63), true},
		{SecWPA, strings.Repeat("a", 64), true}, {SecWPA, strings.Repeat("z", 64), false},
		{SecWPA, "clave\ncon salto", false}, {SecWPA, "contraseña1", false},
		{SecWPA3, strings.Repeat("a", 64), false}, {SecEnterprise, "12345678", false},
	}
	for _, c := range cases {
		err := ValidatePassword(c.sec, c.pw)
		if c.ok != (err == nil) {
			t.Fatalf("%s %q: %v", c.sec, c.pw, err)
		}
	}
}

// * TestKeyfile comprueba el perfil: bytes del SSID, escapes y ausencia de argumentos.
func TestKeyfile(t *testing.T) {
	text, err := Keyfile("Café;`$x", SecWPA, `pa\ss;word1`, "11111111-2222-3333-4444-555555555555")
	if err != nil {
		t.Fatal(err)
	}
	// * "Café;`$x" en UTF-8: 67 97 102 195 169 59 96 36 120.
	for _, want := range []string{
		"id=rebootera-wifi", "type=wifi", "ssid=67;97;102;195;169;59;96;36;120;", "key-mgmt=wpa-psk",
		`psk=pa\\ss;word1`, "uuid=11111111-2222-3333-4444-555555555555",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("falta %q:\n%s", want, text)
		}
	}
	open, err := Keyfile("Cafe", SecOpen, "", "u")
	if err != nil || strings.Contains(open, "wifi-security") {
		t.Fatalf("una red abierta no lleva sección de seguridad:\n%s %v", open, err)
	}
	sae, _ := Keyfile("Nueva", SecWPA3, "clavesegura1", "u")
	if !strings.Contains(sae, "key-mgmt=sae") {
		t.Fatalf("WPA3 usa SAE:\n%s", sae)
	}
	if _, err := Keyfile("Oficina", SecEnterprise, "12345678", "u"); err == nil {
		t.Fatal("no se configuran redes empresariales")
	}
}

// * TestParseStatus lee el primer adaptador Wi-Fi.
func TestParseStatus(t *testing.T) {
	st := ParseStatus("enp0s3:ethernet:connected:Wired connection 1\nwlan0:wifi:connected:rebootera-wifi\nlo:loopback:unmanaged:")
	if !st.Adapter || st.Device != "wlan0" || st.State != "connected" || st.Connection != "rebootera-wifi" {
		t.Fatalf("estado: %+v", st)
	}
	if ParseStatus("enp0s3:ethernet:connected:x").Adapter {
		t.Fatal("sin Wi-Fi no hay adaptador")
	}
}

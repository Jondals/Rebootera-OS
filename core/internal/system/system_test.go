// * Pruebas de la lectura del firmware y de la DMI con un sysfs falso.
package system

import (
	"os"
	"path/filepath"
	"testing"

	"rebootera/core/internal/testutil"
)

// fakeSys crea un sysfs falso y lo activa durante la prueba.
func fakeSys(t *testing.T, files map[string][]byte) {
	t.Helper()
	root := t.TempDir()
	for name, data := range files {
		path := filepath.Join(root, name)
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, data, 0o644)
	}
	t.Cleanup(testutil.Keep(&sysRoot).Restore)
	sysRoot = root
}

// TestFirmwareBIOS comprueba el arranque en BIOS.
func TestFirmwareBIOS(t *testing.T) {
	fakeSys(t, map[string][]byte{"class/dmi/id/sys_vendor": []byte("QEMU\n")})
	mode, sb := firmwareMode()
	if mode != "bios" || sb != SecureBootNA {
		t.Fatalf("obtuve %s %s", mode, sb)
	}
}

// TestFirmwareSecureBoot comprueba los tres estados de Secure Boot en UEFI.
func TestFirmwareSecureBoot(t *testing.T) {
	cases := map[string][]byte{
		SecureBootOn:  {6, 0, 0, 0, 1},
		SecureBootOff: {6, 0, 0, 0, 0},
	}
	for want, value := range cases {
		// Nombre escrito a mano, no con la constante, para que un error en el
		// GUID de la constante haga fallar la prueba.
		fakeSys(t, map[string][]byte{"firmware/efi/efivars/SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c": value})
		if mode, sb := firmwareMode(); mode != "uefi" || sb != want {
			t.Fatalf("esperaba uefi %s, obtuve %s %s", want, mode, sb)
		}
	}
	// Sin la variable, el firmware no admite Secure Boot: desactivado.
	fakeSys(t, map[string][]byte{"firmware/efi/runtime": []byte("")})
	if _, sb := firmwareMode(); sb != SecureBootOff {
		t.Fatalf("sin variable esperaba off, obtuve %s", sb)
	}
	// Una variable truncada no permite saber el estado.
	fakeSys(t, map[string][]byte{"firmware/efi/efivars/SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c": {6, 0}})
	if _, sb := firmwareMode(); sb != SecureBootUnknown {
		t.Fatalf("con variable truncada esperaba unknown, obtuve %s", sb)
	}
}

// TestModeloDeRelleno comprueba que se usa la placa si el modelo es de relleno.
func TestModeloDeRelleno(t *testing.T) {
	fakeSys(t, map[string][]byte{
		"class/dmi/id/sys_vendor":   []byte("ASUS"),
		"class/dmi/id/product_name": []byte("System Product Name"),
		"class/dmi/id/board_name":   []byte("PRIME Z790-P"),
	})
	info := Collect()
	if info.Model != "PRIME Z790-P" {
		t.Fatalf("modelo: %q", info.Model)
	}
	if info.IsVirtual {
		t.Fatal("no debería marcarse como virtual")
	}
}

// * Pruebas de la elección del próximo arranque sobre un ventoy.json real.
package boot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rebootera/core/internal/testutil"
	"rebootera/core/internal/vault"
)

// * fakeVault copia el ventoy.json del proyecto a un Vault temporal.
func fakeVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "rebootera", "state"), 0o755)
	os.MkdirAll(filepath.Join(root, "ventoy"), 0o755)
	os.MkdirAll(filepath.Join(root, "isos", "linux"), 0o755)
	data, err := os.ReadFile("../../../os/usb/ventoy/ventoy.json")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "ventoy", "ventoy.json"), data, 0o644)
	os.WriteFile(filepath.Join(root, "isos", "linux", "debian.iso"), []byte("iso"), 0o644)
	t.Cleanup(testutil.Keep(&vault.Root).Restore)
	vault.Root = root
	return root
}

// * TestSiguienteYRestaurar elige una imagen y vuelve a Rebootera sin tocar el resto.
func TestSiguienteYRestaurar(t *testing.T) {
	root := fakeVault(t)
	if err := SetNext("isos/linux/debian.iso"); err != nil {
		t.Fatal(err)
	}
	if Next() != "/isos/linux/debian.iso" {
		t.Fatalf("preseleccionada: %q", Next())
	}
	config, _ := readConfig()
	if getControl(config, "VTOY_SECURE_BOOT_POLICY") != "1" || getControl(config, "VTOY_WIN11_BYPASS_CHECK") != "0" {
		t.Fatal("se perdieron otros ajustes de seguridad de Ventoy")
	}
	if _, ok := config["theme"].(map[string]any); !ok {
		t.Fatal("se perdió el tema")
	}
	if _, err := os.Stat(filepath.Join(root, "ventoy", "ventoy.json.bak")); err != nil {
		t.Fatal("falta la copia de seguridad")
	}

	changed, err := Restore()
	if err != nil || !changed || Next() != RebooteraImage {
		t.Fatalf("restaurar: %v %v %q", changed, err, Next())
	}
	changed, _ = Restore()
	if changed {
		t.Fatal("una segunda restauración no debería cambiar nada")
	}
	data, _ := os.ReadFile(filepath.Join(root, "ventoy", "ventoy.json"))
	var check map[string]any
	if json.Unmarshal(data, &check) != nil {
		t.Fatal("ventoy.json quedó con JSON no válido")
	}
}

// * TestRutasNoValidas no permite arrancar archivos fuera de la biblioteca.
func TestRutasNoValidas(t *testing.T) {
	fakeVault(t)
	for _, bad := range []string{"../rebootera/rebootera-os.iso", "isos/linux/no-existe.iso", "isos/linux"} {
		if err := SetNext(bad); err == nil {
			t.Fatalf("%q debería rechazarse", bad)
		}
	}
	if !strings.HasPrefix(Next(), "/rebootera/") {
		t.Fatal("un intento rechazado no debe cambiar el arranque")
	}
}

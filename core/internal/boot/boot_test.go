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
	if getControl(config, "VTOY_SECURE_BOOT_POLICY") != "0" || getControl(config, "VTOY_WIN11_BYPASS_CHECK") != "0" {
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

// * TestPrepararInstalacion comprueba las respuestas, los drivers y el atajo
// * de Windows 11, y que Restore lo deja todo como estaba sin tocar lo del usuario.
func TestPrepararInstalacion(t *testing.T) {
	root := fakeVault(t)
	// * Una entrada del usuario que Rebootera nunca debe tocar.
	config, _ := readConfig()
	config["auto_install"] = []any{map[string]any{"image": "/isos/otra.iso", "template": "/ventoy/script/mia.xml"}}
	if err := writeConfig(config); err != nil {
		t.Fatal(err)
	}
	archive, ventoyArchive := ArchivePath()
	os.MkdirAll(filepath.Dir(archive), 0o755)
	os.WriteFile(archive, []byte("7z"), 0o644)

	err := Prepare("isos/linux/debian.iso", Prep{Answers: "d-i x", AnswersName: "preseed.cfg", DriversArchive: ventoyArchive, WinBypass: true})
	if err != nil {
		t.Fatal(err)
	}
	config, _ = readConfig()
	auto, _ := config["auto_install"].([]any)
	if len(auto) != 2 || auto[1].(map[string]any)["autosel"] != float64(1) && auto[1].(map[string]any)["autosel"] != 1 {
		t.Fatalf("debería haber la entrada del usuario y la de Rebootera con autosel: %v", auto)
	}
	injection, _ := config["injection"].([]any)
	if len(injection) != 1 || injection[0].(map[string]any)["archive"] != "/ventoy/rebootera/drivers.7z" {
		t.Fatalf("inyección: %v", injection)
	}
	if getControl(config, "VTOY_WIN11_BYPASS_CHECK") != "1" || getControl(config, "VTOY_WIN11_BYPASS_NRO") != "1" {
		t.Fatal("el atajo de Windows 11 debería estar activo durante la instalación")
	}
	answers, err := os.ReadFile(filepath.Join(root, "ventoy", "rebootera", "preseed.cfg"))
	if err != nil || string(answers) != "d-i x" {
		t.Fatalf("archivo de respuestas: %q %v", answers, err)
	}

	// * Preparar de nuevo no duplica entradas.
	if err := Prepare("isos/linux/debian.iso", Prep{Answers: "d-i y", AnswersName: "preseed.cfg"}); err != nil {
		t.Fatal(err)
	}
	config, _ = readConfig()
	if auto, _ = config["auto_install"].([]any); len(auto) != 2 || config["injection"] != nil {
		t.Fatalf("segunda preparación: %v %v", auto, config["injection"])
	}

	changed, err := Restore()
	if err != nil || !changed {
		t.Fatalf("restaurar: %v %v", changed, err)
	}
	config, _ = readConfig()
	auto, _ = config["auto_install"].([]any)
	if len(auto) != 1 || auto[0].(map[string]any)["template"] != "/ventoy/script/mia.xml" {
		t.Fatalf("la entrada del usuario debe sobrevivir: %v", auto)
	}
	if getControl(config, "VTOY_WIN11_BYPASS_CHECK") != "0" || Next() != RebooteraImage {
		t.Fatal("Restore debe quitar el atajo y volver a Rebootera")
	}
	if _, err := os.Stat(filepath.Join(root, "ventoy", "rebootera")); err == nil {
		t.Fatal("la carpeta de trabajo (con contraseñas codificadas) debe borrarse")
	}
}

// * TestRespuestasNombreNoValido no deja escribir archivos con otro nombre.
func TestRespuestasNombreNoValido(t *testing.T) {
	fakeVault(t)
	err := Prepare("isos/linux/debian.iso", Prep{Answers: "x", AnswersName: "../../evil.sh"})
	if err == nil {
		t.Fatal("un nombre de archivo de respuestas ajeno debe rechazarse")
	}
}

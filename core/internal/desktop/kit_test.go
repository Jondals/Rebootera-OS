// * Pruebas del kit del USB y de la actualización de Rebootera OS.
package desktop

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"rebootera/core/internal/testutil"
)

// * TestKit crea la estructura del USB con su configuración.
func TestKit(t *testing.T) {
	root := t.TempDir()
	kit := fstest.MapFS{
		"ventoy/ventoy.json":     {Data: []byte(`{"control":[]}`)},
		"ventoy/theme/theme.txt": {Data: []byte("theme")},
		"README.txt":             {Data: []byte("readme")},
	}
	if err := WriteKit(root, kit); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"isos/windows", "drivers/.ventoyignore", "ventoy/ventoy.json", "ventoy/theme/theme.txt", "README.txt", "rebootera/state/.ventoyignore"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Fatalf("falta %s", path)
		}
	}
}

// * TestActualizar instala dos versiones y comprueba que la anterior se borra (y que una copia "previous" vieja también).
func TestActualizar(t *testing.T) {
	root := t.TempDir()
	src := t.TempDir()
	v1 := filepath.Join(src, "rebootera-os-0.1.0-amd64.iso")
	v2 := filepath.Join(src, "rebootera-os-0.2.0-amd64.iso")
	os.WriteFile(v1, []byte("version uno"), 0o644)
	os.WriteFile(v2, []byte("version dos"), 0o644)

	progress := &testutil.Recorder{}
	if err := InstallSystem(root, v1, progress.Record); err != nil {
		t.Fatal(err)
	}
	if err := InstallSystem(root, v2, progress.Record); err != nil {
		t.Fatal(err)
	}
	if progress.Last != 1 {
		t.Fatalf("el progreso debería acabar en 1, acabó en %v", progress.Last)
	}
	current, _ := os.ReadFile(filepath.Join(root, "rebootera", "rebootera-os.iso"))
	if string(current) != "version dos" {
		t.Fatalf("actual %q", current)
	}
	if _, err := os.Stat(filepath.Join(root, "rebootera", "rebootera-os.previous.iso")); !os.IsNotExist(err) {
		t.Fatal("la versión anterior debería haberse borrado")
	}
	if !IsRebootera(root) {
		t.Fatal("el USB debería reconocerse como de Rebootera")
	}
	if _, err := os.Stat(filepath.Join(root, "rebootera", "rebootera-os.iso.new")); !os.IsNotExist(err) {
		t.Fatal("no debería quedar el archivo temporal")
	}
	if err := InstallSystem(root, filepath.Join(src, "no.txt"), nil); err == nil {
		t.Fatal("debería rechazar archivos que no son .iso")
	}
}

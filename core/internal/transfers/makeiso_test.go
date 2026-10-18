// * Prueba del trabajo que empaqueta un instalador en carpetas como ISO de la biblioteca.
package transfers

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"rebootera/core/internal/library"
	"rebootera/core/internal/testutil"
	"rebootera/core/internal/vault"
)

// * TestCrearISODeCarpeta encola, crea y registra la ISO como imagen "sin comprobar" (no tiene suma oficial).
func TestCrearISODeCarpeta(t *testing.T) {
	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("genisoimage"); err != nil {
			t.Skip("falta genisoimage")
		}
	}
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "rebootera", "state"), 0o755)
	t.Cleanup(testutil.Keep(&vault.Root).Restore)
	vault.Root = root
	folder := filepath.Join(t.TempDir(), "Win10")
	for _, name := range []string{"bootmgr", "boot/etfsboot.com", "efi/microsoft/boot/efisys.bin", "sources/boot.wim", "sources/install.esd"} {
		path := filepath.Join(folder, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, make([]byte, 4096), 0o644)
	}
	m := New(library.New())
	if _, err := m.MakeISO(t.TempDir()); err == nil {
		t.Fatal("una carpeta vacía no es un instalador")
	}
	if _, err := m.MakeISO(folder); err != nil {
		t.Fatal(err)
	}
	job := runOne(t, m)
	if job.Status != StatusDone || job.Kind != KindMakeISO {
		t.Fatalf("trabajo: %+v", job)
	}
	items, _ := m.lib.List()
	if len(items) != 1 || items[0].Path != "isos/windows/Win10.iso" || items[0].SHA256 == "" || items[0].State == library.StateMismatch {
		t.Fatalf("biblioteca: %+v", items)
	}
}

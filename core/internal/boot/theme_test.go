// * Pruebas de los colores del menú de Ventoy.
package boot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rebootera/core/internal/testutil"
	"rebootera/core/internal/vault"
)

// * TestApplyTheme comprueba que se cambian solo los cuatro colores y que un tema desconocido no toca nada.
func TestApplyTheme(t *testing.T) {
	t.Cleanup(testutil.Keep(&vault.Root).Restore)
	vault.Root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault.Root, "rebootera", "state"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := vault.Path("ventoy", "theme", "theme.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	original := "desktop-color: \"#0b0d11\"\ntitle-text: \"\"\n+ boot_menu {\n        item_color = \"#a39e8f\"\n        selected_item_color = \"#ffffff\"\n}\n+ label {\n        color = \"#d8ae62\"\n        text = \"x\"\n}\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	ApplyTheme("nope")
	if data, _ := os.ReadFile(path); string(data) != original {
		t.Fatal("un tema desconocido no debe cambiar nada")
	}
	ApplyTheme("orange")
	data, _ := os.ReadFile(path)
	text := string(data)
	for _, want := range []string{`desktop-color: "#0d0905"`, `item_color = "#b9a28a"`, `selected_item_color = "#f6ece0"`, `color = "#ff8a1f"`, `text = "x"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("falta %q en:\n%s", want, text)
		}
	}
}

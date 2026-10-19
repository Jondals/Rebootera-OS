// * Pruebas de los ajustes del puntero guardados: sin archivo no se debe aplicar nada distinto de lo normal.
package api

import (
	"os"
	"path/filepath"
	"testing"

	"rebootera/core/internal/agentproto"
	"rebootera/core/internal/testutil"
	"rebootera/core/internal/vault"
)

// * TestLoadInputSinArchivo comprueba que, sin ajustes guardados, se devuelven los valores por defecto
// * (velocidad normal y toque para clic activado) y no un valor vacío que reiniciaría el kiosco en cada arranque.
func TestLoadInputSinArchivo(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "rebootera", "state"), 0o755)
	t.Cleanup(testutil.Keep(&vault.Root).Restore)
	vault.Root = root
	saved := LoadInput()
	if saved == nil || *saved != agentproto.DefaultInput {
		t.Fatalf("esperaba los valores por defecto, obtuve %+v", saved)
	}
}

// * TestLoadInputGuardado comprueba que unos ajustes guardados se leen tal cual.
func TestLoadInputGuardado(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "rebootera", "state"), 0o755)
	t.Cleanup(testutil.Keep(&vault.Root).Restore)
	vault.Root = root
	want := agentproto.InputSettings{Speed: 80, Tap: false, Natural: true}
	if err := vault.WriteJSON(filepath.Join(vault.StateDir(), inputFile), want); err != nil {
		t.Fatal(err)
	}
	if got := LoadInput(); got == nil || *got != want {
		t.Fatalf("esperaba %+v, obtuve %+v", want, got)
	}
}

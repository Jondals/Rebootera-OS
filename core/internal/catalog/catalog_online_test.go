//go:build online

// * Prueba contra las fuentes oficiales reales (necesita internet):
// *   go test -tags online ./internal/catalog/
// * Resuelve cada entrada descargable y comprueba que la URL de la imagen existe.
package catalog

import (
	"net/http"
	"testing"
)

// * TestFuentesReales resuelve todo el catálogo y hace HEAD a cada imagen.
func TestFuentesReales(t *testing.T) {
	entries, err := Entries()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		got, err := Resolve(entry)
		if err == ErrManual {
			t.Logf("%-24s manual: %s", entry.ID, entry.Homepage)
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", entry.ID, err)
			continue
		}
		resp, err := Client.Head(got.URL)
		if err != nil {
			t.Errorf("%s: HEAD %s: %v", entry.ID, got.URL, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: HEAD %s: %d", entry.ID, got.URL, resp.StatusCode)
			continue
		}
		if entry.Resolver.Signature != nil && got.SignedBy == "" {
			t.Errorf("%s: debería venir firmado", entry.ID)
		}
		t.Logf("%-24s %s  %d MB  sha256 %s…  firmado por: %s", entry.ID, got.FileName, resp.ContentLength>>20, got.SHA256[:12], got.SignedBy)
	}
}

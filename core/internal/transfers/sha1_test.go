// * Pruebas de la comprobación con el SHA-1 que publica el Internet Archive.
package transfers

import (
	"crypto/sha1" //nolint:gosec // solo para generar el valor esperado de la prueba
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"rebootera/core/internal/library"
)

// * TestSHA1Archive: con el SHA-1 correcto entra en la biblioteca "sin comprobar" (no hay SHA-256 oficial);
// * con uno distinto se rechaza y se borra.
func TestSHA1Archive(t *testing.T) {
	m, lib, server, _ := setup(t)
	sum := sha1.Sum(payload) //nolint:gosec
	good := hex.EncodeToString(sum[:])
	if _, err := m.enqueue(&Job{Kind: KindDownload, Name: "Archive", CatalogID: "archivo", URL: server.URL + "/a.iso", FileName: "a.iso", Category: "tools", SHA1: good}); err != nil {
		t.Fatal(err)
	}
	job := runOne(t, m)
	if job.Status != StatusDone {
		t.Fatalf("con el SHA-1 correcto debería terminar: %+v", job)
	}
	items, _ := lib.List()
	if len(items) != 1 || items[0].State == library.StateMismatch || items[0].SHA256 == "" {
		t.Fatalf("biblioteca: %+v", items)
	}
	if _, err := m.enqueue(&Job{Kind: KindDownload, Name: "Mala", URL: server.URL + "/b.iso", FileName: "b.iso", Category: "tools", SHA1: strings.Repeat("0", 40)}); err != nil {
		t.Fatal(err)
	}
	bad := runOne(t, m)
	if bad.Status != StatusFailed || bad.ErrorCode != CodeChecksum {
		t.Fatalf("con otro SHA-1 debería fallar por la suma: %+v", bad)
	}
	if _, err := os.Stat(partialPath(&bad)); !os.IsNotExist(err) {
		t.Fatal("el parcial debería haberse borrado")
	}
}

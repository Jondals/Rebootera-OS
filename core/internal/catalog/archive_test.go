// * Prueba del resolvedor del Internet Archive con un servidor HTTPS falso.
package catalog

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"rebootera/core/internal/testutil"
)

const archiveSHA1 = "0c52420f047d877f8526575e39b561e7bf189540"

// * archiveMetadata sirve los metadatos de un elemento del Archive con un archivo original y un derivado.
func archiveMetadata(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/metadata/ITEM" {
		http.NotFound(w, r)
		return
	}
	w.Write([]byte(`{"files":[
		{"name":"Disco-1.iso","source":"original","size":"1000","sha1":"` + archiveSHA1 + `"},
		{"name":"Disco-1.iso_meta.xml","source":"original","size":"10","sha1":"` + archiveSHA1 + `"},
		{"name":"sub carpeta/Disco-2.iso","source":"original","size":"2000","sha1":"` + archiveSHA1 + `"},
		{"name":"Disco-3.iso","source":"derivative","size":"3000","sha1":"` + archiveSHA1 + `"},
		{"name":"Disco-4.iso","source":"original","size":"4000"}]}`))
}

// * TestArchive comprueba que se elige el archivo original con suma, con su URL y su SHA-1.
func TestArchive(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(archiveMetadata))
	defer server.Close()
	t.Cleanup(testutil.Keep(&Client).Restore)
	t.Cleanup(testutil.Keep(&ArchiveBase).Restore)
	Client = server.Client()
	ArchiveBase = server.URL

	got, err := resolveArchive(Resolver{Type: "archive", Index: "ITEM", Pattern: `^Disco-1\.iso$`})
	if err != nil {
		t.Fatal(err)
	}
	if got.FileName != "Disco-1.iso" || got.SHA1 != archiveSHA1 || got.SHA256 != "" || got.SizeBytes != 1000 || got.URL != server.URL+"/download/ITEM/Disco-1.iso" {
		t.Fatalf("resultado: %+v", got)
	}
	// * Un archivo en una subcarpeta conserva la ruta en la URL pero el nombre queda limpio.
	got, err = resolveArchive(Resolver{Type: "archive", Index: "ITEM", Pattern: `Disco-2\.iso$`})
	if err != nil || got.FileName != "Disco-2.iso" || got.URL != server.URL+"/download/ITEM/sub%20carpeta/Disco-2.iso" {
		t.Fatalf("subcarpeta: %+v %v", got, err)
	}
	// * Sin suma, derivado o inexistente: no se ofrece.
	for _, pattern := range []string{`^Disco-3\.iso$`, `^Disco-4\.iso$`, `^nada\.iso$`} {
		if _, err := resolveArchive(Resolver{Type: "archive", Index: "ITEM", Pattern: pattern}); err == nil {
			t.Fatalf("%s no debería resolverse", pattern)
		}
	}
}

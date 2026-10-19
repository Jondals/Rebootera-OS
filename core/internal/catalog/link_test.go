// * Pruebas de la descarga por enlace: archivos directos y elementos del Internet Archive.
package catalog

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rebootera/core/internal/testutil"
)

// * linkHandler sirve un archivo directo (HEAD) y los metadatos de un elemento del Archive.
func linkHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/imagen.iso":
		w.Header().Set("Content-Length", "12345")
	case "/metadata/ITEM":
		w.Write([]byte(`{"files":[
			{"name":"Disco.iso","source":"original","size":"900","sha1":"` + archiveSHA1 + `"},
			{"name":"Disco.iso.torrent","source":"metadata","size":"9"},
			{"name":"Notas.txt","source":"original","size":"1"}]}`))
	default:
		http.NotFound(w, r)
	}
}

// * linkServer arranca el servidor falso y lo usa como cliente del catálogo y como Internet Archive.
func linkServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(linkHandler))
	t.Cleanup(server.Close)
	t.Cleanup(testutil.Keep(&Client).Restore)
	t.Cleanup(testutil.Keep(&ArchiveBase).Restore)
	Client = server.Client()
	ArchiveBase = server.URL
	return server
}

// * TestInspectLinkDirecto: un enlace directo a una imagen devuelve su tamaño; otros se rechazan.
func TestInspectLinkDirecto(t *testing.T) {
	server := linkServer(t)
	got, err := InspectLink(server.URL + "/imagen.iso")
	if err != nil || len(got) != 1 || got[0].Name != "imagen.iso" || got[0].SizeBytes != 12345 {
		t.Fatalf("directo: %+v %v", got, err)
	}
	for _, bad := range []string{"http://ejemplo.org/a.iso", server.URL + "/pagina.html", "no es una dirección", ""} {
		if _, err := InspectLink(bad); err == nil {
			t.Fatalf("%q debería rechazarse", bad)
		}
	}
}

// * TestInspectLinkArchive: la página de un elemento del Archive lista solo las imágenes originales, con su SHA-1.
func TestInspectLinkArchive(t *testing.T) {
	server := linkServer(t)
	parsed := strings.Replace(server.URL, "https://", "", 1)
	// * El Archive real se reconoce por su dominio; en la prueba se llama a la función interna con el elemento.
	got, err := inspectArchive("ITEM")
	if err != nil || len(got) != 1 || got[0].Name != "Disco.iso" || got[0].SHA1 != archiveSHA1 || got[0].URL != "https://"+parsed+"/download/ITEM/Disco.iso" {
		t.Fatalf("archive: %+v %v", got, err)
	}
	if _, err := inspectArchive("VACIO"); err == nil {
		t.Fatal("un elemento inexistente debería fallar")
	}
}

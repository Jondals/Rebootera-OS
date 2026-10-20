package catalog

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"rebootera/core/internal/testutil"
)

// * TestManifest lee la versión del JSON del proyecto y la suma que hay junto a la imagen.
func TestManifest(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nas/manifest.json":
			w.Write([]byte(`{"filename": "x.update", "version": "25.10.7", "checksum": "abc"}`))
		case "/nas/bad-manifest.json":
			w.Write([]byte(`{"version": "../../etc"}`))
		case "/nas/25.10.7/NAS-25.10.7.iso.sha256":
			w.Write([]byte(hashA + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	t.Cleanup(testutil.Keep(&Client).Restore)
	Client = server.Client()
	base := server.URL

	r := Resolver{Index: base + "/nas/manifest.json", Link: base + "/nas/{version}/NAS-{version}.iso", SumSuffix: ".sha256"}
	got, err := resolveManifest(r)
	if err != nil {
		t.Fatal(err)
	}
	if got.FileName != "NAS-25.10.7.iso" || got.SHA256 != hashA || got.URL != base+"/nas/25.10.7/NAS-25.10.7.iso" {
		t.Fatalf("obtuve %+v", got)
	}
	// * Una versión con barras (que cambiaría la dirección) se rechaza, y sin plantilla o sin suma falla.
	bad := r
	bad.Index = base + "/nas/bad-manifest.json"
	if _, err := resolveManifest(bad); err == nil {
		t.Fatal("una versión con barras debe rechazarse")
	}
	if _, err := resolveManifest(Resolver{Index: r.Index, Link: base + "/fijo.iso", SumSuffix: ".sha256"}); err == nil {
		t.Fatal("sin {version} debe fallar")
	}
	if _, err := resolveManifest(Resolver{Index: r.Index, Link: r.Link}); err == nil {
		t.Fatal("sin sumSuffix debe fallar")
	}
}

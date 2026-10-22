// * Pruebas del catálogo: el JSON embebido es válido y los resolvedores leen
// * correctamente los formatos reales de cada fuente (servidor HTTPS falso).
package catalog

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rebootera/core/internal/testutil"
)

const (
	hashA = "a7ef94ac2fb9a7fec454552abd629b7cc9d5155c886165a45649f5ce6167e355"
	hashB = "601e30fbf5d97759367c632e2c33630665039b7e2158fd068403da3ccf1bda1f"
	hashC = "1620295f6a00c27c3208f0c00b8ece4eab1ec69b9002152d97488bf26a426ddf"
)

// * fakeSources sirve copias reducidas de los índices oficiales.
func fakeSources(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/ubuntu/SHA256SUMS":
		w.Write([]byte(hashA + " *ubuntu-26.04-desktop-amd64.iso\n" +
			hashA + " *ubuntu-26.04-live-server-amd64.iso\n" +
			hashB + " *ubuntu-26.04.1-desktop-amd64.iso\n"))
	case "/fedora/":
		w.Write([]byte(`<a href="../">../</a><a href="Fedora-Workstation-44-1.7-x86_64-CHECKSUM">x</a>` +
			`<a href="Fedora-Workstation-Live-44-1.7.x86_64.iso">x</a>`))
	case "/fedora/Fedora-Workstation-44-1.7-x86_64-CHECKSUM":
		w.Write([]byte("-----BEGIN PGP SIGNED MESSAGE-----\nHash: SHA256\n\n" +
			"# Fedora-Workstation-Live-44-1.7.x86_64.iso: 2851612672 bytes\n" +
			"SHA256 (Fedora-Workstation-Live-44-1.7.x86_64.iso) = " + hashC + "\n" +
			"SHA256 (Fedora-Workstation-Live-44-20260424.n.0.x86_64.iso) = " + hashA + "\n" +
			"-----BEGIN PGP SIGNATURE-----\nabc\n"))
	case "/alpine/latest-releases.yaml":
		w.Write([]byte("---\n-\n  title: \"Mini\"\n  flavor: alpine-minirootfs\n  iso: mini.tar.gz\n  size: 10\n  sha256: " + hashA + "\n" +
			"-\n  title: \"Virtual\"\n  desc: |\n    Slimmed down kernel.\n  flavor: alpine-virt\n  iso: alpine-virt-3.24.2-x86_64.iso\n  time: 17:32:03\n  size: 69206016\n  sha256: " + hashB + "\n"))
	case "/latest":
		http.Redirect(w, r, "/images/2026-10-06-raspios-lite.img.xz", http.StatusFound)
	case "/images/2026-10-06-raspios-lite.img.xz":
		w.Header().Set("Content-Length", "1234")
	case "/images/2026-10-06-raspios-lite.img.xz.sha256":
		w.Write([]byte(hashA + "  2026-10-06-raspios-lite.img.xz\n"))
	case "/latest-bad":
		http.Redirect(w, r, "/images/other.img.xz", http.StatusFound)
	case "/images/other.img.xz":
	case "/images/other.img.xz.sha256":
		w.Write([]byte(hashB + "  distinto.img.xz\n"))
	default:
		http.NotFound(w, r)
	}
}

// * withServer arranca el servidor falso y lo usa como cliente del catálogo.
func withServer(t *testing.T) string {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(fakeSources))
	t.Cleanup(testutil.Keep(&Client).Restore)
	Client = server.Client()
	t.Cleanup(server.Close)
	return server.URL
}

// * TestCatalogoEmbebido comprueba que el JSON se lee y que las fuentes son HTTPS.
func TestCatalogoEmbebido(t *testing.T) {
	entries, err := Entries()
	if err != nil || len(entries) == 0 {
		t.Fatalf("catálogo: %v", err)
	}
	for _, e := range entries {
		// * SourceForge y el Internet Archive se indican por el nombre del proyecto o elemento; el resto, por una URL HTTPS.
		if e.Resolver.Type != "manual" && e.Resolver.Type != "microsoft" && e.Resolver.Type != "sourceforge" && e.Resolver.Type != "archive" && !strings.HasPrefix(e.Resolver.Index, "https://") {
			t.Fatalf("%s no usa HTTPS: %s", e.ID, e.Resolver.Index)
		}
		if e.Notes["en"] == "" || e.Notes["es"] == "" {
			t.Fatalf("%s no tiene notas en los dos idiomas", e.ID)
		}
	}
}

// * TestSums elige la versión más reciente (26.04.1 sobre 26.04).
func TestSums(t *testing.T) {
	base := withServer(t)
	got, err := resolveSums(Resolver{Index: base + "/ubuntu/SHA256SUMS", Pattern: `^ubuntu-26\.04(\.\d+)?-desktop-amd64\.iso$`})
	if err != nil {
		t.Fatal(err)
	}
	if got.FileName != "ubuntu-26.04.1-desktop-amd64.iso" || got.SHA256 != hashB || got.URL != base+"/ubuntu/ubuntu-26.04.1-desktop-amd64.iso" {
		t.Fatalf("obtuve %+v", got)
	}
}

// * TestFedora lee el CHECKSUM firmado y descarta las compilaciones nocturnas.
func TestFedora(t *testing.T) {
	base := withServer(t)
	got, err := resolveFedora(Resolver{Index: base + "/fedora/", Pattern: `^Fedora-Workstation-Live-44-\d+\.\d+\.x86_64\.iso$`})
	if err != nil {
		t.Fatal(err)
	}
	if got.FileName != "Fedora-Workstation-Live-44-1.7.x86_64.iso" || got.SHA256 != hashC {
		t.Fatalf("obtuve %+v", got)
	}
}

// * TestAlpine lee la variante pedida del YAML.
func TestAlpine(t *testing.T) {
	base := withServer(t)
	got, err := resolveAlpine(Resolver{Index: base + "/alpine/latest-releases.yaml", Pattern: "alpine-virt"})
	if err != nil {
		t.Fatal(err)
	}
	if got.FileName != "alpine-virt-3.24.2-x86_64.iso" || got.SHA256 != hashB || got.SizeBytes != 69206016 {
		t.Fatalf("obtuve %+v", got)
	}
}

// * TestManual comprueba que una entrada manual no se intenta descargar.
func TestManual(t *testing.T) {
	entry := Entry{ID: "x", Resolver: Resolver{Type: "manual"}}
	if _, err := Resolve(entry); err != ErrManual {
		t.Fatalf("esperaba ErrManual, obtuve %v", err)
	}
}

// * TestSoloHTTPS rechaza índices por HTTP sin cifrar.
func TestSoloHTTPS(t *testing.T) {
	if _, err := fetchText("http://example.com/SHA256SUMS"); err == nil {
		t.Fatal("debería rechazar HTTP")
	}
}

// * TestCompatibilidad comprueba que toda entrada declara su compatibilidad de
// * arranque, con fecha si está probada y notas en los dos idiomas.
func TestCompatibilidad(t *testing.T) {
	entries, _ := Entries()
	for _, e := range entries {
		switch e.Boot.Status {
		case "verified", "incompatible":
			if e.Boot.Tested == "" {
				t.Fatalf("%s: estado %s sin fecha de prueba", e.ID, e.Boot.Status)
			}
		case "untested":
		default:
			t.Fatalf("%s: estado de arranque desconocido %q", e.ID, e.Boot.Status)
		}
		if e.Boot.Notes["en"] == "" || e.Boot.Notes["es"] == "" {
			t.Fatalf("%s: faltan notas de arranque", e.ID)
		}
	}
}

// * TestSiblingRutaAbsoluta: un enlace con ruta absoluta cuelga del servidor, no de la carpeta del índice.
func TestSiblingRutaAbsoluta(t *testing.T) {
	if got := sibling("https://mirror.example/pub/iso/", "a.iso"); got != "https://mirror.example/pub/iso/a.iso" {
		t.Fatalf("relativo: %s", got)
	}
	if got := sibling("https://mirror.example/pub/iso/", "/fedora/x/CHECKSUM"); got != "https://mirror.example/fedora/x/CHECKSUM" {
		t.Fatalf("absoluto: %s", got)
	}
	if got := sibling("https://mirror.example/pub/", "https://otro.example/x/CHECKSUM"); got != "https://otro.example/x/CHECKSUM" {
		t.Fatalf("con servidor: %s", got)
	}
}

// * TestLatestSum sigue el enlace "latest" hasta la imagen y lee su suma publicada al lado.
func TestLatestSum(t *testing.T) {
	base := withServer(t)
	got, err := resolveLatestSum(Resolver{Index: base + "/latest", SumSuffix: ".sha256"})
	if err != nil {
		t.Fatal(err)
	}
	if got.FileName != "2026-10-06-raspios-lite.img.xz" || got.SHA256 != hashA || got.URL != base+"/images/2026-10-06-raspios-lite.img.xz" || got.SizeBytes != 1234 {
		t.Fatalf("obtuve %+v", got)
	}
}

// * TestLatestSumRechazaOtraSuma no acepta una suma que es de otro archivo.
func TestLatestSumRechazaOtraSuma(t *testing.T) {
	base := withServer(t)
	if _, err := resolveLatestSum(Resolver{Index: base + "/latest-bad", SumSuffix: ".sha256"}); err == nil {
		t.Fatal("debería rechazar la suma de otro archivo")
	}
	if _, err := resolveLatestSum(Resolver{Index: base + "/latest"}); err == nil {
		t.Fatal("sin sumSuffix debería fallar")
	}
}

// * TestSinEvaluacion comprueba que el catálogo no ofrece ediciones de evaluación (las de 90 o 180 días de Windows y
// * Windows Server): solo imágenes que no caducan.
func TestSinEvaluacion(t *testing.T) {
	entries, _ := Entries()
	for _, e := range entries {
		text := strings.ToLower(e.ID + " " + e.Edition + " " + e.Version + " " + e.Notes["en"])
		if strings.Contains(text, "evaluation") || strings.Contains(text, "-eval") || e.Resolver.Type == "direct" {
			t.Fatalf("%s parece una edición de evaluación: %q", e.ID, text)
		}
	}
	families, _ := Families()
	used := map[string]bool{}
	for _, e := range entries {
		used[e.Family] = true
	}
	for _, f := range families {
		if !used[f.ID] {
			t.Fatalf("la familia %s no tiene ninguna imagen", f.ID)
		}
	}
}

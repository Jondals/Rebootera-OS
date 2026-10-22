// * Pruebas de la biblioteca con un Vault falso: listado, categorías, cambios
// * hechos desde fuera, verificación de hash y borrado seguro.
package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"rebootera/core/internal/testutil"
	"rebootera/core/internal/vault"
)

// * fakeVault crea un Vault vacío en una carpeta temporal.
func fakeVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "rebootera", "state"), 0o755)
	os.MkdirAll(filepath.Join(root, "isos", "linux"), 0o755)
	os.MkdirAll(filepath.Join(root, "isos", ".partial"), 0o755)
	t.Cleanup(testutil.Keep(&vault.Root).Restore)
	vault.Root = root
	// * Sin caché: estas pruebas cambian los archivos desde fuera y esperan verlo al momento.
	t.Cleanup(testutil.Keep(&cacheTTL).Restore)
	cacheTTL = 0
	return root
}

// * TestListado comprueba extensiones, categorías y que .partial se ignora.
func TestListado(t *testing.T) {
	root := fakeVault(t)
	os.WriteFile(filepath.Join(root, "isos/linux/debian.iso"), []byte("iso"), 0o644)
	os.WriteFile(filepath.Join(root, "isos/linux/notas.txt"), []byte("txt"), 0o644)
	os.WriteFile(filepath.Join(root, "isos/.partial/a.iso.part"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(root, "isos/rescate.img"), []byte("img"), 0o644)

	items, err := New().List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("esperaba 2 imágenes, obtuve %+v", items)
	}
	if items[0].Path != "isos/linux/debian.iso" || items[0].Category != "linux" || items[0].State != StateUnchecked {
		t.Fatalf("debian: %+v", items[0])
	}
	if items[1].Category != "other" || items[1].Kind != "img" {
		t.Fatalf("rescate: %+v", items[1])
	}
}

// * TestCambioDesdeFuera invalida el hash si el archivo cambia (por ejemplo, desde Windows).
func TestCambioDesdeFuera(t *testing.T) {
	root := fakeVault(t)
	path := filepath.Join(root, "isos/linux/a.iso")
	os.WriteFile(path, []byte("original"), 0o644)
	lib := New()
	if err := lib.Register("isos/linux/a.iso", "abc", "abc", "test"); err != nil {
		t.Fatal(err)
	}
	items, _ := lib.List()
	if items[0].State != StateVerified {
		t.Fatalf("debería estar verificada: %+v", items[0])
	}
	os.WriteFile(path, []byte("modificado"), 0o644)
	os.Chtimes(path, time.Now().Add(time.Hour), time.Now().Add(time.Hour))
	items, _ = lib.List()
	if items[0].State != StateUnchecked || items[0].Source != "test" {
		t.Fatalf("tras modificarla debería quedar sin comprobar: %+v", items[0])
	}
}

// * TestVerificar calcula el hash en segundo plano y detecta un hash distinto.
func TestVerificar(t *testing.T) {
	root := fakeVault(t)
	os.WriteFile(filepath.Join(root, "isos/linux/a.iso"), []byte("contenido"), 0o644)
	lib := New()
	lib.List()
	lib.mu.Lock()
	m := lib.db["isos/linux/a.iso"]
	m.Expected = "0000000000000000000000000000000000000000000000000000000000000000"
	lib.db["isos/linux/a.iso"] = m
	lib.mu.Unlock()
	if err := lib.Verify("isos/linux/a.iso"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		items, _ := lib.List()
		if items[0].State != StateHashing {
			if items[0].State != StateMismatch {
				t.Fatalf("esperaba mismatch: %+v", items[0])
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("el cálculo del hash no terminó")
}

// * TestBorradoSeguro solo permite borrar imágenes dentro de isos/.
func TestBorradoSeguro(t *testing.T) {
	root := fakeVault(t)
	os.WriteFile(filepath.Join(root, "rebootera", "rebootera-os.iso"), []byte("sistema"), 0o644)
	os.WriteFile(filepath.Join(root, "isos/linux/a.iso"), []byte("x"), 0o644)
	lib := New()
	lib.List()
	for _, bad := range []string{"../rebootera/rebootera-os.iso", "isos/../rebootera/rebootera-os.iso", "isos/linux"} {
		if err := lib.Delete(bad); err == nil {
			t.Fatalf("%q no debería poder borrarse", bad)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "rebootera", "rebootera-os.iso")); err != nil {
		t.Fatal("el sistema no debería haberse borrado")
	}
	if err := lib.Delete("isos/linux/a.iso"); err != nil {
		t.Fatal(err)
	}
}

// * TestCacheDelListado comprueba que el recorrido se reutiliza unos segundos y que una mutación lo descarta.
func TestCacheDelListado(t *testing.T) {
	root := fakeVault(t)
	cacheTTL = time.Minute
	os.WriteFile(filepath.Join(root, "isos/linux/a.iso"), []byte("uno"), 0o644)
	lib := New()
	items, _ := lib.List()
	if len(items) != 1 {
		t.Fatalf("debería haber una imagen: %+v", items)
	}
	// * Una imagen nueva no aparece mientras vale el recorrido guardado.
	os.WriteFile(filepath.Join(root, "isos/linux/b.iso"), []byte("dos"), 0o644)
	if items, _ = lib.List(); len(items) != 1 {
		t.Fatalf("debería servir el recorrido guardado: %+v", items)
	}
	// * Registrar una imagen descarta el recorrido y se ven las dos.
	if err := lib.Register("isos/linux/b.iso", "abc", "", "test"); err != nil {
		t.Fatal(err)
	}
	if items, _ = lib.List(); len(items) != 2 {
		t.Fatalf("tras registrar deberían verse las dos: %+v", items)
	}
}

// * TestNombreYFoto guarda el nombre y la foto de una imagen, la conserva al refrescar y rechaza lo no válido.
func TestNombreYFoto(t *testing.T) {
	root := fakeVault(t)
	rel := "isos/linux/a.iso"
	os.WriteFile(filepath.Join(root, rel), []byte("contenido"), 0o644)
	lib := New()
	if _, err := lib.ListFresh(); err != nil {
		t.Fatal(err)
	}
	good := "data:image/png;base64,iVBORw0KGgo="
	if err := lib.SetLook(rel, "  Mi Windows  ", good); err != nil {
		t.Fatal(err)
	}
	items, err := lib.ListFresh()
	if err != nil || len(items) != 1 {
		t.Fatalf("listado: %v %d", err, len(items))
	}
	if items[0].Title != "Mi Windows" || items[0].Image != good {
		t.Fatalf("nombre o foto perdidos: %+v", items[0])
	}
	for _, bad := range []string{"data:text/html;base64,AAAA", "data:image/png;base64,AAA<script>", "http://x/y.png"} {
		if err := lib.SetLook(rel, "x", bad); err == nil {
			t.Fatalf("debería rechazar %q", bad)
		}
	}
	if err := lib.SetLook(rel, strings.Repeat("a", 81), ""); err == nil {
		t.Fatal("un nombre largo debería rechazarse")
	}
	if err := lib.SetLook("isos/linux/no-existe.iso", "x", ""); err == nil {
		t.Fatal("una imagen que no existe debería rechazarse")
	}
	if err := lib.SetLook(rel, "", ""); err != nil {
		t.Fatal(err)
	}
}

// * TestVistaPreviaGIF guarda un GIF de vista previa, lo ofrece en el listado, lo conserva al cambiar el archivo de nombre y
// * lo borra con la imagen; y rechaza lo que no es un GIF.
func TestVistaPreviaGIF(t *testing.T) {
	root := fakeVault(t)
	rel := "isos/linux/a.iso"
	os.WriteFile(filepath.Join(root, rel), []byte("contenido"), 0o644)
	lib := New()
	if _, err := lib.ListFresh(); err != nil {
		t.Fatal(err)
	}
	// * "R0lGODlhAQABAAAAACw=" es un GIF89a de un píxel.
	gif := "data:image/gif;base64,R0lGODlhAQABAAAAACw="
	if err := lib.SetPreview(rel, gif); err != nil {
		t.Fatal(err)
	}
	items, err := lib.ListFresh()
	if err != nil || len(items) != 1 || !strings.HasPrefix(items[0].Preview, "/api/library/preview?path=isos%2Flinux%2Fa.iso&v=") {
		t.Fatalf("el listado debe ofrecer la vista previa: %v %+v", err, items)
	}
	path, err := lib.PreviewPath(rel)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || !strings.HasPrefix(string(data), "GIF89a") {
		t.Fatalf("el GIF guardado no es el enviado: %v %q", err, data)
	}
	for _, bad := range []string{"data:image/png;base64,iVBORw0KGgo=", "data:image/gif;base64,AAAA", "data:image/gif;base64,@@@", "http://x/y.gif"} {
		if err := lib.SetPreview(rel, bad); err == nil {
			t.Fatalf("debería rechazar %q", bad)
		}
	}
	if err := lib.SetPreview("isos/linux/no-existe.iso", gif); err == nil {
		t.Fatal("una imagen que no existe debería rechazarse")
	}
	// * Cambiar la foto o el nombre no pierde el GIF.
	if err := lib.SetLook(rel, "Mi Linux", ""); err != nil {
		t.Fatal(err)
	}
	if items, _ := lib.ListFresh(); len(items) != 1 || items[0].Preview == "" {
		t.Fatalf("el GIF se perdió al cambiar el nombre: %+v", items)
	}
	// * Borrar la imagen borra también su GIF.
	if err := lib.Delete(rel); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("el GIF debería haberse borrado con la imagen")
	}
	// * Quitar el GIF (texto vacío).
	os.WriteFile(filepath.Join(root, rel), []byte("contenido"), 0o644)
	if _, err := lib.ListFresh(); err != nil {
		t.Fatal(err)
	}
	if err := lib.SetPreview(rel, gif); err != nil {
		t.Fatal(err)
	}
	if err := lib.SetPreview(rel, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.PreviewPath(rel); err == nil {
		t.Fatal("tras quitarlo ya no debe tener vista previa")
	}
}

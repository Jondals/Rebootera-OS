// * Pruebas de la importación de descargas: un .zip se descomprime en drivers/Descargas, lo demás se copia, lo que está a medias
// * se deja y una ruta que intenta salir de la carpeta se rechaza.
package drivers

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// * makeZip crea un .zip con los archivos indicados (nombre -> contenido).
func makeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(out)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		entry.Write([]byte(content))
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	out.Close()
	old := time.Now().Add(-time.Minute)
	os.Chtimes(path, old, old)
}

// * TestImportDownloadsZip descomprime un .zip de drivers y lo quita de la carpeta de descargas.
func TestImportDownloadsZip(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	makeZip(t, filepath.Join(from, "realtek-lan.zip"), map[string]string{"net/rtl.inf": "[Version]", "net/rtl.sys": "x"})
	imported, err := ImportDownloads(from, to)
	if err != nil || len(imported) != 1 {
		t.Fatalf("importado %v, error %v", imported, err)
	}
	if _, err := os.Stat(filepath.Join(to, "Descargas", "realtek-lan", "net", "rtl.inf")); err != nil {
		t.Fatalf("falta el INF descomprimido: %v", err)
	}
	if _, err := os.Stat(filepath.Join(from, "realtek-lan.zip")); err == nil {
		t.Fatal("el .zip debería haberse quitado de la carpeta de descargas")
	}
}

// * TestImportDownloadsOtros copia un instalador y no pisa uno que ya estaba.
func TestImportDownloadsOtros(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	for range 2 {
		path := filepath.Join(from, "setup.exe")
		os.WriteFile(path, []byte("MZ"), 0o600)
		old := time.Now().Add(-time.Minute)
		os.Chtimes(path, old, old)
		if _, err := ImportDownloads(from, to); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"setup.exe", "setup (2).exe"} {
		if _, err := os.Stat(filepath.Join(to, "Descargas", name)); err != nil {
			t.Fatalf("falta %s: %v", name, err)
		}
	}
}

// * TestImportDownloadsAMedias deja para después lo que aún se está descargando o es demasiado reciente.
func TestImportDownloadsAMedias(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(from, "x.zip.wkdownload"), []byte("PK"), 0o600)
	os.WriteFile(filepath.Join(from, "nuevo.exe"), []byte("MZ"), 0o600)
	imported, err := ImportDownloads(from, to)
	if err != nil || len(imported) != 0 {
		t.Fatalf("no debería importar nada: %v, %v", imported, err)
	}
}

// * TestImportDownloadsZipSlip rechaza un .zip con una ruta que sale de la carpeta de destino.
func TestImportDownloadsZipSlip(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	makeZip(t, filepath.Join(from, "malo.zip"), map[string]string{"../../fuera.txt": "x"})
	if _, err := ImportDownloads(from, to); err == nil {
		t.Fatal("debería rechazar la ruta que sale de la carpeta")
	}
	if _, err := os.Stat(filepath.Join(to, "fuera.txt")); err == nil {
		t.Fatal("no debe escribir fuera de la carpeta de destino")
	}
	if _, err := os.Stat(filepath.Join(to, "Descargas", "malo")); err == nil {
		t.Fatal("la carpeta a medias debería haberse borrado")
	}
}

// * TestImportDownloadsSinCarpeta no falla si la carpeta de descargas no existe.
func TestImportDownloadsSinCarpeta(t *testing.T) {
	if imported, err := ImportDownloads(filepath.Join(t.TempDir(), "no-existe"), t.TempDir()); err != nil || imported != nil {
		t.Fatalf("esperaba nada y sin error: %v, %v", imported, err)
	}
}

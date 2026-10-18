// * Pruebas de la detección de instaladores en carpetas y de la creación de la ISO.
package installer

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
)

// * makeTree crea una carpeta con el aspecto de un Windows extraído por Rufus (nombres con mayúsculas variadas).
func makeTree(t *testing.T, big bool) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "Win11 Pro")
	files := map[string]int{
		"bootmgr":                       2048,
		"Boot/etfsboot.com":             4096,
		"EFI/Microsoft/Boot/efisys.bin": 4096,
		"sources/boot.wim":              8192,
		"sources/install.wim":           8192,
		"EFI/Boot/bootx64.efi":          2048,
		"setup.exe":                     1024,
	}
	for name, size := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, bytes.Repeat([]byte{7}, size), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if big {
		file, err := os.Create(filepath.Join(root, "sources", "big.wim"))
		if err != nil {
			t.Fatal(err)
		}
		file.Truncate(largeFile + 10)
		file.Close()
	}
	return root
}

// * TestDetect reconoce el instalador, ignora mayúsculas y devuelve rutas reales.
func TestDetect(t *testing.T) {
	root := makeTree(t, false)
	info := Detect(root)
	if info.Kind != KindWindows || info.BIOSBoot != "Boot/etfsboot.com" || info.UEFIBoot != "EFI/Microsoft/Boot/efisys.bin" {
		t.Fatalf("detección: %+v", info)
	}
	if info.Name != "Win11 Pro" || info.Label != "WIN11_PRO" {
		t.Fatalf("nombres: %+v", info)
	}
	measured, err := Measure(info)
	if err != nil || measured.Bytes != 2048+4096+4096+8192+8192+2048+1024 || measured.LargeFile {
		t.Fatalf("medida: %+v %v", measured, err)
	}
	if !measured.Bootable() {
		t.Fatal("debería poder arrancar")
	}
}

// * TestDetectNoInstalador: una carpeta cualquiera o un instalador sin install.wim no cuentan.
func TestDetectNoInstalador(t *testing.T) {
	if Detect(t.TempDir()).Kind != "" {
		t.Fatal("una carpeta vacía no es un instalador")
	}
	root := makeTree(t, false)
	os.Remove(filepath.Join(root, "sources", "install.wim"))
	if Detect(root).Kind != "" {
		t.Fatal("sin install.wim no es un instalador")
	}
}

// * TestArchivoGrande marca los archivos de más de 4 GiB (sparse: no ocupa disco).
func TestArchivoGrande(t *testing.T) {
	info, err := Measure(Detect(makeTree(t, true)))
	if err != nil || !info.LargeFile {
		t.Fatalf("debería detectar el archivo grande: %+v %v", info, err)
	}
}

// * TestNombres limpia nombres y etiquetas.
func TestNombres(t *testing.T) {
	if SafeName(`E:\`) != "E" && SafeName(`E:\`) != "Windows-installer" {
		t.Fatalf("SafeName: %q", SafeName(`E:\`))
	}
	if SafeName("  ") != "Windows-installer" {
		t.Fatal("un nombre vacío usa el nombre por defecto")
	}
	if got := Label("windows 11 (es) con un nombre larguísimo de verdad"); len(got) > 32 || got == "" {
		t.Fatalf("Label: %q", got)
	}
}

// counter guarda el último avance recibido.
type counter struct{ last atomic.Int64 }

// Report anota el avance.
func (c *counter) Report(done int64) { c.last.Store(done) }

// * TestMakeISO crea una ISO real y comprueba su firma ISO 9660 y el catálogo de arranque El Torito.
func TestMakeISO(t *testing.T) {
	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("genisoimage"); err != nil {
			t.Skip("falta genisoimage")
		}
	}
	info, err := Measure(Detect(makeTree(t, false)))
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "salida.iso")
	var progress counter
	if err := MakeISO(context.Background(), info, dest, &progress); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 40000 || string(data[0x8001:0x8006]) != "CD001" {
		t.Fatalf("no parece una ISO 9660 (%d bytes)", len(data))
	}
	if !bytes.Contains(data[0x8800:0xA000], []byte("EL TORITO SPECIFICATION")) {
		t.Fatal("falta el catálogo de arranque El Torito")
	}
	catalog := int(binary.LittleEndian.Uint32(data[0x8800+71:])) * 2048
	if data[catalog+1] != 0 || data[catalog+65] != 0xEF {
		t.Fatalf("plataformas del catálogo: %x %x", data[catalog+1], data[catalog+65])
	}
	if !bytes.Contains(data[:0x20000], []byte("WIN11_PRO")) {
		t.Fatal("falta la etiqueta del volumen")
	}
}

// * TestMakeISOCancelada: con el contexto ya cancelado devuelve error y no deja una ISO válida a medias.
func TestMakeISOCancelada(t *testing.T) {
	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("genisoimage"); err != nil {
			t.Skip("falta genisoimage")
		}
	}
	info, _ := Measure(Detect(makeTree(t, false)))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := MakeISO(ctx, info, filepath.Join(t.TempDir(), "x.iso"), nil); err == nil {
		t.Fatal("debería fallar con el contexto cancelado")
	}
}

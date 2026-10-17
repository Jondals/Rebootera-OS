// * Pruebas de lo que se añade al USB "del tirón": imágenes y carpetas de drivers.
package desktop

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testINF es un INF mínimo de x64 que el análisis de drivers acepta.
const testINF = `[Version]
Signature="$Windows NT$"
Class=SCSIAdapter
Provider=Prueba
DriverVer = 01/14/2025,1.0.0.1
[Manufacturer]
Prueba = Models,NTamd64
[Models.NTamd64]
Controlador = inst, PCI\VEN_1AF4&DEV_1004
`

// * write crea un archivo con su carpeta.
func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// * fakeUSB crea la raíz de un USB de Rebootera con su ISO del sistema.
func fakeUSB(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "rebootera", "rebootera-os.iso"), []byte("iso del sistema"))
	return root
}

// * waitIdle espera a que termine la operación en segundo plano.
func waitIdle(t *testing.T) CreateStatus {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if status := Status(); !status.Running {
			return status
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("la operación no terminó")
	return CreateStatus{}
}

// * TestExtrasValidate rechaza imágenes y carpetas que no sirven antes de tocar nada.
func TestExtrasValidate(t *testing.T) {
	source := t.TempDir()
	iso := filepath.Join(source, "W11-LTSC.iso")
	write(t, iso, []byte("windows"))
	write(t, filepath.Join(source, "notas.txt"), []byte("no es una imagen"))
	empty := filepath.Join(source, "vacia")
	os.MkdirAll(empty, 0o755)
	good := filepath.Join(source, "red")
	write(t, filepath.Join(good, "net.inf"), []byte(testINF))

	cases := []struct {
		name   string
		extras Extras
		want   string
	}{
		{"imagen buena y drivers buenos", Extras{Images: []Image{{iso, "windows"}}, DriverDirs: []string{good}}, ""},
		{"categoría desconocida", Extras{Images: []Image{{iso, "otra"}}}, "categoría"},
		{"no es una imagen", Extras{Images: []Image{{filepath.Join(source, "notas.txt"), "tools"}}}, "no es una imagen"},
		{"imagen que no existe", Extras{Images: []Image{{filepath.Join(source, "no.iso"), "linux"}}}, "no se encuentra"},
		{"imagen repetida", Extras{Images: []Image{{iso, "windows"}, {iso, "windows"}}}, "repetida"},
		{"carpeta sin drivers", Extras{DriverDirs: []string{empty}}, "ningún paquete"},
		{"carpeta que no existe", Extras{DriverDirs: []string{filepath.Join(source, "x")}}, "no se encuentra"},
	}
	for _, c := range cases {
		err := c.extras.Validate()
		if c.want == "" && err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)) {
			t.Fatalf("%s: se esperaba %q y salió %v", c.name, c.want, err)
		}
	}
}

// * TestApplyKeepSystem añade una ISO y drivers a un USB existente sin tocar Rebootera OS.
func TestApplyKeepSystem(t *testing.T) {
	root := fakeUSB(t)
	source := t.TempDir()
	payload := bytes.Repeat([]byte("W11"), 100000)
	iso := filepath.Join(source, "W11-LTSC.iso")
	write(t, iso, payload)
	drivers := filepath.Join(source, "intel-red")
	write(t, filepath.Join(drivers, "x64", "net.inf"), []byte(testINF))
	write(t, filepath.Join(drivers, "x64", "net.sys"), []byte("sys"))
	write(t, filepath.Join(drivers, "setup.exe"), []byte("instalador"))
	write(t, filepath.Join(drivers, "otra.iso"), []byte("imagen que no es un driver"))

	request := ApplyRequest{Root: root, System: SystemKeep, Extras: Extras{Images: []Image{{iso, "windows"}}, DriverDirs: []string{drivers}}}
	if err := Apply(request); err != nil {
		t.Fatal(err)
	}
	status := waitIdle(t)
	if status.Error != "" || !status.Done || status.Drive != root {
		t.Fatalf("estado final: %+v", status)
	}
	copied, err := os.ReadFile(filepath.Join(root, "isos", "windows", "W11-LTSC.iso"))
	if err != nil || !bytes.Equal(copied, payload) {
		t.Fatalf("la ISO no se copió entera: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "isos", "windows", "W11-LTSC.iso.part")); err == nil {
		t.Fatal("quedó un .part")
	}
	for _, name := range []string{"net.inf", "net.sys"} {
		if _, err := os.Stat(filepath.Join(root, "drivers", "intel-red", "x64", name)); err != nil {
			t.Fatalf("falta %s: %v", name, err)
		}
	}
	// * De una carpeta de drivers no se copian instaladores ni imágenes de disco.
	for _, name := range []string{"setup.exe", "otra.iso"} {
		if _, err := os.Stat(filepath.Join(root, "drivers", "intel-red", name)); err == nil {
			t.Fatalf("%s no debería copiarse", name)
		}
	}
	system, _ := os.ReadFile(filepath.Join(root, "rebootera", "rebootera-os.iso"))
	if string(system) != "iso del sistema" {
		t.Fatal("Rebootera OS cambió con system=keep")
	}
}

// * TestApplyRejects comprueba las comprobaciones previas de Apply.
func TestApplyRejects(t *testing.T) {
	notUSB := t.TempDir()
	if err := Apply(ApplyRequest{Root: notUSB, System: SystemKeep}); err == nil {
		t.Fatal("una carpeta que no es un USB de Rebootera debe rechazarse")
	}
	root := fakeUSB(t)
	if err := Apply(ApplyRequest{Root: root, System: SystemKeep}); err == nil || !strings.Contains(err.Error(), "nada que actualizar") {
		t.Fatalf("sin sistema ni extras no hay nada que hacer: %v", err)
	}
	if err := Apply(ApplyRequest{Root: root, System: SystemFile, ISO: filepath.Join(root, "no.iso")}); err == nil {
		t.Fatal("una ISO inexistente debe rechazarse")
	}
	if err := Apply(ApplyRequest{Root: root, System: "raro"}); err == nil {
		t.Fatal("un origen desconocido debe rechazarse")
	}
}

// * TestApplyNewSystem instala una ISO de Rebootera nueva y deja la anterior de rescate.
func TestApplyNewSystem(t *testing.T) {
	root := fakeUSB(t)
	iso := filepath.Join(t.TempDir(), "rebootera-os-9.9.9-amd64.iso")
	write(t, iso, []byte("iso nueva"))
	if err := Apply(ApplyRequest{Root: root, System: SystemFile, ISO: iso}); err != nil {
		t.Fatal(err)
	}
	if status := waitIdle(t); status.Error != "" || !status.Done {
		t.Fatalf("estado final: %+v", status)
	}
	current, _ := os.ReadFile(filepath.Join(root, "rebootera", "rebootera-os.iso"))
	previous, _ := os.ReadFile(filepath.Join(root, "rebootera", "rebootera-os.previous.iso"))
	if string(current) != "iso nueva" || string(previous) != "iso del sistema" {
		t.Fatalf("actual %q, anterior %q", current, previous)
	}
}

// * TestDriversAddRemove añade y quita una carpeta de drivers; solo acepta nombres de primer nivel.
func TestDriversAddRemove(t *testing.T) {
	root := fakeUSB(t)
	source := filepath.Join(t.TempDir(), "wifi")
	write(t, filepath.Join(source, "w.inf"), []byte(testINF))
	if err := AddDrivers(root, source); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "drivers", "wifi", "w.inf")); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "..", ".", "wifi/x", `..\rebootera`, "../rebootera"} {
		if err := RemoveDrivers(root, bad); err == nil {
			t.Fatalf("%q debería rechazarse", bad)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "rebootera", "rebootera-os.iso")); err != nil {
		t.Fatal("RemoveDrivers tocó fuera de drivers/")
	}
	if err := RemoveDrivers(root, "wifi"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "drivers", "wifi")); err == nil {
		t.Fatal("la carpeta sigue ahí")
	}
}

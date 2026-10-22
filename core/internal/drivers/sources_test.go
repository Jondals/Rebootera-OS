// * Pruebas de los orígenes de drivers: el USB, una partición con carpeta de drivers y un Windows instalado.
package drivers

import (
	"os"
	"path/filepath"
	"testing"

	"rebootera/core/internal/hardware"
)

// * TestDiscoverYScan monta a mano el aspecto de /run/rebootera/media con tres particiones y comprueba que
// * cada origen se reconoce (sin distinguir mayúsculas), se recorre y deja anotado de dónde sale cada paquete.
func TestDiscoverYScan(t *testing.T) {
	vaultRoot := t.TempDir()
	media := t.TempDir()
	writeFile(t, filepath.Join(vaultRoot, "drivers", "disco", "vioscsi.inf"), []byte(storageINF))
	// * Partición con carpeta "Drivers" (como la de un portátil con partición de drivers).
	writeFile(t, filepath.Join(media, "sda3", "Drivers", "red", "vioscsi.inf"), []byte(storageINF))
	// * Un Windows instalado: almacén de drivers con mayúsculas distintas.
	writeFile(t, filepath.Join(media, "nvme0n1p3", "Windows", "System32", "DriverStore", "FileRepository", "x", "vioscsi.inf"), []byte(storageINF))
	// * Una partición sin nada que sirva.
	writeFile(t, filepath.Join(media, "sdb1", "fotos", "a.jpg"), []byte("x"))

	found := Discover(vaultRoot, media)
	kinds := map[string]string{}
	for _, source := range found {
		kinds[source.ID] = source.Kind
	}
	if len(found) != 3 || kinds["vault"] != KindUSB || kinds["sda3:drivers"] != KindPartition || kinds["nvme0n1p3:windows/system32/driverstore/filerepository"] != KindWindows {
		t.Fatalf("orígenes: %+v", found)
	}

	packages, used := ScanSources(found)
	if len(packages) != 3 || len(used) != 3 {
		t.Fatalf("paquetes %d, orígenes %d", len(packages), len(used))
	}
	for _, pkg := range packages {
		if pkg.SourceID == "" || pkg.Root == "" {
			t.Fatalf("paquete sin origen: %+v", pkg)
		}
	}
	// * Sin USB ni medios no hay orígenes.
	if got := Discover("", filepath.Join(media, "no-existe")); len(got) != 0 {
		t.Fatalf("debería estar vacío: %+v", got)
	}
}

// * TestParticionDriversPorEtiqueta comprueba que una partición etiquetada "DRIVERS" se recorre entera (sus INF están en
// * la raíz, sin carpeta "Drivers") y que, frente a un Windows instalado, gana siempre la partición de fábrica.
func TestParticionDriversPorEtiqueta(t *testing.T) {
	media := t.TempDir()
	labels := t.TempDir()
	writeFile(t, filepath.Join(media, "nvme0n1p5", "intel", "vioscsi.inf"), []byte(storageINF))
	writeFile(t, filepath.Join(media, "nvme0n1p3", "Windows", "System32", "DriverStore", "FileRepository", "x", "vioscsi.inf"), []byte(storageINF))
	if err := os.Symlink("../../nvme0n1p5", filepath.Join(labels, `MY\x20DRIVERS`)); err != nil {
		t.Skip("sin enlaces simbólicos: " + err.Error())
	}
	previous := byLabelDir
	byLabelDir = labels
	defer func() { byLabelDir = previous }()

	found := Discover("", media)
	packages, _ := ScanSources(found)
	if len(packages) != 2 {
		t.Fatalf("debería haber un paquete de la partición y otro de Windows: %+v", found)
	}
	devices := []hardware.Device{{HardwareIDs: []string{`PCI\VEN_1AF4&DEV_1004`}}}
	plan := BuildPlan(devices, packages)
	if len(plan.Matches) != 1 || plan.Matches[0].Package.SourceID != "nvme0n1p5:label" {
		t.Fatalf("debería elegir la partición DRIVERS, no el Windows instalado: %+v", plan.Matches)
	}
}

// * TestParticionDriversSinAbrir comprueba que una partición "DRIVERS" que el agente no pudo abrir sale con su motivo, y que
// * la etiqueta anotada por el agente basta aunque udev no publicara el enlace by-label.
func TestParticionDriversSinAbrir(t *testing.T) {
	media := t.TempDir()
	report := filepath.Join(t.TempDir(), "drivers-scan")
	os.WriteFile(report, []byte("sdb3\tDRIVERS\terror: no se pudo abrir sdb3 (ntfs): invalid argument\nsdb4\tMis controladores\topen\n"), 0o644)
	writeFile(t, filepath.Join(media, "sdb4", "intel", "vioscsi.inf"), []byte(storageINF))
	previousReport, previousLabels := ReportPath, byLabelDir
	ReportPath, byLabelDir = report, filepath.Join(t.TempDir(), "sin-udev")
	defer func() { ReportPath, byLabelDir = previousReport, previousLabels }()

	packages, used := ScanSources(Discover("", media))
	if len(packages) != 1 || packages[0].SourceID != "sdb4:label" {
		t.Fatalf("la partición con etiqueta del agente debe recorrerse entera: %+v", packages)
	}
	var failed *Source
	for index := range used {
		if used[index].Device == "sdb3" {
			failed = &used[index]
		}
	}
	if failed == nil || failed.Error == "" || failed.Packages != 0 {
		t.Fatalf("falta la partición que no se pudo abrir, con su motivo: %+v", used)
	}
}

// * TestEtiquetasDeDrivers comprueba qué etiquetas se toman por una partición de drivers.
func TestEtiquetasDeDrivers(t *testing.T) {
	for _, label := range []string{"DRIVERS", "Drivers HP", "SWSetup", "SW_SETUP", "Controladores", "DRV"} {
		if !IsDriversLabel(label) {
			t.Fatalf("%q debería ser una partición de drivers", label)
		}
	}
	for _, label := range []string{"REBOOTERA", "Windows", "DATOS", ""} {
		if IsDriversLabel(label) {
			t.Fatalf("%q no debería serlo", label)
		}
	}
}

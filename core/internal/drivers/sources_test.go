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

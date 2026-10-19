// * Pruebas de los orígenes de drivers: el USB, una partición con carpeta de drivers y un Windows instalado.
package drivers

import (
	"path/filepath"
	"testing"
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

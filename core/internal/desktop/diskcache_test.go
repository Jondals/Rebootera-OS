// * Pruebas de la caché de discos USB: la segunda petición no vuelve a leer, y una fresca sí.
package desktop

import (
	"testing"
	"time"

	"rebootera/core/internal/testutil"
)

// reads cuenta las lecturas reales.
var reads int

// fakeList sustituye a la lectura real y devuelve un disco.
func fakeList() ([]USBDisk, error) {
	reads++
	return []USBDisk{{Number: reads, FriendlyName: "Prueba"}}, nil
}

// * TestCacheDeDiscos comprueba que la caché evita lecturas repetidas y que fresh la salta.
func TestCacheDeDiscos(t *testing.T) {
	t.Cleanup(testutil.Keep(&listDisks).Restore)
	listDisks = fakeList
	reads = 0
	diskCache.disks, diskCache.at = nil, time.Time{}

	first, _ := CachedUSBDisks(false)
	second, _ := CachedUSBDisks(false)
	if reads != 1 || first[0].Number != 1 || second[0].Number != 1 {
		t.Fatalf("la segunda petición debía salir de la caché (lecturas: %d)", reads)
	}
	fresh, _ := CachedUSBDisks(true)
	if reads != 2 || fresh[0].Number != 2 {
		t.Fatalf("una petición fresca debía leer de nuevo (lecturas: %d)", reads)
	}
	diskCache.at = time.Now().Add(-2 * diskCacheTTL)
	CachedUSBDisks(false)
	if reads != 3 {
		t.Fatalf("una lista vieja debía volver a leerse (lecturas: %d)", reads)
	}
}

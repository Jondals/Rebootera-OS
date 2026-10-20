// * Caché breve de la lista de discos USB. Listarlos en Windows lanza PowerShell (Get-Disk, Get-Partition y
// * Get-Volume), que tarda unos segundos: sin caché, abrir "Crear USB" dejaba la ventana vacía todo ese tiempo.
// * La aplicación calienta la lista al arrancar y la interfaz pide la caché primero (al instante) y después una
// * lectura fresca en segundo plano.
package desktop

import (
	"sync"
	"time"
)

// diskCacheTTL es cuánto vale una lista guardada si no se pide una fresca.
const diskCacheTTL = 8 * time.Second

// listDisks es la lectura real de los discos (se cambia en las pruebas).
var listDisks = USBDisks

// diskCache guarda la última lista leída y cuándo.
var diskCache struct {
	mu    sync.Mutex
	at    time.Time
	disks []USBDisk
}

// * CachedUSBDisks devuelve la lista de discos USB: la guardada si tiene menos de diskCacheTTL, o una nueva si
// * hace falta o si fresh es true. Varias peticiones a la vez comparten una sola lectura.
func CachedUSBDisks(fresh bool) ([]USBDisk, error) {
	diskCache.mu.Lock()
	defer diskCache.mu.Unlock()
	if !fresh && diskCache.disks != nil && time.Since(diskCache.at) < diskCacheTTL {
		return diskCache.disks, nil
	}
	disks, err := listDisks()
	if err != nil {
		return nil, err
	}
	diskCache.disks, diskCache.at = disks, time.Now()
	return disks, nil
}

// * PrewarmDisks lee los discos en segundo plano para que la primera vez que se abra "Crear USB" ya estén.
func PrewarmDisks() {
	go warmDisks()
}

// * warmDisks hace la lectura de PrewarmDisks (el resultado queda en la caché; un error no importa).
func warmDisks() {
	CachedUSBDisks(true)
}

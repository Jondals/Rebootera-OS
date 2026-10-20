package storage

import (
	"sync"
	"time"
)

// Tiempos de la caché de uso de las particiones montadas.
const (
	// usageFresh es lo que vale un dato antes de volver a medir.
	usageFresh = 60 * time.Second
	// usageWait es lo máximo que se espera a una primera medida: si tarda más se contesta sin ella y se sigue midiendo.
	usageWait = 250 * time.Millisecond
)

// usageEntry es lo último que se midió de un punto de montaje.
type usageEntry struct {
	used, free uint64
	at         time.Time
	ready      bool
	busy       bool
}

var (
	usageMu    sync.Mutex
	usageCache = map[string]*usageEntry{}
)

// * cachedUsage da el uso de un punto de montaje sin bloquear al que lo pide. Medir el espacio libre de algunos sistemas de
// * archivos (NTFS por FUSE, sobre todo en discos grandes o en un pendrive) puede tardar segundos; el listado de discos lo
// * pedía en cada lectura. Ahora se mide en segundo plano, se espera como mucho un instante a la primera medida y después se
// * contesta con la última conocida mientras se refresca.
func cachedUsage(path string) (used, free uint64) {
	usageMu.Lock()
	entry := usageCache[path]
	if entry == nil {
		entry = &usageEntry{}
		usageCache[path] = entry
	}
	if entry.ready && time.Since(entry.at) < usageFresh {
		used, free = entry.used, entry.free
		usageMu.Unlock()
		return used, free
	}
	var done chan struct{}
	if !entry.busy {
		entry.busy = true
		done = make(chan struct{})
		go measureUsage(path, entry, done)
	}
	ready := entry.ready
	used, free = entry.used, entry.free
	usageMu.Unlock()
	if ready || done == nil {
		return used, free
	}
	select {
	case <-done:
	case <-time.After(usageWait):
	}
	usageMu.Lock()
	defer usageMu.Unlock()
	return entry.used, entry.free
}

// * measureUsage mide un punto de montaje y guarda el resultado.
func measureUsage(path string, entry *usageEntry, done chan struct{}) {
	used, free := FSUsage(path)
	usageMu.Lock()
	entry.used, entry.free, entry.at, entry.ready, entry.busy = used, free, time.Now(), true, false
	usageMu.Unlock()
	close(done)
}

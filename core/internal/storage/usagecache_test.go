package storage

import (
	"testing"
	"time"
)

// * TestCachedUsage comprueba que la primera lectura mide, que la siguiente sale de la caché y que un dato viejo se
// * contesta al momento mientras se refresca.
func TestCachedUsage(t *testing.T) {
	dir := t.TempDir()
	used, free := cachedUsage(dir)
	if used+free == 0 {
		t.Fatal("la primera lectura debería medir el directorio temporal")
	}
	again1, again2 := cachedUsage(dir)
	if again1 != used || again2 != free {
		t.Fatalf("la segunda lectura sale de la caché: %d %d contra %d %d", again1, again2, used, free)
	}
	// * Un dato caducado se devuelve igualmente (sin esperar) y se refresca por detrás.
	usageMu.Lock()
	usageCache[dir].at = time.Now().Add(-2 * usageFresh)
	usageMu.Unlock()
	start := time.Now()
	stale1, stale2 := cachedUsage(dir)
	if time.Since(start) > usageWait || stale1 != used || stale2 != free {
		t.Fatalf("el dato viejo debe contestarse al momento: %d %d en %v", stale1, stale2, time.Since(start))
	}
	time.Sleep(50 * time.Millisecond)
	usageMu.Lock()
	refreshed := time.Since(usageCache[dir].at) < usageFresh
	usageMu.Unlock()
	if !refreshed {
		t.Fatal("el dato viejo debería haberse refrescado por detrás")
	}
}

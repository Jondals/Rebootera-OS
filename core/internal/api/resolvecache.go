// * Caché de las consultas "última versión" del catálogo. Cada consulta pide un
// * índice (y a veces su firma) a la web del proyecto; al volver a abrir el
// * Centro de descargas se reutiliza el resultado en vez de repetir siete
// * peticiones de red.
package api

import (
	"sync"
	"time"

	"rebootera/core/internal/catalog"
)

// resolveTTL es cuánto tiempo vale una consulta que salió bien.
const resolveTTL = 10 * time.Minute

// cachedResolve es una consulta guardada con su momento.
type cachedResolve struct {
	result catalog.Resolved
	at     time.Time
}

// resolveCache guarda las consultas correctas por identificador de entrada.
type resolveCache struct {
	mu      sync.Mutex
	entries map[string]cachedResolve
}

// * lookup devuelve una consulta reciente, si la hay.
func (c *resolveCache) lookup(id string) (catalog.Resolved, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[id]
	if !ok || time.Since(entry.at) > resolveTTL {
		return catalog.Resolved{}, false
	}
	return entry.result, true
}

// * store guarda una consulta que salió bien (los fallos no se guardan).
func (c *resolveCache) store(id string, result catalog.Resolved) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]cachedResolve{}
	}
	c.entries[id] = cachedResolve{result: result, at: time.Now()}
}

// * Caché de las consultas "última versión" del catálogo. Cada consulta pide un
// * índice (y a veces su firma) a la web del proyecto; al volver a abrir el
// * Centro de descargas se reutiliza el resultado en vez de repetir las
// * peticiones de red. Se guarda también en disco: lo último que se supo (aunque
// * esté caducado) sirve para enseñar los tamaños al instante mientras se confirma.
package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"rebootera/core/internal/catalog"
)

// resolveTTL es cuánto tiempo vale una consulta que salió bien.
const resolveTTL = 10 * time.Minute

// resolveKeep es cuánto se conserva en disco una consulta (pasado esto se olvida del todo).
const resolveKeep = 30 * 24 * time.Hour

// cachedResolve es una consulta guardada con su momento.
type cachedResolve struct {
	Result catalog.Resolved `json:"result"`
	At     time.Time        `json:"at"`
}

// resolveCache guarda las consultas correctas por identificador de entrada.
type resolveCache struct {
	mu      sync.Mutex
	entries map[string]cachedResolve
	path    string
}

// * open carga la caché guardada en disco y guarda en ella los cambios siguientes. Sin ruta, solo vive en memoria.
func (c *resolveCache) open(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.path = path
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var saved map[string]cachedResolve
	if json.Unmarshal(data, &saved) != nil {
		return
	}
	if c.entries == nil {
		c.entries = map[string]cachedResolve{}
	}
	for id, entry := range saved {
		if time.Since(entry.At) < resolveKeep {
			c.entries[id] = entry
		}
	}
}

// * lookup devuelve una consulta reciente, si la hay.
func (c *resolveCache) lookup(id string) (catalog.Resolved, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[id]
	if !ok || time.Since(entry.At) > resolveTTL {
		return catalog.Resolved{}, false
	}
	return entry.Result, true
}

// * known devuelve todo lo que se sabe, también lo caducado: es solo para mostrar tamaños, nunca para descargar.
func (c *resolveCache) known() map[string]catalog.Resolved {
	c.mu.Lock()
	defer c.mu.Unlock()
	all := make(map[string]catalog.Resolved, len(c.entries))
	for id, entry := range c.entries {
		all[id] = entry.Result
	}
	return all
}

// * store guarda una consulta que salió bien (los fallos no se guardan).
func (c *resolveCache) store(id string, result catalog.Resolved) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]cachedResolve{}
	}
	c.entries[id] = cachedResolve{Result: result, At: time.Now()}
	c.saveLocked()
}

// * saveLocked escribe la caché en disco (con el cerrojo ya tomado). Si no se puede, no pasa nada: es solo una ayuda.
func (c *resolveCache) saveLocked() {
	if c.path == "" {
		return
	}
	data, err := json.Marshal(c.entries)
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(c.path), 0o700) != nil {
		return
	}
	temporary := c.path + ".tmp"
	if os.WriteFile(temporary, data, 0o600) == nil {
		_ = os.Rename(temporary, c.path)
	}
}

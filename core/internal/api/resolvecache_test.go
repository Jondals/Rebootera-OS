// * Pruebas de la caché de consultas del catálogo.
package api

import (
	"testing"
	"time"

	"rebootera/core/internal/catalog"
)

// * TestResolveCache comprueba que se guarda, caduca y no mezcla entradas.
func TestResolveCache(t *testing.T) {
	var cache resolveCache
	if _, ok := cache.lookup("debian"); ok {
		t.Fatal("una caché vacía no debe devolver nada")
	}
	cache.store("debian", catalog.Resolved{FileName: "debian.iso", SizeBytes: 5})
	got, ok := cache.lookup("debian")
	if !ok || got.FileName != "debian.iso" {
		t.Fatalf("debería devolver lo guardado: %+v %v", got, ok)
	}
	if _, ok := cache.lookup("ubuntu"); ok {
		t.Fatal("otra entrada no debe salir")
	}
	// * Una consulta antigua caduca.
	cache.entries["debian"] = cachedResolve{result: got, at: time.Now().Add(-2 * resolveTTL)}
	if _, ok := cache.lookup("debian"); ok {
		t.Fatal("una consulta caducada no debe devolverse")
	}
}

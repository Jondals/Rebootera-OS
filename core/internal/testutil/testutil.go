// * Package testutil: ayudas para las pruebas. Keep guarda el valor de una
// * variable de paquete para restaurarlo al final de la prueba sin usar
// * funciones anónimas:
// *
// *   t.Cleanup(testutil.Keep(&vault.Root).Restore)
// *   vault.Root = t.TempDir()
package testutil

// Saved es el valor guardado de una variable.
type Saved[T any] struct {
	ptr   *T
	value T
}

// * Keep guarda el valor actual de la variable a la que apunta ptr.
func Keep[T any](ptr *T) Saved[T] {
	return Saved[T]{ptr: ptr, value: *ptr}
}

// * Restore devuelve la variable a su valor guardado.
func (s Saved[T]) Restore() {
	*s.ptr = s.value
}

// Recorder guarda el último valor recibido (por ejemplo, el progreso).
type Recorder struct {
	Last float64
}

// * Record anota un valor; sirve como función de progreso en las pruebas.
func (r *Recorder) Record(value float64) {
	r.Last = value
}

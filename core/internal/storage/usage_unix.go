//go:build linux || darwin

// * Uso de un sistema de archivos montado en Linux (statfs).
package storage

import "syscall"

// * FSUsage devuelve los bytes usados y libres del sistema de archivos que
// * contiene path. Devuelve ceros si no se puede consultar.
func FSUsage(path string) (used, free uint64) {
	var st syscall.Statfs_t
	if syscall.Statfs(path, &st) != nil {
		return 0, 0
	}
	return (st.Blocks - st.Bfree) * uint64(st.Bsize), st.Bavail * uint64(st.Bsize)
}

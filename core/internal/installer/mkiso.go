// * Tipos comunes de la creación de una ISO desde la carpeta de un instalador: cada sistema operativo aporta su MakeISO.
package installer

// Reporter recibe el avance de la creación de la ISO (bytes ya escritos, aproximado).
type Reporter interface {
	Report(done int64)
}

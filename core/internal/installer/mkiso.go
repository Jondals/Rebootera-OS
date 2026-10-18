package installer

// Reporter recibe el avance de la creación de la ISO (bytes ya escritos, aproximado).
type Reporter interface {
	Report(done int64)
}

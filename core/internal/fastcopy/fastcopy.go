// * Package fastcopy: copia de archivos grandes en tubería. Una gorrutina lee
// * mientras otra escribe, con búferes de 4 MiB, de modo que el disco de origen
// * (o la red) y el USB trabajan a la vez en vez de esperarse. Antes cada
// * trozo se leía, se escribía y se calculaba su hash uno detrás de otro, lo que
// * en un USB lento duplicaba el tiempo.
package fastcopy

import (
	"context"
	"errors"
	"io"
)

// Tamaño de cada búfer y cuántos circulan a la vez (3 x 4 MiB = 12 MiB de memoria).
const (
	BufferSize = 4 << 20
	buffers    = 3
)

// Visitor recibe cada trozo después de escribirlo (para el hash y el progreso).
type Visitor interface {
	Visit(chunk []byte)
}

// ReadError envuelve un fallo al leer del origen.
type ReadError struct{ Err error }

// Error devuelve el texto del fallo de lectura.
func (e *ReadError) Error() string { return e.Err.Error() }

// Unwrap permite usar errors.Is y errors.As con el fallo original.
func (e *ReadError) Unwrap() error { return e.Err }

// WriteError envuelve un fallo al escribir en el destino.
type WriteError struct{ Err error }

// Error devuelve el texto del fallo de escritura.
func (e *WriteError) Error() string { return e.Err.Error() }

// Unwrap permite usar errors.Is y errors.As con el fallo original.
func (e *WriteError) Unwrap() error { return e.Err }

// chunk es un búfer lleno y lo que pasó al llenarlo.
type chunk struct {
	buf []byte
	n   int
	err error
}

// reader llena búferes libres con lo que lee del origen.
type reader struct {
	src  io.Reader
	free chan []byte
	full chan chunk
	stop chan struct{}
}

// * run lee hasta el final, un error o la parada, y siempre cierra full.
func (r *reader) run() {
	defer close(r.full)
	for {
		var buf []byte
		select {
		case buf = <-r.free:
		case <-r.stop:
			return
		}
		n, err := r.src.Read(buf)
		select {
		case r.full <- chunk{buf: buf, n: n, err: err}:
		case <-r.stop:
			return
		}
		if err != nil {
			return
		}
	}
}

// * Copy copia src en dst y devuelve los bytes escritos. Un fallo de lectura se
// * devuelve como *ReadError y uno de escritura como *WriteError; la
// * cancelación del contexto, tal cual. visitor puede ser nil.
func Copy(ctx context.Context, dst io.Writer, src io.Reader, visitor Visitor) (int64, error) {
	r := &reader{
		src:  src,
		free: make(chan []byte, buffers),
		full: make(chan chunk, buffers),
		stop: make(chan struct{}),
	}
	for i := 0; i < buffers; i++ {
		r.free <- make([]byte, BufferSize)
	}
	go r.run()
	// * Al salir (bien o mal) se avisa a la lectora para que no se quede esperando un búfer.
	defer close(r.stop)

	var written int64
	for {
		var c chunk
		var ok bool
		select {
		case c, ok = <-r.full:
		case <-ctx.Done():
			return written, ctx.Err()
		}
		if !ok {
			return written, errors.New("la lectura terminó sin fin de archivo")
		}
		if c.n > 0 {
			if _, err := dst.Write(c.buf[:c.n]); err != nil {
				return written, &WriteError{Err: err}
			}
			written += int64(c.n)
			if visitor != nil {
				visitor.Visit(c.buf[:c.n])
			}
		}
		if c.err == io.EOF {
			return written, nil
		}
		if c.err != nil {
			return written, &ReadError{Err: c.err}
		}
		r.free <- c.buf
	}
}

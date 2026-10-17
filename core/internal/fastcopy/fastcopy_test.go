// * Pruebas de la copia en tubería: integridad, errores y cancelación.
package fastcopy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"hash"
	"io"
	"math/rand"
	"testing"
	"time"
)

// hashVisitor calcula el SHA-256 de lo escrito y cuenta los trozos.
type hashVisitor struct {
	hasher hash.Hash
	chunks int
}

// Visit anota un trozo.
func (h *hashVisitor) Visit(chunk []byte) {
	h.hasher.Write(chunk)
	h.chunks++
}

// failingReader falla tras entregar unos bytes.
type failingReader struct {
	data []byte
	err  error
}

// Read entrega los datos y después el error.
func (f *failingReader) Read(p []byte) (int, error) {
	if len(f.data) == 0 {
		return 0, f.err
	}
	n := copy(p, f.data)
	f.data = f.data[n:]
	return n, nil
}

// failingWriter falla en la escritura número n.
type failingWriter struct {
	writes int
	failAt int
}

// Write falla al llegar a failAt.
func (f *failingWriter) Write(p []byte) (int, error) {
	f.writes++
	if f.writes >= f.failAt {
		return 0, errors.New("disco lleno")
	}
	return len(p), nil
}

// copyRun ejecuta Copy en segundo plano y deja su resultado en done.
type copyRun struct {
	ctx  context.Context
	src  io.Reader
	done chan error
}

// copy lanza la copia y publica el error.
func (c *copyRun) copy() {
	_, err := Copy(c.ctx, io.Discard, c.src, nil)
	c.done <- err
}

// blockedReader no devuelve nada hasta que se cierra su canal.
type blockedReader struct{ release chan struct{} }

// Read espera a la liberación.
func (b *blockedReader) Read(p []byte) (int, error) {
	<-b.release
	return 0, io.EOF
}

// * TestCopyIntegrity copia más de dos búferes con tamaños raros y comprueba el hash.
func TestCopyIntegrity(t *testing.T) {
	data := make([]byte, 2*BufferSize+12345)
	rand.New(rand.NewSource(1)).Read(data)
	want := sha256.Sum256(data)
	for _, size := range []int{0, 1, BufferSize - 1, BufferSize, BufferSize + 1, len(data)} {
		var out bytes.Buffer
		hasher := sha256.New()
		visitor := &hashVisitor{hasher: hasher}
		n, err := Copy(context.Background(), &out, bytes.NewReader(data[:size]), visitor)
		if err != nil || n != int64(size) || !bytes.Equal(out.Bytes(), data[:size]) {
			t.Fatalf("tamaño %d: n=%d err=%v", size, n, err)
		}
		if size == len(data) && !bytes.Equal(hasher.Sum(nil), want[:]) {
			t.Fatal("el hash visto no coincide con el de los datos")
		}
	}
}

// * TestCopyErrors distingue los fallos de lectura de los de escritura.
func TestCopyErrors(t *testing.T) {
	boom := errors.New("se cortó la red")
	_, err := Copy(context.Background(), io.Discard, &failingReader{data: make([]byte, 100), err: boom}, nil)
	var read *ReadError
	if !errors.As(err, &read) || !errors.Is(err, boom) {
		t.Fatalf("debería ser un fallo de lectura: %v", err)
	}
	data := make([]byte, 3*BufferSize)
	_, err = Copy(context.Background(), &failingWriter{failAt: 2}, bytes.NewReader(data), nil)
	var write *WriteError
	if !errors.As(err, &write) {
		t.Fatalf("debería ser un fallo de escritura: %v", err)
	}
}

// * TestCopyCancel vuelve en cuanto se cancela, aunque el origen no responda.
func TestCopyCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	source := &blockedReader{release: make(chan struct{})}
	defer close(source.release)
	run := &copyRun{ctx: ctx, src: source, done: make(chan error, 1)}
	done := run.done
	go run.copy()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("debería devolver la cancelación: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Copy no volvió tras cancelar")
	}
}

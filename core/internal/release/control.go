// * Control para pausar, reanudar y cancelar la descarga de la última ISO desde otra goroutine.
package release

import (
	"context"
	"errors"
	"sync"
)

// ErrCancelled se devuelve cuando la descarga se cancela.
var ErrCancelled = errors.New("descarga cancelada")

// * Control deja pausar, reanudar y cancelar una descarga. El valor nil no controla nada: la descarga
// * sigue hasta el final.
type Control struct {
	mu        sync.Mutex
	paused    bool
	cancelled bool
	// resume se cierra al reanudar o cancelar, y despierta a quien espera en hold.
	resume chan struct{}
	// abort corta la conexión en curso, para que pausar y cancelar surtan efecto aunque la red esté parada.
	abort context.CancelFunc
}

// * NewControl crea un control sin pausar.
func NewControl() *Control {
	return &Control{}
}

// * Pause corta la conexión y deja la descarga esperando, conservando lo descargado.
func (c *Control) Pause() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.paused || c.cancelled {
		return
	}
	c.paused = true
	c.resume = make(chan struct{})
	if c.abort != nil {
		c.abort()
	}
}

// * Resume continúa una descarga pausada desde donde se quedó.
func (c *Control) Resume() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.wake()
}

// * Cancel corta la descarga para siempre: lo descargado se borra.
func (c *Control) Cancel() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cancelled = true
	c.wake()
	if c.abort != nil {
		c.abort()
	}
}

// * wake despierta a quien espera en hold (con el bloqueo tomado).
func (c *Control) wake() {
	if c.paused {
		c.paused = false
		close(c.resume)
	}
}

// * Paused indica si la descarga está en pausa.
func (c *Control) Paused() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.paused
}

// * Cancelled indica si la descarga se ha cancelado.
func (c *Control) Cancelled() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cancelled
}

// * interrupted indica si hay que dejar la conexión en curso (pausa o cancelación).
func (c *Control) interrupted() bool {
	return c.Paused() || c.Cancelled()
}

// * hold espera mientras la descarga esté en pausa y devuelve ErrCancelled si se cancela.
func (c *Control) hold() error {
	if c == nil {
		return nil
	}
	for {
		c.mu.Lock()
		cancelled, paused, resume := c.cancelled, c.paused, c.resume
		c.mu.Unlock()
		switch {
		case cancelled:
			return ErrCancelled
		case !paused:
			return nil
		}
		<-resume
	}
}

// * connect da el contexto de una conexión nueva. Si ya se pidió pausar o cancelar, nace cortado.
func (c *Control) connect() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	if c == nil {
		return ctx, cancel
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.abort = cancel
	if c.paused || c.cancelled {
		cancel()
	}
	return ctx, cancel
}

// * Ajustes de las descargas: límite de velocidad y número de conexiones por
// * descarga ("Ajustar ancho de banda" en el Centro de descargas). Se guardan en
// * el Vault y se aplican en caliente, también a la descarga que está en marcha.
package transfers

import (
	"errors"
	"path/filepath"
	"sync"
	"time"

	"rebootera/core/internal/vault"
)

// Límites admitidos.
const (
	MaxConnections     = 8
	defaultConnections = 4
	// maxRate es el tope de velocidad que se acepta (10 GB/s: en la práctica, "sin límite").
	maxRate = 10 << 30
)

// Settings son los ajustes de ancho de banda.
type Settings struct {
	// MaxBytesPerSecond es el límite de velocidad de las descargas (0 = sin límite).
	MaxBytesPerSecond int64 `json:"maxBytesPerSecond"`
	// Connections es cuántas conexiones se usan a la vez para descargar un archivo (1 a 8).
	Connections int `json:"connections"`
}

// settingsPath es el archivo de ajustes dentro del Vault.
func settingsPath() string {
	return filepath.Join(vault.StateDir(), "bandwidth.json")
}

// * Settings devuelve los ajustes actuales.
func (m *Manager) Settings() Settings {
	m.setMu.Lock()
	defer m.setMu.Unlock()
	m.loadSettings()
	return m.settings
}

// * SetSettings valida, aplica y guarda los ajustes.
func (m *Manager) SetSettings(settings Settings) error {
	if settings.Connections < 1 || settings.Connections > MaxConnections {
		return errors.New("el número de conexiones debe estar entre 1 y 8")
	}
	if settings.MaxBytesPerSecond < 0 || settings.MaxBytesPerSecond > maxRate {
		return errors.New("límite de velocidad no válido")
	}
	m.setMu.Lock()
	defer m.setMu.Unlock()
	m.loadSettings()
	m.settings = settings
	if vault.Available() {
		if err := vault.WriteJSON(settingsPath(), settings); err != nil {
			return err
		}
	}
	return nil
}

// * loadSettings lee los ajustes guardados la primera vez (con setMu tomado).
func (m *Manager) loadSettings() {
	if m.settingsLoaded {
		return
	}
	m.settings = Settings{Connections: defaultConnections}
	if vault.Available() {
		saved := m.settings
		if err := vault.ReadJSON(settingsPath(), &saved); err == nil && saved.Connections >= 1 && saved.Connections <= MaxConnections && saved.MaxBytesPerSecond >= 0 {
			m.settings = saved
		}
		m.settingsLoaded = true
	}
}

// limiter reparte el ancho de banda permitido entre todas las conexiones: una cubeta de fichas con
// una ráfaga de un segundo.
type limiter struct {
	mu        sync.Mutex
	allowance float64
	last      time.Time
}

// * take anota que se van a usar n bytes y devuelve cuánto hay que esperar antes de usarlos para no
// * pasar de rate bytes por segundo (0 = sin límite).
func (l *limiter) take(n, rate int64) time.Duration {
	return l.takeAt(time.Now(), n, rate)
}

// * takeAt es take con la hora indicada (para probarlo sin esperar).
func (l *limiter) takeAt(now time.Time, n, rate int64) time.Duration {
	if rate <= 0 {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.last.IsZero() {
		l.last = now
		l.allowance = float64(rate)
	}
	l.allowance = min(l.allowance+now.Sub(l.last).Seconds()*float64(rate), float64(rate))
	l.last = now
	l.allowance -= float64(n)
	if l.allowance >= 0 {
		return 0
	}
	return time.Duration(-l.allowance / float64(rate) * float64(time.Second))
}

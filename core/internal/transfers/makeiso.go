package transfers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"rebootera/core/internal/installer"
	"rebootera/core/internal/vault"
)

// * MakeISO añade a la cola la creación de una ISO arrancable a partir de la carpeta de un instalador
// * de Windows (el contenido que extrae Rufus). La carpeta solo se lee; la ISO queda en la biblioteca.
func (m *Manager) MakeISO(folder string) (Job, error) {
	if !vault.Available() {
		return Job{}, vault.ErrUnavailable
	}
	info, err := installer.Measure(installer.Detect(folder))
	if err != nil {
		return Job{}, fmt.Errorf("no se puede leer %s: %w", folder, err)
	}
	if info.Kind == "" {
		return Job{}, fmt.Errorf("%s no es un instalador de Windows en carpetas", folder)
	}
	if !info.Bootable() {
		return Job{}, fmt.Errorf("%s no tiene imágenes de arranque (boot/etfsboot.com o efi/microsoft/boot/efisys.bin)", folder)
	}
	job := &Job{
		Kind:       KindMakeISO,
		Name:       info.Name,
		SourcePath: folder,
		FileName:   info.Name + ".iso",
		Category:   "windows",
		TotalBytes: info.Bytes,
	}
	return m.enqueue(job)
}

// isoReporter publica el avance de la creación de la ISO en el trabajo.
type isoReporter struct {
	m   *Manager
	job *Job
}

// Report anota los bytes ya escritos.
func (r *isoReporter) Report(done int64) {
	r.m.setProgress(r.job, done, 0, false)
}

// * makeISO crea la ISO en la carpeta de parciales y devuelve su SHA-256.
func (m *Manager) makeISO(ctx context.Context, job *Job) (string, error) {
	info, err := installer.Measure(installer.Detect(job.SourcePath))
	if err != nil || info.Kind == "" {
		return "", coded(CodeSource, fmt.Errorf("la carpeta del instalador ya no se puede leer: %s", job.SourcePath))
	}
	// * La ISO puede salir algo más grande que la carpeta (UDF + ISO 9660): se deja un margen del 2 %.
	if err := checkSpace(info.Bytes + info.Bytes/50); err != nil {
		return "", err
	}
	m.setTotal(job, info.Bytes)
	partial := partialPath(job)
	removePartial(job)
	if err := installer.MakeISO(ctx, info, partial, &isoReporter{m: m, job: job}); err != nil {
		os.Remove(partial)
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", coded(CodeWrite, err)
	}
	file, err := os.Open(partial)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	stat, err := file.Stat()
	if err == nil {
		m.setProgress(job, stat.Size(), 0, true)
		m.setTotal(job, stat.Size())
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

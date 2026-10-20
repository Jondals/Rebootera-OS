//go:build linux || darwin

// * Creación de un USB de Rebootera desde Linux y macOS. Los pasos comunes (validar, descargar Rebootera OS,
// * copiar el contenido del USB y las imágenes elegidas) están aquí; preparar el disco (particionar, instalar el
// * arranque de Ventoy y montarlo) es propio de cada sistema: ver create_linux.go y create_darwin.go.
// ! Preparar el disco BORRA todo su contenido. Solo se aceptan discos USB externos que no son del sistema, y la
// ! interfaz exige una confirmación explícita antes de llegar aquí.
package desktop

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"rebootera/core/internal/release"
)

// * Create empieza a crear un USB de Rebootera en segundo plano.
func Create(diskNumber int, isoPath string, kit fs.FS, desktopExe string, extras Extras, keepData bool) error {
	if err := begin(); err != nil {
		return err
	}
	// ! Las imágenes y carpetas de drivers también se validan antes de borrar nada.
	if err := extras.Validate(); err != nil {
		finish("", err)
		return err
	}
	if keepData {
		err := errors.New("añadir Rebootera a un USB sin borrarlo solo está disponible en Windows por ahora")
		finish("", err)
		return err
	}
	// ! Todo se comprueba ANTES de borrar el pendrive: si la ISO no sirve, el USB no debe quedar borrado y sin sistema.
	if isoPath != "" {
		if info, err := os.Stat(isoPath); err != nil || !info.Mode().IsRegular() || !strings.EqualFold(filepath.Ext(isoPath), ".iso") {
			err := errors.New("elige una imagen .iso de Rebootera OS que exista")
			finish("", err)
			return err
		}
	}
	if kit == nil {
		err := errors.New("falta el contenido del USB dentro de la aplicación")
		finish("", err)
		return err
	}
	disks, err := USBDisks()
	if err != nil {
		finish("", err)
		return err
	}
	var chosen *USBDisk
	for index := range disks {
		if disks[index].Number == diskNumber {
			chosen = &disks[index]
		}
	}
	if chosen == nil || chosen.Path == "" {
		err := fmt.Errorf("el disco %d no es un disco USB disponible", diskNumber)
		finish("", err)
		return err
	}
	go (&createJob{disk: *chosen, iso: isoPath, kit: kit, extras: extras}).run()
	return nil
}

// createJob guarda los datos de una creación.
type createJob struct {
	disk   USBDisk
	iso    string
	kit    fs.FS
	extras Extras
}

// * run ejecuta todos los pasos e informa del resultado.
func (j *createJob) run() {
	root, err := j.steps()
	finish(root, err)
}

// * steps realiza la creación paso a paso.
func (j *createJob) steps() (string, error) {
	// ! La descarga va antes de tocar el disco: si falla, el USB no se borra.
	if j.iso == "" {
		setPhase("download", 0)
		path, err := release.Fetch(release.CacheDir(), downloadProgress, control)
		if err != nil {
			return "", err
		}
		if err := downloadCancelled(); err != nil {
			return "", err
		}
		j.iso = path
	}
	root, err := prepareDisk(j)
	if err != nil {
		return "", err
	}
	setPhase("copy-kit", 0)
	if err := WriteKit(root, j.kit); err != nil {
		return root, err
	}
	setPhase("copy-system", 0)
	if err := InstallSystem(root, j.iso, systemProgress); err != nil {
		return root, err
	}
	// * Imágenes y drivers elegidos antes de crear el USB: salen ya dentro, sin mover nada a mano.
	if err := ApplyExtras(root, j.extras); err != nil {
		return root, err
	}
	syscall.Sync()
	return root, nil
}

// * PrepareVentoy descarga y verifica Ventoy sin tocar ningún disco (lo usa la opción -prepare-ventoy).
func PrepareVentoy() (string, error) {
	return ensureVentoyTree()
}

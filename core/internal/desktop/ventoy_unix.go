//go:build linux || darwin

// * Ventoy en Linux y macOS: se usa el paquete de Linux de Ventoy (Ventoy2Disk.sh y las imágenes de arranque).
package desktop

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ventoyTarball es el archivo de Ventoy para Linux.
const ventoyTarball = "ventoy-" + ventoyVersion + "-linux.tar.gz"

// * ensureVentoyTree descarga (verificado), extrae y devuelve la carpeta de Ventoy con Ventoy2Disk.sh.
func ensureVentoyTree() (string, error) {
	dir, err := cacheRoot()
	if err != nil {
		return "", err
	}
	target := filepath.Join(dir, "ventoy-"+ventoyVersion)
	if _, err := os.Stat(filepath.Join(target, "Ventoy2Disk.sh")); err == nil {
		return target, nil
	}
	archive, err := fetchVentoyArchive(ventoyTarball)
	if err != nil {
		return "", err
	}
	// * Se extrae a una carpeta temporal y se renombra al final: una extracción cortada no deja una carpeta a medias.
	staging, err := os.MkdirTemp(dir, "ventoy-extract-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)
	if err := untar(archive, staging); err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(staging, "ventoy-"+ventoyVersion, "Ventoy2Disk.sh")); err != nil {
		return "", errors.New("el paquete de Ventoy no tiene la estructura esperada")
	}
	os.RemoveAll(target)
	if err := os.Rename(filepath.Join(staging, "ventoy-"+ventoyVersion), target); err != nil {
		return "", err
	}
	return target, nil
}

// * untar extrae un .tar.gz sin permitir rutas que salgan de la carpeta destino ni enlaces: conserva los permisos de
// * ejecución (Ventoy2Disk.sh y sus herramientas los necesitan) pero nunca setuid, setgid ni escritura para otros.
func untar(archive, dest string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	zipped, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer zipped.Close()
	reader := tar.NewReader(zipped)
	root := filepath.Clean(dest) + string(filepath.Separator)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dest, filepath.FromSlash(header.Name))
		// ! Protección frente a rutas con ".." dentro del archivo.
		if target != filepath.Clean(dest) && !strings.HasPrefix(target, root) {
			return fmt.Errorf("ruta no válida en el paquete: %s", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(header.Mode) & 0o755
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode|0o600)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, reader); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
			if err := os.Chmod(target, mode|0o600); err != nil {
				return err
			}
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("el paquete trae un enlace (%s): no se acepta", header.Name)
		}
	}
}

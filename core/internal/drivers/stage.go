// * Copia de los paquetes de drivers que casan a la carpeta que ve el instalador de Windows, sin seguir enlaces
// * simbólicos ni salir de la carpeta de origen.
package drivers

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DirName es la carpeta que contiene los drivers dentro del archivo que se
// inyecta en el instalador (en WinPE queda en X:\rebootera-drivers).
const DirName = "rebootera-drivers"

// maxStageBytes limita lo que se copia al archivo de inyección: va a la RAM
// del instalador de Windows (WinPE).
const maxStageBytes = 400 << 20

// ErrNoArchiver indica que no hay herramienta 7z en el sistema.
var ErrNoArchiver = errors.New("falta la herramienta 7z para crear el archivo de drivers")

// * Stage copia las carpetas de los paquetes a dest/rebootera-drivers/N-nombre.
// * Solo copia archivos normales: nunca enlaces simbólicos ni nada fuera de
// * root. Devuelve las carpetas creadas, relativas a dest.
func Stage(root string, packages []*Package, dest string) ([]string, error) {
	var staged []string
	c := &copier{}
	for index, pkg := range packages {
		name := fmt.Sprintf("%02d-%s", index+1, safeName(pkg.INF, pkg.Dir))
		c.target = filepath.Join(dest, DirName, name)
		if err := os.MkdirAll(c.target, 0o755); err != nil {
			return nil, err
		}
		// * Un INF suelto en la raíz de drivers/ no arrastra el resto de la raíz.
		c.recursive = pkg.Dir != "."
		// * Cada paquete se copia desde la raíz de su origen (el USB o una partición del equipo).
		base := root
		if pkg.Root != "" {
			base = pkg.Root
		}
		c.source = filepath.Join(base, filepath.FromSlash(pkg.Dir))
		if err := filepath.WalkDir(c.source, c.visit); err != nil {
			return nil, err
		}
		staged = append(staged, filepath.ToSlash(filepath.Join(DirName, name)))
	}
	return staged, nil
}

// copier copia la carpeta de un paquete llevando la cuenta de los bytes.
type copier struct {
	source, target string
	recursive      bool
	total          int64
}

// * visit copia cada archivo normal que recorre WalkDir.
func (c *copier) visit(path string, entry fs.DirEntry, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}
	rel, _ := filepath.Rel(c.source, path)
	if entry.IsDir() {
		if rel != "." && !c.recursive {
			return filepath.SkipDir
		}
		return os.MkdirAll(filepath.Join(c.target, rel), 0o755)
	}
	if !entry.Type().IsRegular() {
		return nil
	}
	info, err := entry.Info()
	if err != nil {
		return err
	}
	if c.total += info.Size(); c.total > maxStageBytes {
		return fmt.Errorf("los drivers elegidos superan %d MB", maxStageBytes>>20)
	}
	return copyFile(path, filepath.Join(c.target, rel))
}

// * safeName deja un nombre de carpeta con caracteres seguros.
func safeName(candidates ...string) string {
	for _, candidate := range candidates {
		candidate = strings.TrimSuffix(filepath.Base(candidate), filepath.Ext(candidate))
		var out strings.Builder
		for _, c := range candidate {
			if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
				out.WriteRune(c)
			}
		}
		if out.Len() > 0 && candidate != "." {
			return out.String()
		}
	}
	return "driver"
}

// * copyFile copia un archivo.
func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// * archiver devuelve la herramienta 7z disponible.
func archiver() (string, error) {
	for _, name := range []string{"7z", "7zz", "7za"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", ErrNoArchiver
}

// * Archive empaqueta el contenido de dir (con la carpeta rebootera-drivers en
// * su raíz) en un archivo 7z, que Ventoy extrae en X:\ de WinPE.
func Archive(dir, output string) error {
	tool, err := archiver()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o750); err != nil {
		return err
	}
	os.Remove(output)
	cmd := exec.Command(tool, "a", "-t7z", "-mx=1", "-bd", "-y", output, DirName)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("7z falló: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// * Package installer: medios de instalación "en carpetas". Rufus (y otras herramientas) no copian
// * la ISO de Windows al USB: extraen su contenido (bootmgr, boot/, efi/, sources/...). Esas carpetas
// * no arrancan desde un menú de ISO, pero se pueden volver a empaquetar en una ISO arrancable
// * (BIOS y UEFI) que sí aparece en el menú de Rebootera. Aquí está cómo reconocerlas y cómo
// * crear la ISO; la creación depende del sistema (IMAPI en Windows, genisoimage en Linux).
// ! Las carpetas solo se leen: nunca se modifica el medio de origen.
package installer

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Tipos de instalador.
const KindWindows = "windows"

// Info describe un instalador en carpetas.
type Info struct {
	// Kind es el tipo ("windows") o "" si la carpeta no es un instalador.
	Kind string `json:"kind"`
	Root string `json:"root"`
	// Name es el nombre sugerido para la ISO (sin extensión).
	Name string `json:"name"`
	// Label es la etiqueta del volumen de la ISO (letras, números y guion bajo, hasta 32).
	Label string `json:"label"`
	// BIOSBoot y UEFIBoot son las imágenes de arranque (rutas relativas) o "" si no están.
	BIOSBoot string `json:"biosBoot"`
	UEFIBoot string `json:"uefiBoot"`
	// Bytes es el tamaño total y LargeFile indica un archivo de más de 4 GiB (exige UDF).
	Bytes     int64 `json:"bytes"`
	LargeFile bool  `json:"largeFile"`
}

// Bootable indica que la ISO arrancará al menos en un modo.
func (i Info) Bootable() bool { return i.BIOSBoot != "" || i.UEFIBoot != "" }

// biosImage y efiImages son los nombres de las imágenes de arranque de Windows.
const biosImage = "boot/etfsboot.com"

var efiImages = []string{"efi/microsoft/boot/efisys.bin", "efi/microsoft/boot/efisys_noprompt.bin"}

// installFiles son los archivos que deben estar en sources/ (uno de ellos) para que sea un instalador.
var installFiles = []string{"install.wim", "install.esd", "install.swm"}

// labelCleaner quita lo que no puede ir en la etiqueta de un volumen.
var labelCleaner = regexp.MustCompile(`[^A-Za-z0-9_]+`)

// * lookup busca una ruta ignorando mayúsculas y minúsculas (los sistemas de archivos de Windows
// * no distinguen y en Linux la carpeta puede venir de uno de ellos). Devuelve la ruta real.
func lookup(root, relative string) (string, bool) {
	current := root
	for _, part := range strings.Split(relative, "/") {
		entries, err := os.ReadDir(current)
		if err != nil {
			return "", false
		}
		found := false
		for _, entry := range entries {
			if strings.EqualFold(entry.Name(), part) {
				current = filepath.Join(current, entry.Name())
				found = true
				break
			}
		}
		if !found {
			return "", false
		}
	}
	return current, true
}

// * Detect mira si una carpeta es un instalador de Windows en carpetas. Es barata (unas pocas
// * consultas): sirve para recorrer discos. El tamaño se mide aparte con Measure.
func Detect(root string) Info {
	info := Info{Root: root}
	if _, ok := lookup(root, "bootmgr"); !ok {
		if _, ok := lookup(root, "bootmgr.efi"); !ok {
			return info
		}
	}
	if _, ok := lookup(root, "sources/boot.wim"); !ok {
		return info
	}
	hasInstall := false
	for _, name := range installFiles {
		if _, ok := lookup(root, "sources/"+name); ok {
			hasInstall = true
		}
	}
	if !hasInstall {
		return info
	}
	info.Kind = KindWindows
	// * Se guardan las rutas reales (con las mayúsculas del disco): la herramienta de creación las necesita exactas.
	if real, ok := lookup(root, biosImage); ok {
		info.BIOSBoot = relative(root, real)
	}
	for _, image := range efiImages {
		if real, ok := lookup(root, image); ok {
			info.UEFIBoot = relative(root, real)
			break
		}
	}
	info.Name = SafeName(filepath.Base(strings.TrimRight(root, `/\`)))
	info.Label = Label(info.Name)
	return info
}

// * relative devuelve la ruta de real respecto a root, con barras normales.
func relative(root, real string) string {
	rel, err := filepath.Rel(root, real)
	if err != nil {
		return real
	}
	return filepath.ToSlash(rel)
}

// * SafeName convierte el nombre de una carpeta en un nombre de archivo válido para la ISO.
func SafeName(name string) string {
	name = strings.TrimSpace(strings.Trim(name, `:\/.`))
	name = regexp.MustCompile(`[^A-Za-z0-9._ -]+`).ReplaceAllString(name, "")
	name = strings.TrimSpace(name)
	if name == "" {
		return "Windows-installer"
	}
	return name
}

// * Label devuelve una etiqueta de volumen válida (ISO 9660: mayúsculas, números y guion bajo, hasta 32).
func Label(name string) string {
	label := strings.ToUpper(labelCleaner.ReplaceAllString(name, "_"))
	label = strings.Trim(label, "_")
	if label == "" {
		label = "WINDOWS_INSTALL"
	}
	if len(label) > 32 {
		label = label[:32]
	}
	return label
}

// largeFile es el límite de ISO 9660 para un archivo: 4 GiB menos un byte.
const largeFile = 1<<32 - 1

// sizer acumula el tamaño de los archivos que recorre filepath.WalkDir.
type sizer struct {
	total int64
	large bool
}

// visit suma un archivo normal; los errores de lectura y lo que no es archivo se ignoran.
func (s *sizer) visit(path string, entry os.DirEntry, err error) error {
	if err != nil || entry.IsDir() || !entry.Type().IsRegular() {
		return nil
	}
	if file, infoErr := entry.Info(); infoErr == nil {
		s.total += file.Size()
		if file.Size() > largeFile {
			s.large = true
		}
	}
	return nil
}

// * Measure suma el tamaño de los archivos de la carpeta e indica si alguno pasa de 4 GiB.
func Measure(info Info) (Info, error) {
	var s sizer
	err := filepath.WalkDir(info.Root, s.visit)
	info.Bytes, info.LargeFile = s.total, s.large
	return info, err
}

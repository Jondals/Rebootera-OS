// * Package drivers: análisis de paquetes de drivers de Windows (archivos INF)
// * y casado con los dispositivos del equipo por su Hardware ID. Prepara los
// * drivers críticos (disco y red) para inyectarlos en el instalador de Windows.
// ! Nunca se ejecuta nada de un paquete: solo se leen sus INF y se copian sus
// ! archivos. Los instaladores .exe no se usan.
package drivers

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"strings"
	"unicode/utf16"
)

// Model es un dispositivo que un INF declara que soporta.
type Model struct {
	Description string   `json:"description"`
	HardwareIDs []string `json:"hardwareIds"`
}

// Package es un INF analizado, con la carpeta que lo acompaña.
type Package struct {
	// Dir es la carpeta del INF, relativa a la raíz de drivers.
	Dir string `json:"dir"`
	// INF es la ruta del archivo, relativa a la raíz de drivers.
	INF       string `json:"inf"`
	Provider  string `json:"provider"`
	Class     string `json:"class"`
	Version   string `json:"version"`
	Date      string `json:"date"`
	Amd64     bool   `json:"amd64"`
	Catalog   string `json:"catalog"`
	Signature string `json:"signature"`
	// SourceID es el origen del paquete (ver Source); Root, la carpeta de ese origen que contiene Dir.
	SourceID string  `json:"sourceId"`
	Root     string  `json:"-"`
	Models   []Model `json:"-"`
	// Rank es la prioridad del origen: cuanto menor, antes se usa (el USB, luego las carpetas de fábrica y, al
	// final, el almacén de un Windows instalado, que trae de todo y casi nunca es lo que se busca).
	Rank int `json:"-"`
	// ids es el índice de Hardware ID (en mayúsculas) del paquete.
	ids map[string]string
}

// Estados de la firma del catálogo (.cat) del paquete. La cadena de
// certificados no se verifica: solo se comprueba que haya un catálogo firmado.
const (
	SignaturePresent = "present"
	SignatureMissing = "missing"
	SignatureInvalid = "invalid"
)

// section es una sección de un INF: pares clave = valor en orden.
type section struct {
	keys   []string
	values []string
}

// infFile es un INF leído y troceado en secciones.
type infFile struct {
	sections map[string]*section
	strings  map[string]string
}

// ErrNotINF indica un archivo que no es un INF de Windows.
var ErrNotINF = errors.New("el archivo no parece un INF de Windows")

// * decodeText convierte el contenido de un INF (UTF-16 con o sin BOM, UTF-8
// * o ANSI) en texto.
func decodeText(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		return decodeUTF16(data[2:], binary.LittleEndian)
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		return decodeUTF16(data[2:], binary.BigEndian)
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		return string(data[3:])
	}
	// * Sin BOM: muchos NUL seguidos de ASCII indican UTF-16 de 8 bits por carácter.
	if len(data) > 4 && data[1] == 0 && data[3] == 0 {
		return decodeUTF16(data, binary.LittleEndian)
	}
	return string(data)
}

// * decodeUTF16 decodifica UTF-16 con el orden de bytes indicado.
func decodeUTF16(data []byte, order binary.ByteOrder) string {
	units := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		units = append(units, order.Uint16(data[i:]))
	}
	return string(utf16.Decode(units))
}

// * stripComment quita el comentario (;) de una línea respetando las comillas.
func stripComment(line string) string {
	inQuotes := false
	for i, c := range line {
		switch c {
		case '"':
			inQuotes = !inQuotes
		case ';':
			if !inQuotes {
				return line[:i]
			}
		}
	}
	return line
}

// * parseINF trocea el texto de un INF en secciones y recoge sus cadenas.
func parseINF(text string) (*infFile, error) {
	file := &infFile{sections: map[string]*section{}, strings: map[string]string{}}
	var current *section
	var pending string
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(stripComment(scanner.Text()))
		// * Una línea que acaba en \ continúa en la siguiente.
		if strings.HasSuffix(line, "\\") {
			pending += strings.TrimSuffix(line, "\\")
			continue
		}
		line = pending + line
		pending = ""
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			current = file.sections[name]
			if current == nil {
				current = &section{}
				file.sections[name] = current
			}
			continue
		}
		if current == nil {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			key, value = "", line
		}
		current.keys = append(current.keys, strings.TrimSpace(key))
		current.values = append(current.values, strings.TrimSpace(value))
	}
	if file.sections["version"] == nil {
		return nil, ErrNotINF
	}
	for name, sec := range file.sections {
		if name != "strings" && !strings.HasPrefix(name, "strings.") {
			continue
		}
		for i, key := range sec.keys {
			// * [Strings] manda sobre las cadenas localizadas.
			if _, exists := file.strings[strings.ToLower(key)]; exists && name != "strings" {
				continue
			}
			file.strings[strings.ToLower(key)] = unquote(sec.values[i])
		}
	}
	return file, nil
}

// * unquote quita las comillas que rodean un valor.
func unquote(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		return value[1 : len(value)-1]
	}
	return value
}

// * expand sustituye los %token% de un valor por las cadenas del INF.
func (f *infFile) expand(value string) string {
	if !strings.Contains(value, "%") {
		return unquote(value)
	}
	var out strings.Builder
	rest := value
	for {
		start := strings.Index(rest, "%")
		if start < 0 {
			out.WriteString(rest)
			break
		}
		out.WriteString(rest[:start])
		end := strings.Index(rest[start+1:], "%")
		if end < 0 {
			out.WriteString(rest[start:])
			break
		}
		token := rest[start+1 : start+1+end]
		switch {
		case token == "":
			out.WriteString("%")
		case f.strings[strings.ToLower(token)] != "":
			out.WriteString(f.strings[strings.ToLower(token)])
		default:
			out.WriteString("%" + token + "%")
		}
		rest = rest[start+end+2:]
	}
	return unquote(out.String())
}

// * lookup devuelve el valor de una clave de una sección.
func (f *infFile) lookup(sectionName, key string) string {
	sec := f.sections[strings.ToLower(sectionName)]
	if sec == nil {
		return ""
	}
	for i, k := range sec.keys {
		if strings.EqualFold(k, key) {
			return sec.values[i]
		}
	}
	return ""
}

// * isAmd64Target indica si la decoración de un sistema de destino (NTamd64,
// * NTamd64.10.0...) es para x64.
func isAmd64Target(target string) bool {
	return strings.HasPrefix(strings.ToLower(target), "ntamd64")
}

// * modelSections devuelve los nombres de las secciones de modelos que valen
// * para x64, según [Manufacturer].
func (f *infFile) modelSections() []string {
	manufacturer := f.sections["manufacturer"]
	if manufacturer == nil {
		return nil
	}
	var names []string
	for _, value := range manufacturer.values {
		parts := splitList(value)
		if len(parts) == 0 {
			continue
		}
		base := parts[0]
		if len(parts) == 1 {
			// * Sin sistemas de destino: sección sin decorar y la genérica NT.
			names = append(names, base, base+".NT")
			continue
		}
		for _, target := range parts[1:] {
			if isAmd64Target(target) {
				names = append(names, base+"."+target)
			}
		}
	}
	return names
}

// * splitList separa una lista separada por comas, quitando espacios.
func splitList(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// * ParseFile lee un INF del disco. dir y inf son sus rutas relativas a la
// * raíz de drivers.
func ParseFile(path, dir, inf string) (*Package, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	file, err := parseINF(decodeText(data))
	if err != nil {
		return nil, err
	}
	pkg := &Package{Dir: dir, INF: inf, ids: map[string]string{}}
	pkg.Class = file.expand(file.lookup("version", "Class"))
	pkg.Provider = file.expand(file.lookup("version", "Provider"))
	date, version, _ := strings.Cut(file.lookup("version", "DriverVer"), ",")
	pkg.Date, pkg.Version = strings.TrimSpace(date), strings.TrimSpace(version)
	pkg.Catalog = file.expand(firstNonEmpty(file.lookup("version", "CatalogFile.NTamd64"), file.lookup("version", "CatalogFile")))

	for _, name := range file.modelSections() {
		sec := file.sections[strings.ToLower(name)]
		if sec == nil {
			continue
		}
		pkg.Amd64 = true
		for i, key := range sec.keys {
			// * "descripción = sección de instalación, HWID, HWID de compatibilidad..."
			parts := splitList(sec.values[i])
			if len(parts) < 2 {
				continue
			}
			model := Model{Description: file.expand(key)}
			for _, id := range parts[1:] {
				id = strings.ToUpper(id)
				model.HardwareIDs = append(model.HardwareIDs, id)
				if _, exists := pkg.ids[id]; !exists {
					pkg.ids[id] = model.Description
				}
			}
			pkg.Models = append(pkg.Models, model)
		}
	}
	return pkg, nil
}

// * firstNonEmpty devuelve el primer texto no vacío.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// * Describe devuelve la descripción con la que el paquete declara un
// * Hardware ID, o "" si no lo declara.
func (p *Package) Describe(hardwareID string) string {
	return p.ids[strings.ToUpper(hardwareID)]
}

// * Supports indica si el paquete declara un Hardware ID.
func (p *Package) Supports(hardwareID string) bool {
	_, ok := p.ids[strings.ToUpper(hardwareID)]
	return ok
}

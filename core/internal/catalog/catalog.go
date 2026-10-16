// * Package catalog: catálogo de sistemas operativos con sus fuentes oficiales.
// * El catálogo no guarda hashes ni nombres de archivo fijos: cada entrada dice
// * dónde está el índice oficial de sumas de control y cómo leerlo, y Resolve
// * obtiene en ese momento el archivo más reciente y su SHA-256 por HTTPS.
// ! Una entrada "manual" (Windows) no se descarga: el fabricante solo ofrece
// ! enlaces temporales desde su web y Rebootera no los inventa ni los rasca.
// TODO: verificar las firmas GPG de los índices (Debian, Ubuntu, Fedora, Arch, Alpine).
package catalog

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

//go:embed catalog.json
var catalogJSON []byte

// Entry es un sistema del catálogo.
type Entry struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Vendor     string            `json:"vendor"`
	Version    string            `json:"version"`
	Arch       string            `json:"arch"`
	Category   string            `json:"category"`
	Homepage   string            `json:"homepage"`
	Resolver   Resolver          `json:"resolver"`
	SecureBoot string            `json:"secureBoot"`
	Signature  string            `json:"signature"`
	Notes      map[string]string `json:"notes"`
	Boot       BootCompat        `json:"boot"`
}

// BootCompat es la compatibilidad de arranque desde el menú del USB de
// Rebootera, comprobada en máquina virtual (scripts/compat-matrix.sh).
type BootCompat struct {
	// Status es "verified", "incompatible" o "untested".
	Status string            `json:"status"`
	Tested string            `json:"tested"`
	Notes  map[string]string `json:"notes"`
}

// Resolver indica cómo encontrar el archivo y su hash en la fuente oficial.
type Resolver struct {
	Type    string `json:"type"`
	Index   string `json:"index,omitempty"`
	Pattern string `json:"pattern,omitempty"`
}

// Resolved es el archivo concreto que se descargará.
type Resolved struct {
	URL       string `json:"url"`
	FileName  string `json:"fileName"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"sizeBytes"`
}

// ErrManual indica una entrada que no se puede descargar automáticamente.
var ErrManual = errors.New("esta imagen se descarga manualmente desde la web del fabricante")

// Client es el cliente HTTP usado para leer índices; se cambia en las pruebas.
var Client = &http.Client{Timeout: 30 * time.Second}

// UserAgent identifica a Rebootera ante los servidores.
var UserAgent = "RebooteraOS"

// * Entries devuelve el catálogo embebido.
func Entries() ([]Entry, error) {
	var entries []Entry
	if err := json.Unmarshal(catalogJSON, &entries); err != nil {
		return nil, fmt.Errorf("catálogo dañado: %w", err)
	}
	return entries, nil
}

// * Find devuelve la entrada con el id indicado.
func Find(id string) (Entry, error) {
	entries, err := Entries()
	if err != nil {
		return Entry{}, err
	}
	for _, entry := range entries {
		if entry.ID == id {
			return entry, nil
		}
	}
	return Entry{}, fmt.Errorf("no existe la entrada %q en el catálogo", id)
}

// * Resolve averigua el archivo actual y su SHA-256 desde el índice oficial.
func Resolve(entry Entry) (Resolved, error) {
	switch entry.Resolver.Type {
	case "sums":
		return resolveSums(entry.Resolver)
	case "fedora":
		return resolveFedora(entry.Resolver)
	case "alpine":
		return resolveAlpine(entry.Resolver)
	case "manual":
		return Resolved{}, ErrManual
	}
	return Resolved{}, fmt.Errorf("tipo de resolución desconocido: %q", entry.Resolver.Type)
}

// * fetchAttempts es cuántas veces se intenta leer un índice. Algunos
// * proyectos (como Fedora) redirigen cada petición a un mirror distinto y
// * alguno puede fallar de forma puntual.
const fetchAttempts = 3

// * fetchText descarga un índice de texto (como mucho 4 MB) por HTTPS,
// * reintentando ante errores de red o respuestas 5xx.
func fetchText(rawURL string) (string, error) {
	if !strings.HasPrefix(rawURL, "https://") {
		return "", fmt.Errorf("solo se admiten fuentes HTTPS: %s", rawURL)
	}
	var lastErr error
	for attempt := 1; attempt <= fetchAttempts; attempt++ {
		text, retry, err := fetchOnce(rawURL)
		if err == nil {
			return text, nil
		}
		lastErr = err
		if !retry {
			break
		}
		time.Sleep(time.Duration(attempt) * 700 * time.Millisecond)
	}
	return "", lastErr
}

// * fetchOnce hace un intento de lectura e indica si merece la pena repetirlo.
func fetchOnce(rawURL string) (string, bool, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := Client.Do(req)
	if err != nil {
		return "", true, fmt.Errorf("no se pudo leer %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return "", true, fmt.Errorf("%s respondió %d", rawURL, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("%s respondió %d", rawURL, resp.StatusCode)
	}
	if resp.Request.URL.Scheme != "https" {
		return "", false, fmt.Errorf("%s redirigió a una conexión sin cifrar", rawURL)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", true, err
	}
	return string(data), false, nil
}

// * resolveSums lee un índice estilo GNU ("hash  archivo" o "hash *archivo")
// * y elige el archivo más reciente que encaja con el patrón.
func resolveSums(r Resolver) (Resolved, error) {
	text, err := fetchText(r.Index)
	if err != nil {
		return Resolved{}, err
	}
	pattern, err := regexp.Compile(r.Pattern)
	if err != nil {
		return Resolved{}, err
	}
	hashes := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 || !isSHA256(fields[0]) {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if pattern.MatchString(name) {
			hashes[name] = strings.ToLower(fields[0])
		}
	}
	name, err := newest(hashes)
	if err != nil {
		return Resolved{}, fmt.Errorf("%s: %w", r.Index, err)
	}
	return Resolved{URL: sibling(r.Index, name), FileName: name, SHA256: hashes[name]}, nil
}

// fedoraSum es una línea "SHA256 (archivo) = hash" de los CHECKSUM de Fedora.
var fedoraSum = regexp.MustCompile(`^SHA256 \(([^)]+)\) = ([0-9a-fA-F]{64})$`)

// fedoraChecksumLink busca el enlace al archivo CHECKSUM en el listado.
var fedoraChecksumLink = regexp.MustCompile(`href="([^"]+-CHECKSUM)"`)

// * resolveFedora busca el CHECKSUM firmado en el listado del directorio
// * oficial y lee de él el archivo y su hash.
func resolveFedora(r Resolver) (Resolved, error) {
	listing, err := fetchText(r.Index)
	if err != nil {
		return Resolved{}, err
	}
	match := fedoraChecksumLink.FindStringSubmatch(listing)
	if match == nil {
		return Resolved{}, fmt.Errorf("%s: no se encontró el archivo CHECKSUM", r.Index)
	}
	text, err := fetchText(sibling(r.Index, match[1]))
	if err != nil {
		return Resolved{}, err
	}
	pattern, err := regexp.Compile(r.Pattern)
	if err != nil {
		return Resolved{}, err
	}
	hashes := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		m := fedoraSum.FindStringSubmatch(strings.TrimSpace(line))
		if m != nil && pattern.MatchString(m[1]) {
			hashes[m[1]] = strings.ToLower(m[2])
		}
	}
	name, err := newest(hashes)
	if err != nil {
		return Resolved{}, fmt.Errorf("%s: %w", r.Index, err)
	}
	return Resolved{URL: sibling(r.Index, name), FileName: name, SHA256: hashes[name]}, nil
}

// * resolveAlpine lee latest-releases.yaml y busca la variante (flavor) indicada.
func resolveAlpine(r Resolver) (Resolved, error) {
	text, err := fetchText(r.Index)
	if err != nil {
		return Resolved{}, err
	}
	var block map[string]string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "-" {
			if block["flavor"] == r.Pattern {
				break
			}
			block = map[string]string{}
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if ok && block != nil {
			block[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	if block == nil || block["flavor"] != r.Pattern || block["iso"] == "" || !isSHA256(block["sha256"]) {
		return Resolved{}, fmt.Errorf("%s: no se encontró la variante %s", r.Index, r.Pattern)
	}
	size, _ := strconv.ParseInt(block["size"], 10, 64)
	return Resolved{URL: sibling(r.Index, block["iso"]), FileName: block["iso"], SHA256: strings.ToLower(block["sha256"]), SizeBytes: size}, nil
}

// * sibling devuelve la URL de un archivo en la misma carpeta que indexURL.
func sibling(indexURL, name string) string {
	u, err := url.Parse(indexURL)
	if err != nil {
		return ""
	}
	if !strings.HasSuffix(u.Path, "/") {
		u.Path = path.Dir(u.Path) + "/"
	}
	u.Path += name
	return u.String()
}

// * newest elige el nombre de archivo con la versión más alta.
func newest(hashes map[string]string) (string, error) {
	if len(hashes) == 0 {
		return "", errors.New("ningún archivo coincide con el patrón")
	}
	names := make([]string, 0, len(hashes))
	for name := range hashes {
		names = append(names, name)
	}
	slices.SortFunc(names, compareVersions)
	return names[len(names)-1], nil
}

// numberRun separa un nombre en tramos de dígitos y de texto.
var numberRun = regexp.MustCompile(`\d+|\D+`)

// * compareVersions compara dos nombres tramo a tramo, numéricamente cuando
// * ambos tramos son números. Un nombre que es prefijo del otro va antes.
func compareVersions(a, b string) int {
	pa := numberRun.FindAllString(stripISO(a), -1)
	pb := numberRun.FindAllString(stripISO(b), -1)
	for i := 0; i < len(pa) && i < len(pb); i++ {
		na, errA := strconv.Atoi(pa[i])
		nb, errB := strconv.Atoi(pb[i])
		if errA == nil && errB == nil {
			if na != nb {
				return na - nb
			}
			continue
		}
		// * Un punto seguido de más versión ("26.04" frente a "26.04.1")
		// * indica una versión posterior a un final de nombre.
		if pa[i] != pb[i] {
			if strings.HasPrefix(pa[i], ".") && !strings.HasPrefix(pb[i], ".") {
				return 1
			}
			if strings.HasPrefix(pb[i], ".") && !strings.HasPrefix(pa[i], ".") {
				return -1
			}
			return strings.Compare(pa[i], pb[i])
		}
	}
	return len(pa) - len(pb)
}

// * stripISO quita la extensión para comparar solo la parte de versión.
func stripISO(name string) string {
	return strings.TrimSuffix(name, ".iso")
}

// * isSHA256 indica si un texto es un hash SHA-256 en hexadecimal.
func isSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range strings.ToLower(s) {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

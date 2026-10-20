// * Package catalog: catálogo de sistemas operativos con sus fuentes oficiales.
// * El catálogo no guarda hashes ni nombres de archivo fijos: cada entrada dice
// * dónde está el índice oficial de sumas de control y cómo leerlo, y Resolve
// * obtiene en ese momento el archivo más reciente y su SHA-256 por HTTPS.
// ! Una entrada "manual" (Windows) no se descarga: el fabricante solo ofrece
// ! enlaces temporales desde su web y Rebootera no los inventa ni los rasca.
package catalog

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
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

//go:embed families.json
var familiesJSON []byte

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
	// Family agrupa las versiones de un mismo sistema en una tarjeta de la interfaz.
	Family string `json:"family"`
	// Group es la pestaña donde sale: windows, linux, servers o rescue.
	Group string `json:"group"`
	// Edition es el nombre corto de esta versión dentro de su familia ("Desktop 24.04 LTS").
	Edition string `json:"edition"`
	// Tags son etiquetas de la interfaz: recommended, lts, rolling, light, live, server, legacy.
	Tags []string `json:"tags"`
}

// Family es un sistema con sus versiones: la tarjeta del Centro de descargas.
type Family struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Vendor   string            `json:"vendor"`
	Group    string            `json:"group"`
	Icon     string            `json:"icon"`
	Homepage string            `json:"homepage"`
	Summary  map[string]string `json:"summary"`
}

// * Families devuelve las familias del catálogo.
func Families() ([]Family, error) {
	var families []Family
	if err := json.Unmarshal(familiesJSON, &families); err != nil {
		return nil, fmt.Errorf("familias del catálogo dañadas: %w", err)
	}
	return families, nil
}

// BootCompat es la compatibilidad de arranque desde el menú del USB de
// Rebootera, comprobada en máquina virtual (scripts/test/compat-matrix.sh).
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
	// Link, si está, es la URL de la imagen cuando no cuelga del mismo directorio que el índice
	// (por ejemplo, el enlace "latest" de NixOS, que redirige al archivo con nombre y versión).
	Link string `json:"link,omitempty"`
	// SumSuffix (resolvedor "latest-sum") es lo que se añade a la URL final de la imagen para encontrar su suma
	// SHA-256 (por ejemplo ".sha256").
	SumSuffix string `json:"sumSuffix,omitempty"`
	// Signature describe cómo está firmado el índice (nil = sin firma).
	Signature *SigSpec `json:"signature,omitempty"`
	// ISOSignature describe la firma de la propia imagen, para los proyectos que
	// firman cada ISO y no su lista de sumas (Arch Linux, Alpine).
	ISOSignature *SigSpec `json:"isoSignature,omitempty"`
}

// Resolved es el archivo concreto que se descargará.
type Resolved struct {
	URL      string `json:"url"`
	FileName string `json:"fileName"`
	SHA256   string `json:"sha256"`
	// SHA1 es la suma que publica el Internet Archive (no publica SHA-256): comprueba que la copia
	// llegó entera, pero no la autenticidad de la imagen. Vacía si hay SHA-256 o ninguna.
	SHA1      string `json:"sha1,omitempty"`
	SizeBytes int64  `json:"sizeBytes"`
	// SignedBy es la clave oficial que firmó el índice, comprobada; vacío si no está firmado.
	SignedBy string `json:"signedBy,omitempty"`
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
	var resolved Resolved
	var err error
	switch entry.Resolver.Type {
	case "sums":
		resolved, err = resolveSums(entry.Resolver)
	case "bsd":
		resolved, err = resolveBSD(entry.Resolver)
	case "fedora":
		resolved, err = resolveFedora(entry.Resolver)
	case "alpine":
		resolved, err = resolveAlpine(entry.Resolver)
	case "github":
		resolved, err = resolveGitHub(entry.Resolver)
	case "sourceforge":
		resolved, err = resolveSourceForge(entry.Resolver)
	case "direct":
		resolved, err = resolveDirect(entry.Resolver)
	case "archive":
		resolved, err = resolveArchive(entry.Resolver)
	case "manifest":
		resolved, err = resolveManifest(entry.Resolver)
	case "latest-sum":
		resolved, err = resolveLatestSum(entry.Resolver)
	case "manual":
		return Resolved{}, ErrManual
	default:
		return Resolved{}, fmt.Errorf("tipo de resolución desconocido: %q", entry.Resolver.Type)
	}
	if err != nil {
		return Resolved{}, err
	}
	// * Casi ningún índice dice el tamaño: se pregunta al servidor (sin descargar nada) para enseñarlo.
	if resolved.SizeBytes == 0 {
		resolved.SizeBytes = headSize(resolved.URL)
	}
	return resolved, nil
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
	text := string(data)
	// * Si la petición acabó en un espejo distinto (download.fedoraproject.org redirige), sus enlaces con
	// * ruta absoluta ("/fedora/...") cuelgan de ese espejo, no del servidor al que se preguntó.
	if final := resp.Request.URL; final.Host != req.URL.Host {
		text = strings.ReplaceAll(text, `href="/`, `href="`+final.Scheme+"://"+final.Host+"/")
	}
	return text, false, nil
}

// * resolveSums lee un índice estilo GNU ("hash  archivo" o "hash *archivo")
// * y elige el archivo más reciente que encaja con el patrón.
func resolveSums(r Resolver) (Resolved, error) {
	text, signedBy, err := fetchIndex(r)
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
	link := sibling(r.Index, name)
	if r.Link != "" {
		link = r.Link
	}
	return Resolved{URL: link, FileName: name, SHA256: hashes[name], SignedBy: signedBy}, nil
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
	// * El CHECKSUM de Fedora va firmado en claro; se comprueba y solo se
	// * lee lo que está dentro de la firma.
	text, signedBy, err := fetchIndex(Resolver{Index: sibling(r.Index, match[1]), Signature: r.Signature})
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
	return Resolved{URL: sibling(r.Index, name), FileName: name, SHA256: hashes[name], SignedBy: signedBy}, nil
}

// * resolveBSD lee un archivo de sumas con líneas "SHA256 (archivo) = hash" (Rocky, AlmaLinux, FreeBSD,
// * OpenBSD) que está directamente en la URL del índice, con su firma si la hay.
func resolveBSD(r Resolver) (Resolved, error) {
	text, signedBy, err := fetchIndex(r)
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
	return Resolved{URL: sibling(r.Index, name), FileName: name, SHA256: hashes[name], SignedBy: signedBy}, nil
}

// githubRelease es lo que se lee de la API de releases de GitHub.
type githubRelease struct {
	Assets []struct {
		Name   string `json:"name"`
		Size   int64  `json:"size"`
		Digest string `json:"digest"`
		URL    string `json:"browser_download_url"`
	} `json:"assets"`
}

// * resolveGitHub elige, entre los archivos de la última release de un proyecto en GitHub, el que encaja
// * con el patrón; su SHA-256 es el "digest" que GitHub calcula al subirlo.
func resolveGitHub(r Resolver) (Resolved, error) {
	text, err := fetchText(r.Index)
	if err != nil {
		return Resolved{}, err
	}
	var release githubRelease
	if err := json.Unmarshal([]byte(text), &release); err != nil {
		return Resolved{}, fmt.Errorf("respuesta de GitHub no válida: %w", err)
	}
	pattern, err := regexp.Compile(r.Pattern)
	if err != nil {
		return Resolved{}, err
	}
	hashes := map[string]string{}
	found := map[string]Resolved{}
	for _, asset := range release.Assets {
		sum, ok := strings.CutPrefix(asset.Digest, "sha256:")
		if !pattern.MatchString(asset.Name) || !ok || !isSHA256(sum) || !strings.HasPrefix(asset.URL, "https://github.com/") {
			continue
		}
		hashes[asset.Name] = strings.ToLower(sum)
		found[asset.Name] = Resolved{URL: asset.URL, FileName: asset.Name, SHA256: strings.ToLower(sum), SizeBytes: asset.Size}
	}
	name, err := newest(hashes)
	if err != nil {
		return Resolved{}, fmt.Errorf("%s: %w", r.Index, err)
	}
	return found[name], nil
}

// sourceForgeRelease es lo que devuelve best_release.json de un proyecto de SourceForge.
type sourceForgeRelease struct {
	Release struct {
		Filename string `json:"filename"`
		Bytes    int64  `json:"bytes"`
	} `json:"release"`
}

// * resolveSourceForge averigua la última versión de un proyecto de SourceForge (Index es el nombre del
// * proyecto) y lee su SHA-256 del archivo .sha256 que publica junto a la imagen.
func resolveSourceForge(r Resolver) (Resolved, error) {
	base := "https://sourceforge.net/projects/" + url.PathEscape(r.Index)
	text, err := fetchText(base + "/best_release.json")
	if err != nil {
		return Resolved{}, err
	}
	var best sourceForgeRelease
	if err := json.Unmarshal([]byte(text), &best); err != nil {
		return Resolved{}, fmt.Errorf("respuesta de SourceForge no válida: %w", err)
	}
	pattern, err := regexp.Compile(r.Pattern)
	if err != nil {
		return Resolved{}, err
	}
	file := best.Release.Filename
	if file == "" || !pattern.MatchString(path.Base(file)) {
		return Resolved{}, fmt.Errorf("%s: la última versión (%q) no es la imagen buscada", r.Index, file)
	}
	sumText, err := fetchText(base + "/files" + file + ".sha256/download")
	if err != nil {
		return Resolved{}, fmt.Errorf("no se pudo leer la suma de %s: %w", path.Base(file), err)
	}
	fields := strings.Fields(sumText)
	if len(fields) == 0 || !isSHA256(fields[0]) {
		return Resolved{}, fmt.Errorf("%s.sha256 no contiene una suma SHA-256", path.Base(file))
	}
	return Resolved{URL: base + "/files" + file + "/download", FileName: path.Base(file), SHA256: strings.ToLower(fields[0]), SizeBytes: best.Release.Bytes}, nil
}

// archiveFile es un archivo de un elemento del Internet Archive.
type archiveFile struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Size   string `json:"size"`
	SHA1   string `json:"sha1"`
}

// ArchiveBase es la dirección del Internet Archive; se cambia en las pruebas.
var ArchiveBase = "https://archive.org"

// * archiveFiles lee la lista de archivos de un elemento del Internet Archive.
func archiveFiles(id string) ([]archiveFile, error) {
	text, err := fetchText(ArchiveBase + "/metadata/" + url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	var item struct {
		Files []archiveFile `json:"files"`
	}
	if err := json.Unmarshal([]byte(text), &item); err != nil {
		return nil, fmt.Errorf("respuesta del Internet Archive no válida: %w", err)
	}
	return item.Files, nil
}

// * archiveCandidate construye la dirección de descarga de un archivo de un elemento del Archive.
func archiveCandidate(id string, file archiveFile) Candidate {
	size, _ := strconv.ParseInt(file.Size, 10, 64)
	// * El nombre puede llevar subcarpetas: cada tramo se escapa por separado.
	parts := strings.Split(file.Name, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return Candidate{
		Name:      path.Base(file.Name),
		URL:       ArchiveBase + "/download/" + url.PathEscape(id) + "/" + strings.Join(parts, "/"),
		SizeBytes: size,
		SHA1:      strings.ToLower(file.SHA1),
	}
}

// * resolveArchive elige un archivo de un elemento del Internet Archive (Index es su identificador y
// * Pattern el nombre del archivo). El Archive publica SHA-1, no SHA-256: se descarga con esa suma.
func resolveArchive(r Resolver) (Resolved, error) {
	files, err := archiveFiles(r.Index)
	if err != nil {
		return Resolved{}, err
	}
	pattern, err := regexp.Compile(r.Pattern)
	if err != nil {
		return Resolved{}, err
	}
	name := ""
	found := map[string]Candidate{}
	for _, file := range files {
		if file.Source != "original" || !pattern.MatchString(file.Name) || len(file.SHA1) != 40 {
			continue
		}
		found[file.Name] = archiveCandidate(r.Index, file)
		if name == "" || compareVersions(file.Name, name) > 0 {
			name = file.Name
		}
	}
	if name == "" {
		return Resolved{}, fmt.Errorf("%s: no hay ningún archivo %q con suma en el Internet Archive", r.Index, r.Pattern)
	}
	pick := found[name]
	return Resolved{URL: pick.URL, FileName: pick.Name, SHA1: pick.SHA1, SizeBytes: pick.SizeBytes}, nil
}

// * resolveLatestSum resuelve imágenes cuyo enlace estable redirige a la versión más reciente (los enlaces "latest"
// * de Raspberry Pi OS, la imagen fija de DietPi) y cuya suma SHA-256 se publica junto a la imagen, con otro final
// * (SumSuffix). El archivo y la suma se leen por HTTPS en el momento; si el nombre de la suma no coincide con el
// * de la imagen, se rechaza.
func resolveLatestSum(r Resolver) (Resolved, error) {
	if r.SumSuffix == "" {
		return Resolved{}, errors.New("falta sumSuffix en el resolvedor latest-sum")
	}
	req, err := http.NewRequest("HEAD", r.Index, nil)
	if err != nil {
		return Resolved{}, err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := Client.Do(req)
	if err != nil {
		return Resolved{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Resolved{}, fmt.Errorf("%s respondió %d", r.Index, resp.StatusCode)
	}
	final := resp.Request.URL
	if final.Scheme != "https" {
		return Resolved{}, fmt.Errorf("%s redirigió a una conexión sin cifrar", r.Index)
	}
	name := path.Base(final.Path)
	text, err := fetchText(final.String() + r.SumSuffix)
	if err != nil {
		return Resolved{}, err
	}
	fields := strings.Fields(text)
	if len(fields) == 0 || !isSHA256(fields[0]) {
		return Resolved{}, fmt.Errorf("%s: la suma publicada no es un SHA-256", final.String()+r.SumSuffix)
	}
	if len(fields) > 1 && strings.TrimPrefix(fields[1], "*") != name {
		return Resolved{}, fmt.Errorf("%s: la suma es de otro archivo (%s)", final.String()+r.SumSuffix, fields[1])
	}
	return Resolved{URL: final.String(), FileName: name, SHA256: strings.ToLower(fields[0]), SizeBytes: max(resp.ContentLength, 0)}, nil
}

// * resolveDirect sigue el enlace oficial del fabricante (por ejemplo, las ediciones de evaluación de
// * Microsoft) hasta el archivo y lee su nombre y su tamaño. No hay suma publicada que leer: la
// * imagen entra en la biblioteca con su SHA-256 calculado pero sin comprobar contra una oficial.
func resolveDirect(r Resolver) (Resolved, error) {
	req, err := http.NewRequest("HEAD", r.Index, nil)
	if err != nil {
		return Resolved{}, err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := Client.Do(req)
	if err != nil {
		return Resolved{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Resolved{}, fmt.Errorf("%s respondió %d", r.Index, resp.StatusCode)
	}
	final := resp.Request.URL
	if final.Scheme != "https" {
		return Resolved{}, fmt.Errorf("%s redirigió a una conexión sin cifrar", r.Index)
	}
	name := path.Base(final.Path)
	if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil && params["filename"] != "" {
		name = params["filename"]
	}
	if !strings.HasSuffix(strings.ToLower(name), ".iso") {
		return Resolved{}, fmt.Errorf("%s no lleva a una imagen .iso", r.Index)
	}
	return Resolved{URL: final.String(), FileName: name, SizeBytes: max(resp.ContentLength, 0)}, nil
}

// * headSize pregunta al servidor el tamaño de un archivo sin descargarlo (0 si no lo dice).
func headSize(rawURL string) int64 {
	if !strings.HasPrefix(rawURL, "https://") {
		return 0
	}
	req, err := http.NewRequest("HEAD", rawURL, nil)
	if err != nil {
		return 0
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := Client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0
	}
	return max(resp.ContentLength, 0)
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
	if strings.HasPrefix(name, "https://") {
		return name
	}
	// * Algunos espejos listan los archivos con rutas absolutas ("/fedora/releases/..."): cuelgan del servidor.
	if strings.HasPrefix(name, "/") {
		u.Path = name
		return u.String()
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

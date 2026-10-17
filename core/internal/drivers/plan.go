package drivers

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"rebootera/core/internal/hardware"
)

// Límites del análisis: un Vault con miles de paquetes no debe bloquear a Core.
const (
	maxINFSize   = 4 << 20
	maxINFCount  = 3000
	maxScanDepth = 8
)

// * Scan recorre root buscando archivos INF y devuelve los paquetes válidos
// * para x64. Ignora enlaces simbólicos y archivos que no son INF de Windows.
func Scan(root string) []*Package {
	s := &scanner{root: root}
	filepath.WalkDir(root, s.visit)
	slices.SortFunc(s.packages, compareINF)
	return s.packages
}

// scanner acumula los paquetes que encuentra WalkDir.
type scanner struct {
	root     string
	packages []*Package
	count    int
}

// * visit analiza cada archivo que recorre WalkDir.
func (s *scanner) visit(path string, entry fs.DirEntry, err error) error {
	if err != nil {
		return nil
	}
	rel, relErr := filepath.Rel(s.root, path)
	if relErr != nil {
		return nil
	}
	if entry.IsDir() {
		if depth(rel) > maxScanDepth {
			return filepath.SkipDir
		}
		return nil
	}
	if !entry.Type().IsRegular() || !strings.EqualFold(filepath.Ext(path), ".inf") {
		return nil
	}
	if info, infoErr := entry.Info(); infoErr != nil || info.Size() > maxINFSize {
		return nil
	}
	if s.count++; s.count > maxINFCount {
		return filepath.SkipAll
	}
	dir := filepath.ToSlash(filepath.Dir(rel))
	pkg, parseErr := ParseFile(path, dir, filepath.ToSlash(rel))
	if parseErr != nil || !pkg.Amd64 || len(pkg.ids) == 0 {
		return nil
	}
	pkg.Signature = checkCatalog(filepath.Join(filepath.Dir(path), pkg.Catalog), pkg.Catalog)
	s.packages = append(s.packages, pkg)
	return nil
}

// * compareINF ordena los paquetes por la ruta de su INF.
func compareINF(a, b *Package) int {
	return strings.Compare(a.INF, b.INF)
}

// * depth cuenta los niveles de una ruta relativa.
func depth(rel string) int {
	if rel == "." {
		return 0
	}
	return strings.Count(filepath.ToSlash(rel), "/") + 1
}

// pkcs7SignedData es el OID 1.2.840.113549.1.7.2 de un PKCS#7 firmado.
var pkcs7SignedData = []byte{0x2A, 0x86, 0x48, 0x86, 0xF7, 0x0D, 0x01, 0x07, 0x02}

// * checkCatalog comprueba que el catálogo (.cat) del paquete existe y es un
// * PKCS#7 firmado. No verifica la cadena de certificados (eso lo hace
// * Windows al instalar el driver).
func checkCatalog(path, name string) string {
	if name == "" {
		return SignatureMissing
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return SignatureMissing
	}
	// * Un catálogo es una secuencia DER (0x30) con el OID de datos firmados al principio.
	if len(data) < 16 || data[0] != 0x30 || !bytes.Contains(data[:min(len(data), 64)], pkcs7SignedData) {
		return SignatureInvalid
	}
	return SignaturePresent
}

// Match es un dispositivo con el paquete elegido para él.
type Match struct {
	Device      hardware.Device `json:"device"`
	Package     *Package        `json:"package"`
	HardwareID  string          `json:"hardwareId"`
	Specific    bool            `json:"specific"`
	Description string          `json:"description"`
}

// Plan es el resultado de casar los dispositivos con los paquetes.
type Plan struct {
	Matches []Match `json:"matches"`
	// Missing son los dispositivos críticos sin paquete.
	Missing []hardware.Device `json:"missing"`
	// Packages son los paquetes que hay que llevar al instalador, sin repetir.
	Packages []*Package `json:"packages"`
}

// * BuildPlan elige, para cada dispositivo, el paquete más específico y más
// * reciente que declara uno de sus Hardware ID.
func BuildPlan(devices []hardware.Device, packages []*Package) Plan {
	plan := Plan{Matches: []Match{}, Missing: []hardware.Device{}, Packages: []*Package{}}
	used := map[*Package]bool{}
	for _, device := range devices {
		match, ok := bestMatch(device, packages)
		if !ok {
			if device.Critical {
				plan.Missing = append(plan.Missing, device)
			}
			continue
		}
		plan.Matches = append(plan.Matches, match)
		if !used[match.Package] {
			used[match.Package] = true
			plan.Packages = append(plan.Packages, match.Package)
		}
	}
	return plan
}

// * bestMatch busca el mejor paquete para un dispositivo. Los Hardware ID del
// * dispositivo van de más a menos específico; gana el primero que algún
// * paquete declara y, entre paquetes, el de fecha más reciente.
func bestMatch(device hardware.Device, packages []*Package) (Match, bool) {
	for index, id := range device.HardwareIDs {
		var best *Package
		for _, pkg := range packages {
			if !pkg.Supports(id) {
				continue
			}
			if best == nil || newer(pkg, best) {
				best = pkg
			}
		}
		if best != nil {
			return Match{
				Device:      device,
				Package:     best,
				HardwareID:  id,
				Specific:    index < len(device.HardwareIDs)-1,
				Description: best.Describe(id),
			}, true
		}
	}
	return Match{}, false
}

// * newer indica si a es más reciente que b (fecha mm/dd/aaaa del INF).
func newer(a, b *Package) bool {
	da, db := sortableDate(a.Date), sortableDate(b.Date)
	if da != db {
		return da > db
	}
	return a.INF < b.INF
}

// * sortableDate convierte mm/dd/aaaa en aaaammdd para poder comparar.
func sortableDate(date string) string {
	parts := strings.Split(date, "/")
	if len(parts) != 3 {
		return "00000000"
	}
	return parts[2] + pad2(parts[0]) + pad2(parts[1])
}

// * pad2 rellena con ceros a dos dígitos.
func pad2(s string) string {
	if len(s) == 1 {
		return "0" + s
	}
	return s
}

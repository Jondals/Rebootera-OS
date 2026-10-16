// * Package sensors: temperaturas de los sensores hwmon del kernel
// * (/sys/class/hwmon), legibles sin privilegios. Incluye CPU (coretemp,
// * k10temp), discos NVMe y otros chips que el kernel exponga.
package sensors

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Reading es una lectura de temperatura.
type Reading struct {
	Chip    string  `json:"chip"`
	Label   string  `json:"label"`
	Kind    string  `json:"kind"`
	Celsius float64 `json:"celsius"`
}

// Tipos de sensor que distingue la interfaz.
const (
	KindCPU     = "cpu"
	KindStorage = "storage"
	KindGPU     = "gpu"
	KindOther   = "other"
)

// sysRoot es la raíz de sysfs; se puede cambiar en las pruebas.
var sysRoot = "/sys"

// * Temperatures devuelve todas las temperaturas disponibles.
func Temperatures() []Reading {
	chips, err := filepath.Glob(filepath.Join(sysRoot, "class/hwmon/hwmon*"))
	if err != nil {
		return []Reading{}
	}
	readings := []Reading{}
	for _, chipDir := range chips {
		chip := readTrim(filepath.Join(chipDir, "name"))
		inputs, _ := filepath.Glob(filepath.Join(chipDir, "temp*_input"))
		for _, input := range inputs {
			milli, err := strconv.ParseFloat(readTrim(input), 64)
			// ! Algunos chips publican valores absurdos cuando el sensor no existe.
			if err != nil || milli <= -40000 || milli >= 150000 {
				continue
			}
			label := readTrim(strings.TrimSuffix(input, "_input") + "_label")
			readings = append(readings, Reading{
				Chip:    chip,
				Label:   firstNonEmpty(label, chip),
				Kind:    kind(chip),
				Celsius: milli / 1000,
			})
		}
	}
	slices.SortFunc(readings, compareReadings)
	return readings
}

// * kind clasifica un chip hwmon por su nombre.
func kind(chip string) string {
	switch chip {
	case "coretemp", "k10temp", "zenpower", "cpu_thermal":
		return KindCPU
	case "nvme", "drivetemp":
		return KindStorage
	case "amdgpu", "nouveau", "radeon", "i915", "xe":
		return KindGPU
	}
	return KindOther
}

// * compareReadings ordena por tipo y etiqueta.
func compareReadings(a, b Reading) int {
	if a.Kind != b.Kind {
		return strings.Compare(a.Kind, b.Kind)
	}
	return strings.Compare(a.Chip+a.Label, b.Chip+b.Label)
}

// * readTrim devuelve el contenido de un archivo sin espacios alrededor.
func readTrim(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
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

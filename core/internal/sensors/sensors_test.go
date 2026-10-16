// * Pruebas de la lectura de temperaturas con un hwmon falso.
package sensors

import (
	"os"
	"path/filepath"
	"testing"

	"rebootera/core/internal/testutil"
)

// * TestTemperaturas comprueba etiquetas, tipos y el descarte de valores absurdos.
func TestTemperaturas(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"class/hwmon/hwmon0/name":        "coretemp",
		"class/hwmon/hwmon0/temp1_input": "45000",
		"class/hwmon/hwmon0/temp1_label": "Package id 0",
		"class/hwmon/hwmon1/name":        "nvme",
		"class/hwmon/hwmon1/temp1_input": "38850",
		"class/hwmon/hwmon2/name":        "acpitz",
		"class/hwmon/hwmon2/temp1_input": "-273000",
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte(content), 0o644)
	}
	t.Cleanup(testutil.Keep(&sysRoot).Restore)
	sysRoot = root

	readings := Temperatures()
	if len(readings) != 2 {
		t.Fatalf("esperaba 2 lecturas válidas, obtuve %+v", readings)
	}
	if readings[0].Kind != KindCPU || readings[0].Label != "Package id 0" || readings[0].Celsius != 45 {
		t.Fatalf("CPU: %+v", readings[0])
	}
	if readings[1].Kind != KindStorage || readings[1].Label != "nvme" || readings[1].Celsius != 38.85 {
		t.Fatalf("NVMe: %+v", readings[1])
	}
}

// * Pruebas del inventario de hardware con un sysfs y un pci.ids falsos.
package hardware

import (
	"os"
	"path/filepath"
	"testing"

	"rebootera/core/internal/testutil"
)

// * write crea un archivo con su carpeta.
func write(t *testing.T, path, content string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// * TestPCI comprueba nombres, categoría, Hardware ID y dispositivos críticos.
func TestPCI(t *testing.T) {
	root := t.TempDir()
	vmd := filepath.Join(root, "sys/bus/pci/devices/0000:00:0e.0")
	write(t, filepath.Join(vmd, "vendor"), "0x8086")
	write(t, filepath.Join(vmd, "device"), "0xa77f")
	write(t, filepath.Join(vmd, "subsystem_vendor"), "0x1028")
	write(t, filepath.Join(vmd, "subsystem_device"), "0x0b1a")
	write(t, filepath.Join(vmd, "revision"), "0x01")
	write(t, filepath.Join(vmd, "class"), "0x010400")

	gpu := filepath.Join(root, "sys/bus/pci/devices/0000:00:02.0")
	write(t, filepath.Join(gpu, "vendor"), "0x1234")
	write(t, filepath.Join(gpu, "device"), "0x1111")
	write(t, filepath.Join(gpu, "class"), "0x030000")
	os.MkdirAll(filepath.Join(root, "sys/bus/pci/drivers/bochs-drm"), 0o755)
	os.Symlink(filepath.Join(root, "sys/bus/pci/drivers/bochs-drm"), filepath.Join(gpu, "driver"))

	write(t, filepath.Join(root, "pci.ids"),
		"# comentario\n8086  Intel Corporation\n\ta77f  Volume Management Device NVMe RAID Controller\n\t\t1028 0b1a  Subsistema\n"+
			"1234  Technical Corp.\n\t1111  QEMU Virtual Video Controller\n"+
			"C 01  Mass storage controller\n\t04  RAID bus controller\nC 03  Display controller\n\t00  VGA compatible controller\n")

	t.Cleanup(testutil.Keep(&sysRoot).Restore)
	t.Cleanup(testutil.Keep(&pciIDsPath).Restore)
	t.Cleanup(testutil.Keep(&usbIDsPath).Restore)
	sysRoot, pciIDsPath, usbIDsPath = filepath.Join(root, "sys"), filepath.Join(root, "pci.ids"), filepath.Join(root, "no-existe")

	devices := List()
	if len(devices) != 2 {
		t.Fatalf("esperaba 2 dispositivos, obtuve %d", len(devices))
	}
	raid := devices[0]
	if !raid.Critical || raid.Category != CatStorage || raid.Driver != "" {
		t.Fatalf("la controladora RAID sin driver debería ser crítica: %+v", raid)
	}
	if raid.Vendor != "Intel Corporation" || raid.ClassName != "RAID bus controller" {
		t.Fatalf("nombres: %+v", raid)
	}
	if raid.HardwareIDs[0] != `PCI\VEN_8086&DEV_A77F&SUBSYS_0B1A1028&REV_01` || raid.HardwareIDs[3] != `PCI\VEN_8086&DEV_A77F` {
		t.Fatalf("Hardware ID: %v", raid.HardwareIDs)
	}
	if devices[1].Category != CatDisplay || devices[1].Driver != "bochs-drm" || devices[1].Critical {
		t.Fatalf("GPU: %+v", devices[1])
	}
}

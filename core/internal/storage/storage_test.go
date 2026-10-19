// * Pruebas del inventario de discos con un sysfs, udev y mountinfo falsos:
// * clasificación de discos y particiones, medio de arranque y Safe Disk Guard.
package storage

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

// * fakeSystem monta un equipo con un NVMe interno con Windows y un USB que
// * es el medio de arranque de Rebootera.
func fakeSystem(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	sys := filepath.Join(root, "sys")
	udev := filepath.Join(root, "udev")
	proc := filepath.Join(root, "proc")

	// NVMe interno: EFI + MSR + Windows + recuperación.
	nvme := filepath.Join(sys, "devices/pci0000:00/nvme/nvme0/nvme0n1")
	write(t, filepath.Join(nvme, "dev"), "259:0")
	write(t, filepath.Join(nvme, "size"), "1953525168")
	write(t, filepath.Join(nvme, "removable"), "0")
	write(t, filepath.Join(nvme, "device/model"), "Samsung SSD 980 PRO 1TB")
	parts := []struct{ name, dev, num, ptype, fs string }{
		{"nvme0n1p1", "259:1", "1", "c12a7328-f81f-11d2-ba4b-00a0c93ec93b", "vfat"},
		{"nvme0n1p2", "259:2", "2", "e3c9e316-0b5c-4db8-817d-f92df00215ae", ""},
		{"nvme0n1p3", "259:3", "3", "ebd0a0a2-b9e5-4433-87c0-68b6b72699c7", "ntfs"},
		{"nvme0n1p4", "259:4", "4", "de94bba4-06d1-4d40-a16a-bfd50179d6ac", "ntfs"},
	}
	for _, p := range parts {
		write(t, filepath.Join(nvme, p.name, "partition"), p.num)
		write(t, filepath.Join(nvme, p.name, "dev"), p.dev)
		write(t, filepath.Join(nvme, p.name, "size"), "2048")
		write(t, filepath.Join(udev, "b"+p.dev), "E:ID_PART_ENTRY_TYPE="+p.ptype+"\nE:ID_FS_TYPE="+p.fs+"\nE:ID_SERIAL_SHORT=SECRETO\n")
	}

	// USB con la partición Vault de Rebootera montada como medio de arranque.
	usb := filepath.Join(sys, "devices/pci0000:00/usb1/1-1/host0/block/sdb")
	write(t, filepath.Join(usb, "dev"), "8:16")
	write(t, filepath.Join(usb, "size"), "62521344")
	write(t, filepath.Join(usb, "removable"), "1")
	write(t, filepath.Join(usb, "device/model"), "Extreme Pro")
	write(t, filepath.Join(usb, "sdb1/partition"), "1")
	write(t, filepath.Join(usb, "sdb1/dev"), "8:17")
	write(t, filepath.Join(usb, "sdb1/size"), "62000000")
	write(t, filepath.Join(udev, "b8:17"), "E:ID_FS_TYPE=exfat\nE:ID_FS_LABEL=REBOOTERA\n")
	write(t, filepath.Join(udev, "b8:16"), "E:ID_PART_TABLE_TYPE=gpt\nE:ID_SERIAL=SECRETO\n")

	// Dispositivo loop que debe ignorarse.
	write(t, filepath.Join(sys, "devices/virtual/block/loop0/size"), "100")

	os.MkdirAll(filepath.Join(sys, "block"), 0o755)
	os.Symlink(nvme, filepath.Join(sys, "block/nvme0n1"))
	os.Symlink(usb, filepath.Join(sys, "block/sdb"))
	os.Symlink(filepath.Join(sys, "devices/virtual/block/loop0"), filepath.Join(sys, "block/loop0"))

	write(t, filepath.Join(proc, "self/mountinfo"),
		"36 25 8:17 / /run/live/medium ro,noatime - exfat /dev/sdb1 ro\n"+
			"37 25 0:5 / /dev rw - devtmpfs udev rw\n")

	t.Cleanup(testutil.Keep(&sysRoot).Restore)
	t.Cleanup(testutil.Keep(&udevRoot).Restore)
	t.Cleanup(testutil.Keep(&procRoot).Restore)
	sysRoot, udevRoot, procRoot = sys, udev, proc
}

// * TestClasificacion comprueba roles, transporte, orden y protección.
func TestClasificacion(t *testing.T) {
	fakeSystem(t)
	disks := List()
	if len(disks) != 2 {
		t.Fatalf("esperaba 2 discos (sin loop), obtuve %d", len(disks))
	}
	live, nvme := disks[0], disks[1]
	if live.Name != "sdb" || live.Role != RoleLiveMedia || live.Transport != "usb" || !live.Protected {
		t.Fatalf("medio de arranque mal clasificado: %+v", live)
	}
	if live.PartTable != "gpt" || live.Partitions[0].Role != PartVault || live.Partitions[0].MountPoint != "/run/live/medium" {
		t.Fatalf("partición Vault mal clasificada: %+v", live.Partitions)
	}
	if nvme.Role != RoleInternal || nvme.Transport != "nvme" || !nvme.Protected || nvme.SizeBytes != 1953525168*512 {
		t.Fatalf("NVMe mal clasificado: %+v", nvme)
	}
	want := []string{PartEFI, PartMSR, PartWindows, PartRecovery}
	for i, role := range want {
		if nvme.Partitions[i].Role != role {
			t.Fatalf("partición %d: esperaba %s, obtuve %s", i+1, role, nvme.Partitions[i].Role)
		}
	}
}

// * fakeDM crea un dispositivo device-mapper falso con una sola partición debajo.
func fakeDM(t *testing.T, name, slaveDir string) {
	t.Helper()
	write(t, filepath.Join(sysRoot, "devices/virtual/block", name, "dev"), "253:0")
	os.MkdirAll(filepath.Join(sysRoot, "devices/virtual/block", name, "slaves"), 0o755)
	os.Symlink(slaveDir, filepath.Join(sysRoot, "devices/virtual/block", name, "slaves", filepath.Base(slaveDir)))
	os.Symlink(filepath.Join(sysRoot, "devices/virtual/block", name), filepath.Join(sysRoot, "block", name))
}

// * TestVentoy comprueba el arranque a través de Ventoy: el medio live está en
// * un dispositivo dm (la ISO mapeada) y el Vault se monta con VentoyPart; ambos
// * deben resolverse a la partición física del USB.
func TestVentoy(t *testing.T) {
	fakeSystem(t)
	usbPart := filepath.Join(sysRoot, "devices/pci0000:00/usb1/1-1/host0/block/sdb/sdb1")
	fakeDM(t, "dm-0", usbPart)
	fakeDM(t, "dm-1", usbPart)
	os.MkdirAll(filepath.Join(sysRoot, "dev/block"), 0o755)
	os.Symlink("../../devices/virtual/block/dm-0", filepath.Join(sysRoot, "dev/block/253:0"))
	os.Symlink("../../devices/virtual/block/dm-1", filepath.Join(sysRoot, "dev/block/253:1"))
	write(t, filepath.Join(procRoot, "self/mountinfo"),
		"36 25 253:0 / /run/live/medium ro - iso9660 /dev/mapper/ventoy ro\n"+
			"40 25 253:1 / /run/rebootera/vault rw - exfat /dev/mapper/VentoyPart rw\n")

	disks := List()
	live := disks[0]
	if live.Name != "sdb" || live.Role != RoleLiveMedia {
		t.Fatalf("el USB de Ventoy debería ser el medio de arranque: %+v", live)
	}
	if live.Partitions[0].MountPoint != "/run/rebootera/vault" {
		t.Fatalf("el Vault debería verse montado en /run/rebootera/vault: %+v", live.Partitions[0])
	}
}

// * TestSinSeries comprueba que los números de serie de udev no se exponen.
func TestSinSeries(t *testing.T) {
	fakeSystem(t)
	props := udevProps("8:16")
	for key := range props {
		if key == "ID_SERIAL" || key == "ID_SERIAL_SHORT" {
			t.Fatalf("se expuso %s", key)
		}
	}
}

// * TestUnescapeMount comprueba el escapado de espacios en mountinfo.
func TestUnescapeMount(t *testing.T) {
	if got := unescapeMount(`/media/Mi\040USB`); got != "/media/Mi USB" {
		t.Fatalf("obtuve %q", got)
	}
}

// * TestUnescapeUdev: udev escapa los espacios y otros caracteres como \xNN.
func TestUnescapeUdev(t *testing.T) {
	cases := map[string]string{
		`basic\x20data\x20partition`: "basic data partition",
		`Mis\x2fDatos`:               "Mis/Datos",
		`sin escapes`:                "sin escapes",
		`mal\xZZ`:                    `mal\xZZ`,
		`corto\x2`:                   `corto\x2`,
	}
	for in, want := range cases {
		if got := unescapeUdev(in); got != want {
			t.Fatalf("%q: quería %q y salió %q", in, want, got)
		}
	}
	if friendlyLabel("Basic data partition") != "" || friendlyLabel("Windows") != "Windows" {
		t.Fatal("los nombres genéricos se descartan y los propios se conservan")
	}
}

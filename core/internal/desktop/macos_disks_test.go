// * Pruebas de la lógica de macOS: lectura de diskutil, tamaño de las particiones y script de preparación.
package desktop

import (
	"strings"
	"testing"
)

// diskutilListSample es "diskutil list -plist external physical" (ya pasado a JSON con plutil) con dos discos externos.
const diskutilListSample = `{"AllDisksAndPartitions":[
 {"DeviceIdentifier":"disk4","Size":31914983424,"Partitions":[{"DeviceIdentifier":"disk4s1","VolumeName":"REBOOTERA","Content":"Microsoft Basic Data","Size":31000000000},{"DeviceIdentifier":"disk4s2","Content":"EFI","Size":33554432}]},
 {"DeviceIdentifier":"disk5","Size":1000204886016,"Partitions":[]},
 {"DeviceIdentifier":"disk9","Size":8000000000,"Partitions":[]}
]}`

// * TestUSBDisksFromJSON comprueba que solo se ofrecen discos externos conectados por USB.
func TestUSBDisksFromJSON(t *testing.T) {
	infos := map[string][]byte{
		"disk4": []byte(`{"MediaName":"SanDisk Ultra","Internal":false,"BusProtocol":"USB"}`),
		"disk5": []byte(`{"MediaName":"Disco Thunderbolt","Internal":false,"BusProtocol":"PCI-Express"}`),
		"disk9": []byte(`{"MediaName":"Imagen","Internal":false,"BusProtocol":"USB","SystemImage":true}`),
	}
	disks, err := usbDisksFromJSON([]byte(diskutilListSample), infos)
	if err != nil {
		t.Fatal(err)
	}
	if len(disks) != 1 {
		t.Fatalf("solo disk4 es un USB externo ofrecible: %+v", disks)
	}
	disk := disks[0]
	if disk.Number != 4 || disk.Path != "/dev/disk4" || disk.FriendlyName != "SanDisk Ultra" || !disk.Rebootera || len(disk.Volumes) != 2 {
		t.Fatalf("disco mal leído: %+v", disk)
	}
	// * Sin información de un disco, no se ofrece (mejor no borrar algo que no se sabe qué es).
	if none, _ := usbDisksFromJSON([]byte(diskutilListSample), nil); len(none) != 0 {
		t.Fatalf("sin diskutil info no debe ofrecerse nada: %+v", none)
	}
}

// * TestPartitionSizeMiB comprueba el reparto del disco: la de arranque de 32 MiB va aparte y un USB minúsculo se rechaza.
func TestPartitionSizeMiB(t *testing.T) {
	got, err := partitionSizeMiB(32 << 30)
	if err != nil || got != 32*1024-espSizeMiB-4 {
		t.Fatalf("32 GiB: %d %v", got, err)
	}
	if _, err := partitionSizeMiB(100 << 20); err == nil {
		t.Fatal("un USB de 100 MiB no sirve")
	}
}

// * TestInstallScript comprueba que el script borra solo el disco indicado y deja la partición de arranque al final.
func TestInstallScript(t *testing.T) {
	script := macInstallScript("disk4", "/Users/ana/Library/Caches/Rebootera/ventoy-1.1.18/ventoy/ventoy.disk.img", 30000)
	for _, want := range []string{
		"diskutil unmountDisk force /dev/disk4\n",
		"diskutil partitionDisk /dev/disk4 GPT ExFAT REBOOTERA 30000m '%C12A7328-F81F-11D2-BA4B-00A0C93EC93B%' VTOYEFI 32m\n",
		"dd if='/Users/ana/Library/Caches/Rebootera/ventoy-1.1.18/ventoy/ventoy.disk.img' of=/dev/rdisk4s2 bs=1m\n",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("falta %q en:\n%s", want, script)
		}
	}
	if strings.Count(script, "/dev/") != 4 || strings.Contains(script, "disk0") {
		t.Fatalf("el script solo debe tocar disk4:\n%s", script)
	}
}

// * TestDiskID comprueba que solo se aceptan identificadores de disco sencillos.
func TestDiskID(t *testing.T) {
	for _, good := range []string{"disk0", "disk4", "disk12"} {
		if !diskID.MatchString(good) {
			t.Errorf("%s debería aceptarse", good)
		}
	}
	for _, bad := range []string{"", "disk", "disk4s1", "/dev/disk4", "disk4; rm -rf /", "rdisk4"} {
		if diskID.MatchString(bad) {
			t.Errorf("%q no debería aceptarse", bad)
		}
	}
}

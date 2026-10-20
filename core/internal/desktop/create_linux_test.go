//go:build linux

// * Pruebas de la lista de discos USB de Linux a partir de salidas de lsblk.
package desktop

import "testing"

// lsblkSample es la salida de un portátil con su disco NVMe del sistema y dos USB (uno ya de Rebootera).
const lsblkSample = `{"blockdevices":[
 {"name":"nvme0n1","path":"/dev/nvme0n1","type":"disk","size":512110190592,"tran":"nvme","rm":false,"model":"Samsung SSD","vendor":null,"maj:min":"259:0","children":[
   {"name":"nvme0n1p1","path":"/dev/nvme0n1p1","type":"part","size":536870912,"fstype":"vfat","label":null,"mountpoint":"/boot/efi"},
   {"name":"nvme0n1p2","path":"/dev/nvme0n1p2","type":"part","size":511000000000,"fstype":"ext4","label":null,"mountpoint":"/"}]},
 {"name":"sda","path":"/dev/sda","type":"disk","size":"32000000000","tran":"usb","rm":"1","model":"Ultra","vendor":"SanDisk ","maj:min":"8:0","children":[
   {"name":"sda1","path":"/dev/sda1","type":"part","size":"31900000000","fstype":"exfat","label":"REBOOTERA","mountpoint":null},
   {"name":"sda2","path":"/dev/sda2","type":"part","size":"33554432","fstype":"vfat","label":"VTOYEFI","mountpoint":null}]},
 {"name":"sdb","path":"/dev/sdb","type":"disk","size":16000000000,"tran":"usb","rm":true,"model":"Cruzer","vendor":"","maj:min":"8:16","children":[]},
 {"name":"sdc","path":"/dev/sdc","type":"disk","size":1000000000000,"tran":"sata","rm":false,"model":"Disco duro","maj:min":"8:32","children":[]},
 {"name":"loop0","path":"/dev/loop0","type":"loop","size":1000000,"tran":null,"rm":false,"maj:min":"7:0","children":[]},
 {"name":"sdd","path":"/dev/sdd","type":"disk","size":8000000000,"tran":"usb","rm":true,"model":"Con el sistema","maj:min":"8:48","children":[
   {"name":"sdd1","path":"/dev/sdd1","type":"part","size":8000000000,"fstype":"ext4","label":null,"mountpoint":"/"}]}
]}`

// * TestUSBDisksFrom comprueba qué discos se ofrecen: solo los USB, nunca el del sistema ni los internos o loop.
func TestUSBDisksFrom(t *testing.T) {
	disks, err := usbDisksFrom([]byte(lsblkSample), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(disks) != 2 {
		t.Fatalf("debería haber 2 discos USB (sda y sdb): %+v", disks)
	}
	first := disks[0]
	if first.Path != "/dev/sda" || first.FriendlyName != "SanDisk Ultra" || first.SizeBytes != 32000000000 || !first.Rebootera {
		t.Fatalf("primer disco mal leído: %+v", first)
	}
	if first.Number != 8*1048576 || len(first.Volumes) != 2 || first.Volumes[0].Label != "REBOOTERA" {
		t.Fatalf("número o volúmenes mal leídos: %+v", first)
	}
	if disks[1].Path != "/dev/sdb" || disks[1].FriendlyName != "Cruzer" || disks[1].Rebootera {
		t.Fatalf("segundo disco mal leído: %+v", disks[1])
	}
	// * Solo con la marca de pruebas se admiten los dispositivos loop.
	withLoop, _ := usbDisksFrom([]byte(lsblkSample), true)
	if len(withLoop) != 3 {
		t.Fatalf("con allowLoop debería sumar el loop0: %+v", withLoop)
	}
}

// * TestDevicePath comprueba que solo se aceptan nodos /dev/<nombre> sencillos (nada que se pueda colar en una orden).
func TestDevicePath(t *testing.T) {
	for _, good := range []string{"/dev/sdb", "/dev/nvme0n1", "/dev/mmcblk0", "/dev/loop3"} {
		if !devicePath.MatchString(good) {
			t.Errorf("%s debería aceptarse", good)
		}
	}
	for _, bad := range []string{"", "sdb", "/dev/sdb; rm -rf /", "/dev/../etc/passwd", "/dev/sd b", "/dev/disk/by-id/x", "$(id)"} {
		if devicePath.MatchString(bad) {
			t.Errorf("%q no debería aceptarse", bad)
		}
	}
}

// * Pruebas de la regla que decide si se puede añadir Rebootera sin borrar un disco.
package desktop

import "testing"

// * TestAdoptable cubre un USB tipo Rufus, uno FAT32, uno vacío y uno que ya es de Rebootera.
func TestAdoptable(t *testing.T) {
	cases := []struct {
		name string
		disk USBDisk
		want bool
	}{
		{"Rufus NTFS con datos", USBDisk{UsedBytes: 5 << 30, Volumes: []USBVolume{{FileSystem: "NTFS"}}}, true},
		{"exFAT con datos", USBDisk{UsedBytes: 1 << 30, Volumes: []USBVolume{{FileSystem: "exFAT"}}}, true},
		{"FAT32: no cabe una ISO de Windows", USBDisk{UsedBytes: 1 << 30, Volumes: []USBVolume{{FileSystem: "FAT32"}}}, false},
		{"disco vacío: nada que conservar", USBDisk{Volumes: []USBVolume{{FileSystem: "NTFS"}}}, false},
		{"ya es de Rebootera: se actualiza", USBDisk{Rebootera: true, UsedBytes: 1 << 30, Volumes: []USBVolume{{FileSystem: "exFAT"}}}, false},
		{"sin letras de unidad", USBDisk{UsedBytes: 1 << 30}, false},
	}
	for _, c := range cases {
		if got := adoptable(c.disk); got != c.want {
			t.Errorf("%s: adoptable = %v, se esperaba %v", c.name, got, c.want)
		}
	}
}

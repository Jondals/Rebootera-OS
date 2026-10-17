//go:build linux

// * Prueba del ejecutor del gestor de particiones sobre un disco virtual real (un
// * archivo montado como dispositivo loop): ext4 y NTFS con datos, todas las operaciones,
// * y comprobación de que los archivos y los sistemas de archivos salen intactos.
// ! Solo se ejecuta como root y con REBOOTERA_LOOP_TESTS=1: crea y borra dispositivos loop.
// ! Trabaja únicamente con archivos de imagen que crea en un directorio temporal.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"rebootera/core/internal/partman"
	"rebootera/core/internal/storage"
)

// loopDisk es un disco virtual para las pruebas.
type loopDisk struct {
	t      *testing.T
	image  string
	device string
	name   string
}

// * sh ejecuta una orden y falla la prueba si da error.
func sh(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v: %s", name, args, err, out)
	}
	return string(out)
}

// * newLoopDisk crea un disco GPT de sizeMiB con particiones (inicio y fin en MiB, con su sistema de archivos).
func newLoopDisk(t *testing.T, sizeMiB int, parts [][3]string) *loopDisk {
	t.Helper()
	if os.Getenv("REBOOTERA_LOOP_TESTS") != "1" || os.Geteuid() != 0 {
		t.Skip("solo con REBOOTERA_LOOP_TESTS=1 y como root")
	}
	image := filepath.Join(t.TempDir(), "disco.img")
	sh(t, "truncate", "-s", strconv.Itoa(sizeMiB)+"M", image)
	device := strings.TrimSpace(sh(t, "losetup", "-fP", "--show", image))
	d := &loopDisk{t: t, image: image, device: device, name: filepath.Base(device)}
	t.Cleanup(d.close)
	sh(t, "parted", "-s", device, "mklabel", "gpt")
	for i, p := range parts {
		sh(t, "parted", "-s", device, "mkpart", "p"+strconv.Itoa(i+1), p[0]+"MiB", p[1]+"MiB")
	}
	sh(t, "partprobe", device)
	sh(t, "udevadm", "settle")
	for i, p := range parts {
		d.format(i+1, p[2])
	}
	return d
}

// * close libera el dispositivo loop.
func (d *loopDisk) close() {
	exec.Command("umount", "-q", d.dev(1)).Run()
	exec.Command("umount", "-q", d.dev(2)).Run()
	exec.Command("losetup", "-d", d.device).Run()
}

// * dev devuelve la ruta de la partición n.
func (d *loopDisk) dev(n int) string { return d.device + "p" + strconv.Itoa(n) }

// * format crea el sistema de archivos de la partición n.
func (d *loopDisk) format(n int, fs string) {
	switch fs {
	case "ext4":
		sh(d.t, "mkfs.ext4", "-F", "-q", d.dev(n))
	case "ntfs":
		sh(d.t, "mkntfs", "-F", "-Q", "-L", "PRUEBA", d.dev(n))
	case "exfat":
		sh(d.t, "mkfs.exfat", d.dev(n))
	}
}

// * mount monta una partición y devuelve el punto de montaje.
func (d *loopDisk) mount(n int) string {
	dir := d.t.TempDir()
	fs := strings.TrimSpace(sh(d.t, "blkid", "-o", "value", "-s", "TYPE", d.dev(n)))
	if fs == "ntfs" {
		sh(d.t, "mount", "-t", "ntfs-3g", d.dev(n), dir)
	} else {
		sh(d.t, "mount", d.dev(n), dir)
	}
	return dir
}

// * fill escribe varios archivos de datos aleatorios en la partición n y devuelve las sumas.
func (d *loopDisk) fill(n, files, mibEach int) string {
	dir := d.mount(n)
	for i := 0; i < files; i++ {
		sh(d.t, "dd", "if=/dev/urandom", "of="+filepath.Join(dir, fmt.Sprintf("datos%d.bin", i)), "bs=1M", "count="+strconv.Itoa(mibEach), "status=none")
	}
	sums := sh(d.t, "sh", "-c", "cd "+dir+" && sha256sum datos*.bin")
	sh(d.t, "umount", dir)
	return sums
}

// * verify monta la partición n y comprueba las sumas.
func (d *loopDisk) verify(n int, sums string) {
	dir := d.mount(n)
	defer exec.Command("umount", dir).Run()
	check := exec.Command("sh", "-c", "cd "+dir+" && sha256sum -c -")
	check.Stdin = strings.NewReader(sums)
	if out, err := check.CombinedOutput(); err != nil {
		d.t.Fatalf("los datos de la partición %d cambiaron: %v: %s", n, err, out)
	}
}

// * clean comprueba que el sistema de archivos de la partición n está sano.
func (d *loopDisk) clean(n int) {
	fs := strings.TrimSpace(sh(d.t, "blkid", "-o", "value", "-s", "TYPE", d.dev(n)))
	switch fs {
	case "ext4":
		sh(d.t, "e2fsck", "-fn", d.dev(n))
	case "ntfs":
		sh(d.t, "ntfsresize", "--check", d.dev(n))
	}
}

// * sectors devuelve el inicio y el tamaño (sectores) de la partición n según la tabla.
func (d *loopDisk) sectors(n int) (uint64, uint64) {
	out := sh(d.t, "sfdisk", "--json", d.device)
	var table struct {
		PartitionTable struct {
			Partitions []struct {
				Node  string `json:"node"`
				Start uint64 `json:"start"`
				Size  uint64 `json:"size"`
			} `json:"partitions"`
		} `json:"partitiontable"`
	}
	if err := json.Unmarshal([]byte(out), &table); err != nil {
		d.t.Fatal(err)
	}
	for _, p := range table.PartitionTable.Partitions {
		if p.Node == d.dev(n) {
			return p.Start, p.Size
		}
	}
	d.t.Fatalf("la partición %d no existe", n)
	return 0, 0
}

// * disk lee el estado actual como lo haría el inventario del sistema.
func (d *loopDisk) disk() storage.Disk {
	out := sh(d.t, "sfdisk", "--json", d.device)
	var table struct {
		PartitionTable struct {
			Partitions []struct {
				Node  string `json:"node"`
				Start uint64 `json:"start"`
				Size  uint64 `json:"size"`
			} `json:"partitions"`
		} `json:"partitiontable"`
	}
	if err := json.Unmarshal([]byte(out), &table); err != nil {
		d.t.Fatal(err)
	}
	total, _ := strconv.ParseUint(strings.TrimSpace(sh(d.t, "blockdev", "--getsz", d.device)), 10, 64)
	disk := storage.Disk{Name: d.name, Sectors: total, SizeBytes: total * 512, PartTable: "gpt", Role: storage.RoleRemovable}
	for _, p := range table.PartitionTable.Partitions {
		number, _ := strconv.Atoi(strings.TrimPrefix(p.Node, d.device+"p"))
		fs, _ := exec.Command("blkid", "-o", "value", "-s", "TYPE", p.Node).Output()
		disk.Partitions = append(disk.Partitions, storage.Partition{
			Name: filepath.Base(p.Node), Number: number, StartSector: p.Start, Sectors: p.Size, FSType: strings.TrimSpace(string(fs)),
		})
	}
	return disk
}

// * apply calcula y ejecuta una operación con las mismas piezas que el agente.
func (d *loopDisk) apply(op partman.Op) {
	d.t.Helper()
	op.Disk = d.name
	layout := partman.Build(d.disk())
	info := partman.Info{}
	for _, n := range minimumsNeeded(op) {
		for _, seg := range layout.Segments {
			if seg.Number == n {
				min, err := fsMinimum(d.dev(n), seg.FSType)
				if err != nil {
					d.t.Fatalf("mínimo de %d: %v", n, err)
				}
				info[n] = min
			}
		}
	}
	plan, err := partman.Make(layout, op, info)
	if err != nil {
		d.t.Fatalf("plan: %v", err)
	}
	partMu.Lock()
	partJob = partStatus{Running: true}
	partMu.Unlock()
	run := &partRun{disk: d.name}
	run.execute(plan)
	if status := jobStatus(); status.Error != "" || !status.Done {
		d.t.Fatalf("la operación falló: %+v", status)
	}
}

// * TestLoopBoundary mueve el límite entre una ext4 y una NTFS en las dos direcciones.
func TestLoopBoundary(t *testing.T) {
	d := newLoopDisk(t, 700, [][3]string{{"1", "300", "ext4"}, {"300", "650", "ntfs"}})
	sums1, sums2 := d.fill(1, 3, 20), d.fill(2, 3, 20)
	// * La ext4 (izquierda) crece 60 MiB a costa de la NTFS.
	d.apply(partman.Op{Kind: partman.OpBoundary, Part: 1, Other: 2, SizeMiB: 359})
	start1, size1 := d.sectors(1)
	start2, size2 := d.sectors(2)
	if size1 != 359*2048 || start2 != start1+size1 || start2+size2 != 650*2048 {
		t.Fatalf("tabla tras crecer: p1 %d+%d, p2 %d+%d", start1, size1, start2, size2)
	}
	d.clean(1)
	d.clean(2)
	d.verify(1, sums1)
	d.verify(2, sums2)
	// * Ahora la ext4 encoge 100 MiB y la NTFS la sigue y crece.
	d.apply(partman.Op{Kind: partman.OpBoundary, Part: 1, Other: 2, SizeMiB: 259})
	start1, size1 = d.sectors(1)
	start2, size2 = d.sectors(2)
	if size1 != 259*2048 || start2 != start1+size1 || start2+size2 != 650*2048 {
		t.Fatalf("tabla tras encoger: p1 %d+%d, p2 %d+%d", start1, size1, start2, size2)
	}
	d.clean(1)
	d.clean(2)
	d.verify(1, sums1)
	d.verify(2, sums2)
}

// * TestLoopResize encoge y agranda una NTFS por su final.
func TestLoopResize(t *testing.T) {
	d := newLoopDisk(t, 500, [][3]string{{"1", "300", "ntfs"}})
	sums := d.fill(1, 3, 20)
	d.apply(partman.Op{Kind: partman.OpResize, Part: 1, SizeMiB: 150})
	if _, size := d.sectors(1); size != 150*2048 {
		t.Fatalf("tamaño tras encoger: %d", size)
	}
	d.clean(1)
	d.verify(1, sums)
	d.apply(partman.Op{Kind: partman.OpResize, Part: 1, SizeMiB: 400})
	if _, size := d.sectors(1); size != 400*2048 {
		t.Fatalf("tamaño tras agrandar: %d", size)
	}
	d.clean(1)
	d.verify(1, sums)
}

// * TestLoopMerge junta dos particiones conservando la derecha (hay que mover sus datos) y luego la izquierda.
func TestLoopMerge(t *testing.T) {
	d := newLoopDisk(t, 700, [][3]string{{"1", "250", "ntfs"}, {"250", "600", "ext4"}})
	sums := d.fill(2, 3, 20)
	d.apply(partman.Op{Kind: partman.OpMerge, Part: 2, Other: 1})
	start, size := d.sectors(2)
	if start != 1*2048 || start+size != 600*2048 {
		t.Fatalf("tabla tras fusionar: %d+%d", start, size)
	}
	d.clean(2)
	d.verify(2, sums)
	// * Se vuelve a partir (la ext4 encoge y se crea una NTFS al lado) y se fusiona conservando la izquierda.
	d.apply(partman.Op{Kind: partman.OpResize, Part: 2, SizeMiB: 300})
	d.apply(partman.Op{Kind: partman.OpCreate, StartMiB: 301, SizeMiB: 200, FS: "ntfs", Label: "NUEVA"})
	d.clean(2)
	d.verify(2, sums)
	d.apply(partman.Op{Kind: partman.OpMerge, Part: 2, Other: 1})
	if _, size := d.sectors(2); size != 500*2048 {
		t.Fatalf("tamaño tras la segunda fusión: %d", size)
	}
	d.clean(2)
	d.verify(2, sums)
}

// * TestLoopCreateDelete crea una partición exFAT en el hueco y la borra.
func TestLoopCreateDelete(t *testing.T) {
	d := newLoopDisk(t, 500, [][3]string{{"1", "200", "ext4"}})
	sums := d.fill(1, 2, 10)
	d.apply(partman.Op{Kind: partman.OpCreate, StartMiB: 201, SizeMiB: 200, FS: "exfat", Label: "DATOS"})
	if fs := strings.TrimSpace(sh(t, "blkid", "-o", "value", "-s", "TYPE", d.dev(2))); fs != "exfat" {
		t.Fatalf("la partición nueva debería ser exFAT, es %q", fs)
	}
	d.verify(1, sums)
	d.apply(partman.Op{Kind: partman.OpDelete, Part: 2})
	if out, _ := exec.Command("blkid", d.dev(2)).Output(); len(out) != 0 {
		t.Fatalf("la partición 2 debería haber desaparecido: %s", out)
	}
	d.verify(1, sums)
}

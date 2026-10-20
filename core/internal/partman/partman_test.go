// * Pruebas del gestor de particiones con números: la distribución, los planes y,
// * sobre todo, una simulación que comprueba que ningún plan pisa datos vivos.
package partman

import (
	"errors"
	"testing"

	"rebootera/core/internal/storage"
)

// mib convierte MiB en sectores.
func mib(n uint64) uint64 { return n * Align }

// part crea una partición del inventario.
func part(number int, startMiB, sizeMiB uint64, fs string) storage.Partition {
	return storage.Partition{
		Number: number, Name: "vdb" + string(rune('0'+number)), StartSector: mib(startMiB), Sectors: mib(sizeMiB),
		SizeBytes: mib(sizeMiB) * SectorSize, FSType: fs,
	}
}

// disk crea un disco GPT de 1 GiB con las particiones dadas.
func disk(parts ...storage.Partition) storage.Disk {
	return storage.Disk{Name: "vdb", Sectors: mib(1024), SizeBytes: mib(1024) * SectorSize, PartTable: "gpt", Partitions: parts}
}

// * TestBuild comprueba los huecos libres al principio, entre particiones y al final.
func TestBuild(t *testing.T) {
	layout := Build(disk(part(1, 100, 200, "ext4"), part(2, 400, 100, "ntfs")))
	var free []Segment
	for _, s := range layout.Segments {
		if s.Free() {
			free = append(free, s)
		}
	}
	if len(free) != 3 {
		t.Fatalf("debería haber 3 huecos (antes, entre y después): %+v", layout.Segments)
	}
	if free[0].Start != Align || free[0].End() != mib(100) {
		t.Fatalf("hueco inicial: %+v", free[0])
	}
	if free[1].Start != mib(300) || free[1].End() != mib(400) {
		t.Fatalf("hueco intermedio: %+v", free[1])
	}
	// * En GPT el final deja sitio a la copia de la tabla: menos de 1 GiB entero.
	if free[2].Start != mib(500) || free[2].End() != layout.Limit || layout.Limit >= mib(1024) {
		t.Fatalf("hueco final: %+v límite %d", free[2], layout.Limit)
	}
	if layout.Locked != "" {
		t.Fatalf("un disco GPT normal no está bloqueado: %s", layout.Locked)
	}
}

// * TestLocked comprueba los discos que no se pueden tocar.
func TestLocked(t *testing.T) {
	protected := disk(part(1, 1, 10, "ext4"))
	protected.Role = storage.RoleLiveMedia
	readonly := disk(part(1, 1, 10, "ext4"))
	readonly.ReadOnly = true
	raw := disk()
	raw.PartTable = ""
	extended := disk(part(1, 1, 10, "ext4"), storage.Partition{Number: 5, StartSector: mib(20), Sectors: mib(10)})
	extended.PartTable = "msdos"
	mounted := disk(part(1, 1, 10, "ext4"))
	mounted.Partitions[0].MountPoint = "/mnt/x"
	internal := disk(part(1, 1, 10, "ext4"))
	internal.Role = storage.RoleInternal
	internal.Protected = true
	if Build(internal).Locked != "" {
		t.Fatal("un disco interno se puede editar")
	}
	if len(Build(raw).Segments) != 0 {
		t.Fatal("un disco sin tabla no debe enseñar hueco libre")
	}
	cases := map[string]storage.Disk{"mounted": mounted, "protected": protected, "read-only": readonly, "unsupported-table": raw, "unsupported-table ": extended}
	for want, d := range cases {
		layout := Build(d)
		if layout.Locked == "" || layout.Locked != want[:len(layout.Locked)] {
			t.Fatalf("%s: bloqueo %q", want, layout.Locked)
		}
		if _, err := Make(layout, Op{Kind: OpDelete, Disk: "vdb", Part: 1}, nil); err == nil {
			t.Fatalf("%s: no debería planificar nada", want)
		}
	}
}

// vdisk modela, sector a sector en MiB, qué hay realmente en el disco durante la ejecución de un plan.
type vdisk struct {
	t     *testing.T
	table map[int]TableEdit // lo que dice la tabla de particiones
	fs    map[int]uint64    // tamaño del sistema de archivos de cada partición (sectores)
	data  map[int]uint64    // dónde empiezan realmente sus datos
	limit uint64
}

// newVDisk toma el estado inicial de una distribución: todos los sistemas de archivos llenan su partición.
func newVDisk(t *testing.T, layout Layout) *vdisk {
	v := &vdisk{t: t, table: map[int]TableEdit{}, fs: map[int]uint64{}, data: map[int]uint64{}, limit: layout.Limit}
	for _, s := range layout.Segments {
		if !s.Free() {
			v.table[s.Number] = TableEdit{Part: s.Number, Start: s.Start, Sectors: s.Sectors}
			v.fs[s.Number], v.data[s.Number] = s.Sectors, s.Start
		}
	}
	return v
}

// overlaps indica si dos rangos [a, a+an) y [b, b+bn) se pisan.
func overlaps(a, an, b, bn uint64) bool { return a < b+bn && b < a+an }

// run ejecuta los pasos y falla ante cualquier cosa que pueda perder datos.
func (v *vdisk) run(plan Plan) {
	v.t.Helper()
	for i, step := range plan.Steps {
		switch step.Kind {
		case StepShrinkFS:
			if step.Bytes/SectorSize > v.fs[step.Part] {
				v.t.Fatalf("paso %d: encoger a más de lo que ya mide", i)
			}
			if step.Bytes/SectorSize > v.table[step.Part].Sectors {
				v.t.Fatalf("paso %d: el sistema de archivos no cabe en su partición", i)
			}
			v.fs[step.Part] = step.Bytes / SectorSize
		case StepMove:
			if step.From != v.data[step.Part] {
				v.t.Fatalf("paso %d: la copia no empieza donde están los datos (%d != %d)", i, step.From, v.data[step.Part])
			}
			if step.Length < v.fs[step.Part] {
				v.t.Fatalf("paso %d: la copia no cubre todo el sistema de archivos (%d < %d)", i, step.Length, v.fs[step.Part])
			}
			// * El destino no puede pisar los datos vivos de ninguna otra partición.
			for other, start := range v.data {
				if other != step.Part && overlaps(step.To, step.Length, start, v.fs[other]) {
					v.t.Fatalf("paso %d: la copia pisa los datos de la partición %d", i, other)
				}
			}
			if step.To+step.Length > v.limit {
				v.t.Fatalf("paso %d: la copia se sale del disco", i)
			}
			v.data[step.Part] = step.To
		case StepTable:
			v.applyTable(i, step)
		case StepGrowFS:
			v.fs[step.Part] = v.table[step.Part].Sectors
		}
	}
	v.finalCheck()
}

// applyTable aplica los cambios de la tabla comprobando que cuadran con los datos reales.
func (v *vdisk) applyTable(i int, step Step) {
	for _, e := range step.Edits {
		if e.Delete {
			delete(v.table, e.Part)
			delete(v.fs, e.Part)
			delete(v.data, e.Part)
			continue
		}
		if e.Create {
			v.table[e.Part] = e
			v.data[e.Part], v.fs[e.Part] = e.Start, 0
			continue
		}
		if e.Start != v.data[e.Part] {
			v.t.Fatalf("paso %d: la tabla pone la partición %d en %d pero sus datos están en %d", i, e.Part, e.Start, v.data[e.Part])
		}
		if e.Sectors < v.fs[e.Part] {
			v.t.Fatalf("paso %d: la partición %d queda más pequeña que su sistema de archivos", i, e.Part)
		}
		v.table[e.Part] = e
	}
	v.finalCheck()
}

// finalCheck exige una tabla sin solapes, alineada y dentro del disco.
func (v *vdisk) finalCheck() {
	v.t.Helper()
	for a, ea := range v.table {
		if ea.Start%Align != 0 || ea.Sectors%Align != 0 || ea.Start < Align || ea.Start+ea.Sectors > v.limit {
			v.t.Fatalf("la partición %d no está alineada o se sale del disco: %+v", a, ea)
		}
		for b, eb := range v.table {
			if a < b && overlaps(ea.Start, ea.Sectors, eb.Start, eb.Sectors) {
				v.t.Fatalf("las particiones %d y %d se solapan", a, b)
			}
		}
	}
}

// * TestResize cubre encoger y agrandar hacia un hueco, y los rechazos.
func TestResize(t *testing.T) {
	layout := Build(disk(part(1, 100, 200, "ext4"), part(2, 400, 100, "ext4")))
	info := Info{1: 50 << 20, 2: 20 << 20}
	// * Encoger la 1 a 120 MiB (mínimo 50).
	plan, err := Make(layout, Op{Kind: OpResize, Disk: "vdb", Part: 1, SizeMiB: 120}, info)
	if err != nil || plan.Destructive || plan.MovesBytes != 0 {
		t.Fatalf("encoger: %v %+v", err, plan)
	}
	newVDisk(t, layout).run(plan)
	if _, err := Make(layout, Op{Kind: OpResize, Disk: "vdb", Part: 1, SizeMiB: 40}, info); !errors.Is(err, ErrTooSmall) {
		t.Fatalf("por debajo del mínimo: %v", err)
	}
	// * Agrandar la 1 hacia el hueco de 100 MiB que tiene detrás.
	plan, err = Make(layout, Op{Kind: OpResize, Disk: "vdb", Part: 1, SizeMiB: 290}, info)
	if err != nil {
		t.Fatalf("agrandar: %v", err)
	}
	newVDisk(t, layout).run(plan)
	if _, err := Make(layout, Op{Kind: OpResize, Disk: "vdb", Part: 1, SizeMiB: 301}, info); !errors.Is(err, ErrNoSpace) {
		t.Fatalf("más que el hueco: %v", err)
	}
	if _, err := Make(layout, Op{Kind: OpResize, Disk: "vdb", Part: 1, SizeMiB: 200}, info); !errors.Is(err, ErrNoChange) {
		t.Fatalf("mismo tamaño: %v", err)
	}
	if _, err := Make(layout, Op{Kind: OpResize, Disk: "vdb", Part: 1, SizeMiB: 150}, nil); !errors.Is(err, ErrNoInfo) {
		t.Fatalf("sin mínimos no se encoge: %v", err)
	}
	if _, err := Make(layout, Op{Kind: OpResize, Disk: "vdb", Part: 9, SizeMiB: 150}, info); !errors.Is(err, ErrNotFound) {
		t.Fatalf("partición inexistente: %v", err)
	}
}

// * TestResizeRefusals: exFAT, montada o de otro disco.
func TestResizeRefusals(t *testing.T) {
	mounted := part(1, 100, 200, "ext4")
	mounted.MountPoint = "/mnt/x"
	// * Con cualquier partición montada el disco entero queda bloqueado (el núcleo no podría releer la tabla).
	if _, err := Make(Build(disk(mounted, part(2, 400, 100, "exfat"))), Op{Kind: OpResize, Disk: "vdb", Part: 1, SizeMiB: 100}, Info{1: 1 << 20}); !errors.Is(err, ErrMounted) {
		t.Fatalf("montada: %v", err)
	}
	layout := Build(disk(part(1, 100, 200, "ext4"), part(2, 400, 100, "exfat")))
	if _, err := Make(layout, Op{Kind: OpResize, Disk: "vdb", Part: 2, SizeMiB: 50}, Info{2: 1 << 20}); !errors.Is(err, ErrNotResizable) {
		t.Fatalf("exFAT: %v", err)
	}
	if _, err := Make(layout, Op{Kind: OpResize, Disk: "vdc", Part: 2, SizeMiB: 50}, nil); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("disco distinto: %v", err)
	}
}

// * TestBoundary mueve el límite entre dos vecinas en las dos direcciones y comprueba que no se pisan datos.
func TestBoundary(t *testing.T) {
	layout := Build(disk(part(1, 100, 300, "ext4"), part(2, 400, 300, "ntfs")))
	info := Info{1: 60 << 20, 2: 80 << 20}
	// * La izquierda crece de 300 a 380 MiB: la derecha pierde 80 MiB por delante.
	plan, err := Make(layout, Op{Kind: OpBoundary, Disk: "vdb", Part: 1, Other: 2, SizeMiB: 380}, info)
	if err != nil {
		t.Fatal(err)
	}
	if plan.MovesBytes != mib(220)*SectorSize || plan.Destructive {
		t.Fatalf("crecer la izquierda: %+v", plan)
	}
	v := newVDisk(t, layout)
	v.run(plan)
	if v.table[1].Sectors != mib(380) || v.table[2].Start != mib(480) || v.table[2].Sectors != mib(220) {
		t.Fatalf("tabla final: %+v", v.table)
	}
	if v.fs[1] != mib(380) || v.fs[2] != mib(220) {
		t.Fatalf("sistemas de archivos finales: %+v", v.fs)
	}
	// * La izquierda encoge de 300 a 200 MiB: la derecha se desplaza hacia ella y crece.
	plan, err = Make(layout, Op{Kind: OpBoundary, Disk: "vdb", Part: 1, Other: 2, SizeMiB: 200}, info)
	if err != nil {
		t.Fatal(err)
	}
	v = newVDisk(t, layout)
	v.run(plan)
	if v.table[1].Sectors != mib(200) || v.table[2].Start != mib(300) || v.table[2].Sectors != mib(400) {
		t.Fatalf("tabla final: %+v", v.table)
	}
	if v.fs[2] != mib(400) {
		t.Fatalf("la derecha debe ocupar todo su espacio: %+v", v.fs)
	}
	// * Si la izquierda se indica en el segundo parámetro, es lo mismo.
	swapped, err := Make(layout, Op{Kind: OpBoundary, Disk: "vdb", Part: 2, Other: 1, SizeMiB: 200}, info)
	if err != nil || len(swapped.Steps) != len(plan.Steps) {
		t.Fatalf("orden de las vecinas: %v", err)
	}
	// * Límites: no se puede dejar a ninguna por debajo de su mínimo.
	if _, err := Make(layout, Op{Kind: OpBoundary, Disk: "vdb", Part: 1, Other: 2, SizeMiB: 540}, info); !errors.Is(err, ErrTooSmall) {
		t.Fatalf("derecha por debajo del mínimo: %v", err)
	}
	if _, err := Make(layout, Op{Kind: OpBoundary, Disk: "vdb", Part: 1, Other: 2, SizeMiB: 50}, info); !errors.Is(err, ErrTooSmall) {
		t.Fatalf("izquierda por debajo del mínimo: %v", err)
	}
	if _, err := Make(layout, Op{Kind: OpBoundary, Disk: "vdb", Part: 1, Other: 2, SizeMiB: 600}, info); !errors.Is(err, ErrTooSmall) {
		t.Fatalf("más que las dos juntas: %v", err)
	}
}

// * TestBoundaryNotAdjacent: con un hueco entre medias o una partición en medio no se mueve el límite.
func TestBoundaryNotAdjacent(t *testing.T) {
	layout := Build(disk(part(1, 100, 100, "ext4"), part(2, 250, 100, "ext4"), part(3, 350, 100, "ext4")))
	info := Info{1: 1 << 20, 2: 1 << 20, 3: 1 << 20}
	if _, err := Make(layout, Op{Kind: OpBoundary, Disk: "vdb", Part: 1, Other: 2, SizeMiB: 50}, info); !errors.Is(err, ErrNotAdjacent) {
		t.Fatalf("con hueco: %v", err)
	}
	if _, err := Make(layout, Op{Kind: OpBoundary, Disk: "vdb", Part: 1, Other: 3, SizeMiB: 50}, info); !errors.Is(err, ErrNotAdjacent) {
		t.Fatalf("no vecinas: %v", err)
	}
}

// * TestMerge junta dos vecinas conservando la izquierda o la derecha.
func TestMerge(t *testing.T) {
	layout := Build(disk(part(1, 100, 200, "ext4"), part(2, 300, 150, "ntfs")))
	// * Conservar la izquierda: se borra la derecha y la izquierda ocupa las dos.
	plan, err := Make(layout, Op{Kind: OpMerge, Disk: "vdb", Part: 1, Other: 2}, nil)
	if err != nil || !plan.Destructive || plan.MovesBytes != 0 {
		t.Fatalf("conservar la izquierda: %v %+v", err, plan)
	}
	v := newVDisk(t, layout)
	v.run(plan)
	if len(v.table) != 1 || v.table[1].Sectors != mib(350) || v.fs[1] != mib(350) {
		t.Fatalf("resultado: %+v %+v", v.table, v.fs)
	}
	// * Conservar la derecha: se borra la izquierda y los datos de la derecha se desplazan.
	layout = Build(disk(part(1, 100, 200, "ntfs"), part(2, 300, 150, "ext4")))
	plan, err = Make(layout, Op{Kind: OpMerge, Disk: "vdb", Part: 2, Other: 1}, nil)
	if err != nil || !plan.Destructive || plan.MovesBytes != mib(150)*SectorSize {
		t.Fatalf("conservar la derecha: %v %+v", err, plan)
	}
	v = newVDisk(t, layout)
	v.run(plan)
	if len(v.table) != 1 || v.table[2].Start != mib(100) || v.table[2].Sectors != mib(350) || v.fs[2] != mib(350) {
		t.Fatalf("resultado: %+v %+v", v.table, v.fs)
	}
	// * La que se conserva debe poder crecer.
	layout = Build(disk(part(1, 100, 200, "exfat"), part(2, 300, 150, "ext4")))
	if _, err := Make(layout, Op{Kind: OpMerge, Disk: "vdb", Part: 1, Other: 2}, nil); !errors.Is(err, ErrNotResizable) {
		t.Fatalf("conservar exFAT: %v", err)
	}
}

// * TestDeleteAndCreate borra una partición y crea otra en el hueco.
func TestDeleteAndCreate(t *testing.T) {
	layout := Build(disk(part(1, 100, 200, "ext4"), part(2, 300, 150, "ntfs")))
	plan, err := Make(layout, Op{Kind: OpDelete, Disk: "vdb", Part: 2}, nil)
	if err != nil || !plan.Destructive {
		t.Fatalf("borrar: %v %+v", err, plan)
	}
	v := newVDisk(t, layout)
	v.run(plan)
	layout2 := Build(disk(part(1, 100, 200, "ext4")))
	create := Op{Kind: OpCreate, Disk: "vdb", StartMiB: 300, SizeMiB: 200, FS: "exfat", Label: "DATOS"}
	plan, err = Make(layout2, create, nil)
	if err != nil || plan.Destructive || len(plan.Steps) != 2 || plan.Steps[1].Kind != StepMkfs {
		t.Fatalf("crear: %v %+v", err, plan)
	}
	if plan.Steps[0].Edits[0].Part != 2 {
		t.Fatalf("debería usar el primer número libre: %+v", plan.Steps[0].Edits[0])
	}
	newVDisk(t, layout2).run(plan)
	for name, bad := range map[string]Op{
		"fuera del hueco":    {Kind: OpCreate, Disk: "vdb", StartMiB: 250, SizeMiB: 100, FS: "ext4"},
		"no cabe":            {Kind: OpCreate, Disk: "vdb", StartMiB: 300, SizeMiB: 5000, FS: "ext4"},
		"fs desconocido":     {Kind: OpCreate, Disk: "vdb", StartMiB: 300, SizeMiB: 100, FS: "btrfs"},
		"etiqueta peligrosa": {Kind: OpCreate, Disk: "vdb", StartMiB: 300, SizeMiB: 100, FS: "ext4", Label: "a;rm"},
		"fat32 diminuta":     {Kind: OpCreate, Disk: "vdb", StartMiB: 300, SizeMiB: 10, FS: "fat32"},
	} {
		if _, err := Make(layout2, bad, nil); err == nil {
			t.Fatalf("%s: debería rechazarse", name)
		}
	}
}

// * TestMBRLimit: una tabla MBR no admite una quinta partición primaria.
func TestMBRLimit(t *testing.T) {
	d := disk(part(1, 10, 10, "ext4"), part(2, 30, 10, "ext4"), part(3, 50, 10, "ext4"), part(4, 70, 10, "ext4"))
	d.PartTable = "msdos"
	layout := Build(d)
	if _, err := Make(layout, Op{Kind: OpCreate, Disk: "vdb", StartMiB: 100, SizeMiB: 100, FS: "ext4"}, nil); !errors.Is(err, ErrNoSpace) {
		t.Fatalf("quinta primaria: %v", err)
	}
}

// * TestMontajesDeRebooteraNoBloquean comprueba que un montaje de solo lectura abierto por Rebootera (búsqueda de drivers) no
// * bloquea el disco, pero uno del usuario sí.
func TestMontajesDeRebooteraNoBloquean(t *testing.T) {
	own := disk(part(1, 1, 10, "ntfs"))
	own.Partitions[0].MountPoint = "/run/rebootera/media/vda1"
	if locked := Build(own).Locked; locked != "" {
		t.Fatalf("un montaje de Rebootera no debe bloquear el disco: %q", locked)
	}
	users := disk(part(1, 1, 10, "ntfs"))
	users.Partitions[0].MountPoint = "/mnt/datos"
	if Build(users).Locked != ErrMounted.Error() {
		t.Fatal("un montaje del usuario sí debe bloquear el disco")
	}
}

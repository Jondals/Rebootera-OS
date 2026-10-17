// * Package partman: el gestor de particiones, en su parte pura. A partir de
// * la distribución de un disco calcula los huecos libres y, para cada
// * operación que pide el usuario (cambiar el tamaño, mover el límite entre dos
// * particiones vecinas, fusionar, borrar o crear), el plan de pasos exactos que
// * ejecutará el agente. Aquí no se toca ningún disco: sirve igual para enseñar
// * al usuario lo que va a pasar (la vista previa) y para que el agente lo
// * ejecute, y se prueba entera con números.
// ! Todo se mide en sectores de 512 bytes y se alinea a 1 MiB. Un plan que no se
// ! puede hacer con seguridad se rechaza con un código, nunca se "intenta".
package partman

import (
	"errors"
	"slices"
	"strings"

	"rebootera/core/internal/storage"
)

// Medidas básicas.
const (
	SectorSize = 512
	// Align es la alineación de las particiones: 1 MiB en sectores.
	Align = 2048
	// gptTail son los sectores que reserva GPT al final del disco para su copia de la tabla.
	gptTail = 34
)

// Tipos de operación.
const (
	OpResize   = "resize"
	OpBoundary = "boundary"
	OpMerge    = "merge"
	OpDelete   = "delete"
	OpCreate   = "create"
)

// Tipos de paso del plan.
const (
	StepCheck    = "check"
	StepShrinkFS = "shrink-fs"
	StepGrowFS   = "grow-fs"
	StepMove     = "move"
	StepTable    = "table"
	StepMkfs     = "mkfs"
)

// Errores de planificación: el texto es un código estable que la interfaz traduce.
var (
	ErrReadOnly     = errors.New("read-only")
	ErrProtected    = errors.New("protected")
	ErrUnsupported  = errors.New("unsupported-table")
	ErrMounted      = errors.New("mounted")
	ErrNotFound     = errors.New("not-found")
	ErrNotResizable = errors.New("not-resizable")
	ErrNoChange     = errors.New("no-change")
	ErrTooSmall     = errors.New("too-small")
	ErrNoSpace      = errors.New("no-space")
	ErrNotAdjacent  = errors.New("not-adjacent")
	ErrBadRequest   = errors.New("bad-request")
	ErrNoInfo       = errors.New("no-info")
)

// Segment es un trozo continuo del disco: una partición (Number > 0) o un hueco libre.
type Segment struct {
	Number  int    `json:"number"`
	Name    string `json:"name,omitempty"`
	Start   uint64 `json:"start"`
	Sectors uint64 `json:"sectors"`
	FSType  string `json:"fsType,omitempty"`
	Label   string `json:"label,omitempty"`
	Role    string `json:"role,omitempty"`
	// Mounted es verdad si la partición está montada: no se puede tocar.
	Mounted   bool   `json:"mounted"`
	UsedBytes uint64 `json:"usedBytes"`
	// Resizable indica que su sistema de archivos se puede encoger y agrandar aquí.
	Resizable bool `json:"resizable"`
}

// End es el primer sector después del segmento.
func (s Segment) End() uint64 { return s.Start + s.Sectors }

// Free indica que el segmento es un hueco libre.
func (s Segment) Free() bool { return s.Number == 0 }

// Layout es la distribución completa de un disco.
type Layout struct {
	Disk         string    `json:"disk"`
	Table        string    `json:"table"`
	TotalSectors uint64    `json:"totalSectors"`
	First        uint64    `json:"first"`
	Limit        uint64    `json:"limit"`
	Segments     []Segment `json:"segments"`
	// Locked explica por qué el disco no se puede modificar ("" si se puede).
	Locked string `json:"locked,omitempty"`
}

// Op es lo que pide el usuario.
type Op struct {
	Kind string `json:"kind"`
	Disk string `json:"disk"`
	// Part es la partición sobre la que se actúa (en "boundary", la de la izquierda;
	// en "merge", la que se conserva).
	Part int `json:"part"`
	// Other es la vecina (en "boundary", la de la derecha; en "merge", la que se absorbe).
	Other int `json:"other"`
	// SizeMiB es el tamaño nuevo (en "boundary", el de la izquierda; en "create", el de la nueva).
	SizeMiB uint64 `json:"sizeMiB"`
	// StartMiB es dónde empieza la partición nueva ("create").
	StartMiB uint64 `json:"startMiB"`
	// FS y Label son el sistema de archivos y la etiqueta de la partición nueva.
	FS    string `json:"fs"`
	Label string `json:"label"`
}

// TableEdit es un cambio en la tabla de particiones.
type TableEdit struct {
	Part    int    `json:"part"`
	Start   uint64 `json:"start"`
	Sectors uint64 `json:"sectors"`
	Delete  bool   `json:"delete,omitempty"`
	Create  bool   `json:"create,omitempty"`
	// FS decide el tipo de partición que se crea.
	FS string `json:"fs,omitempty"`
}

// Step es un paso del plan.
type Step struct {
	Kind string `json:"kind"`
	Part int    `json:"part"`
	// Bytes es el tamaño nuevo del sistema de archivos (shrink-fs).
	Bytes uint64 `json:"bytes,omitempty"`
	// From, To y Length describen una copia de datos (move), en sectores.
	From   uint64 `json:"from,omitempty"`
	To     uint64 `json:"to,omitempty"`
	Length uint64 `json:"length,omitempty"`
	// Edits son los cambios de la tabla (table).
	Edits []TableEdit `json:"edits,omitempty"`
	// Start identifica la partición recién creada (mkfs): empieza en este sector.
	Start uint64 `json:"start,omitempty"`
	FS    string `json:"fs,omitempty"`
	Label string `json:"label,omitempty"`
}

// Plan es la lista de pasos de una operación con lo que el usuario debe saber de ella.
type Plan struct {
	Steps []Step `json:"steps"`
	// Destructive indica que la operación borra los datos de alguna partición.
	Destructive bool `json:"destructive"`
	// MovesBytes es cuántos datos se copian de un sitio a otro (lento y delicado).
	MovesBytes uint64 `json:"movesBytes"`
	// Affected lista las particiones que cambian, para el aviso de confirmación.
	Affected []int `json:"affected"`
}

// Info son los mínimos de los sistemas de archivos (bytes) que solo el agente puede calcular.
type Info map[int]uint64

// resizableFS son los sistemas de archivos que se pueden encoger y agrandar.
var resizableFS = []string{"ext2", "ext3", "ext4", "ntfs"}

// createFS son los sistemas de archivos con los que se puede crear una partición.
var createFS = []string{"ext4", "ntfs", "exfat", "fat32"}

// * IsResizable indica si un sistema de archivos se puede cambiar de tamaño sin perder datos.
func IsResizable(fs string) bool { return slices.Contains(resizableFS, fs) }

// * alignDown redondea hacia abajo a un múltiplo de Align.
func alignDown(sectors uint64) uint64 { return sectors / Align * Align }

// * alignUp redondea hacia arriba a un múltiplo de Align.
func alignUp(sectors uint64) uint64 { return (sectors + Align - 1) / Align * Align }

// * Build calcula la distribución de un disco: sus particiones y los huecos libres de al menos 1 MiB.
func Build(disk storage.Disk) Layout {
	layout := Layout{Disk: disk.Name, Table: disk.PartTable, TotalSectors: disk.Sectors, First: Align}
	if layout.Table == "msdos" {
		layout.Table = "dos"
	}
	layout.Limit = alignDown(disk.Sectors)
	if layout.Table == "gpt" {
		layout.Limit = alignDown(disk.Sectors - min(disk.Sectors, gptTail))
	}
	switch {
	case disk.ReadOnly:
		layout.Locked = ErrReadOnly.Error()
	// * El medio de arranque de Rebootera y los discos ópticos nunca se tocan; un disco interno sí
	// * se puede editar (es para lo que sirve), con confirmación escrita.
	case disk.Role == storage.RoleLiveMedia || disk.Role == storage.RoleOptical:
		layout.Locked = ErrProtected.Error()
	case layout.Table != "gpt" && layout.Table != "dos":
		layout.Locked = ErrUnsupported.Error()
	}
	// * Sin tabla de particiones conocida (un disco virgen o una ISO) no hay nada que dibujar.
	if layout.Table != "gpt" && layout.Table != "dos" {
		return layout
	}
	parts := slices.Clone(disk.Partitions)
	slices.SortFunc(parts, compareStart)
	cursor := layout.First
	for _, part := range parts {
		if part.StartSector > cursor && part.StartSector-cursor >= Align {
			layout.Segments = append(layout.Segments, Segment{Start: cursor, Sectors: part.StartSector - cursor})
		}
		layout.Segments = append(layout.Segments, segmentOf(part))
		cursor = max(cursor, part.StartSector+part.Sectors)
		if part.MountPoint != "" && part.MountPoint != "/run/live/medium" && layout.Locked == "" {
			layout.Locked = ErrMounted.Error()
		}
		// * Una tabla MBR con partición extendida o lógica no se edita: es otro mundo.
		if layout.Table == "dos" && (part.Number >= 5 || isExtendedType(part.TypeCode)) {
			layout.Locked = ErrUnsupported.Error()
		}
	}
	if layout.Limit > cursor && layout.Limit-cursor >= Align {
		layout.Segments = append(layout.Segments, Segment{Start: cursor, Sectors: layout.Limit - cursor})
	}
	return layout
}

// * isExtendedType reconoce los tipos de partición extendida de MBR.
func isExtendedType(code string) bool {
	return code == "0x5" || code == "0xf" || code == "0x85"
}

// * compareStart ordena las particiones por su sector de inicio.
func compareStart(a, b storage.Partition) int {
	switch {
	case a.StartSector < b.StartSector:
		return -1
	case a.StartSector > b.StartSector:
		return 1
	}
	return 0
}

// * segmentOf convierte una partición del inventario en un segmento.
func segmentOf(part storage.Partition) Segment {
	mounted := part.MountPoint != "" && part.MountPoint != "/run/live/medium"
	return Segment{
		Number:    part.Number,
		Name:      part.Name,
		Start:     part.StartSector,
		Sectors:   part.Sectors,
		FSType:    part.FSType,
		Label:     part.Label,
		Role:      part.Role,
		Mounted:   mounted,
		UsedBytes: part.UsedBytes,
		Resizable: IsResizable(part.FSType) && !mounted,
	}
}

// * find devuelve la partición con ese número.
func (l Layout) find(number int) (Segment, bool) {
	for _, s := range l.Segments {
		if s.Number == number && number > 0 {
			return s, true
		}
	}
	return Segment{}, false
}

// * freeAfter devuelve el hueco libre que empieza justo donde acaba la partición (0 si no hay).
func (l Layout) freeAfter(part Segment) uint64 {
	for _, s := range l.Segments {
		if s.Free() && s.Start == part.End() {
			return s.Sectors
		}
	}
	return 0
}

// * usable comprueba que el disco admite cambios.
func (l Layout) usable() error {
	switch l.Locked {
	case "":
		return nil
	case ErrReadOnly.Error():
		return ErrReadOnly
	case ErrProtected.Error():
		return ErrProtected
	case ErrMounted.Error():
		return ErrMounted
	}
	return ErrUnsupported
}

// * needIdle exige que la partición no esté montada.
func needIdle(s Segment) error {
	if s.Mounted {
		return ErrMounted
	}
	return nil
}

// * minSectors es el mínimo al que se puede encoger una partición, redondeado a 1 MiB.
func minSectors(number int, info Info) (uint64, error) {
	bytes, ok := info[number]
	if !ok || bytes == 0 {
		return 0, ErrNoInfo
	}
	return alignUp((bytes + SectorSize - 1) / SectorSize), nil
}

// * Make calcula el plan de una operación sobre una distribución. info trae los mínimos
// * de los sistemas de archivos (solo hacen falta para encoger).
func Make(layout Layout, op Op, info Info) (Plan, error) {
	if err := layout.usable(); err != nil {
		return Plan{}, err
	}
	if op.Disk != layout.Disk {
		return Plan{}, ErrBadRequest
	}
	switch op.Kind {
	case OpResize:
		return planResize(layout, op, info)
	case OpBoundary:
		return planBoundary(layout, op, info)
	case OpMerge:
		return planMerge(layout, op)
	case OpDelete:
		return planDelete(layout, op)
	case OpCreate:
		return planCreate(layout, op)
	}
	return Plan{}, ErrBadRequest
}

// * planResize cambia el tamaño de una partición por su final: encoge, o crece hacia el hueco libre que tiene detrás.
func planResize(layout Layout, op Op, info Info) (Plan, error) {
	part, ok := layout.find(op.Part)
	if !ok {
		return Plan{}, ErrNotFound
	}
	if err := needIdle(part); err != nil {
		return Plan{}, err
	}
	if !part.Resizable {
		return Plan{}, ErrNotResizable
	}
	want := op.SizeMiB * Align
	if want == 0 || want%Align != 0 {
		return Plan{}, ErrBadRequest
	}
	switch {
	case want == part.Sectors:
		return Plan{}, ErrNoChange
	case want < part.Sectors:
		floor, err := minSectors(part.Number, info)
		if err != nil {
			return Plan{}, err
		}
		if want < floor {
			return Plan{}, ErrTooSmall
		}
		edit := TableEdit{Part: part.Number, Start: part.Start, Sectors: want}
		return Plan{Affected: []int{part.Number}, Steps: []Step{
			{Kind: StepCheck, Part: part.Number},
			{Kind: StepShrinkFS, Part: part.Number, Bytes: want * SectorSize},
			{Kind: StepTable, Edits: []TableEdit{edit}},
		}}, nil
	}
	if want-part.Sectors > layout.freeAfter(part) {
		return Plan{}, ErrNoSpace
	}
	edit := TableEdit{Part: part.Number, Start: part.Start, Sectors: want}
	return Plan{Affected: []int{part.Number}, Steps: []Step{
		{Kind: StepCheck, Part: part.Number},
		{Kind: StepTable, Edits: []TableEdit{edit}},
		{Kind: StepGrowFS, Part: part.Number},
	}}, nil
}

// * neighbours devuelve las dos particiones vecinas (sin hueco entre ellas), la izquierda primero.
func neighbours(layout Layout, a, b int) (Segment, Segment, error) {
	left, ok1 := layout.find(a)
	right, ok2 := layout.find(b)
	if !ok1 || !ok2 {
		return Segment{}, Segment{}, ErrNotFound
	}
	if left.Start > right.Start {
		left, right = right, left
	}
	if left.End() != right.Start {
		return Segment{}, Segment{}, ErrNotAdjacent
	}
	return left, right, nil
}

// * planBoundary mueve el límite entre dos particiones vecinas: una gana el espacio que pierde la otra.
// * El límite solo se puede mover cambiando dónde EMPIEZA la partición de la derecha, así que sus datos
// * se copian a su sitio nuevo.
func planBoundary(layout Layout, op Op, info Info) (Plan, error) {
	left, right, err := neighbours(layout, op.Part, op.Other)
	if err != nil {
		return Plan{}, err
	}
	for _, s := range []Segment{left, right} {
		if err := needIdle(s); err != nil {
			return Plan{}, err
		}
		if !s.Resizable {
			return Plan{}, ErrNotResizable
		}
	}
	want := op.SizeMiB * Align
	if want == 0 || want%Align != 0 {
		return Plan{}, ErrBadRequest
	}
	if want == left.Sectors {
		return Plan{}, ErrNoChange
	}
	total := left.Sectors + right.Sectors
	if want >= total {
		return Plan{}, ErrTooSmall
	}
	if want > left.Sectors {
		return boundaryGrowLeft(left, right, want, info)
	}
	return boundaryShrinkLeft(left, right, want, info)
}

// * boundaryGrowLeft: la izquierda crece y la derecha encoge por su principio y se desplaza a la derecha.
func boundaryGrowLeft(left, right Segment, want uint64, info Info) (Plan, error) {
	delta := want - left.Sectors
	floor, err := minSectors(right.Number, info)
	if err != nil {
		return Plan{}, err
	}
	if right.Sectors-delta < floor {
		return Plan{}, ErrTooSmall
	}
	newRight := right.Sectors - delta
	return Plan{
		Affected:   []int{left.Number, right.Number},
		MovesBytes: newRight * SectorSize,
		Steps: []Step{
			{Kind: StepCheck, Part: left.Number},
			{Kind: StepCheck, Part: right.Number},
			{Kind: StepShrinkFS, Part: right.Number, Bytes: newRight * SectorSize},
			{Kind: StepMove, Part: right.Number, From: right.Start, To: right.Start + delta, Length: newRight},
			{Kind: StepTable, Edits: []TableEdit{
				{Part: left.Number, Start: left.Start, Sectors: want},
				{Part: right.Number, Start: right.Start + delta, Sectors: newRight},
			}},
			{Kind: StepGrowFS, Part: left.Number},
		},
	}, nil
}

// * boundaryShrinkLeft: la izquierda encoge y la derecha se desplaza a la izquierda y crece hasta su final de siempre.
func boundaryShrinkLeft(left, right Segment, want uint64, info Info) (Plan, error) {
	delta := left.Sectors - want
	floor, err := minSectors(left.Number, info)
	if err != nil {
		return Plan{}, err
	}
	if want < floor {
		return Plan{}, ErrTooSmall
	}
	return Plan{
		Affected:   []int{left.Number, right.Number},
		MovesBytes: right.Sectors * SectorSize,
		Steps: []Step{
			{Kind: StepCheck, Part: left.Number},
			{Kind: StepCheck, Part: right.Number},
			{Kind: StepShrinkFS, Part: left.Number, Bytes: want * SectorSize},
			{Kind: StepMove, Part: right.Number, From: right.Start, To: right.Start - delta, Length: right.Sectors},
			{Kind: StepTable, Edits: []TableEdit{
				{Part: left.Number, Start: left.Start, Sectors: want},
				{Part: right.Number, Start: right.Start - delta, Sectors: right.Sectors + delta},
			}},
			{Kind: StepGrowFS, Part: right.Number},
		},
	}, nil
}

// * planMerge junta dos particiones vecinas en la que se conserva; los datos de la otra se pierden.
func planMerge(layout Layout, op Op) (Plan, error) {
	left, right, err := neighbours(layout, op.Part, op.Other)
	if err != nil {
		return Plan{}, err
	}
	keep, gone := left, right
	if op.Part == right.Number {
		keep, gone = right, left
	}
	if keep.Number == gone.Number {
		return Plan{}, ErrBadRequest
	}
	if err := needIdle(keep); err != nil {
		return Plan{}, err
	}
	if err := needIdle(gone); err != nil {
		return Plan{}, err
	}
	if !keep.Resizable {
		return Plan{}, ErrNotResizable
	}
	joined := keep.Sectors + gone.Sectors
	plan := Plan{Destructive: true, Affected: []int{keep.Number, gone.Number}}
	if keep.Start < gone.Start {
		plan.Steps = []Step{
			{Kind: StepCheck, Part: keep.Number},
			{Kind: StepTable, Edits: []TableEdit{
				{Part: gone.Number, Delete: true},
				{Part: keep.Number, Start: keep.Start, Sectors: joined},
			}},
			{Kind: StepGrowFS, Part: keep.Number},
		}
		return plan, nil
	}
	// * La que se conserva está a la derecha: se borra la de la izquierda y sus datos se desplazan a su sitio.
	plan.MovesBytes = keep.Sectors * SectorSize
	plan.Steps = []Step{
		{Kind: StepCheck, Part: keep.Number},
		{Kind: StepTable, Edits: []TableEdit{{Part: gone.Number, Delete: true}}},
		{Kind: StepMove, Part: keep.Number, From: keep.Start, To: gone.Start, Length: keep.Sectors},
		{Kind: StepTable, Edits: []TableEdit{{Part: keep.Number, Start: gone.Start, Sectors: joined}}},
		{Kind: StepGrowFS, Part: keep.Number},
	}
	return plan, nil
}

// * planDelete borra una partición.
func planDelete(layout Layout, op Op) (Plan, error) {
	part, ok := layout.find(op.Part)
	if !ok {
		return Plan{}, ErrNotFound
	}
	if err := needIdle(part); err != nil {
		return Plan{}, err
	}
	return Plan{Destructive: true, Affected: []int{part.Number}, Steps: []Step{
		{Kind: StepTable, Edits: []TableEdit{{Part: part.Number, Delete: true}}},
	}}, nil
}

// * planCreate crea una partición nueva dentro de un hueco libre y la formatea.
func planCreate(layout Layout, op Op) (Plan, error) {
	if !slices.Contains(createFS, op.FS) {
		return Plan{}, ErrBadRequest
	}
	label := strings.TrimSpace(op.Label)
	if len(label) > 11 || strings.ContainsAny(label, "\"'\\`$;&|<>\n\r\x00/") {
		return Plan{}, ErrBadRequest
	}
	start, size := op.StartMiB*Align, op.SizeMiB*Align
	if size == 0 || start < layout.First {
		return Plan{}, ErrBadRequest
	}
	if op.FS == "fat32" && size < 66*Align {
		return Plan{}, ErrTooSmall
	}
	inside := false
	for _, s := range layout.Segments {
		if s.Free() && start >= s.Start && start+size <= s.End() {
			inside = true
		}
	}
	if !inside {
		return Plan{}, ErrNoSpace
	}
	number := nextNumber(layout)
	if layout.Table == "dos" && number > 4 {
		return Plan{}, ErrNoSpace
	}
	return Plan{Destructive: false, Affected: []int{number}, Steps: []Step{
		{Kind: StepTable, Edits: []TableEdit{{Part: number, Start: start, Sectors: size, Create: true, FS: op.FS}}},
		{Kind: StepMkfs, Part: number, Start: start, FS: op.FS, Label: label},
	}}, nil
}

// * nextNumber devuelve el primer número de partición libre.
func nextNumber(layout Layout) int {
	used := map[int]bool{}
	for _, s := range layout.Segments {
		used[s.Number] = true
	}
	for n := 1; ; n++ {
		if !used[n] {
			return n
		}
	}
}

//go:build linux

// * Gestor de particiones del agente: ejecuta, paso a paso, el plan que calcula
// * partman (encoger y agrandar sistemas de archivos, copiar datos de un sitio
// * a otro, cambiar la tabla de particiones, formatear). Es lo único de
// * Rebootera que escribe en discos que no son suyos, así que:
// !   - solo acepta discos con nombre de dispositivo conocido, nunca el medio de
// !     arranque de Rebootera ni un disco óptico, y ninguno con particiones montadas;
// !   - exige el texto "CHANGE <disco>" que escribió el usuario;
// !   - no se fía de Core: lee de nuevo la distribución del disco y vuelve a calcular
// !     el plan con partman antes de tocar nada;
// !   - hace una sola operación a la vez y las deja en el registro.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"rebootera/core/internal/partman"
	"rebootera/core/internal/storage"
)

// diskPattern son los nombres de disco que se aceptan (los que usa el kernel para discos reales).
var diskPattern = regexp.MustCompile(`^(sd[a-z]{1,2}|vd[a-z]{1,2}|xvd[a-z]{1,2}|nvme[0-9]{1,2}n[0-9]{1,2}|mmcblk[0-9]{1,2})$`)

// copyChunk es el tamaño de cada trozo al copiar datos de una partición a otra.
const copyChunk = 4 << 20

// partStatus es lo que se cuenta de la operación en curso.
type partStatus struct {
	Running  bool    `json:"running"`
	Done     bool    `json:"done"`
	Error    string  `json:"error,omitempty"`
	Disk     string  `json:"disk"`
	Step     int     `json:"step"`
	Total    int     `json:"total"`
	Kind     string  `json:"kind"`
	Part     int     `json:"part"`
	Progress float64 `json:"progress"`
	Seconds  int     `json:"seconds"`
	started  time.Time
}

var (
	partMu  sync.Mutex
	partJob partStatus
	// resized recuerda los volúmenes NTFS que este agente ha redimensionado desde que arrancó: ntfsresize
	// los deja marcados para que Windows los revise y se niega a tocarlos otra vez sin --force. Se fuerza
	// solo en esos (la marca es nuestra); un volumen sucio por Windows (apagado sin cerrar o hibernado)
	// sigue rechazándose y el usuario debe iniciarlo y apagarlo del todo.
	resized = map[string]bool{}
)

// * jobStatus devuelve una copia del estado con el tiempo transcurrido.
func jobStatus() partStatus {
	partMu.Lock()
	defer partMu.Unlock()
	status := partJob
	if status.Running {
		status.Seconds = int(time.Since(status.started).Seconds())
	}
	return status
}

// * partStatusLines devuelve el estado como una línea JSON.
func partStatusLines() ([]string, error) {
	data, err := json.Marshal(jobStatus())
	if err != nil {
		return nil, err
	}
	return []string{string(data)}, nil
}

// * partDevice devuelve la ruta de una partición ("sdb" y 1 dan /dev/sdb1; "nvme0n1" y 1, /dev/nvme0n1p1).
func partDevice(disk string, number int) string {
	if disk != "" && disk[len(disk)-1] >= '0' && disk[len(disk)-1] <= '9' {
		return "/dev/" + disk + "p" + strconv.Itoa(number)
	}
	return "/dev/" + disk + strconv.Itoa(number)
}

// * loadDisk lee el estado actual de un disco y comprueba que se puede tocar.
func loadDisk(name string) (partman.Layout, storage.Disk, error) {
	if !diskPattern.MatchString(name) {
		return partman.Layout{}, storage.Disk{}, fmt.Errorf("disco no admitido: %q", name)
	}
	for _, disk := range storage.List() {
		if disk.Name == name {
			return partman.Build(disk), disk, nil
		}
	}
	return partman.Layout{}, storage.Disk{}, fmt.Errorf("no existe el disco %s", name)
}

// * closeMedia cierra los montajes de solo lectura que abrió Rebootera en los discos que se van a medir o cambiar (la
// * búsqueda de drivers deja abierta cualquier partición con Windows o carpetas de drivers): ntfsresize, resize2fs y sfdisk
// * se niegan a trabajar con una partición en uso. Los montajes de otro tipo (los del usuario) no se tocan: bloquean el disco.
func closeMedia(disk storage.Disk) {
	for _, part := range disk.Partitions {
		if strings.HasPrefix(part.MountPoint, mediaRoot+"/") {
			if err := unmountMedia("/dev/" + part.Name); err != nil {
				log.Printf("particiones: no se pudo cerrar %s: %v", part.Name, err)
			}
		}
	}
}

// * partInfo devuelve el tamaño mínimo (bytes) al que se puede encoger el sistema de archivos de una partición.
func partInfo(diskName string, number int) ([]string, error) {
	layout, disk, err := loadDisk(diskName)
	if err != nil {
		return nil, err
	}
	if layout.Locked != "" {
		return nil, errors.New(layout.Locked)
	}
	closeMedia(disk)
	for _, part := range disk.Partitions {
		if part.Number != number {
			continue
		}
		min, err := fsMinimum(partDevice(diskName, number), part.FSType)
		if err != nil {
			return nil, err
		}
		return []string{"min:" + strconv.FormatUint(min, 10), "fs:" + part.FSType}, nil
	}
	return nil, errors.New("esa partición no existe")
}

// * margin es lo que se suma al mínimo que calcula cada herramienta, por prudencia.
const margin = 16 << 20

var (
	ext4MinPattern = regexp.MustCompile(`Estimated minimum size of the filesystem:\s*(\d+)`)
	ext4BlockSize  = regexp.MustCompile(`Block size:\s*(\d+)`)
	ntfsMinPattern = regexp.MustCompile(`You might resize at (\d+) bytes`)
)

// * fsMinimum calcula el mínimo de un sistema de archivos sin modificarlo.
func fsMinimum(device, fs string) (uint64, error) {
	switch fs {
	case "ext2", "ext3", "ext4":
		// * e2fsck -fn solo mira: deja el sistema de archivos como estaba.
		exec.Command("e2fsck", "-fn", device).Run()
		out, err := exec.Command("resize2fs", "-P", device).CombinedOutput()
		if err != nil {
			return 0, fmt.Errorf("resize2fs -P: %v: %s", err, lastLine(out))
		}
		blocks := ext4MinPattern.FindSubmatch(out)
		if blocks == nil {
			return 0, errors.New("no se pudo leer el mínimo de ext4")
		}
		head, _ := exec.Command("tune2fs", "-l", device).Output()
		size := ext4BlockSize.FindSubmatch(head)
		if size == nil {
			return 0, errors.New("no se pudo leer el tamaño de bloque de ext4")
		}
		count, _ := strconv.ParseUint(string(blocks[1]), 10, 64)
		block, _ := strconv.ParseUint(string(size[1]), 10, 64)
		return count*block + margin, nil
	case "ntfs":
		out, err := exec.Command("ntfsresize", "--info", "--force", device).CombinedOutput()
		found := ntfsMinPattern.FindSubmatch(out)
		if found == nil {
			if hint := ntfsHint(out); hint != "" {
				return 0, errors.New(hint)
			}
			if err != nil {
				return 0, fmt.Errorf("ntfsresize --info: %s", lastLine(out))
			}
			return 0, errors.New("no se pudo leer el mínimo de NTFS")
		}
		size, _ := strconv.ParseUint(string(found[1]), 10, 64)
		return size + margin, nil
	}
	return 0, fmt.Errorf("no se puede cambiar el tamaño de %q", fs)
}

// * ntfsHint traduce los motivos habituales por los que ntfsresize se niega a medir un volumen de Windows a
// * algo que el usuario pueda arreglar: casi siempre es el inicio rápido (hibernación) o un chkdsk pendiente.
func ntfsHint(out []byte) string {
	text := strings.ToLower(string(out))
	switch {
	case strings.Contains(text, "hibernat") || strings.Contains(text, "fast restart") || strings.Contains(text, "fast start"):
		return "Windows dejó este disco hibernado (inicio rápido). Arranca Windows, desactiva el inicio rápido y apágalo del todo (Mayús + Apagar); después vuelve aquí."
	case strings.Contains(text, "inconsistent") || strings.Contains(text, "dirty") || strings.Contains(text, "scheduled for check") || strings.Contains(text, "chkdsk"):
		return "El volumen NTFS tiene errores pendientes. Arranca Windows y ejecuta «chkdsk /f» en esa unidad; después vuelve aquí."
	case strings.Contains(text, "bitlocker") || strings.Contains(text, "encrypted"):
		return "Esta unidad está cifrada con BitLocker: descífrala desde Windows antes de cambiar su tamaño."
	}
	return ""
}

// * lastLine devuelve la última línea no vacía de la salida de una herramienta.
func lastLine(out []byte) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// * partApply valida una operación y la ejecuta en segundo plano.
func partApply(op partman.Op, confirm string) ([]string, error) {
	if confirm != "CHANGE "+op.Disk {
		return nil, errors.New("falta la confirmación escrita del disco que se va a modificar")
	}
	layout, disk, err := loadDisk(op.Disk)
	if err != nil {
		return nil, err
	}
	closeMedia(disk)
	// * Los mínimos solo hacen falta si algo se encoge.
	info := partman.Info{}
	for _, number := range minimumsNeeded(op) {
		for _, seg := range layout.Segments {
			if seg.Number == number {
				if min, err := fsMinimum(partDevice(op.Disk, number), seg.FSType); err == nil {
					info[number] = min
				}
			}
		}
	}
	plan, err := partman.Make(layout, op, info)
	if err != nil {
		return nil, err
	}
	// ! Antes de tocar nada se guarda una copia de la tabla de particiones; sin copia no se hace nada.
	backup, err := backupTable(op.Disk)
	if err != nil {
		return nil, fmt.Errorf("no se pudo guardar una copia de la tabla de particiones: %w", err)
	}
	partMu.Lock()
	if partJob.Running {
		partMu.Unlock()
		return nil, errors.New("ya hay una operación de particiones en marcha")
	}
	partJob = partStatus{Running: true, Disk: op.Disk, Total: len(plan.Steps), started: time.Now()}
	partMu.Unlock()
	log.Printf("particiones: %s sobre %s con %d pasos (destructiva: %v, copia %d MB)", op.Kind, op.Disk, len(plan.Steps), plan.Destructive, plan.MovesBytes>>20)
	run := &partRun{disk: op.Disk, backup: backup}
	go run.execute(plan)
	return nil, nil
}

// * minimumsNeeded devuelve las particiones cuyo mínimo hay que conocer para planificar la operación.
func minimumsNeeded(op partman.Op) []int {
	switch op.Kind {
	case partman.OpResize:
		return []int{op.Part}
	case partman.OpBoundary:
		return []int{op.Part, op.Other}
	case partman.OpMerge:
		return []int{op.Part}
	}
	return nil
}

// partRun ejecuta los pasos de un plan sobre un disco.
type partRun struct {
	disk string
	// backup es la copia de la tabla de particiones de antes de empezar (volcado de sfdisk).
	backup string
	// moved dice si ya se empezó a copiar datos de una partición: desde ahí volver atrás ya no es seguro.
	moved bool
}

// * execute corre todos los pasos y deja el resultado en el estado.
func (r *partRun) execute(plan partman.Plan) {
	for i, step := range plan.Steps {
		partMu.Lock()
		partJob.Step, partJob.Kind, partJob.Part, partJob.Progress = i, step.Kind, step.Part, 0
		partMu.Unlock()
		if step.Kind == partman.StepMove {
			r.moved = true
		}
		if err := r.step(step); err != nil {
			log.Printf("particiones: el paso %d (%s) falló: %v", i, step.Kind, err)
			r.finish(r.rollback(err))
			return
		}
	}
	r.finish(nil)
}

// * rollback devuelve la tabla de particiones a como estaba si el fallo llegó antes de mover datos: así una operación que se
// * cancela (o falla) a medias no deja el disco con la tabla cambiada. Si ya se copiaban datos no se toca nada más: la
// * copia de la tabla queda guardada para restaurarla a mano.
func (r *partRun) rollback(cause error) error {
	if r.moved {
		return fmt.Errorf("%w (la copia de la tabla anterior está en %s)", cause, r.backup)
	}
	file, err := os.Open(r.backup)
	if err != nil {
		return cause
	}
	defer file.Close()
	cmd := exec.Command("sfdisk", "--no-reread", "--no-tell-kernel", "/dev/"+r.disk)
	cmd.Stdin = file
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("particiones: no se pudo restaurar la tabla de %s: %v: %s", r.disk, err, lastLine(out))
		return fmt.Errorf("%w (no se pudo restaurar la tabla anterior: %s)", cause, r.backup)
	}
	log.Printf("particiones: tabla de %s restaurada tras el fallo", r.disk)
	return cause
}

// * backupTable guarda un volcado de la tabla de particiones de un disco en /run (RAM) y, si hay Vault, también en el USB.
// * Devuelve la ruta de la copia en /run.
func backupTable(disk string) (string, error) {
	dump, err := exec.Command("sfdisk", "--dump", "/dev/"+disk).Output()
	if err != nil {
		return "", err
	}
	name := "partition-backup-" + disk + "-" + time.Now().Format("20060102-150405") + ".sfdisk"
	if err := os.MkdirAll("/run/rebootera", 0o755); err != nil {
		return "", err
	}
	path := "/run/rebootera/" + name
	if err := os.WriteFile(path, dump, 0o600); err != nil {
		return "", err
	}
	// * En el USB la copia sobrevive a un apagón o a un reinicio en mitad de la operación (si el USB no se toca).
	if info, err := os.Stat(vaultMount + "/rebootera/state"); err == nil && info.IsDir() {
		os.WriteFile(vaultMount+"/rebootera/state/"+name, dump, 0o600)
	}
	return path, nil
}

// * finish deja el estado final y relee la tabla de particiones.
func (r *partRun) finish(err error) {
	exec.Command("partprobe", "/dev/"+r.disk).Run()
	exec.Command("udevadm", "settle").Run()
	partMu.Lock()
	defer partMu.Unlock()
	partJob.Running = false
	if err != nil {
		partJob.Error = err.Error()
		return
	}
	partJob.Done = true
	partJob.Progress = 1
	log.Printf("particiones: %s terminado", r.disk)
}

// * progress anota el avance del paso en curso.
func (r *partRun) progress(fraction float64) {
	partMu.Lock()
	partJob.Progress = fraction
	partMu.Unlock()
}

// * step ejecuta un paso.
func (r *partRun) step(step partman.Step) error {
	switch step.Kind {
	case partman.StepCheck:
		return r.check(step.Part)
	case partman.StepShrinkFS:
		return r.shrinkFS(step.Part, step.Bytes)
	case partman.StepGrowFS:
		return r.growFS(step.Part)
	case partman.StepMove:
		return r.move(step)
	case partman.StepTable:
		return r.table(step.Edits)
	case partman.StepMkfs:
		return r.mkfs(step)
	case partman.StepLabel:
		return r.relabel(step)
	}
	return fmt.Errorf("paso desconocido: %q", step.Kind)
}

// * fsOf devuelve el sistema de archivos actual de una partición.
func (r *partRun) fsOf(number int) (string, error) {
	out, err := exec.Command("blkid", "-o", "value", "-s", "TYPE", partDevice(r.disk, number)).Output()
	if err != nil {
		return "", fmt.Errorf("no se pudo leer el sistema de archivos de la partición %d", number)
	}
	return strings.TrimSpace(string(out)), nil
}

// * tool ejecuta una herramienta y devuelve su error con lo último que dijo.
func (r *partRun) tool(stdin string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", explainTool(name, out, err))
	}
	return nil
}

// * explainTool convierte el fallo de una herramienta de particiones en una frase útil: primero la causa que el
// * usuario puede arreglar (si se reconoce) y, entre paréntesis, lo último que dijo la herramienta.
func explainTool(name string, out []byte, err error) string {
	detail := lastLines(out, 3)
	text := strings.ToLower(string(out))
	cause := ntfsHint(out)
	switch {
	case cause != "":
	case strings.Contains(text, "device or resource busy") || strings.Contains(text, "is mounted") || strings.Contains(text, "in use"):
		cause = "La partición está en uso (montada o abierta por otro programa). Cierra las unidades abiertas y vuelve a intentarlo."
	case strings.Contains(text, "no space left") || strings.Contains(text, "not enough space") || strings.Contains(text, "too small"):
		cause = "No hay espacio suficiente para ese cambio: libera archivos de la partición o elige un tamaño mayor."
	case strings.Contains(text, "read-only") || strings.Contains(text, "write-protected"):
		cause = "El disco está protegido contra escritura."
	case strings.Contains(text, "input/output error") || strings.Contains(text, "i/o error") || strings.Contains(text, "bad sector"):
		cause = "El disco da errores de lectura o escritura: puede estar fallando. Copia tus datos antes de seguir."
	case name == "e2fsck" || strings.Contains(text, "run e2fsck") || strings.Contains(text, "filesystem has errors") || strings.Contains(text, "unclean"):
		cause = "El sistema de archivos tiene errores y no se puede tocar hasta repararlo (desde Linux con «e2fsck -f» o desde Windows con «chkdsk /f»)."
	case strings.Contains(text, "permission denied") || strings.Contains(text, "operation not permitted"):
		cause = "El sistema no dejó escribir en el disco (permiso denegado)."
	default:
		cause = "La herramienta de particiones se detuvo sin explicar el motivo."
	}
	return fmt.Sprintf("%s (%s: %v: %s)", cause, name, err, detail)
}

// * lastLines devuelve las últimas n líneas no vacías de la salida de una herramienta, en una sola línea.
func lastLines(out []byte, n int) string {
	var kept []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			kept = append(kept, line)
		}
	}
	if len(kept) > n {
		kept = kept[len(kept)-n:]
	}
	return strings.Join(kept, " | ")
}

// * check revisa el sistema de archivos antes de tocarlo: no se cambia de tamaño uno dañado.
func (r *partRun) check(number int) error {
	fs, err := r.fsOf(number)
	if err != nil {
		return err
	}
	device := partDevice(r.disk, number)
	switch fs {
	case "ext2", "ext3", "ext4":
		// * e2fsck sale con 0 (limpio) o 1 (corrigió algo); más es un error.
		cmd := exec.Command("e2fsck", "-f", "-p", device)
		out, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() > 1 {
			return fmt.Errorf("%s", explainTool("e2fsck", out, err))
		}
		return nil
	case "ntfs":
		return r.tool("", "ntfsresize", "--check", device)
	}
	return fmt.Errorf("no se puede comprobar %q", fs)
}

// * shrinkFS encoge el sistema de archivos de una partición a ese tamaño.
func (r *partRun) shrinkFS(number int, bytes uint64) error {
	fs, err := r.fsOf(number)
	if err != nil {
		return err
	}
	device := partDevice(r.disk, number)
	switch fs {
	case "ext2", "ext3", "ext4":
		return r.tool("", "resize2fs", device, strconv.FormatUint(bytes/1024, 10)+"K")
	case "ntfs":
		// * ntfsresize pregunta antes de escribir; se le contesta que sí.
		return r.ntfsResize(number, "--size", strconv.FormatUint(bytes, 10))
	}
	return fmt.Errorf("no se puede encoger %q", fs)
}

// * growFS agranda el sistema de archivos hasta ocupar toda su partición.
func (r *partRun) growFS(number int) error {
	fs, err := r.fsOf(number)
	if err != nil {
		return err
	}
	device := partDevice(r.disk, number)
	switch fs {
	case "ext2", "ext3", "ext4":
		if err := r.check(number); err != nil {
			return err
		}
		return r.tool("", "resize2fs", device)
	case "ntfs":
		return r.ntfsResize(number)
	}
	return fmt.Errorf("no se puede agrandar %q", fs)
}

// * ntfsResize ejecuta ntfsresize sobre una partición, con --force solo si la marca de revisión la dejó este agente.
func (r *partRun) ntfsResize(number int, extra ...string) error {
	device := partDevice(r.disk, number)
	args := append([]string{}, extra...)
	partMu.Lock()
	ours := resized[device]
	partMu.Unlock()
	if ours {
		args = append(args, "--force")
	}
	if err := r.tool("y\n", "ntfsresize", append(args, device)...); err != nil {
		return err
	}
	partMu.Lock()
	resized[device] = true
	partMu.Unlock()
	return nil
}

// * move copia los datos de una partición a su sitio nuevo. Hacia la izquierda se copia de principio a
// * fin y hacia la derecha de fin a principio, así nunca se pisa lo que aún no se ha copiado.
// ! Si se corta la luz a mitad, la partición queda dañada: por eso la interfaz lo avisa antes.
func (r *partRun) move(step partman.Step) error {
	exec.Command("blockdev", "--flushbufs", "/dev/"+r.disk).Run()
	disk, err := os.OpenFile("/dev/"+r.disk, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer disk.Close()
	total := step.Length * partman.SectorSize
	from, to := int64(step.From*partman.SectorSize), int64(step.To*partman.SectorSize)
	buffer := make([]byte, copyChunk)
	left := to < from
	var done uint64
	for done < total {
		n := uint64(copyChunk)
		if total-done < n {
			n = total - done
		}
		offset := done
		if !left {
			// * Hacia la derecha se empieza por el final.
			offset = total - done - n
		}
		chunk := buffer[:n]
		if _, err := disk.ReadAt(chunk, from+int64(offset)); err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("leyendo el disco: %w", err)
		}
		if _, err := disk.WriteAt(chunk, to+int64(offset)); err != nil {
			return fmt.Errorf("escribiendo el disco: %w", err)
		}
		done += n
		r.progress(float64(done) / float64(total))
	}
	if err := disk.Sync(); err != nil {
		return err
	}
	exec.Command("blockdev", "--flushbufs", "/dev/"+r.disk).Run()
	// * El núcleo puede conservar en memoria lo que leyó de las particiones antes de moverlas.
	os.WriteFile("/proc/sys/vm/drop_caches", []byte("1"), 0o200)
	return nil
}

var (
	dumpLine  = regexp.MustCompile(`^(/dev/\S+?)(\d+)\s*:\s*(.*)$`)
	startAttr = regexp.MustCompile(`start=\s*\d+`)
	typeAttr  = regexp.MustCompile(`type=\s*[0-9A-Za-z-]+`)
	sizeAttr  = regexp.MustCompile(`size=\s*\d+`)
)

// * partitionType es el tipo de partición que se escribe al crear una según la tabla y el sistema de archivos.
func partitionType(table, fs string) string {
	if table == "gpt" {
		if fs == "ext4" {
			return "0FC63DAF-8483-4772-8E79-3D69D8477DE4"
		}
		return "EBD0A0A2-B9E5-4433-87C0-68B6B72699C7"
	}
	switch fs {
	case "ext4":
		return "83"
	case "fat32":
		return "c"
	}
	return "7"
}

// * table cambia la tabla de particiones leyéndola con sfdisk --dump, modificando solo lo pedido y
// * escribiéndola de nuevo: las demás particiones conservan sus identificadores y atributos.
func (r *partRun) table(edits []partman.TableEdit) error {
	out, err := exec.Command("sfdisk", "--dump", "/dev/"+r.disk).Output()
	if err != nil {
		return fmt.Errorf("sfdisk --dump: %v", err)
	}
	table := "dos"
	var script []string
	seen := map[int]bool{}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if strings.HasPrefix(line, "label: gpt") {
			table = "gpt"
		}
		match := dumpLine.FindStringSubmatch(line)
		if match == nil {
			script = append(script, line)
			continue
		}
		number, _ := strconv.Atoi(match[2])
		seen[number] = true
		edit, found := findEdit(edits, number)
		switch {
		case found && edit.Delete:
			continue
		case found && edit.Retype:
			script = append(script, match[1]+match[2]+" : "+typeAttr.ReplaceAllString(match[3], "type="+partitionType(table, edit.FS)))
		case found && !edit.Create:
			rest := startAttr.ReplaceAllString(match[3], "start="+strconv.FormatUint(edit.Start, 10))
			rest = sizeAttr.ReplaceAllString(rest, "size="+strconv.FormatUint(edit.Sectors, 10))
			script = append(script, match[1]+match[2]+" : "+rest)
		default:
			script = append(script, line)
		}
	}
	for _, edit := range edits {
		if edit.Create {
			if seen[edit.Part] {
				return fmt.Errorf("la partición %d ya existe", edit.Part)
			}
			script = append(script, fmt.Sprintf("%s : start=%d, size=%d, type=%s", partDevice(r.disk, edit.Part), edit.Start, edit.Sectors, partitionType(table, edit.FS)))
		}
	}
	cmd := exec.Command("sfdisk", "--no-reread", "--no-tell-kernel", "/dev/"+r.disk)
	cmd.Stdin = bytes.NewReader([]byte(strings.Join(script, "\n") + "\n"))
	if combined, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sfdisk: %v: %s", err, lastLine(combined))
	}
	exec.Command("blockdev", "--flushbufs", "/dev/"+r.disk).Run()
	if err := exec.Command("partprobe", "/dev/"+r.disk).Run(); err != nil {
		return fmt.Errorf("el núcleo no pudo releer la tabla: %v", err)
	}
	return exec.Command("udevadm", "settle").Run()
}

// * findEdit busca el cambio de una partición.
func findEdit(edits []partman.TableEdit, number int) (partman.TableEdit, bool) {
	for _, edit := range edits {
		if edit.Part == number {
			return edit, true
		}
	}
	return partman.TableEdit{}, false
}

// * relabel cambia la etiqueta de una partición con la herramienta de su sistema de archivos.
func (r *partRun) relabel(step partman.Step) error {
	fs, err := r.fsOf(step.Part)
	if err != nil {
		return err
	}
	device := partDevice(r.disk, step.Part)
	switch fs {
	case "ext2", "ext3", "ext4":
		return r.tool("", "e2label", device, step.Label)
	case "ntfs":
		return r.tool("", "ntfslabel", "--force", device, step.Label)
	case "exfat":
		return r.tool("", "exfatlabel", device, step.Label)
	case "vfat":
		return r.tool("", "fatlabel", device, strings.ToUpper(step.Label))
	}
	return fmt.Errorf("no se puede cambiar el nombre de un sistema de archivos %q", fs)
}

// * mkfs formatea la partición recién creada (la que empieza en el sector pedido).
func (r *partRun) mkfs(step partman.Step) error {
	device := partDevice(r.disk, step.Part)
	for i := 0; i < 20; i++ {
		if _, err := os.Stat(device); err == nil {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	// * Una partición que ya tenía datos puede conservar firmas del sistema de archivos anterior: se limpian primero.
	if err := r.tool("", "wipefs", "-a", device); err != nil {
		return err
	}
	label := step.Label
	switch step.FS {
	case "ext4":
		return r.tool("", "mkfs.ext4", "-F", "-q", "-L", label, device)
	case "ntfs":
		return r.tool("", "mkntfs", "-F", "-Q", "-L", label, device)
	case "exfat":
		return r.tool("", "mkfs.exfat", "-L", label, device)
	case "fat32":
		return r.tool("", "mkfs.vfat", "-F", "32", "-n", strings.ToUpper(label), device)
	}
	return fmt.Errorf("sistema de archivos no admitido: %q", step.FS)
}

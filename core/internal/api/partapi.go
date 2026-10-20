// * Endpoints del gestor de particiones (solo en Rebootera OS): la distribución de
// * cada disco, la vista previa de lo que haría una operación y su ejecución. Core no
// * escribe en discos: la vista previa la calcula partman con lo que lee de sysfs y la
// * ejecución se la pide al agente, que lo vuelve a comprobar todo.
// ! Aplicar exige el texto "CHANGE <disco>" y el agente lo exige otra vez.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"rebootera/core/internal/agentproto"
	"rebootera/core/internal/partman"
	"rebootera/core/internal/storage"
)

// listDisks lee los discos del equipo (cambiable en las pruebas).
var listDisks = storage.List

// partErrors son los códigos estables de partman que la interfaz traduce.
var partErrors = []error{
	partman.ErrReadOnly, partman.ErrProtected, partman.ErrUnsupported, partman.ErrMounted,
	partman.ErrNotFound, partman.ErrNotResizable, partman.ErrNoChange, partman.ErrTooSmall,
	partman.ErrNoSpace, partman.ErrNotAdjacent, partman.ErrBadRequest, partman.ErrNoInfo,
}

// * registerPartitions añade los endpoints del gestor de particiones.
func (s *Server) registerPartitions(mux *http.ServeMux) {
	mux.Handle("GET /api/partitions", s.requireSession(http.HandlerFunc(s.handlePartitions)))
	mux.Handle("GET /api/partitions/info", s.requireSession(http.HandlerFunc(s.handlePartInfo)))
	mux.Handle("GET /api/partitions/status", s.requireSession(http.HandlerFunc(s.handlePartStatus)))
	mux.Handle("POST /api/partitions/plan", s.requireSession(s.requireCSRF(http.HandlerFunc(s.handlePartPlan))))
	mux.Handle("POST /api/partitions/apply", s.requireSession(s.requireCSRF(http.HandlerFunc(s.handlePartApply))))
}

// partDisk es un disco con su distribución para el gestor.
type partDisk struct {
	Name      string         `json:"name"`
	Model     string         `json:"model"`
	Vendor    string         `json:"vendor"`
	Transport string         `json:"transport"`
	Role      string         `json:"role"`
	SizeBytes uint64         `json:"sizeBytes"`
	Layout    partman.Layout `json:"layout"`
	// Partitions son las particiones con su sistema de archivos y uso (para las etiquetas).
	Partitions []storage.Partition `json:"partitions"`
}

// * handlePartitions devuelve todos los discos con su distribución.
func (s *Server) handlePartitions(w http.ResponseWriter, r *http.Request) {
	disks := []partDisk{}
	for _, disk := range listDisks() {
		if disk.Role == storage.RoleOptical {
			continue
		}
		disks = append(disks, partDisk{
			Name: disk.Name, Model: disk.Model, Vendor: disk.Vendor, Transport: disk.Transport, Role: disk.Role,
			SizeBytes: disk.SizeBytes, Layout: partman.Build(disk), Partitions: disk.Partitions,
		})
	}
	writeJSON(w, http.StatusOK, disks)
}

// * layoutOf busca la distribución de un disco.
func layoutOf(name string) (partman.Layout, bool) {
	for _, disk := range listDisks() {
		if disk.Name == name {
			return partman.Build(disk), true
		}
	}
	return partman.Layout{}, false
}

// * minimums pide al agente el mínimo de las particiones que hay que encoger.
func (s *Server) minimums(op partman.Op) partman.Info {
	info := partman.Info{}
	var numbers []int
	switch op.Kind {
	case partman.OpResize:
		numbers = []int{op.Part}
	case partman.OpBoundary:
		numbers = []int{op.Part, op.Other}
	case partman.OpMerge:
		numbers = []int{op.Part}
	}
	for _, number := range numbers {
		lines, err := s.agent.Send(agentproto.Request{Op: agentproto.OpPartInfo, Disk: op.Disk, Part: number})
		if err != nil {
			continue
		}
		for _, line := range lines {
			if value, ok := strings.CutPrefix(line, "min:"); ok {
				if bytes, err := strconv.ParseUint(value, 10, 64); err == nil {
					info[number] = bytes
				}
			}
		}
	}
	return info
}

// * partFail responde con un código de partman si el error lo es, o con el mensaje tal cual.
func partFail(w http.ResponseWriter, err error) {
	code := "part-failed"
	for _, known := range partErrors {
		if errors.Is(err, known) || err.Error() == known.Error() {
			code = "part-" + known.Error()
		}
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error(), "code": code})
}

// opRequest es el cuerpo de plan y apply.
type opRequest struct {
	Op      partman.Op `json:"op"`
	Confirm string     `json:"confirm"`
}

// planResponse es la vista previa de una operación.
type planResponse struct {
	partman.Plan
	// Confirm es el texto que hay que escribir para aplicarla.
	Confirm string `json:"confirm"`
}

// * handlePartPlan calcula lo que haría una operación, sin tocar nada.
func (s *Server) handlePartPlan(w http.ResponseWriter, r *http.Request) {
	var req opRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	layout, ok := layoutOf(req.Op.Disk)
	if !ok {
		partFail(w, partman.ErrNotFound)
		return
	}
	plan, err := partman.Make(layout, req.Op, s.minimums(req.Op))
	if err != nil {
		partFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, planResponse{Plan: plan, Confirm: "CHANGE " + req.Op.Disk})
}

// * handlePartApply pide al agente que ejecute la operación.
func (s *Server) handlePartApply(w http.ResponseWriter, r *http.Request) {
	var req opRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	if req.Confirm != "CHANGE "+req.Op.Disk {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "falta la confirmación escrita", "code": "part-confirm"})
		return
	}
	// ! Con una descarga en marcha el USB está escribiendo y la memoria y el disco van justos: un cambio de particiones
	// ! a la vez es justo cuando algo sale mal. Se pide pausar o esperar a que termine.
	if s.deps.Transfers != nil && s.deps.Transfers.Busy() {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "hay descargas en marcha", "code": "part-downloading"})
		return
	}
	if _, err := s.agent.Send(agentproto.Request{Op: agentproto.OpPartApply, Partition: &req.Op, Confirm: req.Confirm}); err != nil {
		partFail(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "running"})
}

// * handlePartStatus devuelve el estado de la operación en curso.
func (s *Server) handlePartStatus(w http.ResponseWriter, r *http.Request) {
	lines, err := s.agent.Query(agentproto.OpPartStatus)
	if err != nil || len(lines) == 0 {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "el servicio del sistema no responde", "code": codeAgent})
		return
	}
	var status map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &status); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// * handlePartInfo devuelve el tamaño mínimo al que se puede encoger una partición (lo calcula el agente).
func (s *Server) handlePartInfo(w http.ResponseWriter, r *http.Request) {
	disk := r.URL.Query().Get("disk")
	number, err := strconv.Atoi(r.URL.Query().Get("part"))
	if err != nil || number < 1 {
		partFail(w, partman.ErrBadRequest)
		return
	}
	lines, err := s.agent.Send(agentproto.Request{Op: agentproto.OpPartInfo, Disk: disk, Part: number})
	if err != nil {
		partFail(w, err)
		return
	}
	info := map[string]any{"minBytes": 0, "fs": ""}
	for _, line := range lines {
		if value, ok := strings.CutPrefix(line, "min:"); ok {
			bytes, _ := strconv.ParseUint(value, 10, 64)
			info["minBytes"] = bytes
		}
		if value, ok := strings.CutPrefix(line, "fs:"); ok {
			info["fs"] = value
		}
	}
	writeJSON(w, http.StatusOK, info)
}

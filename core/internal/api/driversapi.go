// * Endpoints de drivers y de instalación: qué drivers de Windows del Vault
// * casan con el hardware de este equipo, y la preparación de un instalador
// * (archivo de respuestas, drivers inyectados, requisitos de Windows 11).
package api

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"rebootera/core/internal/agentproto"
	"rebootera/core/internal/boot"
	"rebootera/core/internal/drivers"
	"rebootera/core/internal/hardware"
	"rebootera/core/internal/library"
	"rebootera/core/internal/unattend"
	"rebootera/core/internal/vault"
)

// Códigos de error propios de la preparación de instaladores.
const (
	codeNoDrivers    = "no-drivers"
	codeDriversOS    = "drivers-unsupported"
	codeUnattendedOS = "unattended-unsupported"
	codeBootOrder    = "boot-order"
)

// driverMatch es un dispositivo con su paquete, tal como lo ve la interfaz.
type driverMatch struct {
	Address     string           `json:"address"`
	Name        string           `json:"name"`
	Vendor      string           `json:"vendor"`
	Category    string           `json:"category"`
	Critical    bool             `json:"critical"`
	HardwareID  string           `json:"hardwareId"`
	Specific    bool             `json:"specific"`
	Description string           `json:"description"`
	Package     *drivers.Package `json:"package"`
}

// driverPlan es la respuesta de GET /api/drivers/plan.
type driverPlan struct {
	// Available indica que hay Vault (y por tanto carpeta drivers/).
	Available bool `json:"available"`
	// Packages es cuántos paquetes x64 hay en todos los orígenes.
	Packages int `json:"packages"`
	// Sources son los sitios de donde salen los drivers (el USB, particiones del equipo, un Windows instalado).
	Sources []drivers.Source `json:"sources"`
	Matches []driverMatch    `json:"matches"`
	Missing []driverMatch    `json:"missing"`
}

// * driversRoot es la carpeta drivers/ del Vault.
func driversRoot() string {
	return vault.Path("drivers")
}

// * buildDriverPlan casa los dispositivos de este equipo con los paquetes de todos los orígenes: la carpeta
// * drivers/ del USB y las carpetas de drivers que el agente encontró en las particiones del equipo.
func buildDriverPlan() (driverPlan, drivers.Plan) {
	result := driverPlan{Matches: []driverMatch{}, Missing: []driverMatch{}, Sources: []drivers.Source{}}
	vaultRoot := ""
	if vault.Available() {
		result.Available = true
		vaultRoot = vault.Root
	}
	packages, used := drivers.ScanSources(drivers.Discover(vaultRoot, MediaRoot))
	if used != nil {
		result.Sources = used
	}
	result.Packages = len(packages)
	plan := drivers.BuildPlan(hardware.List(), packages)
	for _, match := range plan.Matches {
		entry := deviceMatch(match.Device)
		entry.HardwareID, entry.Specific, entry.Description, entry.Package = match.HardwareID, match.Specific, match.Description, match.Package
		result.Matches = append(result.Matches, entry)
	}
	for _, device := range plan.Missing {
		result.Missing = append(result.Missing, deviceMatch(device))
	}
	return result, plan
}

// * deviceMatch rellena los datos del dispositivo de una entrada.
func deviceMatch(device hardware.Device) driverMatch {
	return driverMatch{
		Address: device.Address, Name: device.Name, Vendor: device.Vendor,
		Category: device.Category, Critical: device.Critical,
	}
}

// scan guarda cuándo se buscaron drivers por última vez en las particiones del equipo.
var scan struct {
	mu   sync.Mutex
	last time.Time
}

// * scanForDrivers pide al agente que abra en solo lectura las particiones que pueden traer drivers. Sin
// * forzar, no se repite antes de dos minutos: la interfaz consulta el plan a menudo.
func (s *Server) scanForDrivers(force bool) {
	if s.mode() != ModeLive || s.agent == nil {
		return
	}
	scan.mu.Lock()
	defer scan.mu.Unlock()
	if !force && time.Since(scan.last) < 2*time.Minute {
		return
	}
	scan.last = time.Now()
	if _, err := s.agent.Send(agentproto.Request{Op: agentproto.OpDriversScan}); err != nil {
		log.Printf("drivers: no se pudo buscar en las particiones: %v", err)
	}
}

// * handleDriverPlan devuelve el casado de drivers con el hardware, buscando antes en las particiones.
func (s *Server) handleDriverPlan(w http.ResponseWriter, r *http.Request) {
	s.scanForDrivers(false)
	result, _ := buildDriverPlan()
	writeJSON(w, http.StatusOK, result)
}

// * handleDriverScan vuelve a buscar drivers en las particiones del equipo y devuelve el casado nuevo.
func (s *Server) handleDriverScan(w http.ResponseWriter, r *http.Request) {
	s.scanForDrivers(true)
	result, _ := buildDriverPlan()
	writeJSON(w, http.StatusOK, result)
}

// installRequest es el cuerpo de POST /api/boot/install.
type installRequest struct {
	Path string `json:"path"`
	// Drivers lleva al instalador de Windows los drivers del Vault que casan.
	Drivers bool `json:"drivers"`
	// WinBypass quita los requisitos de TPM, Secure Boot y RAM de Windows 11.
	WinBypass bool `json:"winBypass"`
	// BootOnce (solo UEFI) pone este USB el último en el orden de arranque durante la
	// instalación, para que el instalador no se repita; se restaura al volver a Rebootera.
	BootOnce bool `json:"bootOnce"`
	// Privacy (solo Windows) reduce la telemetría y los anuncios durante la instalación.
	Privacy bool `json:"privacy"`
	// Unattended da respuestas de idioma, zona horaria y cuenta; nunca de disco.
	Unattended *unattend.Options `json:"unattended"`
}

// Sistemas a los que sabemos dar respuestas automáticas.
const (
	osWindows = "windows"
	osDebian  = "debian"
	osOther   = "other"
)

// * installerOS deduce el sistema de una imagen de la biblioteca.
func installerOS(item library.Item) string {
	name := strings.ToLower(filepath.Base(item.Path))
	switch {
	case item.Category == "windows" || strings.HasPrefix(name, "win"):
		return osWindows
	case strings.HasPrefix(item.Source, "debian") || strings.HasPrefix(name, "debian-"):
		return osDebian
	}
	return osOther
}

// * prepareInstall construye lo que Ventoy entrega al instalador. Devuelve el
// * código de error de la API y su mensaje si algo no se puede hacer.
func prepareInstall(item library.Item, req installRequest) (boot.Prep, string, string) {
	var prep boot.Prep
	system := installerOS(item)
	if req.WinBypass {
		if system != osWindows {
			return prep, codeInvalid, "windows 11 bypass only applies to Windows images"
		}
		prep.WinBypass = true
	}
	if req.Drivers {
		if system != osWindows {
			return prep, codeDriversOS, "driver injection only applies to Windows images"
		}
		archive, code, message := stageDrivers()
		if code != "" {
			return prep, code, message
		}
		prep.DriversArchive = archive
	}
	if req.Privacy && system != osWindows {
		return prep, codeInvalid, "privacy settings only apply to Windows images"
	}
	if req.Unattended != nil {
		options := *req.Unattended
		options.Drivers = req.Drivers
		options.ReduceTelemetry = req.Privacy
		var err error
		switch system {
		case osWindows:
			prep.AnswersName = "autounattend.xml"
			prep.Answers, err = unattend.WindowsAnswerFile(options)
		case osDebian:
			prep.AnswersName = "preseed.cfg"
			prep.Answers, err = unattend.Preseed(options)
		default:
			return prep, codeUnattendedOS, "unattended answers are only available for Windows and Debian images"
		}
		if err != nil {
			return prep, codeInvalid, err.Error()
		}
	} else if req.Drivers || req.Privacy {
		// * Sin respuestas del usuario, un autounattend mínimo apunta a los drivers y/o reduce la telemetría,
		// * sin tocar el idioma ni el teclado.
		answers, err := unattend.WindowsAnswerFile(unattend.Options{Drivers: req.Drivers, ReduceTelemetry: req.Privacy})
		if err != nil {
			return prep, codeFailed, err.Error()
		}
		prep.AnswersName, prep.Answers = "autounattend.xml", answers
	}
	return prep, "", ""
}

// * stageDrivers reúne los drivers que casan en un 7z dentro del Vault y
// * devuelve su ruta para Ventoy.
func stageDrivers() (string, string, string) {
	_, plan := buildDriverPlan()
	if len(plan.Packages) == 0 {
		return "", codeNoDrivers, "no driver in the Vault matches this computer"
	}
	if err := os.MkdirAll(boot.WorkDir(), 0o750); err != nil {
		return "", codeFailed, err.Error()
	}
	stage, err := os.MkdirTemp(boot.WorkDir(), "stage-")
	if err != nil {
		return "", codeFailed, err.Error()
	}
	defer os.RemoveAll(stage)
	staged, err := drivers.Stage(driversRoot(), plan.Packages, stage)
	if err != nil {
		return "", codeFailed, err.Error()
	}
	archive, ventoyPath := boot.ArchivePath()
	if err := drivers.Archive(stage, archive); err != nil {
		return "", codeFailed, err.Error()
	}
	log.Printf("drivers: %d paquetes llevados al instalador (%s)", len(staged), strings.Join(staged, ", "))
	return ventoyPath, "", ""
}

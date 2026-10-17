// * Package efiboot: lógica pura del orden de arranque UEFI para el arranque
// * "de una sola vez" del instalador: análisis de la salida de efibootmgr y
// * cálculo del nuevo orden. El agente (root) es quien ejecuta efibootmgr.
// * Con la opción activada, antes de reiniciar al instalador el USB pasa a ser
// * la última opción del orden y se fija como BootNext (un único arranque). Así,
// * cuando el instalador reinicie, el firmware prueba antes el disco interno y
// * el instalador no se repite. Al volver a Rebootera se restaura el orden.
// ! Solo se reordenan entradas que ya existen: nunca se crean ni se borran.
package efiboot

import (
	"errors"
	"regexp"
	"slices"
	"strings"
)

// State es el estado de arranque UEFI que da efibootmgr.
type State struct {
	// Current es la entrada con la que se ha arrancado ahora ("0001").
	Current string
	// Order es el orden de arranque ("0001", "0000"...).
	Order []string
	// Entries relaciona cada entrada con su nombre.
	Entries map[string]string
}

var (
	currentPattern = regexp.MustCompile(`(?m)^BootCurrent:\s*([0-9A-Fa-f]{4})\s*$`)
	orderPattern   = regexp.MustCompile(`(?m)^BootOrder:\s*([0-9A-Fa-f,]*)\s*$`)
	entryPattern   = regexp.MustCompile(`(?m)^Boot([0-9A-Fa-f]{4})\*?\s+(.*)$`)
	idPattern      = regexp.MustCompile(`^[0-9A-F]{4}$`)
)

// ErrNoUEFI indica que no hay información de arranque UEFI utilizable.
var ErrNoUEFI = errors.New("este equipo no arrancó en modo UEFI o el firmware no permite cambiar el orden")

// * Parse lee la salida de efibootmgr.
func Parse(output string) (State, error) {
	state := State{Entries: map[string]string{}}
	if m := currentPattern.FindStringSubmatch(output); m != nil {
		state.Current = strings.ToUpper(m[1])
	}
	if m := orderPattern.FindStringSubmatch(output); m != nil {
		for _, id := range strings.Split(m[1], ",") {
			if id = strings.ToUpper(strings.TrimSpace(id)); id != "" {
				state.Order = append(state.Order, id)
			}
		}
	}
	for _, m := range entryPattern.FindAllStringSubmatch(output, -1) {
		state.Entries[strings.ToUpper(m[1])] = strings.TrimSpace(m[2])
	}
	if state.Current == "" || len(state.Order) == 0 {
		return State{}, ErrNoUEFI
	}
	return state, nil
}

// * PlanOnce calcula el orden con la entrada actual (el USB) al final. Devuelve
// * false si no hay nada que cambiar (ya es la última, o es la única).
func PlanOnce(state State) ([]string, bool) {
	if !slices.Contains(state.Order, state.Current) || len(state.Order) < 2 {
		return state.Order, false
	}
	if state.Order[len(state.Order)-1] == state.Current {
		return state.Order, false
	}
	plan := make([]string, 0, len(state.Order))
	for _, id := range state.Order {
		if id != state.Current {
			plan = append(plan, id)
		}
	}
	return append(plan, state.Current), true
}

// * Valid indica si una lista de entradas es un orden aceptable: ids de cuatro
// * dígitos hexadecimales en mayúsculas, sin repetidos.
func Valid(order []string) bool {
	seen := map[string]bool{}
	for _, id := range order {
		if !idPattern.MatchString(id) || seen[id] {
			return false
		}
		seen[id] = true
	}
	return len(order) > 0
}

// * Restorable filtra un orden guardado dejando solo las entradas que existen
// * ahora, por si el firmware o el sistema instalado borró alguna.
func Restorable(saved []string, state State) []string {
	var order []string
	for _, id := range saved {
		if _, ok := state.Entries[id]; ok {
			order = append(order, id)
		}
	}
	// * Las entradas nuevas (por ejemplo, el sistema recién instalado) conservan su lugar delante.
	var added []string
	for _, id := range state.Order {
		if !slices.Contains(saved, id) {
			if _, ok := state.Entries[id]; ok {
				added = append(added, id)
			}
		}
	}
	return append(added, order...)
}

// * Join une un orden con comas, como lo espera efibootmgr -o.
func Join(order []string) string {
	return strings.Join(order, ",")
}

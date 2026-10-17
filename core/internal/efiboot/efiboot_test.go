// * Pruebas del orden de arranque UEFI con salidas reales de efibootmgr.
package efiboot

import (
	"errors"
	"slices"
	"testing"
)

// usbFirst es la salida de un equipo que arrancó desde el USB (0003), que
// además es la primera opción del orden.
const usbFirst = `BootCurrent: 0003
Timeout: 0 seconds
BootOrder: 0003,0001,0000
Boot0000* UiApp	FvVol(7cb8bdc9-f8eb-4f34-aaea-3ee4af6516a1)/FvFile(462caa21-7614-4503-836e-8ab6f4662331)
Boot0001* Windows Boot Manager	HD(1,GPT,5d6c3b3a-0000-0000-0000-000000000000,0x800,0x32000)/File(\EFI\Microsoft\Boot\bootmgfw.efi)
Boot0003* UEFI USB HARDDISK	PciRoot(0x0)/Pci(0x3,0x0)/USB(0x1,0x0)
`

// * TestParse lee la corriente, el orden y los nombres.
func TestParse(t *testing.T) {
	state, err := Parse(usbFirst)
	if err != nil {
		t.Fatal(err)
	}
	if state.Current != "0003" || !slices.Equal(state.Order, []string{"0003", "0001", "0000"}) {
		t.Fatalf("estado: %+v", state)
	}
	if state.Entries["0001"] != "Windows Boot Manager\tHD(1,GPT,5d6c3b3a-0000-0000-0000-000000000000,0x800,0x32000)/File(\\EFI\\Microsoft\\Boot\\bootmgfw.efi)" {
		t.Fatalf("entradas: %q", state.Entries["0001"])
	}
	if _, err := Parse("EFI variables are not supported on this system."); !errors.Is(err, ErrNoUEFI) {
		t.Fatalf("sin UEFI: %v", err)
	}
}

// * TestPlanOnce pone el USB el último y no toca nada si ya lo es.
func TestPlanOnce(t *testing.T) {
	state, _ := Parse(usbFirst)
	order, changed := PlanOnce(state)
	if !changed || !slices.Equal(order, []string{"0001", "0000", "0003"}) {
		t.Fatalf("plan: %v %v", order, changed)
	}
	state.Order = []string{"0001", "0000", "0003"}
	if _, changed := PlanOnce(state); changed {
		t.Fatal("si el USB ya es el último no hay nada que cambiar")
	}
	state.Order = []string{"0003"}
	if _, changed := PlanOnce(state); changed {
		t.Fatal("con una sola entrada no se reordena")
	}
	// * Si el USB ni siquiera está en el orden, no se inventa nada.
	state.Order = []string{"0001", "0000"}
	if order, changed := PlanOnce(state); changed || !slices.Equal(order, []string{"0001", "0000"}) {
		t.Fatalf("USB fuera del orden: %v %v", order, changed)
	}
}

// * TestRestorable conserva las entradas nuevas delante y descarta las borradas.
func TestRestorable(t *testing.T) {
	// * Orden original 0003,0001,0000; durante la instalación se borró la 0000 y
	// * Windows creó la 0004, que ahora es la primera.
	now := State{Current: "0003", Order: []string{"0004", "0001", "0003"}, Entries: map[string]string{"0001": "a", "0003": "usb", "0004": "Windows nuevo"}}
	got := Restorable([]string{"0003", "0001", "0000"}, now)
	if !slices.Equal(got, []string{"0004", "0003", "0001"}) {
		t.Fatalf("restaurado: %v", got)
	}
}

// * TestValid rechaza identificadores peligrosos.
func TestValid(t *testing.T) {
	if !Valid([]string{"0001", "000A"}) {
		t.Fatal("orden válido rechazado")
	}
	for _, bad := range [][]string{{}, {"0001", "0001"}, {"1"}, {"0001;rm"}, {"000a"}, {"-0001"}} {
		if Valid(bad) {
			t.Fatalf("%v debería rechazarse", bad)
		}
	}
	if Join([]string{"0001", "0002"}) != "0001,0002" {
		t.Fatal("Join")
	}
}

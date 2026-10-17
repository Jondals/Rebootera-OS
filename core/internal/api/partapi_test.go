// * Pruebas de la API del gestor de particiones con un disco simulado.
package api

import (
	"net/http"
	"strings"
	"testing"

	"rebootera/core/internal/agentproto"
	"rebootera/core/internal/storage"
	"rebootera/core/internal/testutil"
)

// fakeDisks devuelve un disco virtual de 1 GiB con dos particiones ext4 pegadas.
func fakeDisks() []storage.Disk {
	const mib = 2048
	return []storage.Disk{{
		Name: "vdb", Sectors: 1024 * mib, SizeBytes: 1024 * mib * 512, PartTable: "gpt", Role: storage.RoleRemovable, Transport: "virtio",
		Partitions: []storage.Partition{
			{Name: "vdb1", Number: 1, StartSector: 100 * mib, Sectors: 300 * mib, FSType: "ext4"},
			{Name: "vdb2", Number: 2, StartSector: 400 * mib, Sectors: 300 * mib, FSType: "ext4"},
		},
	}, {
		Name: "sda", Sectors: 1024 * mib, SizeBytes: 1024 * mib * 512, PartTable: "gpt", Role: storage.RoleLiveMedia,
	}}
}

// * TestParticionesVistaYPlan comprueba la lista de discos, la vista previa y el bloqueo del medio de arranque.
func TestParticionesVistaYPlan(t *testing.T) {
	t.Cleanup(testutil.Keep(&listDisks).Restore)
	listDisks = fakeDisks
	agent := &fakeAgent{replies: map[string][]string{agentproto.OpPartInfo: {"min:62914560", "fs:ext4"}}}
	h := newTestServer(t, agent)
	rec := do(h, "GET", "/api/partitions", "127.0.0.1:7080", true, "", nil)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"name":"vdb"`) || !strings.Contains(body, `"locked":"protected"`) {
		t.Fatalf("lista: %d %s", rec.Code, body)
	}
	plan := `{"op":{"kind":"boundary","disk":"vdb","part":1,"other":2,"sizeMiB":380}}`
	rec = do(h, "POST", "/api/partitions/plan", "127.0.0.1:7080", true, plan, jsonHeaders)
	body = rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"kind":"move"`) || !strings.Contains(body, `"confirm":"CHANGE vdb"`) {
		t.Fatalf("plan: %d %s", rec.Code, body)
	}
	// * El disco del medio de arranque no se puede planificar.
	rec = do(h, "POST", "/api/partitions/plan", "127.0.0.1:7080", true, `{"op":{"kind":"delete","disk":"sda","part":1}}`, jsonHeaders)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"part-protected"`) {
		t.Fatalf("medio de arranque: %d %s", rec.Code, rec.Body.String())
	}
	// * Una operación imposible devuelve su código.
	rec = do(h, "POST", "/api/partitions/plan", "127.0.0.1:7080", true, `{"op":{"kind":"boundary","disk":"vdb","part":1,"other":2,"sizeMiB":595}}`, jsonHeaders)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"part-too-small"`) {
		t.Fatalf("demasiado pequeña: %d %s", rec.Code, rec.Body.String())
	}
}

// * TestParticionesAplicar exige la confirmación escrita antes de llegar al agente.
func TestParticionesAplicar(t *testing.T) {
	agent := &fakeAgent{}
	h := newTestServer(t, agent)
	op := `"op":{"kind":"delete","disk":"vdb","part":1}`
	rec := do(h, "POST", "/api/partitions/apply", "127.0.0.1:7080", true, `{`+op+`,"confirm":"yes"}`, jsonHeaders)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "part-confirm") || len(agent.sent) != 0 {
		t.Fatalf("sin confirmación: %d %s %v", rec.Code, rec.Body.String(), agent.sent)
	}
	rec = do(h, "POST", "/api/partitions/apply", "127.0.0.1:7080", true, `{`+op+`,"confirm":"CHANGE vdb"}`, jsonHeaders)
	if rec.Code != http.StatusAccepted || len(agent.sent) != 1 || agent.sent[0].Op != agentproto.OpPartApply || agent.sent[0].Partition == nil || agent.sent[0].Partition.Part != 1 {
		t.Fatalf("con confirmación: %d %s %v", rec.Code, rec.Body.String(), agent.sent)
	}
	// * Cambiar particiones exige la cabecera anti-CSRF.
	rec = do(h, "POST", "/api/partitions/apply", "127.0.0.1:7080", true, `{`+op+`,"confirm":"CHANGE vdb"}`, map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("sin cabecera anti-CSRF debería dar 403, dio %d", rec.Code)
	}
}

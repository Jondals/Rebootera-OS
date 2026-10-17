// * Pruebas del análisis de INF, el casado con dispositivos y la preparación.
package drivers

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"rebootera/core/internal/hardware"
)

// storageINF imita el INF de un controlador de disco virtual (virtio-scsi).
const storageINF = `; Controlador de prueba
[Version]
Signature="$Windows NT$"
Class=SCSIAdapter
ClassGuid={4D36E97B-E325-11CE-BFC1-08002BE10318}
Provider=%VENDOR%
DriverVer = 01/14/2025,100.101.104.28300
CatalogFile=vioscsi.cat

[Manufacturer]
%VENDOR% = Standard,NTamd64.10.0

[Standard.NTamd64.10.0]
%Scsi.DeviceDesc% = scsi_inst, PCI\VEN_1AF4&DEV_1004&SUBSYS_00081AF4&REV_00, \
    PCI\VEN_1AF4&DEV_1004&SUBSYS_00081AF4, PCI\VEN_1AF4&DEV_1004
%Scsi.DeviceDesc% = scsi_inst, PCI\VEN_1AF4&DEV_1048&SUBSYS_11001AF4 ; comentario

[Strings]
VENDOR = "Red Hat, Inc."
Scsi.DeviceDesc = "Red Hat VirtIO SCSI controller"
`

// x86INF solo declara arquitectura x86: no debe aceptarse.
const x86INF = `[Version]
Signature="$Windows NT$"
Class=Net
Provider=Viejo
[Manufacturer]
Viejo = Models,NTx86
[Models.NTx86]
Tarjeta = inst, PCI\VEN_10EC&DEV_8168
`

// * fakeCatalog es el principio de un PKCS#7 firmado (SEQUENCE + OID signedData).
var fakeCatalog = append([]byte{0x30, 0x82, 0x10, 0x00, 0x06, 0x09}, append(pkcs7SignedData, make([]byte, 40)...)...)

// * writeFile crea un archivo con su carpeta.
func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// * utf16le codifica texto como UTF-16 little endian con BOM, como los INF de Windows.
func utf16le(text string) []byte {
	units := utf16.Encode([]rune(text))
	out := []byte{0xFF, 0xFE}
	for _, u := range units {
		out = binary.LittleEndian.AppendUint16(out, u)
	}
	return out
}

// * TestParseINF comprueba cadenas, continuaciones, comentarios y arquitectura.
func TestParseINF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vioscsi.inf")
	writeFile(t, path, []byte(storageINF))
	pkg, err := ParseFile(path, ".", "vioscsi.inf")
	if err != nil {
		t.Fatal(err)
	}
	if !pkg.Amd64 || pkg.Class != "SCSIAdapter" || pkg.Provider != "Red Hat, Inc." {
		t.Fatalf("cabecera: %+v", pkg)
	}
	if pkg.Date != "01/14/2025" || pkg.Version != "100.101.104.28300" || pkg.Catalog != "vioscsi.cat" {
		t.Fatalf("versión: %q %q %q", pkg.Date, pkg.Version, pkg.Catalog)
	}
	// * La línea con \ se une a la siguiente y el comentario no cuenta.
	for _, id := range []string{`PCI\VEN_1AF4&DEV_1004&SUBSYS_00081AF4&REV_00`, `PCI\VEN_1AF4&DEV_1004`, `PCI\VEN_1AF4&DEV_1048&SUBSYS_11001AF4`} {
		if !pkg.Supports(strings.ToLower(id)) {
			t.Fatalf("falta el Hardware ID %s", id)
		}
	}
	if pkg.Describe(`PCI\VEN_1AF4&DEV_1004`) != "Red Hat VirtIO SCSI controller" {
		t.Fatalf("descripción: %q", pkg.Describe(`PCI\VEN_1AF4&DEV_1004`))
	}
}

// * TestParseUTF16 lee un INF en UTF-16 con BOM, el formato habitual de Windows.
func TestParseUTF16(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.inf")
	writeFile(t, path, utf16le(strings.ReplaceAll(storageINF, "\n", "\r\n")))
	pkg, err := ParseFile(path, ".", "a.inf")
	if err != nil || !pkg.Supports(`PCI\VEN_1AF4&DEV_1004`) {
		t.Fatalf("UTF-16: %v %+v", err, pkg)
	}
}

// * TestNoINF rechaza archivos que no son INF y los x86.
func TestNoINF(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "texto.inf"), []byte("hola\n"))
	if _, err := ParseFile(filepath.Join(dir, "texto.inf"), ".", "texto.inf"); err == nil {
		t.Fatal("debería rechazar un archivo sin [Version]")
	}
	writeFile(t, filepath.Join(dir, "x86", "viejo.inf"), []byte(x86INF))
	if got := Scan(dir); len(got) != 0 {
		t.Fatalf("un INF solo x86 no debe aparecer: %+v", got)
	}
}

// * fakeDevices son un disco virtio-scsi crítico, una tarjeta de red sin
// * paquete y una GPU no crítica.
func fakeDevices() []hardware.Device {
	return []hardware.Device{
		{Address: "0000:00:05.0", Category: hardware.CatStorage, Critical: true, HardwareIDs: []string{
			`PCI\VEN_1AF4&DEV_1004&SUBSYS_00081AF4&REV_00`, `PCI\VEN_1AF4&DEV_1004&SUBSYS_00081AF4`,
			`PCI\VEN_1AF4&DEV_1004&REV_00`, `PCI\VEN_1AF4&DEV_1004`}},
		{Address: "0000:00:06.0", Category: hardware.CatNetwork, Critical: true, HardwareIDs: []string{
			`PCI\VEN_10EC&DEV_8168&REV_15`, `PCI\VEN_10EC&DEV_8168`}},
		{Address: "0000:00:02.0", Category: hardware.CatDisplay, HardwareIDs: []string{`PCI\VEN_1234&DEV_1111`}},
	}
}

// * TestPlan comprueba el casado, el paquete más reciente y los críticos sin driver.
func TestPlan(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "vioscsi", "w11", "amd64", "vioscsi.inf"), []byte(storageINF))
	writeFile(t, filepath.Join(root, "vioscsi", "w11", "amd64", "vioscsi.cat"), fakeCatalog)
	writeFile(t, filepath.Join(root, "vioscsi", "w11", "amd64", "vioscsi.sys"), []byte("MZ"))
	// * Una versión más antigua del mismo driver: debe perder.
	older := strings.Replace(storageINF, "01/14/2025", "03/02/2021", 1)
	writeFile(t, filepath.Join(root, "viejo", "vioscsi.inf"), []byte(older))
	writeFile(t, filepath.Join(root, "viejo", "vioscsi.cat"), []byte("no es un catálogo"))

	packages := Scan(root)
	if len(packages) != 2 {
		t.Fatalf("esperaba 2 paquetes, obtuve %d", len(packages))
	}
	plan := BuildPlan(fakeDevices(), packages)
	if len(plan.Matches) != 1 || len(plan.Packages) != 1 {
		t.Fatalf("plan: %+v", plan)
	}
	match := plan.Matches[0]
	if match.Package.Dir != "vioscsi/w11/amd64" || !match.Specific || match.Package.Signature != SignaturePresent {
		t.Fatalf("paquete elegido: %+v", match.Package)
	}
	if match.HardwareID != `PCI\VEN_1AF4&DEV_1004&SUBSYS_00081AF4&REV_00` {
		t.Fatalf("debería casar por el ID más específico: %s", match.HardwareID)
	}
	if len(plan.Missing) != 1 || plan.Missing[0].Address != "0000:00:06.0" {
		t.Fatalf("la tarjeta de red crítica debería faltar: %+v", plan.Missing)
	}
	// * El paquete antiguo tiene un catálogo que no es PKCS#7.
	for _, pkg := range packages {
		if pkg.Dir == "viejo" && pkg.Signature != SignatureInvalid {
			t.Fatalf("catálogo inválido no detectado: %s", pkg.Signature)
		}
	}
}

// * TestStage comprueba que solo se copian archivos normales del paquete.
func TestStage(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "uno", "a.inf"), []byte(storageINF))
	writeFile(t, filepath.Join(root, "uno", "a.cat"), fakeCatalog)
	writeFile(t, filepath.Join(root, "uno", "sub", "b.sys"), []byte("MZ"))
	writeFile(t, filepath.Join(root, "ajeno", "secreto.txt"), []byte("no debe copiarse"))
	os.Symlink(filepath.Join(root, "ajeno", "secreto.txt"), filepath.Join(root, "uno", "enlace.txt"))
	pkgs := Scan(root)
	dest := t.TempDir()
	staged, err := Stage(root, pkgs, dest)
	if err != nil || len(staged) != 1 {
		t.Fatalf("stage: %v %v", staged, err)
	}
	base := filepath.Join(dest, filepath.FromSlash(staged[0]))
	for _, name := range []string{"a.inf", "a.cat", filepath.Join("sub", "b.sys")} {
		if _, err := os.Stat(filepath.Join(base, name)); err != nil {
			t.Fatalf("falta %s: %v", name, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(base, "enlace.txt")); err == nil {
		t.Fatal("no debe copiar enlaces simbólicos")
	}
	if _, err := os.Stat(filepath.Join(dest, DirName, "secreto.txt")); err == nil {
		t.Fatal("copió algo fuera del paquete")
	}
}

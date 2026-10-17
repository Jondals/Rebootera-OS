// * Pruebas de la verificación OpenPGP con firmas oficiales reales
// * (testdata/) descargadas de Debian, Ubuntu y Fedora.
package catalog

import (
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// * spec devuelve la especificación de firma de una entrada del catálogo.
func spec(t *testing.T, id string) SigSpec {
	t.Helper()
	entry, err := Find(id)
	if err != nil || entry.Resolver.Signature == nil {
		t.Fatalf("la entrada %s debería tener firma: %v", id, err)
	}
	return *entry.Resolver.Signature
}

// * read lee un archivo de testdata.
func read(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// * TestDetachedReal comprueba las firmas separadas reales de Debian y Ubuntu.
func TestDetachedReal(t *testing.T) {
	cases := []struct{ id, sums, sig, who string }{
		{"debian-13-netinst", "debian.SHA256SUMS", "debian.SHA256SUMS.sign", "Debian CD signing key"},
		{"ubuntu-26.04-desktop", "ubuntu.SHA256SUMS", "ubuntu.SHA256SUMS.gpg", "Ubuntu CD Image Automatic Signing Key"},
		{"ubuntu-24.04-desktop", "ubuntu.SHA256SUMS", "ubuntu.SHA256SUMS.gpg", "Ubuntu CD Image Automatic Signing Key"},
	}
	for _, c := range cases {
		who, err := VerifyDetached(read(t, c.sums), read(t, c.sig), spec(t, c.id))
		if err != nil || !strings.Contains(who, c.who) {
			t.Fatalf("%s: %q %v", c.id, who, err)
		}
		// * Un solo carácter cambiado en el índice invalida la firma.
		tampered := append([]byte(nil), read(t, c.sums)...)
		tampered[10] ^= 1
		if _, err := VerifyDetached(tampered, read(t, c.sig), spec(t, c.id)); !errors.Is(err, ErrBadSignature) {
			t.Fatalf("%s: un índice alterado debe rechazarse: %v", c.id, err)
		}
	}
}

// * TestWrongKey rechaza una firma válida hecha por una clave que no es la fijada.
func TestWrongKey(t *testing.T) {
	// * La firma de Ubuntu es válida con la clave de Ubuntu, pero la entrada de
	// * Debian solo acepta su propia clave.
	if _, err := VerifyDetached(read(t, "ubuntu.SHA256SUMS"), read(t, "ubuntu.SHA256SUMS.gpg"), spec(t, "debian-13-netinst")); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("debería rechazar la firma de otra distribución: %v", err)
	}
	// * Con la clave correcta pero sin la huella en la lista, también se rechaza.
	pinned := spec(t, "ubuntu-26.04-desktop")
	pinned.Fingerprints = []string{"0000000000000000000000000000000000000000"}
	if _, err := VerifyDetached(read(t, "ubuntu.SHA256SUMS"), read(t, "ubuntu.SHA256SUMS.gpg"), pinned); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("debería exigir la huella fijada: %v", err)
	}
}

// * TestISOSignatures usa las firmas reales de Arch y Alpine: un archivo que no
// * es la ISO firmada debe rechazarse. Con REBOOTERA_ISO_ARCH / REBOOTERA_ISO_ALPINE
// * apuntando a la ISO real, además debe aceptarse.
func TestISOSignatures(t *testing.T) {
	cases := []struct{ id, sig, envName string }{
		{"archlinux", "arch.iso.sig", "REBOOTERA_ISO_ARCH"},
		{"alpine-virt", "alpine-virt.iso.asc", "REBOOTERA_ISO_ALPINE"},
	}
	other := t.TempDir() + "/otra.iso"
	if err := os.WriteFile(other, []byte("esto no es una ISO"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		entry, err := Find(c.id)
		if err != nil || entry.Resolver.ISOSignature == nil {
			t.Fatalf("%s debería declarar la firma de su ISO: %v", c.id, err)
		}
		isoSpec := *entry.Resolver.ISOSignature
		if _, err := VerifyFile(other, read(t, c.sig), isoSpec); !errors.Is(err, ErrBadSignature) {
			t.Fatalf("%s: un archivo ajeno debe rechazarse: %v", c.id, err)
		}
		real := os.Getenv(c.envName)
		if real == "" {
			continue
		}
		who, err := VerifyFile(real, read(t, c.sig), isoSpec)
		if err != nil || who == "" {
			t.Fatalf("%s: la ISO real debe aceptarse: %q %v", c.id, who, err)
		}
	}
}

// * TestClearsignedReal comprueba el CHECKSUM firmado en claro de Fedora.
func TestClearsignedReal(t *testing.T) {
	plain, who, err := VerifyClearsigned(read(t, "fedora.CHECKSUM"), spec(t, "fedora-44-workstation"))
	if err != nil || !strings.Contains(who, "Fedora") {
		t.Fatalf("fedora: %q %v", who, err)
	}
	if !strings.Contains(plain, "SHA256 (Fedora-Workstation-Live-44-1.7.x86_64.iso) = ") {
		t.Fatalf("falta la suma de la ISO en lo firmado:\n%s", plain)
	}
	tampered := strings.Replace(string(read(t, "fedora.CHECKSUM")), "SHA256 (Fedora-Workstation-Live-44", "SHA256 (Fedora-Workstation-Live-45", 1)
	if _, _, err := VerifyClearsigned([]byte(tampered), spec(t, "fedora-44-workstation")); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("un CHECKSUM alterado debe rechazarse: %v", err)
	}
	if _, _, err := VerifyClearsigned([]byte("SHA256 (x) = 00\n"), spec(t, "fedora-44-workstation")); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("un texto sin firma debe rechazarse: %v", err)
	}
}

// * TestPinnedKeys comprueba que cada huella fijada en el catálogo existe en
// * sus archivos de claves y que cada clave embebida está fijada.
func TestPinnedKeys(t *testing.T) {
	entries, _ := Entries()
	for _, entry := range entries {
		sig := entry.Resolver.Signature
		if sig == nil {
			sig = entry.Resolver.ISOSignature
		}
		if sig == nil {
			if entry.Signature == "gpg" {
				t.Errorf("%s declara firma gpg pero no tiene especificación", entry.ID)
			}
			continue
		}
		ring, err := loadKeyring(*sig)
		if err != nil {
			t.Fatalf("%s: %v", entry.ID, err)
		}
		have := map[string]bool{}
		for _, e := range ring {
			have[strings.ToUpper(hex.EncodeToString(e.PrimaryKey.Fingerprint))] = true
		}
		for _, fp := range sig.Fingerprints {
			if !have[fp] {
				t.Errorf("%s: la huella %s no está en sus claves", entry.ID, fp)
			}
		}
		for fp := range have {
			found := false
			for _, pinned := range sig.Fingerprints {
				found = found || pinned == fp
			}
			if !found {
				t.Errorf("%s: la clave %s está embebida pero no fijada", entry.ID, fp)
			}
		}
	}
	// * El archivo debe ser un anillo OpenPGP válido (sin claves caducadas de más).
	if _, err := openpgp.ReadArmoredKeyRing(strings.NewReader(string(read(t, "../keys/debian.asc")))); err != nil {
		t.Fatal(err)
	}
}

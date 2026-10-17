package catalog

import (
	"bytes"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
)

// Claves públicas oficiales de los proyectos, junto con las huellas que se
// comprobaron contra sus páginas de verificación.
//
//go:embed keys/*.asc
var keyFiles embed.FS

// SigSpec describe cómo está firmado el índice de sumas de una entrada.
type SigSpec struct {
	// Type es "detached" (firma aparte, en URL) o "clearsign" (el propio índice va firmado).
	Type string `json:"type"`
	URL  string `json:"url,omitempty"`
	// Keys son los archivos de keys/ con las claves públicas oficiales.
	Keys []string `json:"keys"`
	// Suffix (solo en la firma de la ISO) es lo que se añade a la URL de la imagen para
	// obtener su firma: ".sig" en Arch, ".asc" en Alpine.
	Suffix string `json:"suffix,omitempty"`
	// Fingerprints son las huellas (hexadecimal, sin espacios) que pueden firmar el índice.
	Fingerprints []string `json:"fingerprints"`
}

// ErrBadSignature indica que la firma del índice no es válida o no es de una clave oficial.
var ErrBadSignature = errors.New("la firma GPG del índice no es válida")

// * loadKeyring lee las claves de un SigSpec.
func loadKeyring(spec SigSpec) (openpgp.EntityList, error) {
	var ring openpgp.EntityList
	for _, name := range spec.Keys {
		data, err := keyFiles.ReadFile("keys/" + name)
		if err != nil {
			return nil, err
		}
		// * Un archivo puede traer varios bloques armados seguidos (uno por clave).
		const end = "-----END PGP PUBLIC KEY BLOCK-----"
		for _, block := range strings.SplitAfter(string(data), end) {
			if !strings.Contains(block, "BEGIN PGP PUBLIC KEY BLOCK") {
				continue
			}
			entities, err := openpgp.ReadArmoredKeyRing(strings.NewReader(block))
			if err != nil {
				return nil, fmt.Errorf("clave %s: %w", name, err)
			}
			ring = append(ring, entities...)
		}
	}
	return ring, nil
}

// * signerName devuelve la huella y el nombre de la clave que firmó.
func signerName(signer *openpgp.Entity) (string, string) {
	fingerprint := strings.ToUpper(hex.EncodeToString(signer.PrimaryKey.Fingerprint))
	name := ""
	for _, identity := range signer.Identities {
		name = identity.Name
		break
	}
	return fingerprint, name
}

// * checkSigner exige que la clave que firmó esté entre las huellas fijadas.
func checkSigner(signer *openpgp.Entity, spec SigSpec) (string, error) {
	fingerprint, name := signerName(signer)
	if !slices.Contains(spec.Fingerprints, fingerprint) {
		return "", fmt.Errorf("%w: la firmó una clave no esperada (%s)", ErrBadSignature, fingerprint)
	}
	return strings.TrimSpace(name + " " + fingerprint), nil
}

// * VerifyDetached comprueba la firma separada de un texto. La firma puede
// * ser binaria o con armadura ASCII. Devuelve quién firmó.
func VerifyDetached(text, signature []byte, spec SigSpec) (string, error) {
	ring, err := loadKeyring(spec)
	if err != nil {
		return "", err
	}
	signer, err := openpgp.CheckDetachedSignature(ring, bytes.NewReader(text), bytes.NewReader(signature), nil)
	if err != nil && bytes.Contains(signature[:min(len(signature), 64)], []byte("BEGIN PGP")) {
		signer, err = openpgp.CheckArmoredDetachedSignature(ring, bytes.NewReader(text), bytes.NewReader(signature), nil)
	}
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrBadSignature, err)
	}
	return checkSigner(signer, spec)
}

// * verifyReader comprueba la firma separada de unos datos que se leen en
// * streaming (una ISO de más de 1 GB no se carga en memoria).
func verifyReader(ring openpgp.EntityList, signed io.Reader, signature []byte, spec SigSpec) (string, error) {
	signer, err := openpgp.CheckDetachedSignature(ring, signed, bytes.NewReader(signature), nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrBadSignature, err)
	}
	return checkSigner(signer, spec)
}

// * VerifyISO comprueba la firma de una ISO ya descargada. La firma se baja de
// * isoURL + spec.Suffix (por HTTPS) y debe ser de una clave de las fijadas.
func VerifyISO(path, isoURL string, spec SigSpec) (string, error) {
	if spec.Suffix == "" {
		return "", errors.New("la entrada no dice dónde está la firma de la imagen")
	}
	signature, err := fetchText(isoURL + spec.Suffix)
	if err != nil {
		return "", fmt.Errorf("no se pudo leer la firma de la imagen: %w", err)
	}
	return VerifyFile(path, []byte(signature), spec)
}

// * VerifyFile comprueba la firma separada (binaria o con armadura) de un archivo.
func VerifyFile(path string, signature []byte, spec SigSpec) (string, error) {
	ring, err := loadKeyring(spec)
	if err != nil {
		return "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	who, err := verifyReader(ring, file, signature, spec)
	if err != nil && bytes.Contains(signature[:min(len(signature), 64)], []byte("BEGIN PGP")) {
		// * Firma con armadura ASCII (Alpine): se relee el archivo desde el principio.
		file.Seek(0, io.SeekStart)
		signer, armorErr := openpgp.CheckArmoredDetachedSignature(ring, file, bytes.NewReader(signature), nil)
		if armorErr != nil {
			return "", fmt.Errorf("%w: %v", ErrBadSignature, armorErr)
		}
		return checkSigner(signer, spec)
	}
	return who, err
}

// * VerifyClearsigned comprueba un texto firmado en claro y devuelve el texto
// * original (solo lo firmado) y quién firmó.
func VerifyClearsigned(document []byte, spec SigSpec) (string, string, error) {
	block, rest := clearsign.Decode(document)
	if block == nil {
		return "", "", fmt.Errorf("%w: no es un documento firmado", ErrBadSignature)
	}
	// ! Lo que viene después de la firma no está firmado y se descarta.
	_ = rest
	ring, err := loadKeyring(spec)
	if err != nil {
		return "", "", err
	}
	body, err := io.ReadAll(block.ArmoredSignature.Body)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrBadSignature, err)
	}
	signer, err := openpgp.CheckDetachedSignature(ring, bytes.NewReader(block.Bytes), bytes.NewReader(body), nil)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrBadSignature, err)
	}
	who, err := checkSigner(signer, spec)
	if err != nil {
		return "", "", err
	}
	return string(block.Plaintext), who, nil
}

// * fetchIndex descarga el índice de una entrada y, si está firmado, comprueba
// * la firma. Devuelve el texto fiable y quién lo firmó ("" si no hay firma).
// ! Si la entrada declara una firma y no se puede comprobar, es un error: el
// ! archivo no se descarga.
func fetchIndex(r Resolver) (string, string, error) {
	text, err := fetchText(r.Index)
	if err != nil {
		return "", "", err
	}
	if r.Signature == nil {
		return text, "", nil
	}
	switch r.Signature.Type {
	case "detached":
		sig, err := fetchText(r.Signature.URL)
		if err != nil {
			return "", "", fmt.Errorf("no se pudo leer la firma del índice: %w", err)
		}
		who, err := VerifyDetached([]byte(text), []byte(sig), *r.Signature)
		return text, who, err
	case "clearsign":
		plain, who, err := VerifyClearsigned([]byte(text), *r.Signature)
		return plain, who, err
	}
	return "", "", fmt.Errorf("tipo de firma desconocido: %q", r.Signature.Type)
}

package transfers

import (
	"crypto/sha1" //nolint:gosec // el Internet Archive solo publica SHA-1: sirve para detectar copias dañadas
	"encoding/hex"
	"io"
	"os"
)

// * sha1File calcula el SHA-1 de un archivo.
func sha1File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha1.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

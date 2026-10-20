// * Descarga verificada de Ventoy, común a Windows, Linux y macOS: se baja de su release oficial de GitHub y se
// * comprueba su SHA-256 con el sha256.txt publicado en el mismo release.
// ! Ventoy no publica firmas criptográficas: la suma detecta descargas corruptas, pero no protege frente a un release
// ! alterado.
package desktop

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Versión de Ventoy que se usa (la misma que en las imágenes de Linux).
const (
	ventoyVersion = "1.1.18"
	ventoyBase    = "https://github.com/ventoy/Ventoy/releases/download/v" + ventoyVersion + "/"
)

// maxVentoyBytes es el tamaño máximo que se acepta de un archivo de Ventoy (los de cada sistema rondan los 20 MB).
const maxVentoyBytes = 256 << 20

// * cacheRoot es la carpeta de datos de Rebootera del usuario (Ventoy descargado, imágenes de Rebootera OS...).
func cacheRoot() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "Rebootera")
	return dir, os.MkdirAll(dir, 0o755)
}

// * fetchVentoyArchive descarga un archivo de Ventoy (por ejemplo el .zip de Windows o el .tar.gz de Linux) a la
// * carpeta de datos, comprobando su suma, y devuelve su ruta. Si ya está descargado y la suma coincide, no se repite.
func fetchVentoyArchive(name string) (string, error) {
	dir, err := cacheRoot()
	if err != nil {
		return "", err
	}
	sums, err := download(ventoyBase + "sha256.txt")
	if err != nil {
		return "", err
	}
	expected := ""
	scanner := bufio.NewScanner(strings.NewReader(string(sums)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[1] == name {
			expected = strings.ToLower(fields[0])
		}
	}
	if expected == "" {
		return "", fmt.Errorf("el release de Ventoy no publica la suma de %s", name)
	}
	path := filepath.Join(dir, name)
	if sum, err := fileSHA256(path); err == nil && sum == expected {
		return path, nil
	}
	if err := downloadTo(ventoyBase+name, path+".part"); err != nil {
		return "", err
	}
	sum, err := fileSHA256(path + ".part")
	if err != nil {
		return "", err
	}
	if sum != expected {
		os.Remove(path + ".part")
		return "", errors.New("la suma SHA-256 de Ventoy no coincide con la publicada")
	}
	return path, os.Rename(path+".part", path)
}

// * fileSHA256 devuelve el SHA-256 de un archivo en hexadecimal.
func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// * download descarga un archivo pequeño por HTTPS en memoria (como mucho 64 MB).
func download(url string) ([]byte, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("no se pudo descargar %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s respondió %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

// * downloadTo descarga un archivo por HTTPS directamente a disco (sin cargarlo entero en memoria).
func downloadTo(url, path string) error {
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("no se pudo descargar %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s respondió %d", url, resp.StatusCode)
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	written, err := io.Copy(out, io.LimitReader(resp.Body, maxVentoyBytes+1))
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err == nil && written > maxVentoyBytes {
		err = errors.New("el archivo de Ventoy es más grande de lo esperado")
	}
	if err != nil {
		os.Remove(path)
	}
	return err
}

// * lastLines devuelve las últimas n líneas de un texto, en una sola línea.
func lastLines(text string, n int) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}

// * copyFile copia un archivo pequeño (la propia aplicación).
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o755)
}

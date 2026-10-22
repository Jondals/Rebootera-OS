// * Package release: descarga la última versión de Rebootera OS publicada
// * (la ISO de la "release" más reciente en GitHub) y comprueba su SHA-256,
// * que se publica junto a la ISO. Lo usan "Crear USB" y "Actualizar" de
// * Rebootera Desktop para que no haga falta tener la ISO a mano.
// ! Solo HTTPS, y la ISO solo se da por buena si su SHA-256 coincide con el
// ! publicado; una descarga a medias nunca se confunde con una completa.
package release

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Nombres de los archivos publicados en la release "latest".
const (
	ISOName = "rebootera-os-amd64.iso"
	ExeName = "Rebootera-Desktop-Windows.exe"
)

// BaseURL es la carpeta de descargas de la última release (se cambia en pruebas).
var BaseURL = "https://github.com/Jondals/Rebootera-OS/releases/latest/download/"

// UserAgent identifica a Rebootera ante GitHub.
var UserAgent = "RebooteraDesktop"

// Client descarga los archivos; sin tiempo máximo total porque la ISO pesa más de 1 GB.
var Client = &http.Client{Transport: &http.Transport{
	ResponseHeaderTimeout: 30 * time.Second,
	TLSHandshakeTimeout:   15 * time.Second,
}}

// Progress recibe el avance de 0 a 1.
type Progress func(fraction float64)

// maxISOBytes es el tamaño máximo que se acepta (la ISO real ronda 1,2 GB).
const maxISOBytes = 4 << 30

// * get abre una URL HTTPS y comprueba la respuesta.
func get(rawURL string) (*http.Response, error) {
	return getContext(context.Background(), rawURL)
}

// * getContext es get con un contexto que permite cortar la conexión.
func getContext(ctx context.Context, rawURL string) (*http.Response, error) {
	if !strings.HasPrefix(rawURL, "https://") {
		return nil, fmt.Errorf("solo se admiten descargas HTTPS: %s", rawURL)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar con %s: %w", rawURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return nil, errors.New("todavía no hay una versión publicada de Rebootera OS para descargar")
		}
		return nil, fmt.Errorf("%s respondió %d", rawURL, resp.StatusCode)
	}
	if resp.Request.URL.Scheme != "https" {
		resp.Body.Close()
		return nil, fmt.Errorf("%s redirigió a una conexión sin cifrar", rawURL)
	}
	return resp, nil
}

// * open abre una URL HTTPS desde offset (con Range) y acepta 200, 206 y 416; el resto son errores.
func open(ctx context.Context, rawURL string, offset int64) (*http.Response, error) {
	if offset == 0 {
		return getContext(ctx, rawURL)
	}
	if !strings.HasPrefix(rawURL, "https://") {
		return nil, fmt.Errorf("solo se admiten descargas HTTPS: %s", rawURL)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
	resp, err := Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar con %s: %w", rawURL, err)
	}
	if resp.Request.URL.Scheme != "https" {
		resp.Body.Close()
		return nil, fmt.Errorf("%s redirigió a una conexión sin cifrar", rawURL)
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusPartialContent, http.StatusRequestedRangeNotSatisfiable:
		return resp, nil
	}
	resp.Body.Close()
	return nil, fmt.Errorf("%s respondió %d", rawURL, resp.StatusCode)
}

// * ExpectedHash lee el SHA-256 publicado junto a la ISO ("hash  archivo").
func ExpectedHash() (string, error) {
	resp, err := get(BaseURL + ISOName + ".sha256")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 || len(fields[0]) != 64 {
		return "", errors.New("el SHA-256 publicado no es válido")
	}
	if _, err := hex.DecodeString(fields[0]); err != nil {
		return "", errors.New("el SHA-256 publicado no es válido")
	}
	return strings.ToLower(fields[0]), nil
}

// * hashFile calcula el SHA-256 de un archivo.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// * Fetch deja la última ISO de Rebootera OS en dir y devuelve su ruta. Si ya
// * hay una copia con el SHA-256 publicado, no vuelve a descargarla.
func Fetch(dir string, progress Progress, ctl *Control) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	final := filepath.Join(dir, ISOName)
	if _, err := FetchTo(final, progress, ctl); err != nil {
		return "", err
	}
	return final, nil
}

// * FetchTo descarga la última ISO publicada directamente en final (puede ser el propio USB) y devuelve
// * su SHA-256. Se escribe en final+".part" y solo se renombra cuando el SHA-256 coincide con el publicado;
// * si la descarga se corta, se reanuda donde quedó. Si final ya tiene el SHA-256 publicado, no descarga nada.
// * Con ctl se puede pausar (se corta la conexión y se reanuda con Range) o cancelar (se borra lo descargado).
func FetchTo(final string, progress Progress, ctl *Control) (string, error) {
	expected, err := ExpectedHash()
	if err != nil {
		return "", err
	}
	if have, err := hashFile(final); err == nil && have == expected {
		if progress != nil {
			progress(1)
		}
		return expected, nil
	}
	part := final + ".part"
	hasher := sha256.New()
	var offset int64
	if existing, err := os.Open(part); err == nil {
		offset, err = io.Copy(hasher, existing)
		existing.Close()
		if err != nil {
			offset = 0
			hasher.Reset()
		}
	}
	out, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	complete, err := fetchLoop(out, hasher, offset, progress, ctl)
	if err != nil {
		out.Close()
		if errors.Is(err, ErrCancelled) {
			os.Remove(part)
		}
		return "", err
	}
	if complete {
		if err := out.Sync(); err != nil {
			out.Close()
			return "", err
		}
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	return finishFetch(part, final, expected, hasher, progress)
}

// * fetchLoop descarga en out desde offset, reconectando tras cada pausa. Devuelve true si escribió datos
// * nuevos hasta el final (false si lo que había ya era el archivo entero).
func fetchLoop(out *os.File, hasher hash.Hash, offset int64, progress Progress, ctl *Control) (bool, error) {
	done := offset
	for {
		if err := ctl.hold(); err != nil {
			return false, err
		}
		ctx, release := ctl.connect()
		resp, err := open(ctx, BaseURL+ISOName, done)
		if err != nil {
			release()
			if ctl.interrupted() {
				continue
			}
			return false, err
		}
		switch resp.StatusCode {
		case http.StatusPartialContent:
		case http.StatusRequestedRangeNotSatisfiable:
			// * Lo que había ya era el archivo entero: solo falta comprobarlo.
			resp.Body.Close()
			release()
			return false, nil
		default:
			// * El servidor no admite reanudar: se empieza de cero.
			done = 0
			hasher.Reset()
			if err := out.Truncate(0); err != nil {
				resp.Body.Close()
				release()
				return false, err
			}
		}
		total := done + resp.ContentLength
		if resp.ContentLength < 0 {
			total = 0
		}
		if total > maxISOBytes {
			resp.Body.Close()
			release()
			return false, errors.New("la imagen publicada es mayor de lo esperado")
		}
		if _, err := out.Seek(done, io.SeekStart); err != nil {
			resp.Body.Close()
			release()
			return false, err
		}
		finished, err := pipe(resp.Body, out, hasher, &done, total, progress, ctl)
		resp.Body.Close()
		release()
		switch {
		case err != nil:
			return false, err
		case finished:
			return true, nil
		}
		// * Se cortó por una pausa o una cancelación: se conserva lo descargado y se vuelve a empezar el bucle.
		out.Sync()
	}
}

// * pipe copia el cuerpo de la respuesta al archivo. Devuelve true al llegar al final y false si la pausa
// * o la cancelación cortaron la conexión antes.
func pipe(body io.Reader, out *os.File, hasher hash.Hash, done *int64, total int64, progress Progress, ctl *Control) (bool, error) {
	buf := make([]byte, 4<<20)
	for {
		n, readErr := body.Read(buf)
		if n > 0 {
			if *done += int64(n); *done > maxISOBytes {
				out.Close()
				os.Remove(out.Name())
				return false, errors.New("la imagen publicada es mayor de lo esperado")
			}
			hasher.Write(buf[:n])
			if _, err := out.Write(buf[:n]); err != nil {
				return false, fmt.Errorf("no se pudo guardar la descarga: %w", err)
			}
			if progress != nil && total > 0 {
				progress(float64(*done) / float64(total))
			}
		}
		if readErr == io.EOF {
			return true, nil
		}
		if readErr != nil {
			if ctl.interrupted() {
				return false, nil
			}
			// * Se conserva lo descargado: al reintentar se reanuda.
			return false, fmt.Errorf("la descarga se interrumpió: %w", readErr)
		}
		// * Una pausa pedida justo cuando ya había datos de sobra se atiende en el siguiente trozo.
		if ctl.interrupted() {
			return false, nil
		}
	}
}

// * finishFetch comprueba el SHA-256 de lo descargado y lo deja en su sitio.
func finishFetch(part, final, expected string, hasher hash.Hash, progress Progress) (string, error) {
	if got := hex.EncodeToString(hasher.Sum(nil)); got != expected {
		os.Remove(part)
		return "", errors.New("el SHA-256 de la descarga no coincide con el publicado: se ha borrado")
	}
	os.Remove(final)
	if err := os.Rename(part, final); err != nil {
		return "", err
	}
	if progress != nil {
		progress(1)
	}
	return expected, nil
}

// * CacheDir es la carpeta donde se guarda la ISO descargada (nunca el USB).
func CacheDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "Rebootera", "downloads")
}

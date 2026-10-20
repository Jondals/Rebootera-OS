//go:build darwin

// * Preparar un disco USB en macOS. Ventoy no tiene instalador para macOS, así que se reproduce su disposición con
// * las herramientas del sistema y las imágenes del paquete oficial de Ventoy (verificado):
// *   1. diskutil crea una tabla GPT con la partición de datos exFAT REBOOTERA y, al final, una partición de 32 MiB
// *      de tipo "EFI System".
// *   2. La imagen de arranque de Ventoy (ventoy.disk.img) se escribe tal cual en esa segunda partición.
// *   3. La partición de datos se monta en /Volumes/REBOOTERA.
// ! Este USB arranca en equipos con UEFI (la inmensa mayoría desde 2012); el arranque BIOS clásico necesita el
// ! instalador de Ventoy para Windows o Linux. Todo lo que borra lo hace un único script con permisos de administrador
// ! (osascript pide la contraseña) sobre un disco externo por USB que se leyó de diskutil.
package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ulikunitz/xz"
)

// plistJSON ejecuta una orden de diskutil con -plist y devuelve su salida convertida a JSON con plutil.
func plistJSON(args ...string) ([]byte, error) {
	raw, err := exec.Command("diskutil", args...).Output()
	if err != nil {
		return nil, err
	}
	convert := exec.Command("plutil", "-convert", "json", "-o", "-", "-")
	convert.Stdin = strings.NewReader(string(raw))
	return convert.Output()
}

// * USBDisks lista los discos externos conectados por USB.
func USBDisks() ([]USBDisk, error) {
	list, err := plistJSON("list", "-plist", "external", "physical")
	if err != nil {
		return nil, errors.New("no se pudieron listar los discos (diskutil falló)")
	}
	var parsed diskutilList
	if err := json.Unmarshal(list, &parsed); err != nil {
		return nil, err
	}
	infos := map[string][]byte{}
	for _, entry := range parsed.Disks {
		if !diskID.MatchString(entry.ID) {
			continue
		}
		if raw, err := plistJSON("info", "-plist", entry.ID); err == nil {
			infos[entry.ID] = raw
		}
	}
	return usbDisksFromJSON(list, infos)
}

// * prepareDisk deja el disco con la disposición de Ventoy y devuelve el punto de montaje de la partición de datos.
func prepareDisk(j *createJob) (string, error) {
	disk := strings.TrimPrefix(j.disk.Path, "/dev/")
	if !diskID.MatchString(disk) {
		return "", fmt.Errorf("dispositivo no válido: %q", j.disk.Path)
	}
	dataMiB, err := partitionSizeMiB(j.disk.SizeBytes)
	if err != nil {
		return "", err
	}
	setPhase("ventoy-download", 0)
	ventoyDir, err := ensureVentoyTree()
	if err != nil {
		return "", err
	}
	image, err := espImage(ventoyDir)
	if err != nil {
		return "", err
	}
	// ! La ruta va entre comillas simples dentro del script: si trajera una comilla o un salto de línea, se rechaza.
	if strings.ContainsAny(image, string([]rune{39, 10, 13})) { // comilla simple, salto de línea y retorno
		return "", errors.New("la carpeta de datos del usuario tiene un nombre que no se admite (comillas o saltos de línea)")
	}

	setPhase("ventoy-install", 0)
	work, err := os.MkdirTemp("/tmp", "rebootera-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(work)
	script := filepath.Join(work, "install.sh")
	if err := os.WriteFile(script, []byte(macInstallScript(disk, image, dataMiB)), 0o700); err != nil {
		return "", err
	}
	// ! La ruta del script la crea la propia aplicación en /tmp (solo letras, números y guiones): nada de texto de fuera.
	prompt := `do shell script "/bin/sh ` + script + `" with administrator privileges`
	if out, err := exec.Command("osascript", "-e", prompt).CombinedOutput(); err != nil {
		if strings.Contains(string(out), "User canceled") {
			return "", errors.New("se canceló la petición de permisos de administrador")
		}
		return "", fmt.Errorf("no se pudo preparar el USB: %s", lastLines(string(out), 4))
	}

	setPhase("label", 0)
	for attempt := 0; attempt < 30; attempt++ {
		if info, err := os.Stat("/Volumes/REBOOTERA"); err == nil && info.IsDir() {
			return "/Volumes/REBOOTERA", nil
		}
		time.Sleep(time.Second)
	}
	return "", errors.New("macOS no montó la partición REBOOTERA del nuevo USB")
}

// * espImage descomprime la imagen de arranque de Ventoy (la partición de 32 MiB) y devuelve su ruta.
func espImage(ventoyDir string) (string, error) {
	target := filepath.Join(ventoyDir, "ventoy", "ventoy.disk.img")
	if info, err := os.Stat(target); err == nil && info.Size() == espSizeMiB<<20 {
		return target, nil
	}
	file, err := os.Open(target + ".xz")
	if err != nil {
		return "", errors.New("el paquete de Ventoy no trae la imagen de arranque")
	}
	defer file.Close()
	reader, err := xz.NewReader(file)
	if err != nil {
		return "", err
	}
	out, err := os.Create(target + ".part")
	if err != nil {
		return "", err
	}
	written, err := io.Copy(out, io.LimitReader(reader, 64<<20))
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil || written != espSizeMiB<<20 {
		os.Remove(target + ".part")
		return "", fmt.Errorf("la imagen de arranque de Ventoy no tiene el tamaño esperado (%d bytes): %v", written, err)
	}
	return target, os.Rename(target+".part", target)
}

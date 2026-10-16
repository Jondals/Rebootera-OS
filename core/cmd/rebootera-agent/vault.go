//go:build linux

// * Montaje del Vault: la partición exFAT "REBOOTERA" del USB desde el que
// * arrancó Rebootera OS. Se monta en /run/rebootera/vault para el usuario de
// * rebootera-core, sin ejecutables, sin setuid y sin dispositivos.
// * Si Ventoy arrancó la ISO desde esa misma partición, el kernel la tiene
// * reservada para su mapeo y no deja montarla directamente; en ese caso se
// * crea (si no existe) el mapeo lineal /dev/mapper/VentoyPart sobre la
// * partición completa y se monta a través de él, como documenta Ventoy.
// ! Solo se monta un Vault del mismo disco que el medio de arranque: un
// ! segundo USB de Rebootera conectado nunca se monta como Vault.
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
)

// Rutas y nombres del Vault.
const (
	vaultLabel     = "REBOOTERA"
	vaultMount     = "/run/rebootera/vault"
	liveMedium     = "/run/live/medium"
	ventoyPartName = "VentoyPart"
)

// * vaultDirs son las carpetas que se crean en un Vault nuevo.
var vaultDirs = []string{"rebootera/state", "isos/windows", "isos/linux", "isos/tools", "drivers", "tools", "files"}

// * mountVault localiza y monta el Vault. Si no hay Vault (por ejemplo, al
// * arrancar desde un DVD) no es un error: el sistema funciona en solo lectura.
func mountVault(clientUser string) error {
	if isMountPoint(vaultMount) {
		log.Printf("vault: ya montado en %s", vaultMount)
		return nil
	}
	bootDisk, err := bootDiskName()
	if err != nil {
		log.Printf("vault: no se pudo identificar el disco de arranque (%v); sesión de solo lectura", err)
		return nil
	}
	part, err := vaultPartition(bootDisk)
	if err != nil {
		log.Printf("vault: %v; sesión de solo lectura", err)
		return nil
	}

	account, err := user.Lookup(clientUser)
	if err != nil {
		return fmt.Errorf("no existe el usuario %q: %w", clientUser, err)
	}
	options := fmt.Sprintf("uid=%s,gid=%s,fmask=0137,dmask=0027,iocharset=utf8", account.Uid, account.Gid)
	flags := uintptr(syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC | syscall.MS_NOATIME)
	if err := os.MkdirAll(vaultMount, 0o755); err != nil {
		return err
	}

	device := "/dev/" + part
	err = syscall.Mount(device, vaultMount, "exfat", flags, options)
	if errors.Is(err, syscall.EBUSY) {
		// * La partición está reservada por el mapeo de Ventoy: se usa VentoyPart.
		device, err = ensureVentoyPart(part)
		if err == nil {
			err = syscall.Mount(device, vaultMount, "exfat", flags, options)
		}
	}
	if err != nil {
		return fmt.Errorf("no se pudo montar %s en %s: %w", device, vaultMount, err)
	}
	for _, dir := range vaultDirs {
		path := filepath.Join(vaultMount, dir)
		if err := os.MkdirAll(path, 0o750); err != nil {
			log.Printf("vault: no se pudo crear %s: %v", path, err)
		}
	}
	log.Printf("vault: %s montado en %s", device, vaultMount)
	return nil
}

// * bootDiskName devuelve el disco (por ejemplo "sdb") que contiene el medio
// * de arranque, atravesando los mapeos device-mapper de Ventoy si los hay.
func bootDiskName() (string, error) {
	majMin, err := mountSource(liveMedium)
	if err != nil {
		return "", err
	}
	name, err := blockName(majMin)
	if err != nil {
		return "", err
	}
	// * Un dispositivo dm (la ISO mapeada por Ventoy) se resuelve a su partición.
	for strings.HasPrefix(name, "dm-") {
		slaves, _ := os.ReadDir(filepath.Join("/sys/block", name, "slaves"))
		if len(slaves) != 1 {
			return "", fmt.Errorf("%s tiene %d dispositivos debajo", name, len(slaves))
		}
		name = slaves[0].Name()
	}
	return parentDisk(name), nil
}

// * vaultPartition busca en el disco la partición exFAT con la etiqueta del Vault.
func vaultPartition(disk string) (string, error) {
	entries, err := os.ReadDir(filepath.Join("/sys/block", disk))
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if _, err := os.Stat(filepath.Join("/sys/block", disk, entry.Name(), "partition")); err != nil {
			continue
		}
		out, err := exec.Command("blkid", "-o", "export", "/dev/"+entry.Name()).Output()
		if err != nil {
			continue
		}
		props := string(out)
		if strings.Contains(props, "LABEL="+vaultLabel+"\n") && strings.Contains(props, "TYPE=exfat") {
			return entry.Name(), nil
		}
	}
	return "", fmt.Errorf("el disco %s no tiene una partición %s", disk, vaultLabel)
}

// * ensureVentoyPart devuelve el dispositivo del mapeo VentoyPart, creándolo
// * como mapeo lineal de la partición completa si no existe.
// ! En el arranque live, Ventoy crea el mapeo pero udev no siempre crea el
// ! enlace /dev/mapper/VentoyPart; por eso se busca también como /dev/dm-N.
func ensureVentoyPart(part string) (string, error) {
	if node := dmNodeByName(ventoyPartName); node != "" {
		return node, nil
	}
	mapper := "/dev/mapper/" + ventoyPartName
	if _, err := os.Stat(mapper); err == nil {
		return mapper, nil
	}
	sectors := strings.TrimSpace(readFile(filepath.Join("/sys/class/block", part, "size")))
	if sectors == "" {
		return "", fmt.Errorf("no se pudo leer el tamaño de %s", part)
	}
	table := fmt.Sprintf("0 %s linear /dev/%s 0", sectors, part)
	if out, err := exec.Command("dmsetup", "create", ventoyPartName, "--table", table).CombinedOutput(); err != nil {
		return "", fmt.Errorf("dmsetup create %s: %v: %s", ventoyPartName, err, out)
	}
	exec.Command("udevadm", "settle").Run()
	if node := dmNodeByName(ventoyPartName); node != "" {
		return node, nil
	}
	return mapper, nil
}

// * dmNodeByName busca un dispositivo device-mapper por su nombre y devuelve
// * su nodo /dev/dm-N, o "" si no existe.
func dmNodeByName(name string) string {
	matches, _ := filepath.Glob("/sys/block/dm-*/dm/name")
	for _, path := range matches {
		if strings.TrimSpace(readFile(path)) == name {
			node := "/dev/" + filepath.Base(filepath.Dir(filepath.Dir(path)))
			if _, err := os.Stat(node); err == nil {
				return node
			}
		}
	}
	return ""
}

// * mountSource devuelve el "mayor:menor" del dispositivo montado en path.
func mountSource(path string) (string, error) {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 5 && fields[4] == path {
			return fields[2], nil
		}
	}
	return "", fmt.Errorf("%s no está montado", path)
}

// * blockName traduce "mayor:menor" al nombre del dispositivo de bloques.
func blockName(majMin string) (string, error) {
	target, err := os.Readlink("/sys/dev/block/" + majMin)
	if err != nil {
		return "", err
	}
	return filepath.Base(target), nil
}

// * parentDisk devuelve el disco de una partición ("sdb1" da "sdb"); un disco
// * se devuelve tal cual.
func parentDisk(name string) string {
	if _, err := os.Stat(filepath.Join("/sys/class/block", name, "partition")); err != nil {
		return name
	}
	target, err := filepath.EvalSymlinks(filepath.Join("/sys/class/block", name))
	if err != nil {
		return name
	}
	return filepath.Base(filepath.Dir(target))
}

// * isMountPoint indica si path es un punto de montaje.
func isMountPoint(path string) bool {
	_, err := mountSource(path)
	return err == nil
}

// * readFile devuelve el contenido de un archivo o una cadena vacía.
func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

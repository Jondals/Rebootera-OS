//go:build linux

// * Montaje de medios externos para importar imágenes: una partición de otro
// * disco (USB, disco interno con Windows, DVD...) se monta en
// * /run/rebootera/media/<partición> para que Core pueda leer sus ISO.
// ! Siempre en SOLO LECTURA y sin reproducir diarios (ext4 "noload", XFS
// ! "norecovery", btrfs "nologreplay"): montar en lectura un sistema con el
// ! diario pendiente escribiría en el disco. Tampoco hay ejecutables, setuid
// ! ni dispositivos. Se rechaza cualquier partición que ya esté montada (por
// ! ejemplo el Vault o el medio de arranque).
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

// mediaRoot es donde se montan los medios externos.
const mediaRoot = "/run/rebootera/media"

// * partitionPattern limita los nombres aceptados a particiones y unidades ópticas.
var partitionPattern = regexp.MustCompile(`^(sd[a-z]+[0-9]+|nvme[0-9]+n[0-9]+p[0-9]+|mmcblk[0-9]+p[0-9]+|vd[a-z]+[0-9]+|sr[0-9]+)$`)

// * readOnlyOptions son las opciones de montaje por sistema de archivos: el
// * tipo de kernel y sus opciones de solo lectura sin reproducir diarios.
var readOnlyOptions = map[string]struct {
	kernelType string
	options    string
	ownerIDs   bool
}{
	"vfat":    {"vfat", "fmask=0137,dmask=0027,iocharset=utf8", true},
	"exfat":   {"exfat", "fmask=0137,dmask=0027,iocharset=utf8", true},
	"ntfs":    {"ntfs3", "fmask=0137,dmask=0027,iocharset=utf8", true},
	"ext2":    {"ext2", "", false},
	"ext3":    {"ext4", "noload", false},
	"ext4":    {"ext4", "noload", false},
	"xfs":     {"xfs", "norecovery", false},
	"btrfs":   {"btrfs", "rescue=nologreplay", false},
	"iso9660": {"iso9660", "", false},
	"udf":     {"udf", "", false},
}

// * validDevice comprueba el nombre y devuelve "sdb1" a partir de "/dev/sdb1".
func validDevice(device string) (string, error) {
	name := strings.TrimPrefix(device, "/dev/")
	if !partitionPattern.MatchString(name) {
		return "", fmt.Errorf("dispositivo no admitido: %q", device)
	}
	if _, err := os.Stat("/sys/class/block/" + name); err != nil {
		return "", fmt.Errorf("no existe el dispositivo %s", name)
	}
	return name, nil
}

// * mountMedia monta una partición en solo lectura para importar imágenes.
func mountMedia(device, clientUser string) error {
	name, err := validDevice(device)
	if err != nil {
		return err
	}
	target := filepath.Join(mediaRoot, name)
	if isMountPoint(target) {
		return nil
	}
	if inUse(name) {
		return fmt.Errorf("%s ya está en uso por el sistema (por ejemplo, es el USB de Rebootera)", name)
	}
	out, err := exec.Command("blkid", "-o", "value", "-s", "TYPE", "/dev/"+name).Output()
	fsType := strings.TrimSpace(string(out))
	if err != nil || fsType == "" {
		return fmt.Errorf("%s no tiene un sistema de archivos reconocible", name)
	}
	spec, ok := readOnlyOptions[fsType]
	if !ok {
		return fmt.Errorf("el sistema de archivos %s no se puede abrir para importar", fsType)
	}
	options := spec.options
	if spec.ownerIDs {
		account, err := user.Lookup(clientUser)
		if err != nil {
			return err
		}
		ids := "uid=" + account.Uid + ",gid=" + account.Gid
		if options == "" {
			options = ids
		} else {
			options = ids + "," + options
		}
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	flags := uintptr(syscall.MS_RDONLY | syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC | syscall.MS_NOATIME)
	if err := syscall.Mount("/dev/"+name, target, spec.kernelType, flags, options); err != nil {
		os.Remove(target)
		return fmt.Errorf("no se pudo abrir %s (%s): %w", name, fsType, err)
	}
	return nil
}

// * unmountMedia desmonta una partición montada con mountMedia.
func unmountMedia(device string) error {
	name, err := validDevice(device)
	if err != nil {
		return err
	}
	target := filepath.Join(mediaRoot, name)
	if !isMountPoint(target) {
		return nil
	}
	if err := syscall.Unmount(target, 0); err != nil {
		if errors.Is(err, syscall.EBUSY) {
			return fmt.Errorf("%s está en uso (¿hay una copia en curso?)", name)
		}
		return err
	}
	return os.Remove(target)
}

// * inUse indica si la partición ya está montada en algún sitio, directamente
// * o a través de un mapeo device-mapper (como VentoyPart sobre el Vault).
func inUse(name string) bool {
	majMin := strings.TrimSpace(readFile("/sys/class/block/" + name + "/dev"))
	data := readFile("/proc/self/mountinfo")
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		if fields[2] == majMin || dmOnTop(fields[2], name) {
			return true
		}
	}
	return false
}

// * dmOnTop indica si el dispositivo majMin es un mapeo dm sobre la partición name.
func dmOnTop(majMin, name string) bool {
	link, err := os.Readlink("/sys/dev/block/" + majMin)
	if err != nil || !strings.HasPrefix(filepath.Base(link), "dm-") {
		return false
	}
	_, err = os.Stat(filepath.Join("/sys/block", filepath.Base(link), "slaves", name))
	return err == nil
}

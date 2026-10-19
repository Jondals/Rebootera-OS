//go:build linux

// * Creación de una ISO arrancable (BIOS y UEFI, con archivos de más de 4 GiB) con genisoimage en Linux.
package installer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// progressLine es la línea de avance de genisoimage ("12.34% done, estimate finish ...").
var progressLine = regexp.MustCompile(`(\d+(?:\.\d+)?)% done`)

// * isoArgs construye la orden de genisoimage: ISO 9660 + Joliet + UDF (los archivos de más de 4 GiB,
// * como install.wim, solo caben en UDF) y las imágenes de arranque BIOS y UEFI de Windows.
func isoArgs(info Info, dest string) []string {
	args := []string{"-iso-level", "3", "-allow-limited-size", "-udf", "-J", "-joliet-long", "-D", "-N", "-relaxed-filenames", "-V", info.Label}
	if info.BIOSBoot != "" {
		args = append(args, "-b", info.BIOSBoot, "-no-emul-boot", "-boot-load-size", "8")
	}
	if info.UEFIBoot != "" {
		if info.BIOSBoot != "" {
			args = append(args, "-eltorito-alt-boot")
		}
		args = append(args, "-b", info.UEFIBoot, "-no-emul-boot")
	}
	return append(args, "-o", dest, info.Root)
}

// * MakeISO crea una ISO arrancable a partir de la carpeta de un instalador de Windows con genisoimage.
func MakeISO(ctx context.Context, info Info, dest string, reporter Reporter) error {
	tool, err := exec.LookPath("genisoimage")
	if err != nil {
		return errors.New("falta genisoimage en este sistema")
	}
	cmd := exec.CommandContext(ctx, tool, isoArgs(info, dest)...)
	pipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	var tail bytes.Buffer
	scanner := bufio.NewScanner(pipe)
	scanner.Split(splitLines)
	for scanner.Scan() {
		line := scanner.Text()
		if match := progressLine.FindStringSubmatch(line); match != nil {
			if percent, parseErr := strconv.ParseFloat(match[1], 64); parseErr == nil && reporter != nil {
				reporter.Report(int64(percent / 100 * float64(info.Bytes)))
			}
			continue
		}
		tail.WriteString(line + "\n")
		if tail.Len() > 4096 {
			tail.Next(tail.Len() - 4096)
		}
	}
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("genisoimage: %v: %s", err, lastLine(tail.String()))
	}
	if info.UEFIBoot != "" {
		return markEFI(dest, info.BIOSBoot != "")
	}
	return nil
}

// * markEFI cambia a UEFI (0xEF) el identificador de plataforma de la entrada de arranque UEFI del
// * catálogo El Torito: genisoimage solo sabe escribir entradas x86 y el firmware UEFI busca la 0xEF.
func markEFI(path string, hasBIOS bool) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	// * Descriptor de arranque en el sector 17; en el byte 71 está el sector del catálogo.
	record := make([]byte, 75)
	if _, err := file.ReadAt(record, 17*2048); err != nil || !bytes.Contains(record[:39], []byte("EL TORITO SPECIFICATION")) {
		return errors.New("la ISO no tiene catálogo de arranque")
	}
	catalog := int64(binary.LittleEndian.Uint32(record[71:75])) * 2048
	// * Catálogo: validación (32), entrada inicial (32) y, si hay BIOS, cabecera de sección (32) + entrada.
	offset := int64(1)
	if hasBIOS {
		offset = 65
	}
	if _, err := file.WriteAt([]byte{0xEF}, catalog+offset); err != nil {
		return err
	}
	return nil
}

// * splitLines parte en saltos de línea y en retornos de carro: genisoimage reescribe su línea de avance.
func splitLines(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// * lastLine devuelve la última línea con texto.
func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

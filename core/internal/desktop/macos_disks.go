// * Lógica de macOS que no depende del sistema en el que se compila (lectura de diskutil, tamaños y el script de
// * preparación), para poder probarla en cualquier sitio. La usa create_darwin.go.
package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"rebootera/core/internal/storage"
)

// diskID es el identificador que se acepta de un disco de macOS: disk4, disk12...
var diskID = regexp.MustCompile(`^disk[0-9]{1,3}$`)

// espSizeMiB es el tamaño de la partición de arranque de Ventoy.
const espSizeMiB = 32

// diskutilList es lo que se lee de "diskutil list -plist external physical".
type diskutilList struct {
	Disks []struct {
		ID         string  `json:"DeviceIdentifier"`
		Size       float64 `json:"Size"`
		Partitions []struct {
			ID    string  `json:"DeviceIdentifier"`
			Name  string  `json:"VolumeName"`
			Mount string  `json:"MountPoint"`
			Size  float64 `json:"Size"`
			Type  string  `json:"Content"`
		} `json:"Partitions"`
	} `json:"AllDisksAndPartitions"`
}

// diskutilInfo es lo que se lee de "diskutil info -plist diskN".
type diskutilInfo struct {
	Name     string `json:"MediaName"`
	Internal bool   `json:"Internal"`
	Bus      string `json:"BusProtocol"`
	System   bool   `json:"SystemImage"`
}

// * usbDisksFromJSON convierte lo que dicen diskutil list y diskutil info en la lista de discos USB ofrecidos.
// * infos da, por disco, su "diskutil info": solo se aceptan discos externos conectados por USB.
func usbDisksFromJSON(list []byte, infos map[string][]byte) ([]USBDisk, error) {
	var parsed diskutilList
	if err := json.Unmarshal(list, &parsed); err != nil {
		return nil, fmt.Errorf("no se pudo leer la lista de discos: %w", err)
	}
	disks := []USBDisk{}
	for _, entry := range parsed.Disks {
		if !diskID.MatchString(entry.ID) {
			continue
		}
		var info diskutilInfo
		if raw, ok := infos[entry.ID]; !ok || json.Unmarshal(raw, &info) != nil {
			continue
		}
		if info.Internal || info.System || info.Bus != "USB" {
			continue
		}
		number, _ := strconv.Atoi(strings.TrimPrefix(entry.ID, "disk"))
		name := info.Name
		if name == "" {
			name = entry.ID
		}
		disk := USBDisk{Number: number, Path: "/dev/" + entry.ID, FriendlyName: name, SizeBytes: uint64(entry.Size)}
		for _, part := range entry.Partitions {
			if part.Name == "REBOOTERA" {
				disk.Rebootera = true
			}
			volume := USBVolume{Letter: part.Mount, Label: part.Name, SizeBytes: uint64(part.Size)}
			if part.Mount != "" {
				used, free := storage.FSUsage(part.Mount)
				volume.FreeBytes = free
				disk.UsedBytes += used
			}
			disk.Volumes = append(disk.Volumes, volume)
		}
		disks = append(disks, disk)
	}
	return disks, nil
}

// * partitionSizeMiB calcula el tamaño de la partición de datos: todo el disco menos la de arranque y un margen para
// * la tabla GPT y el alineado.
func partitionSizeMiB(diskBytes uint64) (int, error) {
	total := int(diskBytes / (1 << 20))
	data := total - espSizeMiB - 4
	if data < 128 {
		return 0, errors.New("el USB es demasiado pequeño para Rebootera (hace falta al menos 1 GB)")
	}
	return data, nil
}

// * macInstallScript genera el script que prepara el disco (todo lo que borra va aquí). disk y image ya están validados.
func macInstallScript(disk, image string, dataMiB int) string {
	return fmt.Sprintf(`#!/bin/sh
set -e
diskutil unmountDisk force /dev/%[1]s
diskutil partitionDisk /dev/%[1]s GPT ExFAT REBOOTERA %[2]dm '%%C12A7328-F81F-11D2-BA4B-00A0C93EC93B%%' VTOYEFI %[3]dm
dd if='%[4]s' of=/dev/r%[1]ss2 bs=1m
diskutil mountDisk /dev/%[1]s
`, disk, dataMiB, espSizeMiB, image)
}

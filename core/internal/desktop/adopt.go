package desktop

import "strings"

// * adoptable decide si Rebootera puede añadirse a un disco sin borrarlo: Ventoy se instala
// * "sin destruir" y los archivos del usuario quedan donde están, pero Rebootera necesita
// * guardar ahí sus imágenes, así que la primera partición debe ser NTFS o exFAT (FAT32
// * no admite una ISO de Windows de más de 4 GB). Un USB que ya es de Rebootera se actualiza
// * con la otra opción, y un disco vacío no tiene nada que conservar.
func adoptable(disk USBDisk) bool {
	if disk.Rebootera || disk.UsedBytes == 0 || len(disk.Volumes) == 0 {
		return false
	}
	switch strings.ToLower(disk.Volumes[0].FileSystem) {
	case "ntfs", "exfat":
		return true
	}
	return false
}

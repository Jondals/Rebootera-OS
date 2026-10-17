package desktop

// USBDisk es un disco físico conectado por USB.
type USBDisk struct {
	Number       int         `json:"number"`
	FriendlyName string      `json:"friendlyName"`
	SizeBytes    uint64      `json:"sizeBytes"`
	IsSystem     bool        `json:"isSystem"`
	IsBoot       bool        `json:"isBoot"`
	Volumes      []USBVolume `json:"volumes"`
	UsedBytes    uint64      `json:"usedBytes"`
	Rebootera    bool        `json:"rebootera"`
	// Adoptable indica que se puede añadir Rebootera a este disco sin borrar lo que tiene
	// (por ejemplo un USB hecho con Rufus): su primera partición es NTFS o exFAT.
	Adoptable bool `json:"adoptable"`
}

// USBVolume es un volumen con letra dentro de un disco USB.
type USBVolume struct {
	Letter string `json:"letter"`
	Label  string `json:"label"`
	// FileSystem es NTFS, exFAT, FAT32...
	FileSystem string `json:"fileSystem"`
	SizeBytes  uint64 `json:"sizeBytes"`
	FreeBytes  uint64 `json:"freeBytes"`
}

#!/bin/bash
# * usb-add-drivers.sh: copia carpetas de drivers de Windows a la carpeta
# * drivers/ del Vault de una imagen de USB de Rebootera. Sirve para probar en
# * VM el casado de drivers (por ejemplo con los INF reales de virtio-win).
# *
# *   usb-add-drivers.sh USB.img CARPETA_DE_DRIVERS
# !   Solo trabaja con archivos de imagen; nunca con dispositivos reales.
set -euo pipefail
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

# * main monta la imagen y copia los drivers.
main() {
	local img="$1" src="$2" loop mnt
	[ -f "$img" ] && [ -d "$src" ] || {
		echo "Uso: $0 USB.img CARPETA_DE_DRIVERS" >&2
		exit 2
	}
	loop="$(losetup -fP --show "$img")"
	mnt="$(mktemp -d)"
	mount -t exfat "${loop}p1" "$mnt" 2>/dev/null || mount.exfat-fuse "${loop}p1" "$mnt"
	mkdir -p "$mnt/drivers"
	cp -r "$src/." "$mnt/drivers/"
	find "$mnt/drivers" -name '*.inf' | wc -l | sed 's/^/INF copiados: /'
	sync
	umount "$mnt"
	rmdir "$mnt"
	losetup -d "$loop"
}

main "$@"
exit

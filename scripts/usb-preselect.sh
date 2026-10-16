#!/bin/bash
# * usb-preselect.sh: prepara una copia de una imagen de USB de Rebootera con
# * una ISO en la biblioteca y preseleccionada en Ventoy, igual que la deja
# * "Instalar" en el asistente. Sirve para comprobar en VM que Ventoy arranca
# * esa ISO (con scripts/vmtest-boot.py).
# *
# *   usb-preselect.sh USB_ORIGEN.img USB_DESTINO.img ISO CATEGORÍA
# !   Solo trabaja con archivos de imagen; nunca con dispositivos reales.
set -euo pipefail
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

# * main copia la imagen, añade la ISO y edita ventoy.json.
main() {
	local src="$1" dst="$2" iso="$3" category="$4" loop mnt name
	[ -f "$src" ] && [ -f "$iso" ] || {
		echo "Uso: $0 USB_ORIGEN.img USB_DESTINO.img ISO CATEGORÍA" >&2
		exit 2
	}
	cp --sparse=always "$src" "$dst"
	loop="$(losetup -fP --show "$dst")"
	mnt="$(mktemp -d)"
	mount -t exfat "${loop}p1" "$mnt" 2>/dev/null || mount.exfat-fuse "${loop}p1" "$mnt"
	name="$(basename "$iso")"
	mkdir -p "$mnt/isos/$category"
	cp "$iso" "$mnt/isos/$category/$name"
	sed -i "s|\"VTOY_DEFAULT_IMAGE\": \"[^\"]*\"|\"VTOY_DEFAULT_IMAGE\": \"/isos/$category/$name\"|; s|\"VTOY_MENU_TIMEOUT\": \"[^\"]*\"|\"VTOY_MENU_TIMEOUT\": \"10\"|" "$mnt/ventoy/ventoy.json"
	grep -o '"VTOY_DEFAULT_IMAGE": "[^"]*"' "$mnt/ventoy/ventoy.json"
	sync
	umount "$mnt"
	rmdir "$mnt"
	losetup -d "$loop"
}

main "$@"
exit

#!/bin/bash
# * compat-matrix.sh: comprueba qué imágenes oficiales arrancan desde el menú
# * de un USB de Rebootera. Para cada "nombre url sha256": descarga la imagen
# * de su fuente oficial, verifica el SHA-256, la preselecciona en una copia
# * del USB (usb-preselect.sh) y la arranca en QEMU guardando capturas.
# *
# *   compat-matrix.sh MODO USB_BASE.img CARPETA_SALIDA LISTA.txt
# *
# * LISTA.txt tiene una imagen por línea: "nombre url sha256 categoría".
# ! Las imágenes se guardan en /root/rebootera-build/usb/isos y se reutilizan.
set -euo pipefail
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

# * main recorre la lista.
main() {
	local mode="$1" base="$2" out="$3" list="$4" here store name url sha category
	here="$(cd "$(dirname "$0")" && pwd)"
	store=/root/rebootera-build/usb/isos
	mkdir -p "$store" "$out"
	while read -r name url sha category; do
		[ -z "$name" ] || [ "${name#\#}" != "$name" ] && continue
		echo "== $name"
		if ! echo "$sha  $store/$name" | sha256sum -c --status - 2>/dev/null; then
			curl -fsSL -o "$store/$name" "$url"
			echo "$sha  $store/$name" | sha256sum -c - || {
				echo "$name: SHA-256 incorrecto, se omite"
				continue
			}
		fi
		bash "$here/usb-preselect.sh" "$base" "/root/rebootera-build/usb/compat.img" "$store/$name" "$category" >/dev/null
		python3 "$here/vmtest-boot.py" "$mode" /root/rebootera-build/usb/compat.img "$out/$name" 150 30 | tail -1
	done <"$list"
	rm -f /root/rebootera-build/usb/compat.img
}

main "$@"
exit

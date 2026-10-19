#!/bin/bash
# * make-partman-disk.sh: crea un disco virtual de pruebas para el gestor de particiones.
# *
# *   make-partman-disk.sh SALIDA.qcow2
# *
# * Disco GPT de 700 MiB con una partición ext4 (1 a 300 MiB) y una NTFS (300 a 650 MiB), cada
# * una con tres archivos de datos aleatorios y su suma en SUMS. Solo usa archivos de imagen y
# * un dispositivo loop; nunca toca un disco real.
set -euo pipefail
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

OUT="${1:?uso: make-partman-disk.sh SALIDA.qcow2}"
RAW="$(mktemp --suffix=.img)"
LOOP=""
MNT=""

# * cleanup libera el loop y borra lo temporal pase lo que pase.
cleanup() {
	[ -n "$MNT" ] && mountpoint -q "$MNT" && umount "$MNT" || true
	[ -n "$LOOP" ] && losetup -d "$LOOP" 2>/dev/null || true
	rm -f "$RAW"
}
trap cleanup EXIT

# * fill escribe tres archivos de 20 MiB y su suma en la partición montada en $MNT.
fill() {
	local i
	for i in 1 2 3; do
		dd if=/dev/urandom of="$MNT/datos$i.bin" bs=1M count=20 status=none
	done
	(cd "$MNT" && sha256sum datos*.bin >SUMS)
	sync
}

truncate -s 700M "$RAW"
parted -s "$RAW" mklabel gpt mkpart datos ext4 1MiB 300MiB mkpart windows ntfs 300MiB 650MiB >/dev/null
LOOP="$(losetup -fP --show "$RAW")"
udevadm settle 2>/dev/null || true
mkfs.ext4 -F -q "${LOOP}p1"
mkntfs -F -Q -L PRUEBA "${LOOP}p2" >/dev/null 2>&1
MNT="$(mktemp -d)"
mount "${LOOP}p1" "$MNT"
fill
umount "$MNT"
mount -t ntfs-3g "${LOOP}p2" "$MNT"
fill
umount "$MNT"
rmdir "$MNT"
MNT=""
losetup -d "$LOOP"
LOOP=""
rm -f "$OUT"
qemu-img convert -f raw -O qcow2 "$RAW" "$OUT"
echo "disco de pruebas listo: $OUT"

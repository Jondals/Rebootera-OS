#!/bin/bash
# * make-usb.sh: crea un USB de Rebootera (Ventoy + Vault + Rebootera OS).
# *
# *   make-usb.sh --image RUTA.img [--size 16G] --iso REBOOTERA.iso
# *       Crea una imagen de disco (para máquinas virtuales o para grabarla).
# *
# *   make-usb.sh --device /dev/sdX --confirm /dev/sdX --iso REBOOTERA.iso
# *       Escribe en un dispositivo real. --confirm debe repetir exactamente el
# *       mismo dispositivo y el dispositivo debe ser extraíble.
# *
# *   make-usb.sh --image RUTA.img --size 2G --adopt --iso REBOOTERA.iso
# *       Imita "añadir Rebootera a un USB que ya tenía cosas" (hecho con Rufus):
# *       crea un USB MBR con una partición NTFS llena de archivos de usuario y una
# *       partición pequeña tipo UEFI:NTFS, encoge el NTFS como hace Ventoy en Windows
# *       y le instala Ventoy SIN borrar (-n). Los archivos de usuario se conservan y
# *       su suma queda en RUFUS-USER-FILES.sha256, en la raíz, para comprobarla.
# *
# * Resultado: tabla GPT; partición 1 exFAT "REBOOTERA" (el Vault: sistema,
# * imágenes, drivers, herramientas y archivos) y partición 2 VTOYEFI (Ventoy).
# ! Escribir en un dispositivo BORRA TODO SU CONTENIDO. El script se niega con
# ! discos no extraíbles, con el disco del sistema y sin la confirmación.
set -euo pipefail
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

SRC="$(cd "$(dirname "$0")/../.." && pwd)"
LOOP=""
MNT=""

# * log escribe un paso del proceso.
log() {
	printf '\n==> %s\n' "$*"
}

# * die termina con un mensaje de error.
die() {
	echo "Error: $*" >&2
	exit 1
}

# * cleanup desmonta y libera el dispositivo loop pase lo que pase.
cleanup() {
	if [ -n "$MNT" ] && mountpoint -q "$MNT"; then
		umount "$MNT" || true
	fi
	[ -n "$MNT" ] && rmdir "$MNT" 2>/dev/null || true
	if [ -n "$LOOP" ]; then
		losetup -d "$LOOP" || true
	fi
}

# * check_device comprueba que un dispositivo real se puede borrar con seguridad.
check_device() {
	local dev="$1" name root_src
	[ -b "$dev" ] || die "$dev no es un dispositivo de bloques"
	name="$(basename "$dev")"
	[ "$(cat "/sys/block/$name/removable" 2>/dev/null || echo 0)" = "1" ] ||
		readlink -f "/sys/block/$name" | grep -q "/usb" ||
		die "$dev no es extraíble ni USB: Rebootera no lo borrará"
	root_src="$(findmnt -no SOURCE / || true)"
	case "$root_src" in
	"$dev"*) die "$dev contiene el sistema en uso" ;;
	esac
	if lsblk -no MOUNTPOINT "$dev" | grep -q .; then
		die "$dev tiene particiones montadas; desmóntalas primero"
	fi
}

# * populate_vault copia el sistema, la configuración de Ventoy y crea las carpetas.
populate_vault() {
	local part="$1" iso="$2" art
	MNT="$(mktemp -d)"
	# * Driver exFAT del kernel si existe; si no (por ejemplo en WSL), FUSE.
	if [ "$(blkid -o value -s TYPE "$part")" = "ntfs" ]; then
		mount -t ntfs-3g "$part" "$MNT"
	else
		mount -t exfat "$part" "$MNT" 2>/dev/null || mount.exfat-fuse "$part" "$MNT"
	fi
	mkdir -p "$MNT/rebootera/state" "$MNT/isos/windows" "$MNT/isos/linux" "$MNT/isos/tools" \
		"$MNT/drivers" "$MNT/tools" "$MNT/files" "$MNT/ventoy/theme"
	# * Ventoy no busca imágenes en carpetas con .ventoyignore.
	touch "$MNT/drivers/.ventoyignore" "$MNT/tools/.ventoyignore" "$MNT/files/.ventoyignore" "$MNT/rebootera/state/.ventoyignore"

	log "Copiando Rebootera OS ($(du -h "$iso" | cut -f1))"
	cp "$iso" "$MNT/rebootera/rebootera-os.iso"
	sha256sum "$iso" | sed "s|  .*|  rebootera-os.iso|" >"$MNT/rebootera/rebootera-os.iso.sha256"
	basename "$iso" >"$MNT/rebootera/VERSION"

	log "Configurando el menú de arranque"
	cp "$SRC/os/usb/ventoy/ventoy.json" "$MNT/ventoy/ventoy.json"
	cp "$SRC/os/usb/ventoy/theme/theme.txt" "$MNT/ventoy/theme/theme.txt"
	art="$(mktemp -d)"
	bash "$SRC/scripts/build/make-boot-art.sh" "$SRC/branding/logo.svg" "$art/grub" "$art/ply" >/dev/null
	cp "$art/grub/splash.png" "$MNT/ventoy/theme/background.png"
	cp "$art/grub/live-theme/"select_*.png "$art/grub/live-theme/"menu_*.png "$art/grub/live-theme/"*.pf2 "$MNT/ventoy/theme/"
	rm -rf "$art"
	cp "$SRC/os/usb/README.txt" "$MNT/README.txt"
	sync
	umount "$MNT"
	rmdir "$MNT"
	MNT=""
}

# * rufus_like prepara en $LOOP un USB como el que hace Rufus con una ISO de Windows y deja el
# * NTFS encogido y etiquetado para añadirle Rebootera sin borrar nada.
rufus_like() {
	local loop="$1" size_mib="$2" end mnt shrink_mib=48
	end=$((size_mib - 2))
	parted -s "$loop" mklabel msdos mkpart primary ntfs 1MiB "${end}MiB" mkpart primary fat16 "${end}MiB" "$((size_mib - 1))MiB" >/dev/null
	partprobe "$loop" 2>/dev/null || true
	udevadm settle 2>/dev/null || true
	mkntfs -F -Q -L RUFUS "${loop}p1" >/dev/null 2>&1
	mkfs.vfat "${loop}p2" >/dev/null
	mnt="$(mktemp -d)"
	mount -t ntfs-3g "${loop}p1" "$mnt"
	# * Archivos "de Windows 11 LTSC" y propios del usuario; ocupan buena parte de la partición.
	mkdir -p "$mnt/sources" "$mnt/Mis cosas"
	head -c 200000000 /dev/urandom >"$mnt/sources/install.wim"
	head -c 30000000 /dev/urandom >"$mnt/Mis cosas/fotos.bin"
	echo "datos de usuario" >"$mnt/Mis cosas/notas.txt"
	(cd "$mnt" && find . -type f ! -name 'RUFUS-USER-FILES.sha256' -print0 | sort -z | xargs -0 sha256sum >RUFUS-USER-FILES.sha256)
	umount "$mnt"
	rmdir "$mnt"
	# * Como hace Ventoy en Windows: se encoge primero el sistema de archivos y la partición.
	ntfsresize -f -s "$((end - 2 - shrink_mib))M" "${loop}p1" >/dev/null 2>&1 <<<"y"
	# * sfdisk en vez de parted: parted pregunta antes de encoger una partición y -s lo cancela.
	echo ",$(( (end - shrink_mib - 1) * 2048 ))" | sfdisk --no-reread -N 1 "$loop" >/dev/null
	partprobe "$loop" 2>/dev/null || true
	udevadm settle 2>/dev/null || true
	# * ntfsresize deja el volumen marcado para que Windows lo revise; Windows, al encogerlo, no lo marca.
	# * Se quita la marca para imitar lo que haría Windows y que Linux pueda montarlo.
	ntfsfix -d "${loop}p1" >/dev/null 2>&1 || true
}

# * main interpreta las opciones y crea el USB.
main() {
	local image="" device="" confirm="" size="16G" iso="" ventoy target adopt=0
	while [ $# -gt 0 ]; do
		case "$1" in
		--image) image="$2"; shift 2 ;;
		--device) device="$2"; shift 2 ;;
		--confirm) confirm="$2"; shift 2 ;;
		--size) size="$2"; shift 2 ;;
		--iso) iso="$2"; shift 2 ;;
		--adopt) adopt=1; shift ;;
		*) die "opción desconocida: $1" ;;
		esac
	done
	[ "$(id -u)" = "0" ] || die "hace falta ejecutarlo como root"
	[ -f "$iso" ] || die "falta --iso con la imagen de Rebootera OS"
	[ -n "$image" ] || [ -n "$device" ] || die "indica --image o --device"
	trap cleanup EXIT

	if [ -n "$device" ]; then
		[ "$confirm" = "$device" ] || die "para borrar $device repite --confirm $device"
		check_device "$device"
		target="$device"
	else
		log "Creando la imagen $image ($size)"
		rm -f "$image"
		truncate -s "$size" "$image"
		LOOP="$(losetup -fP --show "$image")"
		target="$LOOP"
	fi

	if [ "$adopt" = "1" ]; then
		[ -n "$image" ] || die "--adopt solo se admite con --image"
		log "Creando un USB tipo Rufus con archivos de usuario"
		rufus_like "$target" "$(( $(numfmt --from=iec "$size") / 1048576 ))"
	fi

	log "Obteniendo Ventoy verificado"
	ventoy="$(bash "$SRC/scripts/build/fetch-ventoy.sh" linux | tail -n1)"

	if [ "$adopt" = "1" ]; then
		log "Instalando Ventoy SIN borrar en $target (-n)"
		(cd "$ventoy" && printf 'y\ny\n' | ./Ventoy2Disk.sh -I -n -s "$target") >/dev/null
	else
		log "Instalando Ventoy en $target (GPT, Secure Boot, etiqueta REBOOTERA)"
		(cd "$ventoy" && printf 'y\ny\n' | ./Ventoy2Disk.sh -I -g -s -L REBOOTERA "$target") >/dev/null
	fi
	partprobe "$target" 2>/dev/null || true
	udevadm settle 2>/dev/null || true

	local part1
	case "$target" in
	*loop* | *nvme* | *mmcblk*) part1="${target}p1" ;;
	*) part1="${target}1" ;;
	esac
	# * Rebootera encuentra su Vault por la etiqueta: en un USB adoptado se la pone la aplicación.
	[ "$adopt" = "1" ] && ntfslabel "$part1" REBOOTERA
	populate_vault "$part1" "$iso"
	log "USB de Rebootera listo en ${image:-$device}"
}

main "$@"
exit

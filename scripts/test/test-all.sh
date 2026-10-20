#!/bin/bash
# * test-all.sh: ejecuta las pruebas de Rebootera OS en paralelo, cada una en su
# * propia máquina virtual, y muestra un resumen. Se ejecuta como root en la
# * distro WSL RebooteraBuild:
# *
# *   test-all.sh [--full] [--build] [--no-build]
# *
# *   (sin opciones)  rápido (menos de 3 minutos): arranque en UEFI + Secure Boot de la ISO de
# *                   depuración y de la definitiva, a la vez. Solo recompila una ISO si el código
# *                   fuente cambió desde la última vez que se compiló.
# *   --full          todo: arranque en UEFI + Secure Boot y BIOS, el USB de extremo a extremo y los
# *                   escenarios (Wi-Fi, Debian con preseed, Windows, particiones, adopción), unos 17
# *                   minutos con compilación. Úsalo antes de publicar una versión.
# *   --build         recompila siempre las ISO.
# *   --no-build      no compila: usa las ISO que ya hay (la de depuración en
# *                   /root/rebootera-build/out y la definitiva en out/).
# *
# ! Cada VM usa 1,5 GB y 2 CPU y nunca hay más de tres a la vez (unos 5 GB); el USB y los
# ! escenarios escriben solo en capas temporales de QEMU, nunca en la imagen.
set -uo pipefail
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

SRC="$(cd "$(dirname "$0")/../.." && pwd)"
WORK=/root/rebootera-build
OUT="$WORK/tests/$(date +%Y%m%d-%H%M%S)"
QUICK=1
BUILD=auto
for arg in "$@"; do
	case "$arg" in
	--full) QUICK=0 ;;
	--quick) QUICK=1 ;;
	--build) BUILD=1 ;;
	--no-build) BUILD=0 ;;
	*)
		echo "Opción desconocida: $arg" >&2
		exit 2
		;;
	esac
done
export VMTEST_MEM=1536 VMTEST_SMP=2
mkdir -p "$OUT"
START=$(date +%s)

# * log escribe un paso con la hora transcurrida.
log() {
	printf '[%3ds] %s\n' $(($(date +%s) - START)) "$*"
}

# * source_stamp resume el estado del código fuente que entra en las ISO (rutas, tamaños y fechas).
source_stamp() {
	(cd "$SRC" && find core ui os scripts/build -type f -not -path "*/node_modules/*" -not -path "*/dist/*" -printf "%p %s %T@
" | sort | sha1sum | cut -c1-16)
}

# * build compila las ISO que hagan falta: con BUILD=auto, solo si el código fuente cambió desde la última vez.
build() {
	local stamp kind
	stamp="$(source_stamp)"
	for kind in debug final; do
		if [ "$BUILD" = "auto" ] && [ "$(cat "$WORK/stamp-$kind" 2>/dev/null)" = "$stamp" ]; then
			if [ "$kind" = "debug" ] && ls "$WORK"/out/*debug.iso >/dev/null 2>&1; then continue; fi
			if [ "$kind" = "final" ] && ls "$SRC"/out/rebootera-os-*-amd64.iso >/dev/null 2>&1; then continue; fi
		fi
		log "compilando la ISO ($kind)"
		bash "$SRC/scripts/build/build.sh" $([ "$kind" = "debug" ] && echo --debug) >"$OUT/build-$kind.log" 2>&1 || {
			tail -5 "$OUT/build-$kind.log"
			exit 1
		}
		echo "$stamp" >"$WORK/stamp-$kind"
	done
}

# * prepare_usb crea el USB de pruebas (con drivers y, si hace falta, el disco de usuario).
prepare_usb() {
	local img="$1" drivers="$2"
	bash "$SRC/scripts/usb/make-usb.sh" --image "$img" --size 12G --iso "$DBG" >"$OUT/make-$(basename "$img").log" 2>&1 || return 1
	if [ -n "$drivers" ]; then
		bash "$SRC/scripts/usb/usb-add-drivers.sh" "$img" "$drivers" >>"$OUT/make-$(basename "$img").log" 2>&1
	fi
}

# * prepare_adopt crea el USB tipo Rufus al que se añade Rebootera sin borrar nada.
prepare_adopt() {
	bash "$SRC/scripts/usb/make-usb.sh" --image "$WORK/usb/adopt.img" --size 2G --adopt --iso "$DBG" >"$OUT/make-adopt.img.log" 2>&1
}

# * prepare_partdisk crea el disco con ext4 y NTFS con datos para el gestor de particiones.
prepare_partdisk() {
	bash "$SRC/scripts/test/make-partman-disk.sh" "$WORK/usb/partdisk.qcow2" >"$OUT/make-partdisk.log" 2>&1
}

# * user_disk crea el disco "del usuario" con una ISO de Alpine para la prueba del USB.
user_disk() {
	local alp="$WORK/fixtures/alpine-virt-3.24.2-x86_64.iso" raw="$WORK/usb/user.raw" loop mnt
	mkdir -p "$WORK/fixtures"
	[ -f "$alp" ] || curl -fsSL -o "$alp" https://dl-cdn.alpinelinux.org/alpine/latest-stable/releases/x86_64/alpine-virt-3.24.2-x86_64.iso || return 1
	rm -f "$raw" "$WORK/usb/userdisk.qcow2"
	truncate -s 160M "$raw"
	parted -s "$raw" mklabel msdos mkpart primary 1MiB 100% >/dev/null
	loop="$(losetup -fP --show "$raw")"
	mkfs.exfat -L USERDISK "${loop}p1" >/dev/null
	mnt="$(mktemp -d)"
	mount -t exfat "${loop}p1" "$mnt" 2>/dev/null || mount.exfat-fuse "${loop}p1" "$mnt"
	mkdir -p "$mnt/Descargas/ISOs"
	cp "$alp" "$mnt/Descargas/ISOs/"
	sync
	umount "$mnt"
	rmdir "$mnt"
	losetup -d "$loop"
	qemu-img convert -f raw -O qcow2 "$raw" "$WORK/usb/userdisk.qcow2"
	rm -f "$raw"
}

# * lane ejecuta una orden y guarda su registro; las lanes corren a la vez.
lane() {
	local name="$1"
	shift
	local begin=$(date +%s)
	"$@" >"$OUT/$name.log" 2>&1
	log "lane $name terminada en $(($(date +%s) - begin)) s: $(tail -1 "$OUT/$name.log" | cut -c1-90)"
}

[ "$BUILD" = "0" ] || build
DBG="$(ls "$WORK"/out/*debug.iso 2>/dev/null | head -1)"
FINAL="$(ls "$SRC"/out/rebootera-os-*-amd64.iso 2>/dev/null | head -1)"
[ -n "$DBG" ] || {
	echo "Falta la ISO de depuración (quita --no-build)" >&2
	exit 1
}

# * boots ejecuta, una detrás de otra, las pruebas de arranque de una ISO.
boots() {
	local iso="$1" dir="$2" extra="$3" mode
	shift 3
	for mode in "$@"; do
		python3 "$SRC/scripts/test/vmtest.py" "$mode" "$iso" "$dir" $extra >"$OUT/$(basename "$dir")-$mode.log" 2>&1
	done
	echo "arranques terminados"
}

# * v03 ejecuta, una detrás de otra, escenarios de la 0.3.0 en adelante (cada uno en su VM).
# * El escenario "adopt" arranca el USB tipo Rufus (adopt.img); los demás, v03.img.
v03() {
	local scenario image
	for scenario in "$@"; do
		image="$WORK/usb/v03.img"
		[ "$scenario" = "adopt" ] && image="$WORK/usb/adopt.img"
		# * El escenario de particiones lleva un segundo disco virtual (en una capa temporal de QEMU).
		if [ "$scenario" = "partitions" ]; then export VMTEST_DISK="$WORK/usb/partdisk.qcow2"; else unset VMTEST_DISK; fi
		python3 "$SRC/scripts/test/vmtest-v03.py" "$image" "$OUT/v03" "$scenario" >"$OUT/v03-$scenario.log" 2>&1
	done
	echo "escenarios 0.3.0 terminados"
}

# * usb_y_windows: el USB de extremo a extremo y, después, el escenario de Windows.
usb_y_windows() {
	python3 "$SRC/scripts/test/vmtest-usb.py" uefi "$WORK/usb/usb.img" "$WORK/usb/userdisk.qcow2" "$OUT/usb" >"$OUT/usb-test.log" 2>&1
	v03 windows
}

# * arranques: arranque de las dos ISO en UEFI + Secure Boot y BIOS, y el escenario Wi-Fi.
arranques() {
	boots "$DBG" "$OUT/dbg" "" uefi-sb bios
	[ -n "$FINAL" ] && boots "$FINAL" "$OUT/rel" --release uefi-sb bios
	v03 wifi adopt
}

# * Tres máquinas virtuales a la vez como máximo (unos 5 GB en total): el
# * USB de extremo a extremo, Debian con preseed y el resto en las otras dos.
pids=()
if [ "$QUICK" = "1" ]; then
	log "modo rápido"
	lane debug boots "$DBG" "$OUT/dbg" "" uefi-sb &
	pids+=($!)
	[ -n "$FINAL" ] && {
		lane release boots "$FINAL" "$OUT/rel" --release uefi-sb &
		pids+=($!)
	}
else
	log "preparando los USB de pruebas"
	mkdir -p "$WORK/usb"
	rm -f "$WORK/usb/"*.img
	prepare_usb "$WORK/usb/usb.img" "" &
	p1=$!
	prepare_usb "$WORK/usb/v03.img" "$WORK/fixtures/drivers" &
	p2=$!
	user_disk &
	p3=$!
	prepare_adopt &
	p4=$!
	prepare_partdisk &
	p5=$!
	wait $p1 $p2 $p3 $p4 $p5
	log "lanzando las pruebas (3 VM a la vez)"
	lane usb usb_y_windows &
	pids+=($!)
	lane debian v03 debian partitions &
	pids+=($!)
	lane arranques arranques &
	pids+=($!)
fi
wait "${pids[@]}"
# * Libera la caché de disco que han llenado las imágenes (WSL se la devuelve a Windows).
sync
echo 1 >/proc/sys/vm/drop_caches 2>/dev/null || true

# * Resumen: cuenta las comprobaciones de cada archivo de resultados.
log "resumen"
fails=0
while IFS= read -r file; do
	ok=$(grep -c '^OK' "$file")
	bad=$(grep -c '^FALLO' "$file")
	printf '  %-70s %3d OK %2d fallos\n' "${file#$OUT/}" "$ok" "$bad"
	fails=$((fails + bad))
done < <(find "$OUT" -name '*resultado.txt' | sort)
# * Una lane que se cae antes de escribir su resultado (por ejemplo, QEMU no arranca) también cuenta.
crashed=$(grep -l 'Traceback' "$OUT"/*.log 2>/dev/null | wc -l)
fails=$((fails + crashed))
log "terminado en $(($(date +%s) - START)) s; fallos: $fails (lanes caídas: $crashed); registros en $OUT"
[ "$fails" = "0" ]

#!/bin/bash
# * test-create-linux.sh: prueba de verdad "Crear USB" en Linux con un disco virtual (un archivo conectado como
# * dispositivo loop), sin tocar ningún disco del equipo. Compila Rebootera Desktop, lo arranca sin ventana, crea el
# * USB por su API y comprueba el resultado: etiqueta REBOOTERA, partición de arranque de Ventoy, Rebootera OS dentro
# * y la estructura del USB. Se ejecuta como root en la distro WSL RebooteraBuild:
# *
# *   wsl -d RebooteraBuild -u root -- bash "/mnt/d/dev/Rebootera OS/scripts/test/test-create-linux.sh"
# ! REBOOTERA_TEST_ALLOW_LOOP=1 hace que la aplicación trate un dispositivo loop como un USB: solo existe para esta prueba.
set -uo pipefail
export PATH=/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
export GOCACHE=/root/.cache/go-build GOFLAGS=-buildvcs=false CGO_ENABLED=0

SRC="$(cd "$(dirname "$0")/../.." && pwd)"
APP=/root/rebootera-build/desktop/src/core/cmd/rebootera-desktop
T=/root/rebootera-build/createtest
LOOP=""
MNT=""
PID=""
FAILS=0

# * check escribe una comprobación.
check() {
	if [ "$2" = "ok" ]; then echo "OK    $1"; else echo "FALLO $1 ($2)"; FAILS=$((FAILS + 1)); fi
}

# * cleanup desmonta y libera todo pase lo que pase.
cleanup() {
	[ -n "$PID" ] && kill "$PID" 2>/dev/null
	[ -n "$MNT" ] && umount "$MNT" 2>/dev/null
	for p in $(lsblk -nrpo NAME,MOUNTPOINT "$LOOP" 2>/dev/null | awk 'NF > 1 { print $2 }'); do umount "$p" 2>/dev/null; done
	[ -n "$LOOP" ] && losetup -d "$LOOP" 2>/dev/null
	rm -rf "$T/mnt"
}
trap cleanup EXIT

[ "$(id -u)" = "0" ] || { echo "hace falta root"; exit 2; }
[ -d "$APP/ui" ] && [ -d "$APP/kit" ] || { echo "falta compilar antes con scripts/build/build-desktop.sh (ui y kit)"; exit 2; }

mkdir -p "$T"
rsync -a --delete --exclude 'cmd/rebootera-desktop/ui' --exclude 'cmd/rebootera-desktop/kit' "$SRC/core/" "$T/core/"
cp -a "$APP/ui" "$APP/kit" "$T/core/cmd/rebootera-desktop/"
(cd "$T/core" && go build -o "$T/rebootera-desktop" ./cmd/rebootera-desktop) || { echo "no compila"; exit 2; }

head -c 40000000 /dev/urandom >"$T/fake.iso"
rm -f "$T/disk.img"
truncate -s 4G "$T/disk.img"
LOOP="$(losetup -fP --show "$T/disk.img")"
echo "disco virtual: $LOOP"

rm -f "$T/url"
REBOOTERA_TEST_ALLOW_LOOP=1 "$T/rebootera-desktop" -no-window -url-file "$T/url" >"$T/app.log" 2>&1 &
PID=$!
for _ in $(seq 1 50); do [ -s "$T/url" ] && break; sleep 0.2; done
URL="$(cat "$T/url")"
BASE="${URL%%/auth*}"
curl -s -c "$T/jar" -o /dev/null "$URL"
api() { curl -s -b "$T/jar" -H 'X-Rebootera: 1' -H 'Content-Type: application/json' "$@"; }

DISKS="$(api "$BASE/api/desktop/disks?fresh=1")"
NUM="$(echo "$DISKS" | grep -o "\"number\":[0-9]*,\"path\":\"$LOOP\"" | grep -o '[0-9]*,' | tr -d ,)"
[ -n "$NUM" ] && check "el disco virtual aparece en la lista" ok || check "el disco virtual aparece en la lista" "$DISKS"
[ -n "$NUM" ] || exit 1
echo "$DISKS" | grep -q "\"path\":\"$(findmnt -no SOURCE / | sed 's/p\?[0-9]*$//')\"" && check "el disco del sistema NO aparece" "aparece" || check "el disco del sistema NO aparece" ok

# * Una orden sin la confirmación se rechaza.
BAD="$(api -X POST "$BASE/api/desktop/create" -d "{\"disk\":$NUM,\"iso\":\"$T/fake.iso\",\"confirm\":\"nada\"}")"
echo "$BAD" | grep -q error && check "sin confirmación se rechaza" ok || check "sin confirmación se rechaza" "$BAD"
# * Un disco que no existe se rechaza.
BAD="$(api -X POST "$BASE/api/desktop/create" -d "{\"disk\":999,\"iso\":\"$T/fake.iso\",\"confirm\":\"ERASE DISK 999\"}")"
echo "$BAD" | grep -q error && check "un disco que no es USB se rechaza" ok || check "un disco que no es USB se rechaza" "$BAD"

START="$(api -X POST "$BASE/api/desktop/create" -d "{\"disk\":$NUM,\"iso\":\"$T/fake.iso\",\"confirm\":\"ERASE DISK $NUM\"}")"
echo "$START" | grep -q running && check "la creación empieza" ok || { check "la creación empieza" "$START"; exit 1; }
for _ in $(seq 1 300); do
	STATUS="$(api "$BASE/api/desktop/create")"
	echo "$STATUS" | grep -q '"running":false' && break
	sleep 2
done
echo "estado final: $STATUS"
echo "$STATUS" | grep -q '"done":true' && ! echo "$STATUS" | grep -q '"error"' && check "la creación termina sin error" ok || check "la creación termina sin error" "$STATUS"

udevadm settle
LAYOUT="$(lsblk -nrpo NAME,LABEL,FSTYPE,SIZE "$LOOP")"
echo "$LAYOUT"
echo "$LAYOUT" | grep -q "REBOOTERA exfat" && check "partición de datos exFAT REBOOTERA" ok || check "partición de datos exFAT REBOOTERA" "$LAYOUT"
echo "$LAYOUT" | grep -q VTOYEFI && check "partición de arranque de Ventoy (VTOYEFI)" ok || check "partición de arranque de Ventoy (VTOYEFI)" "$LAYOUT"
[ "$(parted -s "$LOOP" print 2>/dev/null | grep -c 'Partition Table: gpt')" = "1" ] && check "tabla de particiones GPT" ok || check "tabla de particiones GPT" "no es GPT"

ROOT="$(echo "$STATUS" | grep -o '"drive":"[^"]*"' | cut -d'"' -f4)"
[ -n "$ROOT" ] && [ -f "$ROOT/rebootera/rebootera-os.iso" ] && cmp -s "$ROOT/rebootera/rebootera-os.iso" "$T/fake.iso" && check "Rebootera OS copiado byte a byte" ok || check "Rebootera OS copiado byte a byte" "$ROOT"
for f in ventoy/ventoy.json ventoy/theme/theme.txt rebootera/state isos/windows drivers README.txt rebootera/VERSION; do
	[ -e "$ROOT/$f" ] && check "existe $f" ok || check "existe $f" "falta"
done
grep -q '"VTOY_DEFAULT_IMAGE"' "$ROOT/ventoy/ventoy.json" && check "ventoy.json con la imagen por defecto" ok || check "ventoy.json con la imagen por defecto" "falta"
echo "Fallos: $FAILS"
[ "$FAILS" = "0" ]

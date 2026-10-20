#!/bin/bash
# * preview-ui.sh: captura la interfaz de Rebootera Desktop (o de Rebootera OS) SIN abrir ninguna ventana
# * en el PC: compila la interfaz, arranca el servidor local en WSL y la dibuja con el mismo WebKitGTK
# * que usa la imagen, dentro de un servidor X virtual. Las capturas quedan en la carpeta de salida.
# *
# *   preview-ui.sh desktop|live SALIDA [ANCHO ALTO] [RUTA...]
# *
# * Ejemplo: preview-ui.sh desktop /tmp/capturas 1440 900 "#/downloads" "#/create"
set -euo pipefail
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/local/go/bin
SRC="$(cd "$(dirname "$0")/../.." && pwd)"
MODE="${1:?modo: desktop o live}"
OUT="${2:?carpeta de salida}"
WIDTH="${3:-1440}"
HEIGHT="${4:-900}"
shift 4 2>/dev/null || shift $#
WORK=/root/rebootera-build/preview
export GOCACHE=/root/.cache/go-build GOFLAGS=-buildvcs=false CGO_ENABLED=0

# * pnpm usa la versión fijada en ui/package.json mediante corepack.
pnpm() {
	local version entry
	version="$(sed -n 's/.*"packageManager": "pnpm@\([^"]*\)".*/\1/p' "$SRC/ui/package.json")"
	entry="$HOME/.cache/node/corepack/pnpm/$version/bin/pnpm.mjs"
	[ -f "$entry" ] || COREPACK_ENABLE_DOWNLOAD_PROMPT=0 corepack prepare "pnpm@$version" >/dev/null
	COREPACK_ENABLE_DOWNLOAD_PROMPT=0 node "$entry" "$@"
}

mkdir -p "$WORK" "$OUT"
rsync -a --delete --exclude node_modules --exclude dist --exclude out "$SRC/" "$WORK/src/"
(cd "$WORK/src/ui" && pnpm install --frozen-lockfile --reporter=silent && pnpm run build >/dev/null)

TOKEN_FILE="$WORK/token"
URL_FILE="$WORK/url"
rm -f "$TOKEN_FILE" "$URL_FILE"
if [ "$MODE" = "desktop" ]; then
	app="$WORK/src/core/cmd/rebootera-desktop"
	rm -rf "$app/ui" "$app/kit"
	mkdir -p "$app/ui" "$app/kit"
	cp -a "$WORK/src/ui/dist/." "$app/ui/"
	cp "$WORK/src/os/usb/README.txt" "$app/kit/README.txt"
	# * Un "USB de Rebootera" falso con unas imágenes y drivers, para que las páginas tengan contenido.
	FAKE="$WORK/fake-usb"
	rm -rf "$FAKE"
	mkdir -p "$FAKE/rebootera/state" "$FAKE/isos/windows" "$FAKE/isos/linux" "$FAKE/isos/tools" "$FAKE/drivers/intel-red"
	echo iso >"$FAKE/rebootera/rebootera-os.iso"
	echo rebootera-os-preview-amd64.iso >"$FAKE/rebootera/VERSION"
	echo x >"$FAKE/isos/linux/debian-13.1.0-amd64-netinst.iso"
	printf '[Version]
Signature="$WINDOWS NT$"
Class=Net
Provider=%%Intel
DriverVer=03/01/2024,12.19.0
CatalogFile=e1d.cat
[Strings]
Intel="Intel"
' >"$FAKE/drivers/intel-red/e1d.inf"
	printf '[Version]
Signature="$WINDOWS NT$"
Class=SCSIAdapter
Provider=%%Intel
DriverVer=02/01/2024,19.5.0
[Strings]
Intel="Intel"
' >"$FAKE/drivers/intel-red/iaStorVD.inf"
	(cd "$WORK/src/core" && go build -o "$WORK/rebootera-desktop" ./cmd/rebootera-desktop)
	"$WORK/rebootera-desktop" -no-window -url-file "$URL_FILE" -usb-root "$FAKE" &
else
	(cd "$WORK/src/core" && go build -o "$WORK/rebootera-core" ./cmd/rebootera-core)
	"$WORK/rebootera-core" -listen 127.0.0.1:7081 -ui "$WORK/src/ui/dist" -token-file "$TOKEN_FILE" &
fi
SERVER=$!
trap 'kill $SERVER 2>/dev/null || true' EXIT
for _ in $(seq 1 60); do
	[ -s "$URL_FILE" ] || [ -s "$TOKEN_FILE" ] && break
	sleep 0.2
done
if [ "$MODE" = "desktop" ]; then
	URL="$(cat "$URL_FILE")"
else
	URL="http://127.0.0.1:7081/auth?token=$(cat "$TOKEN_FILE")"
fi
xvfb-run -a -s "-screen 0 ${WIDTH}x${HEIGHT}x24" python3 "$SRC/scripts/dev/ui-preview.py" "$URL" "$OUT/preview.png" "$WIDTH" "$HEIGHT" 9000 "$@"
ls -la "$OUT"

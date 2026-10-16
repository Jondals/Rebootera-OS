#!/bin/bash
# * build-desktop.sh: construye Rebootera Desktop para Windows (x64).
# *   1. Compila la interfaz con pnpm.
# *   2. Prepara el contenido que va dentro del ejecutable: la interfaz (ui/) y
# *      el kit de un USB nuevo (kit/: configuración y tema de Ventoy, README).
# *   3. Compila "Rebootera Desktop.exe" sin consola (-H windowsgui).
# * Se ejecuta en la distro WSL RebooteraBuild (no necesita root):
# *
# *   wsl -d RebooteraBuild --exec bash "/mnt/d/dev/Rebootera OS/scripts/build-desktop.sh"
# *
# * El ejecutable queda en out/.
set -euo pipefail
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

# * log escribe un paso del proceso.
log() {
	printf '\n==> %s\n' "$*"
}

# * pnpm usa la versión fijada en ui/package.json mediante corepack.
pnpm() {
	local version entry
	version="$(sed -n 's/.*"packageManager": "pnpm@\([^"]*\)".*/\1/p' "$SRC/ui/package.json")"
	entry="$HOME/.cache/node/corepack/pnpm/$version/bin/pnpm.mjs"
	if [ ! -f "$entry" ]; then
		COREPACK_ENABLE_DOWNLOAD_PROMPT=0 corepack prepare "pnpm@$version" >/dev/null
	fi
	COREPACK_ENABLE_DOWNLOAD_PROMPT=0 node "$entry" "$@"
}

# * main construye el ejecutable.
main() {
	SRC="$(cd "$(dirname "$0")/.." && pwd)"
	local work=/root/rebootera-build/desktop version app kit
	version="$(sed -n 's/^var Version = "\(.*\)"/\1/p' "$SRC/core/internal/version/version.go")"
	mkdir -p "$work"
	rsync -a --delete --exclude node_modules --exclude dist --exclude out "$SRC/" "$work/src/"

	log "Compilando la interfaz"
	cd "$work/src/ui"
	pnpm install --frozen-lockfile --reporter=silent
	pnpm run build

	log "Preparando la interfaz y el kit del USB"
	app="$work/src/core/cmd/rebootera-desktop"
	rm -rf "$app/ui" "$app/kit"
	mkdir -p "$app/ui" "$app/kit/ventoy/theme"
	cp -a "$work/src/ui/dist/." "$app/ui/"
	kit="$app/kit"
	cp "$work/src/os/usb/ventoy/ventoy.json" "$kit/ventoy/ventoy.json"
	cp "$work/src/os/usb/ventoy/theme/theme.txt" "$kit/ventoy/theme/theme.txt"
	cp "$work/src/os/usb/README.txt" "$kit/README.txt"
	bash "$work/src/scripts/make-boot-art.sh" "$work/src/branding/logo.svg" "$work/art/grub" "$work/art/ply" 2>/dev/null
	cp "$work/art/grub/splash.png" "$kit/ventoy/theme/background.png"
	cp "$work/art/grub/live-theme/"select_*.png "$work/art/grub/live-theme/"*.pf2 "$kit/ventoy/theme/"

	log "Generando el icono y los recursos de Windows"
	export GOCACHE=/root/.cache/go-build GOFLAGS=-buildvcs=false CGO_ENABLED=0
	local size icons=()
	mkdir -p "$work/icon"
	for size in 16 20 24 32 40 48 64 128 256; do
		rsvg-convert -w "$size" -h "$size" "$work/src/branding/logo.svg" -o "$work/icon/$size.png"
		icons+=("$work/icon/$size.png")
	done
	convert "${icons[@]}" "$work/icon/rebootera.ico"
	# * rsrc da el ID 1 al manifiesto y el 2 al grupo de iconos (window_windows.go usa el 2).
	(cd "$app" && go run github.com/akavel/rsrc@v0.10.2 -arch amd64 \
		-manifest rebootera-desktop.manifest -ico "$work/icon/rebootera.ico" -o rsrc_windows_amd64.syso)

	log "Compilando Rebootera Desktop $version para Windows"
	cd "$work/src/core"
	go test ./...
	GOOS=windows GOARCH=amd64 go build -trimpath \
		-ldflags "-H windowsgui -s -w -X rebootera/core/internal/version.Version=$version" \
		-o "$work/RebooteraDesktop.exe" ./cmd/rebootera-desktop
	mkdir -p "$SRC/out"
	cp "$work/RebooteraDesktop.exe" "$SRC/out/Rebootera Desktop.exe"
	ls -lh "$SRC/out/Rebootera Desktop.exe"
}

main "$@"
exit

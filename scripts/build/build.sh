#!/bin/bash
# * build.sh: construye la imagen ISO híbrida de Rebootera OS (arranca en BIOS
# * y en UEFI, con Secure Boot). Se ejecuta como root en Debian 13, en la
# * distro WSL RebooteraBuild:
# *
# *   wsl -d RebooteraBuild -u root --exec bash "/mnt/d/dev/Rebootera OS/scripts/build/build.sh" [--debug]
# *
# * Sin opciones genera la imagen definitiva en out/ (borrando las anteriores:
# * en out/ solo queda una ISO). Con --debug genera una imagen de pruebas, con
# * consola root en el puerto serie, que se queda dentro de WSL en
# * /root/rebootera-build/out/ y nunca se mezcla con la definitiva.
# ! Todo el proceso está dentro de main(): bash lee la función entera antes de
# ! ejecutarla, así que editar este archivo durante una construcción no la rompe.
set -euo pipefail

# ! PATH fijo: así no se cuelan herramientas de Windows (por ejemplo, el pnpm
# ! de Windows que WSL añade al PATH).
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

# * log escribe un paso del proceso de forma visible.
log() {
	printf '\n==> %s\n' "$*"
}

# * pnpm ejecuta la versión de pnpm fijada en ui/package.json (packageManager)
# * a través de corepack, que la descarga y verifica la primera vez.
pnpm() {
	local version entry
	version="$(sed -n 's/.*"packageManager": "pnpm@\([^"]*\)".*/\1/p' "$UI_DIR/package.json")"
	entry="$HOME/.cache/node/corepack/pnpm/$version/bin/pnpm.mjs"
	if [ ! -f "$entry" ]; then
		COREPACK_ENABLE_DOWNLOAD_PROMPT=0 corepack prepare "pnpm@$version" >/dev/null
	fi
	COREPACK_ENABLE_DOWNLOAD_PROMPT=0 node "$entry" "$@"
}

# * main construye la imagen.
main() {
	local SRC WORK VERSION DEBUG OUT_DIR OUT_NAME LIVE INC ISO
	SRC="$(cd "$(dirname "$0")/../.." && pwd)"
	WORK=/root/rebootera-build
	VERSION="$(sed -n 's/^var Version = "\(.*\)"/\1/p' "$SRC/core/internal/version/version.go")"
	DEBUG=0
	if [ "${1:-}" = "--debug" ]; then
		DEBUG=1
	fi
	if [ "$DEBUG" = "1" ]; then
		OUT_DIR="$WORK/out"
		OUT_NAME="rebootera-os-${VERSION}-amd64-debug.iso"
	else
		OUT_DIR="$SRC/out"
		OUT_NAME="rebootera-os-${VERSION}-amd64.iso"
	fi

	if [ "$(id -u)" != "0" ]; then
		echo "Este script necesita ejecutarse como root (live-build lo exige)." >&2
		exit 1
	fi

	log "Copiando el código a $WORK/src"
	mkdir -p "$WORK"
	rsync -a --delete --exclude node_modules --exclude dist --exclude out "$SRC/" "$WORK/src/"

	log "Compilando rebootera-core y rebootera-agent ($VERSION)"
	export GOCACHE=/root/.cache/go-build GOFLAGS=-buildvcs=false CGO_ENABLED=0
	cd "$WORK/src/core"
	go test ./...
	go build -trimpath -ldflags "-s -w -X rebootera/core/internal/version.Version=$VERSION" -o "$WORK/bin/" ./cmd/...

	log "Compilando Rebootera UI (pnpm)"
	UI_DIR="$WORK/src/ui"
	cd "$UI_DIR"
	pnpm install --frozen-lockfile --reporter=silent
	pnpm run check-types
	pnpm run build

	log "Preparando la configuración de live-build"
	LIVE="$WORK/live"
	mkdir -p "$LIVE"
	cd "$LIVE"
	# * El chroot se rehace siempre para que entren los cambios de includes y
	# * hooks; la caché de paquetes y del bootstrap (cache/) se conserva.
	lb clean >/dev/null 2>&1 || true
	rm -rf config auto chroot.files
	cp -a "$WORK/src/os/auto" "$WORK/src/os/config" .

	INC="$LIVE/config/includes.chroot"
	install -D -m 0755 "$WORK/bin/rebootera-core" "$INC/usr/lib/rebootera/rebootera-core"
	install -D -m 0755 "$WORK/bin/rebootera-agent" "$INC/usr/lib/rebootera/rebootera-agent"
	install -D -m 0755 "$WORK/src/os/shell/rebootera-shell" "$INC/usr/bin/rebootera-shell"
	mkdir -p "$INC/usr/share/rebootera/ui"
	cp -a "$UI_DIR/dist/." "$INC/usr/share/rebootera/ui/"
	mkdir -p "$INC/etc/rebootera"
	echo "$VERSION" >"$INC/etc/rebootera/version"
	if [ "$DEBUG" = "1" ]; then
		touch "$INC/etc/rebootera/debug"
		# * Solo en depuración: un punto de acceso virtual para probar Wi-Fi
		# * con radios simuladas (mac80211_hwsim).
		printf 'hostapd\ndnsmasq-base\nstrace\n' >"$LIVE/config/package-lists/debug.list.chroot"
	fi

	log "Descargando las extensiones del navegador (uBlock Origin y Dark Reader)"
	bash "$WORK/src/scripts/build/fetch-extensions.sh" "$INC/usr/share/rebootera/browser"

	log "Generando los gráficos de arranque"
	bash "$WORK/src/scripts/build/make-boot-art.sh" "$WORK/src/branding/logo.svg" \
		"$LIVE/config/bootloaders/grub-pc" "$INC/usr/share/plymouth/themes/rebootera"

	# ! Los archivos que llegan desde la unidad de Windows tienen permisos 0777;
	# ! dentro de la imagen se dejan los permisos normales de un sistema Debian.
	find "$INC" -type d -exec chmod 0755 {} +
	find "$INC" -type f -exec chmod 0644 {} +
	chmod 0755 "$INC/usr/lib/rebootera/rebootera-core" "$INC/usr/lib/rebootera/rebootera-agent" "$INC/usr/bin/rebootera-shell" "$INC/usr/lib/rebootera/start-kiosk" "$INC/usr/lib/rebootera/session" "$INC/usr/lib/rebootera/browser"
	find "$LIVE/config/hooks" -type f -exec chmod 0755 {} +
	chmod 0755 "$LIVE/auto/config"

	log "Construyendo la imagen (puede tardar)"
	export REBOOTERA_DEBUG="$DEBUG"
	lb config
	lb build 2>&1 | tee "$WORK/build.log"

	ISO="$(ls -t "$LIVE"/rebootera-os*.iso 2>/dev/null | head -n1 || true)"
	if [ -z "$ISO" ]; then
		echo "live-build no generó la ISO; revisa $WORK/build.log" >&2
		exit 1
	fi

	log "Guardando el resultado en $OUT_DIR"
	mkdir -p "$OUT_DIR"
	# * En cada carpeta de salida solo queda la ISO recién construida.
	rm -f "$OUT_DIR"/rebootera-os-*.iso "$OUT_DIR"/rebootera-os-*.iso.sha256
	mv "$ISO" "$OUT_DIR/$OUT_NAME"
	(cd "$OUT_DIR" && sha256sum "$OUT_NAME" >"$OUT_NAME.sha256")
	ls -lh "$OUT_DIR/$OUT_NAME"
	cat "$OUT_DIR/$OUT_NAME.sha256"
}

main "$@"
exit

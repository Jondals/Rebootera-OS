#!/bin/bash
# * fetch-extensions.sh: descarga las dos extensiones que lleva el navegador de Rebootera OS (uBlock Origin y
# * Dark Reader) desde addons.mozilla.org y comprueba su SHA-256 contra el que publica la propia API de
# * Mozilla antes de guardarlas. Las usa build.sh al preparar la imagen:
# *
# *   fetch-extensions.sh DESTINO
# *
# * Deja ublock-origin.xpi y dark-reader.xpi en DESTINO (en la imagen: /usr/share/rebootera/browser), que es
# * donde las busca la política de Firefox (os/config/includes.chroot/usr/lib/firefox-esr/distribution).
# ! Solo HTTPS y solo desde addons.mozilla.org: si la suma o la dirección no encajan, se aborta la construcción.
set -euo pipefail

DEST="${1:?uso: fetch-extensions.sh DESTINO}"
API="https://addons.mozilla.org/api/v5/addons/addon"
mkdir -p "$DEST"

# * field imprime un campo de la versión actual de una extensión (url o hash) según la API de Mozilla.
field() {
	python3 -c 'import json,sys; print(json.load(sys.stdin)["current_version"]["file"][sys.argv[1]])' "$1"
}

# * fetch descarga una extensión por su identificador en addons.mozilla.org y verifica su suma.
fetch() {
	local slug="$1" name="$2" meta url hash got
	meta="$(curl -fsSL "$API/$slug/")"
	url="$(printf '%s' "$meta" | field url)"
	hash="$(printf '%s' "$meta" | field hash)"
	case "$url" in
	https://addons.mozilla.org/*) ;;
	*)
		echo "dirección inesperada para $slug: $url" >&2
		exit 1
		;;
	esac
	curl -fsSL -o "$DEST/$name.xpi" "$url"
	got="sha256:$(sha256sum "$DEST/$name.xpi" | cut -d' ' -f1)"
	if [ "$got" != "$hash" ]; then
		rm -f "$DEST/$name.xpi"
		echo "la suma de $slug no coincide con la de Mozilla ($got frente a $hash)" >&2
		exit 1
	fi
	echo "$slug: $(basename "$url") verificada"
}

fetch ublock-origin ublock-origin
fetch darkreader dark-reader

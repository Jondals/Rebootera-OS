#!/bin/bash
# * fetch-ventoy.sh: descarga la versión fijada de Ventoy desde su release
# * oficial de GitHub, comprueba su SHA-256 con el sha256.txt publicado en el
# * mismo release y la extrae en /root/rebootera-build/vendor/. Imprime la
# * carpeta extraída. Si ya está descargada y verificada, no vuelve a bajarla.
# ! Ventoy no publica firmas criptográficas: la suma SHA-256 del mismo release
# ! detecta descargas corruptas, pero no protege frente a un release alterado.
set -euo pipefail

VENTOY_VERSION="1.1.18"
BASE_URL="https://github.com/ventoy/Ventoy/releases/download/v${VENTOY_VERSION}"
DOWNLOADS=/root/rebootera-build/downloads/ventoy-${VENTOY_VERSION}
VENDOR=/root/rebootera-build/vendor

# * main descarga, verifica y extrae.
main() {
	local archive="ventoy-${VENTOY_VERSION}-linux.tar.gz"
	local kind="${1:-linux}"
	if [ "$kind" = "windows" ]; then
		archive="ventoy-${VENTOY_VERSION}-windows.zip"
	fi
	mkdir -p "$DOWNLOADS" "$VENDOR"
	cd "$DOWNLOADS"
	[ -f sha256.txt ] || curl -fsSL -o sha256.txt "$BASE_URL/sha256.txt"
	[ -f "$archive" ] || curl -fsSL -o "$archive" "$BASE_URL/$archive"
	if ! grep " $archive\$" sha256.txt | sha256sum -c --status -; then
		echo "La suma SHA-256 de $archive no coincide con la publicada: se descarta." >&2
		rm -f "$archive"
		exit 1
	fi
	if [ "$kind" = "windows" ]; then
		echo "$DOWNLOADS/$archive"
		return
	fi
	if [ ! -x "$VENDOR/ventoy-${VENTOY_VERSION}/Ventoy2Disk.sh" ]; then
		tar -xzf "$archive" -C "$VENDOR"
	fi
	echo "$VENDOR/ventoy-${VENTOY_VERSION}"
}

main "$@"
exit

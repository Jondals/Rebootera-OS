#!/bin/bash
# * make-boot-art.sh: genera todos los gráficos de arranque de Rebootera OS a
# * partir de branding/logo.svg y de las fuentes Inter y JetBrains Mono:
# *   - GRUB: fondo con halo, logotipo y nombre; recuadro de selección; fuentes .pf2
# *   - Plymouth: logotipo, halo, nombre y barra de progreso
# *
# *   make-boot-art.sh LOGO.svg DIR_GRUB_THEME DIR_PLYMOUTH_THEME
# *
# * DIR_GRUB_THEME es la carpeta grub-pc de live-build (splash.png va en ella y
# * el resto en live-theme/). Requiere rsvg-convert, ImageMagick y grub-mkfont.
# ! GRUB solo lee bien PNG de 8 bits por canal: todo lo de GRUB se fuerza a PNG24.
set -euo pipefail

INTER_BOLD=/usr/share/fonts/opentype/inter/Inter-Bold.otf
MONO_REGULAR=/usr/share/fonts/truetype/jetbrains-mono/JetBrainsMono-Regular.ttf
MONO_BOLD=/usr/share/fonts/truetype/jetbrains-mono/JetBrainsMono-Bold.ttf
BG='#04070b'
TEAL='#5eead4'
INDIGO='#818cf8'
TEXT='#f1f5f9'

# * wordmark escribe "Rebootera OS" (OS en turquesa) con fondo transparente.
wordmark() {
	local size="$1" out="$2" tmp
	tmp="$(mktemp -d)"
	convert -background none -font "$INTER_BOLD" -pointsize "$size" -fill "$TEXT" label:'Rebootera ' "$tmp/a.png"
	convert -background none -font "$INTER_BOLD" -pointsize "$size" -fill "$TEAL" label:'OS' "$tmp/b.png"
	convert "$tmp/a.png" "$tmp/b.png" -background none -gravity center +append "$out"
	rm -rf "$tmp"
}

# * grub_art genera el fondo, el recuadro de selección y las fuentes de GRUB.
grub_art() {
	local logo="$1" dir="$2" tmp
	tmp="$(mktemp -d)"
	mkdir -p "$dir/live-theme"
	rsvg-convert -w 132 -h 132 "$logo" -o "$tmp/logo.png"
	wordmark 34 "$tmp/word.png"
	# Fondo oscuro con un halo turquesa detrás del logotipo.
	convert -size 1024x768 xc:"$BG" \
		\( -size 520x520 radial-gradient:'rgba(94,234,212,0.30)-rgba(94,234,212,0)' \) -gravity north -geometry +0-60 -composite \
		"$tmp/logo.png" -gravity north -geometry +0+118 -composite \
		"$tmp/word.png" -gravity north -geometry +0+272 -composite \
		-depth 8 -alpha off "PNG24:$dir/splash.png"
	# Recuadro de la entrada seleccionada: las nueve piezas que pide GRUB.
	local piece
	for piece in nw n ne w c e sw s se; do
		convert -size 4x4 xc:"$TEAL" -depth 8 -alpha off "PNG24:$dir/live-theme/select_$piece.png"
	done
	# Fuentes del menú (los nombres internos los usa theme.txt).
	grub-mkfont -s 16 -o "$dir/live-theme/jbmono-16.pf2" "$MONO_REGULAR"
	grub-mkfont -s 16 -o "$dir/live-theme/jbmono-bold-16.pf2" "$MONO_BOLD"
	grub-mkfont -s 12 -o "$dir/live-theme/jbmono-12.pf2" "$MONO_REGULAR"
	rm -rf "$tmp"
}

# * plymouth_art genera los gráficos del tema script de Plymouth.
plymouth_art() {
	local logo="$1" dir="$2"
	mkdir -p "$dir"
	rsvg-convert -w 148 -h 148 "$logo" -o "$dir/logo.png"
	convert -size 440x440 radial-gradient:'rgba(94,234,212,0.55)-rgba(94,234,212,0)' -depth 8 "PNG32:$dir/glow.png"
	wordmark 40 "$dir/wordmark.png"
	convert -size 260x3 xc:'rgba(255,255,255,0.09)' -depth 8 "PNG32:$dir/bar-track.png"
	convert -size 3x260 gradient:"$TEAL"-"$INDIGO" -rotate -90 -depth 8 "PNG32:$dir/bar-fill.png"
}

# * main comprueba las herramientas y genera todo.
main() {
	if [ $# -ne 3 ]; then
		echo "Uso: $0 LOGO.svg DIR_GRUB_THEME DIR_PLYMOUTH_THEME" >&2
		exit 2
	fi
	local tool
	for tool in rsvg-convert convert grub-mkfont; do
		command -v "$tool" >/dev/null || {
			echo "Falta $tool" >&2
			exit 1
		}
	done
	grub_art "$1" "$2"
	plymouth_art "$1" "$3"
}

main "$@"
exit

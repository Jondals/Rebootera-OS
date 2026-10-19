#!/bin/bash
# * make-boot-art.sh: genera todos los gráficos de arranque de Rebootera OS a
# * partir de branding/logo.svg y de la fuente Inter:
# *   - GRUB y menú de Ventoy: fondo oscuro con luz de latón y anillos que repiten el logotipo, tarjeta del
# *     menú y recuadro de selección con esquinas redondeadas (nueve piezas cada uno) y fuentes .pf2
# *   - Plymouth: logotipo, halo, nombre y barra de progreso
# *
# *   make-boot-art.sh LOGO.svg DIR_GRUB_THEME DIR_PLYMOUTH_THEME
# *
# * DIR_GRUB_THEME es la carpeta grub-pc de live-build (splash.png va en ella y
# * el resto en live-theme/). Requiere rsvg-convert, ImageMagick y grub-mkfont.
# ! GRUB solo lee bien PNG de 8 bits por canal: todo lo de GRUB se fuerza a PNG24 o PNG32.
set -euo pipefail

INTER=/usr/share/fonts/opentype/inter
BG='#0b0d11'
BRASS='#d8ae62'
BRASS_LIGHT='#f1d28f'
TEXT='#ece7db'

# * wordmark escribe "Rebootera OS" (OS en latón) con fondo transparente.
wordmark() {
	local size="$1" out="$2" tmp
	tmp="$(mktemp -d)"
	convert -background none -font "$INTER/Inter-Bold.otf" -pointsize "$size" -fill "$TEXT" label:'Rebootera ' "$tmp/a.png"
	convert -background none -font "$INTER/Inter-Bold.otf" -pointsize "$size" -fill "$BRASS" label:'OS' "$tmp/b.png"
	convert "$tmp/a.png" "$tmp/b.png" -background none -gravity center +append "$out"
	rm -rf "$tmp"
}

# * nine_slice corta una imagen cuadrada de lado SIZE en las nueve piezas que pide GRUB para un recuadro
# * (nw n ne w c e sw s se), con esquinas de EDGE píxeles. Las piezas se guardan como PREFIX_<pieza>.png.
nine_slice() {
	local source="$1" prefix="$2" size="$3" edge="$4" mid far
	mid=$((size / 2))
	far=$((size - edge))
	convert "$source" -crop "${edge}x${edge}+0+0" +repage -depth 8 "PNG32:${prefix}_nw.png"
	convert "$source" -crop "1x${edge}+${mid}+0" +repage -depth 8 "PNG32:${prefix}_n.png"
	convert "$source" -crop "${edge}x${edge}+${far}+0" +repage -depth 8 "PNG32:${prefix}_ne.png"
	convert "$source" -crop "${edge}x1+0+${mid}" +repage -depth 8 "PNG32:${prefix}_w.png"
	convert "$source" -crop "1x1+${mid}+${mid}" +repage -depth 8 "PNG32:${prefix}_c.png"
	convert "$source" -crop "${edge}x1+${far}+${mid}" +repage -depth 8 "PNG32:${prefix}_e.png"
	convert "$source" -crop "${edge}x${edge}+0+${far}" +repage -depth 8 "PNG32:${prefix}_sw.png"
	convert "$source" -crop "1x${edge}+${mid}+${far}" +repage -depth 8 "PNG32:${prefix}_s.png"
	convert "$source" -crop "${edge}x${edge}+${far}+${far}" +repage -depth 8 "PNG32:${prefix}_se.png"
}

# * rounded dibuja un cuadrado SIZE con esquinas redondeadas de RADIO, relleno y filete.
rounded() {
	local out="$1" size="$2" radius="$3" fill="$4" stroke="$5"
	convert -size "${size}x${size}" xc:none -fill "$fill" -stroke "$stroke" -strokewidth 1 \
		-draw "roundrectangle 0.5,0.5 $((size - 1)).5,$((size - 1)).5 $radius,$radius" -depth 8 "PNG32:$out"
}

# * grub_art genera el fondo, la tarjeta del menú, el recuadro de selección y las fuentes de GRUB.
grub_art() {
	local logo="$1" dir="$2" tmp
	tmp="$(mktemp -d)"
	mkdir -p "$dir/live-theme"
	rsvg-convert -w 168 -h 168 "$logo" -o "$tmp/logo.png"
	wordmark 40 "$tmp/word.png"
	# * Fondo 1920x1080: luz de latón arriba, dos anillos finos alrededor del logotipo, logotipo y nombre.
	convert -size 1920x1080 xc:"$BG" \
		\( -size 1500x1500 radial-gradient:'rgba(216,174,98,0.26)-rgba(216,174,98,0)' \) -gravity north -geometry +0-520 -composite \
		\( -size 700x700 xc:none -stroke 'rgba(216,174,98,0.14)' -strokewidth 2 -fill none -draw 'circle 350,350 350,2' \) -gravity north -geometry +0-76 -composite \
		\( -size 1000x1000 xc:none -stroke 'rgba(216,174,98,0.07)' -strokewidth 2 -fill none -draw 'circle 500,500 500,2' \) -gravity north -geometry +0-226 -composite \
		"$tmp/logo.png" -gravity north -geometry +0+190 -composite \
		"$tmp/word.png" -gravity north -geometry +0+390 -composite \
		-depth 8 -alpha off "PNG24:$dir/splash.png"
	# * Tarjeta del menú (translúcida, filete de latón) y recuadro de la entrada elegida.
	rounded "$tmp/card.png" 48 14 'rgba(16,20,28,0.80)' 'rgba(216,174,98,0.32)'
	nine_slice "$tmp/card.png" "$dir/live-theme/menu" 48 14
	rounded "$tmp/pick.png" 32 9 'rgba(216,174,98,0.22)' 'rgba(241,210,143,0.85)'
	nine_slice "$tmp/pick.png" "$dir/live-theme/select" 32 9
	# * Fuentes del menú (los nombres internos los usa theme.txt).
	grub-mkfont -s 18 -o "$dir/live-theme/inter-18.pf2" "$INTER/Inter-Regular.otf"
	grub-mkfont -s 18 -o "$dir/live-theme/inter-bold-18.pf2" "$INTER/Inter-Bold.otf"
	grub-mkfont -s 13 -o "$dir/live-theme/inter-13.pf2" "$INTER/Inter-Regular.otf"
	rm -rf "$tmp"
}

# * plymouth_art genera los gráficos del tema script de Plymouth.
plymouth_art() {
	local logo="$1" dir="$2"
	mkdir -p "$dir"
	rsvg-convert -w 148 -h 148 "$logo" -o "$dir/logo.png"
	convert -size 440x440 radial-gradient:'rgba(216,174,98,0.50)-rgba(216,174,98,0)' -depth 8 "PNG32:$dir/glow.png"
	wordmark 40 "$dir/wordmark.png"
	convert -size 260x3 xc:'rgba(255,255,255,0.09)' -depth 8 "PNG32:$dir/bar-track.png"
	convert -size 3x260 gradient:"$BRASS_LIGHT"-"$BRASS" -rotate -90 -depth 8 "PNG32:$dir/bar-fill.png"
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

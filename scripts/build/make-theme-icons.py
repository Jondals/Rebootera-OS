#!/usr/bin/env python3
# * make-theme-icons.py: dibuja el logotipo con los colores de cada tema de la interfaz (los mismos que usa el
# * logotipo de la pantalla, ver .lg-* en ui/src/styles/lux.css) y deja un SVG por tema en la carpeta de salida.
# * Rebootera Desktop los convierte en iconos de Windows y cambia el de la ventana al cambiar de tema.
# * Uso: make-theme-icons.py <lux.css> <logo.svg> <carpeta de salida>
# * Imprime los temas, uno por línea y en el orden en que se incrustan los iconos (ID 2, 3, 4...).
import re
import sys

# * Orden de los temas = THEMES de ui/src/prefs.ts = orden de los grupos de iconos del ejecutable.
THEMES = ["amoled", "obsidian", "sapphire", "emerald", "amethyst", "ember", "orange", "graphite"]


def theme_block(css, theme):
    """Devuelve el texto de las variables del tema."""
    if theme == "obsidian":
        # * El tema por defecto comparte bloque con :root.
        pattern = r':root,\s*:root\[data-theme="obsidian"\]\s*\{(.*?)\n\}'
    else:
        pattern = r':root\[data-theme="%s"\]\s*\{(.*?)\n\}' % theme
    match = re.search(pattern, css, re.S)
    if match is None:
        sys.exit("no se encontró el tema " + theme)
    return match.group(1)


def variable(block, name):
    """Lee una variable de color del bloque de un tema."""
    match = re.search(r"--%s:\s*([^;]+);" % re.escape(name), block)
    if match is None:
        sys.exit("falta --" + name)
    return match.group(1).strip()


def rgb(color):
    """Convierte #rrggbb o rgba(r, g, b, a) en una tupla de enteros."""
    color = color.strip()
    if color.startswith("#"):
        return tuple(int(color[i : i + 2], 16) for i in (1, 3, 5))
    parts = re.findall(r"[\d.]+", color)
    return tuple(int(float(p)) for p in parts[:3])


def mix(a, b, weight):
    """Mezcla dos colores: weight de a y el resto de b (como color-mix en srgb)."""
    return tuple(round(x * weight + y * (1 - weight)) for x, y in zip(a, b))


def hexa(color):
    """Tupla a #rrggbb."""
    return "#%02x%02x%02x" % color


def main():
    css = open(sys.argv[1], encoding="utf8").read()
    logo = open(sys.argv[2], encoding="utf8").read()
    out = sys.argv[3]
    for theme in THEMES:
        block = theme_block(css, theme)
        bg = rgb(variable(block, "bg"))
        surface = rgb(variable(block, "surface-solid"))
        accent = rgb(variable(block, "accent"))
        strong = rgb(variable(block, "accent-strong"))
        edge = rgb(variable(block, "logo-edge")) if "logo-edge" in block else accent
        svg = logo
        # * Mosaico: de .lg-t1 (superficie con un toque de acento) a .lg-t2 (fondo).
        svg = svg.replace("#1d2536", hexa(mix(surface, accent, 0.82))).replace("#0a0c11", hexa(bg))
        # * Sol: de .lg-s1 (acento aclarado) a .lg-s2 (acento fuerte).
        svg = svg.replace("#ffe3a0", hexa(mix(accent, (255, 255, 255), 0.55))).replace("#d49a3a", hexa(strong))
        # * Resplandor, arcos y bordes con el acento.
        svg = svg.replace("#e6b85c", hexa(accent)).replace("#e8b95a", hexa(accent))
        svg = svg.replace('stroke="#fff" stroke-opacity=".1"', 'stroke="%s" stroke-opacity=".4"' % hexa(edge))
        open("%s/%s.svg" % (out, theme), "w", encoding="utf8").write(svg)
        print(theme)


main()

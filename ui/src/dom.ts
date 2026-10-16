// * Utilidades para construir el DOM sin innerHTML con datos externos: el
// * texto siempre se inserta como texto, y los iconos SVG son constantes del
// * código (icons.ts).

/** Atributos admitidos por el(): texto o booleano (presente o ausente). */
type Attrs = Record<string, string | boolean>;

/** Crea un elemento con atributos e hijos (nodos o texto). */
export function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  attrs: Attrs = {},
  children: (Node | string | null)[] = [],
): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag);
  for (const name of Object.keys(attrs)) {
    const value = attrs[name];
    if (value === true) {
      node.setAttribute(name, "");
    } else if (value !== false) {
      node.setAttribute(name, value);
    }
  }
  for (const child of children) {
    if (child !== null) {
      node.append(child);
    }
  }
  return node;
}

/** Crea un icono SVG a partir de un trazado constante de icons.ts. */
export function icon(paths: string, className = "icon"): SVGSVGElement {
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("viewBox", "0 0 24 24");
  svg.setAttribute("aria-hidden", "true");
  svg.setAttribute("class", className);
  svg.innerHTML = paths;
  return svg;
}

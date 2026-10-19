// * Piezas reutilizables de las páginas: cabecera, paneles, insignias,
// * barras de progreso, estados vacíos y cajas de error.

import { ApiError } from "./api.js";
import { el, icon } from "./dom.js";
import { t } from "./i18n.js";
import { icons } from "./icons.js";
import { LOGOS } from "./logos.js";

/** Tonos de color de insignias y barras. */
export type Tone = "accent" | "ok" | "warn" | "danger" | "muted" | "violet" | "amber";

/** Cabecera de página con título, subtítulo y acciones opcionales. */
export function pageHeader(title: string, subtitle: string, actions: Node[] = []): HTMLElement {
  return el("header", { class: "page-header" }, [
    el("div", {}, [
      el("h1", { class: "page-title" }, [title]),
      el("p", { class: "page-subtitle" }, [subtitle]),
    ]),
    actions.length > 0 ? el("div", { class: "page-actions" }, actions) : null,
  ]);
}

/** Panel con título, subtítulo opcional, acción a la derecha y cuerpo. */
export function panel(
  title: string,
  body: (Node | null)[],
  options: { subtitle?: string; action?: Node; iconPaths?: string; className?: string } = {},
): HTMLElement {
  return el("section", { class: "panel " + (options.className ?? "") }, [
    el("header", { class: "panel-header" }, [
      options.iconPaths ? icon(options.iconPaths, "icon panel-icon") : null,
      el("div", { class: "panel-heading" }, [
        el("h2", { class: "panel-title" }, [title]),
        options.subtitle ? el("p", { class: "panel-subtitle" }, [options.subtitle]) : null,
      ]),
      options.action ?? null,
    ]),
    el("div", { class: "panel-body" }, body),
  ]);
}

/** Insignia pequeña con tono de color. */
export function badge(text: string, tone: Tone = "accent"): HTMLElement {
  return el("span", { class: "badge badge-" + tone }, [text]);
}

/** Etiqueta en mayúsculas y monoespaciada, como en el diseño. */
export function monoLabel(text: string): HTMLElement {
  return el("span", { class: "mono-label" }, [text]);
}

/** Barra de progreso de 0 a 1. */
export function progress(fraction: number, tone: Tone = "accent"): HTMLElement {
  const bar = el("span", { class: "progress-fill progress-" + tone });
  bar.style.width = Math.round(Math.min(1, Math.max(0, fraction)) * 100) + "%";
  return el("span", { class: "progress", role: "presentation" }, [bar]);
}

/** Estado vacío con icono, título, texto y acción opcional. */
export function emptyState(
  iconPaths: string,
  title: string,
  text: string,
  action?: Node,
): HTMLElement {
  return el("div", { class: "empty-state" }, [
    el("span", { class: "empty-icon" }, [icon(iconPaths)]),
    el("h3", {}, [title]),
    el("p", {}, [text]),
    action ?? null,
  ]);
}

/** Caja de error con el mensaje traducido. */
export function errorBox(error: unknown): HTMLElement {
  const message =
    error instanceof ApiError || error instanceof Error ? error.message : t("errors.service");
  return el("div", { class: "error-box", role: "alert" }, [
    icon(icons.alert),
    el("span", {}, [message]),
  ]);
}

/** Botón con icono y texto. */
export function button(
  text: string,
  onClick: (event: MouseEvent) => void,
  options: {
    variant?: "primary" | "ghost" | "danger";
    iconPaths?: string;
    trailingIcon?: string;
  } = {},
): HTMLButtonElement {
  const node = el("button", { class: "btn btn-" + (options.variant ?? "ghost"), type: "button" }, [
    options.iconPaths ? icon(options.iconPaths) : null,
    el("span", {}, [text]),
    options.trailingIcon ? icon(options.trailingIcon) : null,
  ]);
  node.addEventListener("click", onClick);
  return node;
}

/** Tarjeta grande y pulsable: icono, título y una frase. Es la pieza de las portadas. */
export function tile(
  iconPaths: string,
  title: string,
  text: string,
  onClick: () => void,
  options: { primary?: boolean; badge?: Node | null } = {},
): HTMLElement {
  const card = el(
    "button",
    { class: "tile" + (options.primary ? " tile-primary" : ""), type: "button" },
    [
      el("span", { class: "tile-icon" }, [icon(iconPaths)]),
      el("span", { class: "tile-body" }, [
        el("strong", { class: "tile-title" }, [title]),
        el("span", { class: "tile-text" }, [text]),
      ]),
      options.badge ?? null,
      icon(icons.chevronRight, "icon tile-arrow"),
    ],
  );
  card.addEventListener("click", onClick);
  return card;
}

/** Secciones plegables abiertas por el usuario (sobreviven a que la página se redibuje). */
const openSections = new Set<string>();

/**
 * Sección plegable: lo secundario se esconde hasta que se pide. Con `key`, recuerda si está abierta
 * aunque la página se vuelva a dibujar.
 */
export function more(summary: string, body: (Node | null)[], key = ""): HTMLElement {
  const node = el("details", { class: "more" }, [
    el("summary", {}, [el("span", {}, [summary]), icon(icons.chevronRight, "icon more-arrow")]),
    el("div", { class: "more-body" }, body),
  ]);
  if (key !== "") {
    node.open = openSections.has(key);
    node.addEventListener("toggle", function remember() {
      if (node.open) {
        openSections.add(key);
      } else {
        openSections.delete(key);
      }
    });
  }
  return node;
}

/** Nombre del espacio de nombres SVG. */
const SVG_NS = "http://www.w3.org/2000/svg";

/** Color estable (por el nombre) para el monograma de un sistema sin logo. */
function monogramColor(name: string): string {
  let hash = 0;
  for (const letter of name) {
    hash = (hash * 31 + letter.charCodeAt(0)) % 360;
  }
  return "hsl(" + hash + ", 42%, 40%)";
}

/**
 * Logo de un sistema sobre una ficha clara (así se ve igual de bien en los tres temas). Si el catálogo
 * no tiene el logo de esa familia, se dibuja un monograma con la inicial.
 */
export function logoTile(family: string, name: string): HTMLElement {
  const svg = document.createElementNS(SVG_NS, "svg");
  svg.setAttribute("viewBox", "0 0 24 24");
  svg.setAttribute("class", "logo-svg");
  svg.setAttribute("aria-hidden", "true");
  const logo = LOGOS[family];
  if (logo) {
    const path = document.createElementNS(SVG_NS, "path");
    path.setAttribute("d", logo.d);
    path.setAttribute("fill", logo.color);
    svg.append(path);
  } else {
    const text = document.createElementNS(SVG_NS, "text");
    text.setAttribute("x", "12");
    text.setAttribute("y", "17");
    text.setAttribute("text-anchor", "middle");
    text.setAttribute("font-size", "15");
    text.setAttribute("font-weight", "700");
    text.setAttribute("font-family", "Fraunces, Georgia, serif");
    text.setAttribute("fill", monogramColor(name));
    text.textContent = name.charAt(0).toUpperCase();
    svg.append(text);
  }
  return el("span", { class: "logo-tile" }, [svg]);
}

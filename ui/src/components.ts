// * Piezas reutilizables de las páginas: cabecera, paneles, insignias,
// * barras de progreso, estados vacíos y cajas de error.

import { ApiError } from "./api.js";
import { el, icon } from "./dom.js";
import { t } from "./i18n.js";
import { icons } from "./icons.js";

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

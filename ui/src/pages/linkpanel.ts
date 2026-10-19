// * Panel "Descargar desde un enlace" de la página de descargas: para imágenes que no están en el catálogo
// * (una ISO propia, un elemento del Internet Archive, un enlace directo https). Se pega la dirección, Core
// * dice qué archivos de imagen hay detrás (con su tamaño) y se elige cuál guardar en el USB. Rebootera no
// * recomienda ni comprueba esas imágenes: lo que se baja de un enlace queda marcado como "sin comprobar".

import { getJSON, postJSON } from "../api.js";
import { button, errorBox, more } from "../components.js";
import { el } from "../dom.js";
import * as fmt from "../format.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";

/** Archivo de imagen encontrado detrás de un enlace (GET /api/catalog/link). */
interface Candidate {
  name: string;
  url: string;
  sizeBytes: number;
  sha1?: string;
}

/** Categorías en las que se guarda la imagen descargada. */
const CATEGORIES = ["windows", "linux", "tools"] as const;

let input: HTMLInputElement | null = null;
let results: HTMLElement | null = null;
let category: (typeof CATEGORIES)[number] = "tools";
let busy = false;

/** Dibuja un mensaje sencillo en la zona de resultados. */
function say(text: string): void {
  results?.replaceChildren(el("p", { class: "muted" }, [text]));
}

/** Pregunta a Core qué archivos de imagen hay detrás del enlace escrito. */
async function inspect(): Promise<void> {
  const link = input?.value.trim() ?? "";
  if (busy || results === null) {
    return;
  }
  if (!link.startsWith("https://")) {
    say(t("link.needHttps"));
    return;
  }
  busy = true;
  say(t("link.checking"));
  try {
    const found = await getJSON<Candidate[]>("/api/catalog/link?url=" + encodeURIComponent(link));
    drawCandidates(found);
  } catch (error) {
    results.replaceChildren(errorBox(error));
  }
  busy = false;
}

/** Pone en cola la descarga del archivo elegido. */
async function download(event: MouseEvent): Promise<void> {
  const url = (event.currentTarget as HTMLElement).dataset.url ?? "";
  try {
    await postJSON("/api/transfers/url", { url, category });
    say(t("link.queued"));
  } catch (error) {
    results?.replaceChildren(errorBox(error));
  }
}

/** Cambia la categoría en la que se guardará la imagen. */
function pickCategory(event: MouseEvent): void {
  category = (event.currentTarget as HTMLElement).dataset.value as typeof category;
  for (const node of results?.parentElement?.querySelectorAll(".segment-button") ?? []) {
    node.setAttribute("aria-checked", String(node.getAttribute("data-value") === category));
  }
}

/** Dibuja los archivos encontrados, cada uno con su botón. */
function drawCandidates(found: Candidate[]): void {
  if (results === null) {
    return;
  }
  if (found.length === 0) {
    say(t("link.none"));
    return;
  }
  results.replaceChildren(
    ...found.map(function row(item) {
      const get = button(t("link.download"), download, { iconPaths: icons.download });
      get.dataset.url = item.url;
      get.classList.add("btn-small");
      return el("div", { class: "link-row" }, [
        el("div", { class: "link-name" }, [
          el("strong", {}, [item.name]),
          el("span", { class: "muted" }, [
            item.sizeBytes > 0 ? fmt.bytes(item.sizeBytes) : t("link.sizeUnknown"),
          ]),
        ]),
        get,
      ]);
    }),
  );
}

/** Botones de categoría. */
function categoryGroup(): HTMLElement {
  const group = el("div", {
    class: "segmented",
    role: "radiogroup",
    "aria-label": t("link.saveAs"),
  });
  for (const value of CATEGORIES) {
    const node = el(
      "button",
      {
        class: "segment-button",
        type: "button",
        role: "radio",
        "aria-checked": String(value === category),
        "data-value": value,
      },
      [t("link.category." + value)],
    );
    node.addEventListener("click", pickCategory);
    group.append(node);
  }
  return group;
}

/** Atiende Intro en el campo del enlace. */
function onKey(event: KeyboardEvent): void {
  if (event.key === "Enter") {
    void inspect();
  }
}

/** Sección plegable "Descargar desde un enlace". */
export function linkPanel(): HTMLElement {
  input = el("input", {
    class: "text-input",
    type: "url",
    placeholder: "https://",
    "aria-label": t("link.title"),
    spellcheck: "false",
  });
  input.addEventListener("keydown", onKey);
  results = el("div", { class: "link-results", "aria-live": "polite" });
  const check = button(t("link.check"), inspect, { iconPaths: icons.search });
  return more(
    t("link.title"),
    [
      el("p", { class: "muted" }, [t("link.hint")]),
      el("div", { class: "link-form" }, [input, check]),
      el("div", { class: "link-save" }, [el("span", {}, [t("link.saveAs")]), categoryGroup()]),
      results,
    ],
    "downloads-link",
  );
}

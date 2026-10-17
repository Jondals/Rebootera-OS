// * Página "Descargar sistemas": una lista corta y clara de imágenes oficiales
// * con un botón por fila. Las transferencias solo aparecen cuando las hay; los
// * detalles (compatibilidad de arranque, firma, Secure Boot) quedan plegados.
// * La última versión y su tamaño se consultan en segundo plano (Core las
// * guarda unos minutos) para que la página salga al instante.
// * Windows se descarga desde la web de Microsoft (no hay enlace directo).

import { getJSON, postJSON, type CatalogEntry, type Job } from "../api.js";
import {
  badge,
  button,
  emptyState,
  errorBox,
  pageHeader,
  progress,
  type Tone,
} from "../components.js";
import { el, icon } from "../dom.js";
import * as fmt from "../format.js";
import { getLanguage, t } from "../i18n.js";
import { icons } from "../icons.js";
import { appMode } from "../mode.js";
import type { Page } from "../router.js";
import { refreshVault, snapshot } from "../state.js";

/** Tono de cada estado de transferencia. */
const STATUS_TONES: Record<Job["status"], Tone> = {
  queued: "muted",
  running: "accent",
  verifying: "violet",
  paused: "warn",
  done: "ok",
  failed: "danger",
  cancelled: "muted",
};

/** Filtros de categoría de la lista. */
const FILTERS = ["all", "linux", "windows", "tools"] as const;
type Filter = (typeof FILTERS)[number];

/** Resultado de consultar la última versión de una entrada. */
interface Resolved {
  fileName: string;
  sizeBytes: number;
}

let transfersBody: HTMLElement | null = null;
let listBody: HTMLElement | null = null;
let filterBody: HTMLElement | null = null;
let message: HTMLElement | null = null;
let catalog: CatalogEntry[] | null = null;
let filter: Filter = "all";
const open = new Set<string>();
let lastQueue = "";
let doneCount = -1;
const resolved = new Map<string, Resolved | "error">();

/** Dibuja la estructura y carga el catálogo. */
function render(root: HTMLElement): void {
  transfersBody = el("div", { class: "transfers-box" });
  filterBody = el("div", { class: "filter-row", role: "tablist" });
  listBody = el("div", { class: "os-list" });
  message = el("div", { class: "page-message", "aria-live": "polite" });
  lastQueue = "";
  root.append(
    el("div", { class: "page" }, [
      pageHeader(t("downloads.title"), t("downloads.subtitle")),
      message,
      transfersBody,
      filterBody,
      listBody,
    ]),
  );
  drawFilters();
  loadCatalog();
  update();
}

/** Lee el catálogo y consulta en segundo plano la última versión de cada entrada. */
async function loadCatalog(): Promise<void> {
  try {
    catalog = await getJSON<CatalogEntry[]>("/api/catalog");
  } catch (error) {
    listBody?.replaceChildren(errorBox(error));
    return;
  }
  renderList();
  for (const entry of catalog) {
    if (entry.resolver.type !== "manual" && !resolved.has(entry.id)) {
      resolveEntry(entry.id);
    }
  }
}

/** Consulta en la fuente oficial el archivo y tamaño actuales de una entrada. */
async function resolveEntry(id: string): Promise<void> {
  try {
    resolved.set(id, await postJSON<Resolved>("/api/catalog/resolve", { id }));
  } catch {
    resolved.set(id, "error");
  }
  renderList();
}

/** Cambia el filtro de categoría. */
function chooseFilter(event: MouseEvent): void {
  filter = (event.currentTarget as HTMLElement).dataset.filter as Filter;
  drawFilters();
  renderList();
}

/** Despliega o pliega los detalles de una fila. */
function toggleDetails(event: MouseEvent): void {
  const id = (event.currentTarget as HTMLElement).dataset.id ?? "";
  if (open.has(id)) {
    open.delete(id);
  } else {
    open.add(id);
  }
  renderList();
}

/** Añade una descarga a la cola. */
async function startDownload(event: MouseEvent): Promise<void> {
  const id = (event.currentTarget as HTMLElement).dataset.id ?? "";
  const entry = catalog?.find(function sameId(item) {
    return item.id === id;
  });
  try {
    await postJSON("/api/transfers/download", { id });
    message?.replaceChildren(
      el("div", { class: "notice notice-ok" }, [
        icon(icons.check),
        t("downloads.added", { name: entry?.name ?? id }),
      ]),
    );
    await refreshVault();
  } catch (error) {
    message?.replaceChildren(errorBox(error));
  }
}

/** Pausa, reanuda o cancela según data-action. */
async function transferAction(event: MouseEvent): Promise<void> {
  const target = event.currentTarget as HTMLElement;
  try {
    await postJSON("/api/transfers/" + target.dataset.action, { id: target.dataset.id });
    await refreshVault();
  } catch (error) {
    message?.replaceChildren(errorBox(error));
  }
}

/** Quita de la lista las transferencias terminadas. */
async function clearFinished(): Promise<void> {
  try {
    await postJSON("/api/transfers/clear", {});
    await refreshVault();
  } catch (error) {
    message?.replaceChildren(errorBox(error));
  }
}

/** Botón pequeño de una acción sobre una transferencia. */
function jobButton(job: Job, action: string, label: string, iconPaths: string): HTMLButtonElement {
  const node = button(label, transferAction, { iconPaths });
  node.classList.add("btn-small");
  node.dataset.id = job.id;
  node.dataset.action = action;
  return node;
}

/** Fila de una transferencia. */
function jobRow(job: Job): HTMLElement {
  const fraction = job.totalBytes > 0 ? job.doneBytes / job.totalBytes : 0;
  const details: string[] = [];
  if (job.totalBytes > 0) {
    details.push(fmt.bytes(job.doneBytes) + " / " + fmt.bytes(job.totalBytes));
  }
  if (job.status === "running" && job.speedBps > 0) {
    details.push(fmt.rate(job.speedBps));
    if (job.totalBytes > job.doneBytes) {
      details.push(
        t("downloads.eta", { time: fmt.duration((job.totalBytes - job.doneBytes) / job.speedBps) }),
      );
    }
  }
  const actions: HTMLElement[] = [];
  if (job.status === "running" || job.status === "queued") {
    actions.push(jobButton(job, "pause", t("downloads.pause"), icons.clock));
  }
  if (job.status === "paused") {
    actions.push(jobButton(job, "resume", t("downloads.resume"), icons.download));
  }
  if (job.status === "failed") {
    actions.push(jobButton(job, "resume", t("downloads.retry"), icons.refresh));
  }
  if (job.status !== "done" && job.status !== "cancelled" && job.status !== "verifying") {
    actions.push(jobButton(job, "cancel", t("downloads.cancel"), icons.x));
  }
  const errorKey = "errors.codes." + (job.errorCode ?? "failed");
  return el("article", { class: "transfer-row" }, [
    el("span", { class: "device-icon" }, [
      icon(job.kind === "import" ? icons.usb : icons.download),
    ]),
    el("div", { class: "transfer-main" }, [
      el("div", { class: "transfer-title" }, [
        el("strong", {}, [job.name]),
        badge(
          job.kind === "import" && job.status === "running"
            ? t("downloads.importing")
            : t("downloads.status." + job.status),
          STATUS_TONES[job.status],
        ),
      ]),
      job.status === "running" || job.status === "paused" || job.status === "verifying"
        ? progress(
            job.status === "verifying" ? 1 : fraction,
            job.status === "paused" ? "warn" : "accent",
          )
        : null,
      el("span", { class: "mono-small muted" }, [details.join(" · ") || job.fileName]),
      job.status === "failed"
        ? el("span", { class: "transfer-error", title: job.error ?? "" }, [t(errorKey)])
        : null,
    ]),
    el("div", { class: "image-actions" }, actions),
  ]);
}

/** Primera frase de una nota: lo justo para la fila. */
function firstSentence(text: string): string {
  const end = text.indexOf(". ");
  return end > 0 ? text.slice(0, end + 1) : text;
}

/** Tamaño o estado de la consulta de una entrada, para el lado derecho de la fila. */
function sizeText(entry: CatalogEntry): string {
  if (entry.resolver.type === "manual") {
    return "";
  }
  const info = resolved.get(entry.id);
  if (info === undefined) {
    return "…";
  }
  if (info === "error") {
    return "—";
  }
  return info.sizeBytes > 0 ? fmt.bytes(info.sizeBytes) : "";
}

/** Botón o insignia de la derecha de una fila. */
function rowAction(entry: CatalogEntry): HTMLElement {
  if (entry.inLibrary) {
    return badge(t("downloads.inLibrary"), "ok");
  }
  if (entry.resolver.type === "manual") {
    return badge(t("downloads.manual"), "amber");
  }
  const node = button(t("downloads.download"), startDownload, {
    variant: "primary",
    iconPaths: icons.download,
  });
  node.dataset.id = entry.id;
  node.disabled = !snapshot.vault?.available;
  return node;
}

/** Línea de un dato en los detalles plegables. */
function detailLine(label: string, value: string): HTMLElement {
  return el("div", { class: "detail-line" }, [
    el("span", { class: "muted" }, [label]),
    el("span", {}, [value]),
  ]);
}

/** Detalles de una entrada: todo lo que antes eran insignias, en texto. */
function details(entry: CatalogEntry): HTMLElement {
  const lang = getLanguage();
  const info = resolved.get(entry.id);
  const signature = entry.resolver.isoSignature
    ? t("downloads.signedImage")
    : entry.resolver.signature
      ? t("downloads.signedIndex")
      : t("downloads.unsigned");
  const boot =
    entry.boot.status === "verified"
      ? t("downloads.bootVerified")
      : entry.boot.status === "incompatible"
        ? t("downloads.bootIncompatible")
        : t("downloads.bootUntested");
  return el("div", { class: "os-details" }, [
    el("p", {}, [entry.notes[lang] ?? entry.notes.en]),
    detailLine(
      t("downloads.detailBoot"),
      boot + ". " + (entry.boot.notes[lang] ?? entry.boot.notes.en),
    ),
    detailLine(
      t("downloads.detailSecure"),
      entry.secureBoot === "signed" ? t("downloads.secureSigned") : t("downloads.secureUnsigned"),
    ),
    detailLine(t("downloads.detailSignature"), signature),
    detailLine(
      t("downloads.detailFile"),
      entry.resolver.type === "manual"
        ? entry.homepage
        : info === undefined
          ? t("downloads.resolving")
          : info === "error"
            ? t("errors.codes.source")
            : info.fileName,
    ),
  ]);
}

/** Fila de una entrada del catálogo. */
function osRow(entry: CatalogEntry): HTMLElement {
  const lang = getLanguage();
  const expanded = open.has(entry.id);
  const toggle = el(
    "button",
    {
      class: "os-toggle",
      type: "button",
      "data-id": entry.id,
      "aria-expanded": String(expanded),
      title: t("downloads.details"),
    },
    [icon(icons.info)],
  );
  toggle.addEventListener("click", toggleDetails);
  const incompatible = entry.boot.status === "incompatible";
  return el("article", { class: "os-row" + (expanded ? " os-open" : "") }, [
    el("div", { class: "os-line" }, [
      el("span", { class: "feature-icon" }, [
        icon(
          entry.category === "windows"
            ? icons.monitor
            : entry.category === "tools"
              ? icons.recover
              : icons.library,
        ),
      ]),
      el("div", { class: "os-main" }, [
        el("strong", {}, [entry.name + " " + entry.version]),
        el("span", { class: "muted os-note" }, [
          firstSentence(entry.notes[lang] ?? entry.notes.en),
        ]),
        incompatible
          ? el("span", { class: "os-warning" }, [t("downloads.bootIncompatible")])
          : null,
      ]),
      el("span", { class: "mono-small muted os-size" }, [sizeText(entry)]),
      rowAction(entry),
      toggle,
    ]),
    expanded ? details(entry) : null,
  ]);
}

/** Dibuja los botones de filtro. */
function drawFilters(): void {
  if (filterBody === null) {
    return;
  }
  filterBody.replaceChildren(
    ...FILTERS.map(function filterButton(name) {
      const node = el(
        "button",
        {
          class: "chip-button" + (filter === name ? " chip-active" : ""),
          type: "button",
          role: "tab",
          "aria-selected": String(filter === name),
          "data-filter": name,
        },
        [t(name === "all" ? "downloads.all" : "downloads.categories." + name)],
      );
      node.addEventListener("click", chooseFilter);
      return node;
    }),
  );
}

/** Dibuja la lista según el filtro. */
function renderList(): void {
  if (listBody === null || catalog === null || !listBody.isConnected) {
    return;
  }
  const rows = catalog.filter(function visible(entry) {
    return filter === "all" || entry.category === filter;
  });
  listBody.replaceChildren(...rows.map(osRow));
}

/** Refresca las transferencias (solo se ven si hay alguna). */
function update(): void {
  if (transfersBody === null || !transfersBody.isConnected || snapshot.vault === null) {
    return;
  }
  const signature = JSON.stringify([snapshot.vault.available, snapshot.transfers]);
  if (signature === lastQueue) {
    return;
  }
  lastQueue = signature;
  if (!snapshot.vault.available) {
    transfersBody.replaceChildren(
      emptyState(icons.lock, t("needsVault.title"), t("needsVault.text")),
    );
    renderList();
    return;
  }
  // * Cuando termina una transferencia se recarga el catálogo para marcar
  // * "En tu biblioteca" sin salir de la página.
  const done = snapshot.transfers.filter(isDone).length;
  if (doneCount >= 0 && done > doneCount) {
    loadCatalog();
  }
  doneCount = done;
  if (snapshot.transfers.length === 0) {
    transfersBody.replaceChildren(
      ...(appMode.library
        ? [
            el("p", { class: "muted small-note" }, [
              t("desktop.localNote", { path: appMode.library }),
            ]),
          ]
        : []),
    );
    return;
  }
  const clear = button(t("downloads.clear"), clearFinished, { iconPaths: icons.check });
  clear.classList.add("btn-small");
  transfersBody.replaceChildren(
    el("div", { class: "transfers-head" }, [el("strong", {}, [t("downloads.queue")]), clear]),
    ...[...snapshot.transfers].reverse().map(jobRow),
  );
}

/** Indica si una transferencia ha terminado bien. */
function isDone(job: Job): boolean {
  return job.status === "done";
}

export const downloadsPage: Page = { render, update };

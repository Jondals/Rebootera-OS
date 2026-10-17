// * Página "Centro de descargas": cola de transferencias (descargas e
// * importaciones con progreso, velocidad y tiempo restante) y catálogo
// * oficial. Cada tarjeta consulta en la fuente oficial la última versión y su
// * tamaño. Windows se descarga desde la web de Microsoft (enlace manual).

import { getJSON, postJSON, type CatalogEntry, type Job } from "../api.js";
import {
  badge,
  button,
  emptyState,
  errorBox,
  pageHeader,
  panel,
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

/** Resultado de consultar la última versión de una entrada. */
interface Resolved {
  fileName: string;
  sizeBytes: number;
}

let queueBody: HTMLElement | null = null;
let catalogBody: HTMLElement | null = null;
let message: HTMLElement | null = null;
let catalog: CatalogEntry[] | null = null;
let lastQueue = "";
let doneCount = -1;
const resolved = new Map<string, Resolved | "error">();

/** Dibuja la estructura y carga el catálogo. */
function render(root: HTMLElement): void {
  queueBody = el("div", { class: "transfer-list" });
  catalogBody = el("div", { class: "catalog-grid" });
  message = el("div", { class: "page-message", "aria-live": "polite" });
  lastQueue = "";
  const clear = button(t("downloads.clear"), clearFinished, { iconPaths: icons.check });
  clear.classList.add("btn-small");
  root.append(
    el("div", { class: "page" }, [
      pageHeader(t("downloads.title"), t("downloads.subtitle")),
      message,
      panel(t("downloads.queue"), [queueBody], { iconPaths: icons.download, action: clear }),
      panel(t("downloads.catalog"), [catalogBody], { iconPaths: icons.globe }),
    ]),
  );
  loadCatalog();
}

/** Lee el catálogo y consulta la última versión de cada entrada. */
async function loadCatalog(): Promise<void> {
  try {
    catalog = await getJSON<CatalogEntry[]>("/api/catalog");
  } catch (error) {
    catalogBody?.replaceChildren(errorBox(error));
    return;
  }
  renderCatalog();
  for (const entry of catalog) {
    if (entry.resolver.type !== "manual" && !resolved.has(entry.id)) {
      resolveEntry(entry.id);
    }
  }
}

/** Consulta en la fuente oficial el archivo y tamaño actuales de una entrada. */
async function resolveEntry(id: string): Promise<void> {
  try {
    const result = await postJSON<Resolved>("/api/catalog/resolve", { id });
    resolved.set(id, result);
  } catch {
    resolved.set(id, "error");
  }
  renderCatalog();
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

/** Tarjeta de una entrada del catálogo. */
function catalogCard(entry: CatalogEntry): HTMLElement {
  const info = resolved.get(entry.id);
  let versionLine: string;
  if (entry.resolver.type === "manual") {
    versionLine = entry.homepage;
  } else if (info === undefined) {
    versionLine = t("downloads.resolving");
  } else if (info === "error") {
    versionLine = t("errors.codes.source");
  } else {
    versionLine = info.fileName + (info.sizeBytes > 0 ? " · " + fmt.bytes(info.sizeBytes) : "");
  }
  let action: HTMLElement;
  if (entry.inLibrary) {
    action = badge(t("downloads.inLibrary"), "ok");
  } else if (entry.resolver.type === "manual") {
    action = badge(t("downloads.manual"), "amber");
  } else {
    const node = button(t("downloads.download"), startDownload, {
      variant: "primary",
      iconPaths: icons.download,
    });
    node.classList.add("btn-small");
    node.dataset.id = entry.id;
    node.disabled = !snapshot.vault?.available;
    action = node;
  }
  return el("article", { class: "catalog-card" }, [
    el("header", { class: "catalog-head" }, [
      el("span", { class: "feature-icon" }, [
        icon(
          entry.category === "windows"
            ? icons.monitor
            : entry.category === "tools"
              ? icons.recover
              : icons.library,
        ),
      ]),
      el("div", {}, [
        el("strong", {}, [entry.name + " " + entry.version]),
        el("span", { class: "muted" }, [entry.vendor + " · " + entry.arch]),
      ]),
    ]),
    el("p", { class: "catalog-notes" }, [entry.notes[getLanguage()] ?? entry.notes.en]),
    entry.boot.status === "incompatible"
      ? el("p", { class: "catalog-warning" }, [
          entry.boot.notes[getLanguage()] ?? entry.boot.notes.en,
        ])
      : null,
    el("span", { class: "mono-small muted catalog-version", title: versionLine }, [versionLine]),
    el("div", { class: "catalog-badges" }, [bootBadge(entry), signedBadge(entry)]),
    el("footer", { class: "catalog-foot" }, [
      badge(
        entry.secureBoot === "signed" ? t("downloads.secureSigned") : t("downloads.secureUnsigned"),
        entry.secureBoot === "signed" ? "ok" : "warn",
      ),
      action,
    ]),
  ]);
}

/** Insignia con la compatibilidad de arranque comprobada de una entrada. */
function bootBadge(entry: CatalogEntry): HTMLElement {
  const node =
    entry.boot.status === "verified"
      ? badge(t("downloads.bootVerified"), "ok")
      : entry.boot.status === "incompatible"
        ? badge(t("downloads.bootIncompatible"), "danger")
        : badge(t("downloads.bootUntested"), "muted");
  node.title = entry.boot.notes[getLanguage()] ?? entry.boot.notes.en;
  return node;
}

/** Insignia de índice firmado: solo si Core comprueba su firma GPG antes de descargar. */
function signedBadge(entry: CatalogEntry): HTMLElement | null {
  if (entry.resolver.isoSignature) {
    const image = badge(t("downloads.signedImage"), "violet");
    image.title = t("downloads.signedImageHint");
    return image;
  }
  if (!entry.resolver.signature) {
    return null;
  }
  const node = badge(t("downloads.signedIndex"), "violet");
  node.title = t("downloads.signedIndexHint");
  return node;
}

/** Dibuja el catálogo. */
function renderCatalog(): void {
  if (catalogBody === null || catalog === null || !catalogBody.isConnected) {
    return;
  }
  catalogBody.replaceChildren(...catalog.map(catalogCard));
}

/** Refresca la cola de transferencias. */
function update(): void {
  if (queueBody === null || !queueBody.isConnected || snapshot.vault === null) {
    return;
  }
  const signature = JSON.stringify([snapshot.vault.available, snapshot.transfers]);
  if (signature === lastQueue) {
    return;
  }
  lastQueue = signature;
  if (!snapshot.vault.available) {
    queueBody.replaceChildren(emptyState(icons.lock, t("needsVault.title"), t("needsVault.text")));
    renderCatalog();
    return;
  }
  // * Sin USB, Rebootera Desktop guarda en la carpeta del usuario: se dice dónde.
  const where = appMode.library
    ? el("p", { class: "muted small-note" }, [t("desktop.localNote", { path: appMode.library })])
    : null;
  // * Cuando termina una transferencia se recarga el catálogo para marcar
  // * "En tu biblioteca" sin salir de la página.
  const done = snapshot.transfers.filter(isDone).length;
  if (doneCount >= 0 && done > doneCount) {
    loadCatalog();
  }
  doneCount = done;
  const jobs = [...snapshot.transfers].reverse();
  queueBody.replaceChildren(
    ...(jobs.length > 0
      ? jobs.map(jobRow)
      : [el("p", { class: "muted" }, [t("downloads.queueEmpty")])]),
    ...(where ? [where] : []),
  );
}

/** Indica si una transferencia ha terminado bien. */
function isDone(job: Job): boolean {
  return job.status === "done";
}

export const downloadsPage: Page = { render, update };

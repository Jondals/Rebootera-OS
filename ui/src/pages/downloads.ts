// * Página "Descargar sistemas" (Centro de descargas): una tarjeta por sistema con sus
// * versiones para elegir, buscador, pestañas (Windows, Linux, Servidores y BSD, Rescate),
// * el estado del USB de destino arriba y, abajo, la barra de descargas con el ajuste del
// * ancho de banda. La última versión y su tamaño se consultan en segundo plano y Core las
// * guarda unos minutos, así que la página sale al instante.
// * Windows Home y Pro no tienen enlace directo (Microsoft solo da enlaces temporales desde su
// * web); sus ediciones de evaluación sí se descargan directamente.

import {
  getJSON,
  postJSON,
  type BandwidthSettings,
  type CatalogEntry,
  type Family,
  type Job,
} from "../api.js";
import { badge, button, errorBox, progress, type Tone } from "../components.js";
import { openModal } from "../dialogs.js";
import { el, icon } from "../dom.js";
import * as fmt from "../format.js";
import { getLanguage, t } from "../i18n.js";
import { icons } from "../icons.js";
import { appMode } from "../mode.js";
import type { Page } from "../router.js";
import { refreshVault, snapshot } from "../state.js";

/** Pestañas de la lista. */
const TABS = ["all", "windows", "linux", "servers", "rescue"] as const;
type Tab = (typeof TABS)[number];

/** Icono de cada tipo de familia. */
const FAMILY_ICONS: Record<string, string> = {
  windows: icons.monitor,
  ubuntu: icons.library,
  debian: icons.library,
  fedora: icons.library,
  mint: icons.library,
  arch: icons.terminal,
  suse: icons.library,
  kali: icons.terminal,
  server: icons.cpu,
  bsd: icons.terminal,
  alpine: icons.recover,
  rescue: icons.recover,
};

/** Tono de cada etiqueta de una versión. */
const TAG_TONES: Record<string, Tone> = {
  recommended: "accent",
  lts: "ok",
  rolling: "violet",
  light: "amber",
  live: "muted",
  legacy: "muted",
};

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
  sha256: string;
}

/** Unidad de un disco de /api/desktop/drives, para enseñar el USB de destino. */
interface Drive {
  root: string;
  label: string;
  fileSystem: string;
}

/** Cuántas consultas de versión se hacen a la vez. */
const RESOLVE_AT_ONCE = 3;

let page: HTMLElement | null = null;
let message: HTMLElement | null = null;
let destinationBody: HTMLElement | null = null;
let listBody: HTMLElement | null = null;
let tabsBody: HTMLElement | null = null;
let barBody: HTMLElement | null = null;
let families: Family[] | null = null;
let catalog: CatalogEntry[] | null = null;
let tab: Tab = "all";
let query = "";
let drive: Drive | null = null;
let lastQueue = "";
let doneCount = -1;
const picked = new Map<string, string>();
const resolved = new Map<string, Resolved | "error">();
const resolveQueue: string[] = [];
let resolving = 0;

/** Dibuja la estructura y carga el catálogo. */
function render(root: HTMLElement): void {
  message = el("div", { class: "page-message", "aria-live": "polite" });
  destinationBody = el("div", { class: "dl-destination" });
  tabsBody = el("div", { class: "filter-row", role: "tablist" });
  listBody = el("div", { class: "fam-grid" });
  barBody = el("div", { class: "dl-bar" });
  const search = el("input", {
    class: "text-input dl-search",
    type: "search",
    placeholder: t("downloads.search"),
    "aria-label": t("downloads.search"),
  });
  search.value = query;
  search.addEventListener("input", onSearch);
  page = el("div", { class: "page dl-page" }, [
    el("header", { class: "dl-hero" }, [
      el("div", {}, [
        el("span", { class: "mono-label" }, [t("downloads.official")]),
        el("h1", { class: "page-title" }, [t("downloads.title")]),
        el("p", { class: "page-subtitle" }, [t("downloads.subtitle")]),
      ]),
      destinationBody,
    ]),
    message,
    el("div", { class: "dl-tools" }, [search, tabsBody]),
    listBody,
    barBody,
  ]);
  root.append(page);
  lastQueue = "";
  drawTabs();
  loadAll();
  update();
}

/** Lee el catálogo, las familias y (en el escritorio) el USB de destino. */
async function loadAll(): Promise<void> {
  try {
    [families, catalog] = await Promise.all([
      getJSON<Family[]>("/api/catalog/families"),
      getJSON<CatalogEntry[]>("/api/catalog"),
    ]);
    if (appMode.mode === "desktop") {
      const drives = await getJSON<Drive[]>("/api/desktop/drives");
      drive =
        drives.find(function isDestination(item) {
          return item.root === appMode.usb;
        }) ?? null;
    }
  } catch (error) {
    listBody?.replaceChildren(errorBox(error));
    return;
  }
  drawTabs();
  drawDestination();
  renderList();
}

/** Recarga solo el catálogo (para marcar lo que ya está en la biblioteca). */
async function reloadCatalog(): Promise<void> {
  try {
    catalog = await getJSON<CatalogEntry[]>("/api/catalog");
  } catch {
    return;
  }
  renderList();
}

/** Texto escrito en el buscador. */
function onSearch(event: Event): void {
  query = (event.currentTarget as HTMLInputElement).value.trim().toLowerCase();
  renderList();
}

/** Cambia la pestaña. */
function chooseTab(event: MouseEvent): void {
  tab = (event.currentTarget as HTMLElement).dataset.tab as Tab;
  drawTabs();
  renderList();
}

/** Elige la versión de una familia. */
function chooseVersion(event: Event): void {
  const input = event.currentTarget as HTMLInputElement;
  picked.set(input.dataset.family ?? "", input.value);
  renderList();
}

/** Pone en cola la consulta de la última versión de una entrada. */
function queueResolve(id: string): void {
  if (resolved.has(id) || resolveQueue.includes(id)) {
    return;
  }
  resolveQueue.push(id);
  pumpResolve();
}

/** Lanza consultas pendientes sin pasar de RESOLVE_AT_ONCE a la vez. */
function pumpResolve(): void {
  while (resolving < RESOLVE_AT_ONCE && resolveQueue.length > 0) {
    const id = resolveQueue.shift() as string;
    resolving += 1;
    resolveEntry(id);
  }
}

/** Consulta en la fuente oficial el archivo, el tamaño y la suma de una entrada. */
async function resolveEntry(id: string): Promise<void> {
  try {
    resolved.set(id, await postJSON<Resolved>("/api/catalog/resolve", { id }));
  } catch {
    resolved.set(id, "error");
  }
  resolving -= 1;
  renderList();
  pumpResolve();
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
        t("downloads.added", { name: (entry?.name ?? id) + " " + (entry?.version ?? "") }),
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

/** Copia texto al portapapeles (con un respaldo para webviews sin la API moderna). */
async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    const area = el("textarea", { class: "visually-hidden" });
    area.value = text;
    document.body.append(area);
    area.select();
    const ok = document.execCommand("copy");
    area.remove();
    return ok;
  }
}

/** Copia la suma SHA-256 de la entrada del botón pulsado. */
async function copySha(event: MouseEvent): Promise<void> {
  const node = event.currentTarget as HTMLElement;
  const info = resolved.get(node.dataset.id ?? "");
  if (!info || info === "error" || !info.sha256) {
    return;
  }
  if (await copyText(info.sha256)) {
    message?.replaceChildren(
      el("div", { class: "notice notice-ok" }, [icon(icons.check), t("downloads.copied")]),
    );
  }
}

/** Abre la página del fabricante (en el escritorio) o la copia para pegarla en un navegador. */
async function openVendor(event: MouseEvent): Promise<void> {
  const url = (event.currentTarget as HTMLElement).dataset.url ?? "";
  try {
    if (appMode.mode === "desktop") {
      await postJSON("/api/desktop/open", { url });
      return;
    }
  } catch {
    // * Si no se pudo abrir, se copia el enlace.
  }
  if (await copyText(url)) {
    message?.replaceChildren(
      el("div", { class: "notice notice-ok" }, [icon(icons.check), t("downloads.linkCopied")]),
    );
  }
}

/** Tipo de verificación de una entrada, para la insignia. */
function verification(entry: CatalogEntry): { text: string; tone: Tone; hint: string } {
  if (entry.resolver.type === "manual") {
    return { text: t("downloads.vendorPage"), tone: "amber", hint: t("downloads.vendorPageHint") };
  }
  if (entry.signature === "gpg") {
    return {
      text: t("downloads.verifiedGpg"),
      tone: "violet",
      hint: t("downloads.verifiedGpgHint"),
    };
  }
  if (entry.resolver.type === "direct") {
    return { text: t("downloads.noChecksum"), tone: "amber", hint: t("downloads.noChecksumHint") };
  }
  return { text: t("downloads.verifiedSha"), tone: "ok", hint: t("downloads.verifiedShaHint") };
}

/** Insignia de compatibilidad de arranque desde el USB de Rebootera. */
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

/** Primera frase de una nota. */
function firstSentence(text: string): string {
  const end = text.indexOf(". ");
  return end > 0 ? text.slice(0, end + 1) : text;
}

/** Tamaño de una entrada, o "…" mientras se consulta. */
function sizeText(entry: CatalogEntry, selected: boolean): string {
  if (entry.resolver.type === "manual") {
    return "";
  }
  const info = resolved.get(entry.id);
  if (info === undefined) {
    // * Solo se consulta la versión elegida de cada tarjeta; las demás no enseñan nada hasta elegirlas.
    return selected ? "…" : "";
  }
  if (info === "error") {
    return "—";
  }
  return info.sizeBytes > 0 ? fmt.bytes(info.sizeBytes) : "";
}

/** Trabajo activo (en cola, en marcha o en pausa) de una entrada del catálogo. */
function activeJob(entry: CatalogEntry): Job | undefined {
  return snapshot.transfers.find(function sameEntry(job) {
    return (
      job.catalogId === entry.id &&
      (job.status === "queued" ||
        job.status === "running" ||
        job.status === "paused" ||
        job.status === "verifying")
    );
  });
}

/** Una versión dentro de la tarjeta de su familia. */
function versionRow(entry: CatalogEntry, selected: boolean): HTMLElement {
  const input = el("input", {
    type: "radio",
    name: "version-" + entry.family,
    value: entry.id,
    checked: selected,
    "data-family": entry.family,
  });
  input.addEventListener("change", chooseVersion);
  // * Las etiquetas de la versión elegida ya salen en la cabecera de la tarjeta.
  const tags = selected
    ? []
    : entry.tags.map(function tagBadge(tag) {
        return badge(t("downloads.tags." + tag), TAG_TONES[tag] ?? "muted");
      });
  return el("label", { class: "fam-version" + (selected ? " fam-selected" : "") }, [
    input,
    el("div", { class: "fam-version-main" }, [
      el("strong", {}, [entry.edition]),
      el("span", { class: "muted fam-version-note" }, [
        firstSentence(entry.notes[getLanguage()] ?? entry.notes.en),
      ]),
    ]),
    el("div", { class: "fam-version-tags" }, tags),
    el("span", { class: "mono-small muted fam-version-size" }, [sizeText(entry, selected)]),
  ]);
}

/** Barra de progreso de una descarga dentro de la tarjeta. */
function inlineProgress(job: Job): HTMLElement {
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
  if (job.status === "paused") {
    actions.push(jobButton(job, "resume", t("downloads.resume"), icons.download));
  } else if (job.status !== "verifying") {
    actions.push(jobButton(job, "pause", t("downloads.pause"), icons.clock));
  }
  if (job.status !== "verifying") {
    actions.push(jobButton(job, "cancel", t("downloads.cancel"), icons.x));
  }
  return el("div", { class: "fam-progress" }, [
    el("div", { class: "fam-progress-head" }, [
      badge(t("downloads.status." + job.status), STATUS_TONES[job.status]),
      el("strong", {}, [Math.round(fraction * 100) + " %"]),
    ]),
    progress(
      job.status === "verifying" ? 1 : fraction,
      job.status === "paused" ? "warn" : "accent",
    ),
    el("span", { class: "mono-small muted" }, [details.join(" · ")]),
    el("div", { class: "image-actions" }, actions),
  ]);
}

/** Botón pequeño de una acción sobre una transferencia. */
function jobButton(job: Job, action: string, label: string, iconPaths: string): HTMLButtonElement {
  const node = button(label, transferAction, { iconPaths });
  node.classList.add("btn-small");
  node.dataset.id = job.id;
  node.dataset.action = action;
  return node;
}

/** Pie de una tarjeta: la acción principal según el estado de la versión elegida. */
function familyFooter(entry: CatalogEntry): HTMLElement {
  const job = activeJob(entry);
  if (job) {
    return inlineProgress(job);
  }
  const destination =
    appMode.usb || appMode.library ? t("downloads.toUsb") : t("downloads.download");
  const actions: HTMLElement[] = [];
  const info = resolved.get(entry.id);
  if (entry.resolver.type === "manual") {
    const open = button(t("downloads.openVendor"), openVendor, {
      variant: "primary",
      iconPaths: icons.globe,
    });
    open.dataset.url = entry.homepage;
    actions.push(open);
  } else if (entry.inLibrary) {
    actions.push(badge(t("downloads.inLibrary"), "ok"));
  } else {
    const get = button(destination, startDownload, {
      variant: "primary",
      iconPaths: icons.download,
    });
    get.dataset.id = entry.id;
    get.disabled = !snapshot.vault?.available;
    actions.push(get);
  }
  if (entry.resolver.type !== "manual") {
    const copy = button(t("downloads.copySha"), copySha, { iconPaths: icons.check });
    copy.classList.add("btn-small");
    copy.dataset.id = entry.id;
    copy.disabled = !(info && info !== "error" && info.sha256);
    if (copy.disabled) {
      copy.title = t("downloads.copyShaWait");
    }
    actions.push(copy);
  }
  return el("div", { class: "fam-actions" }, actions);
}

/** Tarjeta de una familia con sus versiones. */
function familyCard(family: Family, entries: CatalogEntry[]): HTMLElement {
  const chosenId = picked.get(family.id);
  const chosen =
    entries.find(function isChosen(entry) {
      return entry.id === chosenId;
    }) ??
    entries.find(function isRecommended(entry) {
      return entry.tags.includes("recommended");
    }) ??
    entries[0];
  queueResolve(chosen.id);
  const check = verification(chosen);
  const verified = badge(check.text, check.tone);
  verified.title = check.hint;
  const iconPaths = FAMILY_ICONS[family.icon] ?? icons.library;
  return el("article", { class: "fam-card" }, [
    el("header", { class: "fam-head" }, [
      el("span", { class: "fam-icon" }, [icon(iconPaths)]),
      el("div", { class: "fam-title" }, [
        el("h3", {}, [family.name]),
        el("span", { class: "muted" }, [family.vendor]),
      ]),
      el(
        "div",
        { class: "fam-badges" },
        chosen.tags.slice(0, 2).map(function tagBadge(tag) {
          return badge(t("downloads.tags." + tag), TAG_TONES[tag] ?? "muted");
        }),
      ),
    ]),
    el("p", { class: "fam-summary" }, [family.summary[getLanguage()] ?? family.summary.en]),
    el(
      "div",
      { class: "fam-versions", role: "radiogroup", "aria-label": family.name },
      entries.map(function row(entry) {
        return versionRow(entry, entry.id === chosen.id);
      }),
    ),
    el("div", { class: "fam-facts" }, [verified, bootBadge(chosen)]),
    familyFooter(chosen),
  ]);
}

/** Indica si una familia encaja con la pestaña y el texto buscado. */
function matches(family: Family, entries: CatalogEntry[]): boolean {
  if (tab !== "all" && family.group !== tab) {
    return false;
  }
  if (query === "") {
    return true;
  }
  const lang = getLanguage();
  const haystack = [
    family.name,
    family.vendor,
    family.summary[lang] ?? "",
    ...entries.map(function names(entry) {
      return entry.name + " " + entry.version + " " + entry.edition;
    }),
  ]
    .join(" ")
    .toLowerCase();
  return query.split(/\s+/).every(function hasWord(word) {
    return haystack.includes(word);
  });
}

/** Dibuja las tarjetas según la pestaña y el buscador. */
function renderList(): void {
  if (listBody === null || families === null || catalog === null || !listBody.isConnected) {
    return;
  }
  const byFamily = new Map<string, CatalogEntry[]>();
  for (const entry of catalog) {
    byFamily.set(entry.family, [...(byFamily.get(entry.family) ?? []), entry]);
  }
  const cards: HTMLElement[] = [];
  for (const family of families) {
    const entries = byFamily.get(family.id) ?? [];
    if (entries.length > 0 && matches(family, entries)) {
      cards.push(familyCard(family, entries));
    }
  }
  listBody.replaceChildren(
    ...(cards.length > 0 ? cards : [el("p", { class: "muted" }, [t("downloads.noResults")])]),
  );
  drawTabs();
}

/** Número de familias de cada pestaña (para el contador). */
function tabCount(name: Tab): number {
  if (families === null) {
    return 0;
  }
  return families.filter(function inTab(family) {
    return name === "all" || family.group === name;
  }).length;
}

/** Dibuja los botones de las pestañas. */
function drawTabs(): void {
  if (tabsBody === null) {
    return;
  }
  tabsBody.replaceChildren(
    ...TABS.map(function tabButton(name) {
      const count = tabCount(name);
      const node = el(
        "button",
        {
          class: "chip-button" + (tab === name ? " chip-active" : ""),
          type: "button",
          role: "tab",
          "aria-selected": String(tab === name),
          "data-tab": name,
        },
        [
          t("downloads.tabs." + name),
          count > 0 ? el("span", { class: "chip-count" }, [String(count)]) : null,
        ],
      );
      node.addEventListener("click", chooseTab);
      return node;
    }),
  );
}

/** Tarjeta del destino de las descargas: el USB (o la carpeta del PC) y su espacio. */
function drawDestination(): void {
  if (destinationBody === null) {
    return;
  }
  const vault = snapshot.vault;
  if (vault === null) {
    destinationBody.replaceChildren();
    return;
  }
  if (!vault.available) {
    destinationBody.replaceChildren(
      el("div", { class: "dl-dest-head" }, [
        icon(icons.lock),
        el("strong", {}, [t("downloads.noStorage")]),
      ]),
      el("p", { class: "muted small-note" }, [t("downloads.noStorageHint")]),
    );
    return;
  }
  const total = vault.usedBytes + vault.freeBytes;
  const where = appMode.usb
    ? t("downloads.destUsb", { name: drive?.label || appMode.usb })
    : appMode.library
      ? t("downloads.destLocal")
      : t("downloads.destLive");
  destinationBody.replaceChildren(
    el("div", { class: "dl-dest-head" }, [
      icon(appMode.library && !appMode.usb ? icons.disk : icons.usb),
      el("strong", {}, [where]),
      drive?.fileSystem ? badge(drive.fileSystem, "muted") : null,
    ]),
    progress(vault.usedBytes / Math.max(1, total), "accent"),
    el("span", { class: "mono-small muted" }, [
      t("downloads.destFree", { free: fmt.bytes(vault.freeBytes), total: fmt.bytes(total) }),
    ]),
  );
}

/** Texto y números de la barra inferior. */
function drawBar(): void {
  if (barBody === null) {
    return;
  }
  const active = snapshot.transfers.filter(function isActive(job) {
    return (
      job.status === "running" ||
      job.status === "queued" ||
      job.status === "verifying" ||
      job.status === "paused"
    );
  });
  const running = active.find(function isRunning(job) {
    return job.status === "running" || job.status === "verifying";
  });
  let summary: HTMLElement;
  if (active.length === 0) {
    summary = el("span", { class: "muted" }, [t("downloads.queueIdle")]);
  } else {
    const lead = running ?? active[0];
    const fraction = lead.totalBytes > 0 ? lead.doneBytes / lead.totalBytes : 0;
    const speed = running && running.speedBps > 0 ? " · " + fmt.rate(running.speedBps) : "";
    summary = el("div", { class: "dl-bar-summary" }, [
      el("strong", {}, [t("downloads.queueActive", { n: active.length })]),
      el("span", { class: "mono-small" }, [
        lead.name + " · " + Math.round(fraction * 100) + " %" + speed,
      ]),
      progress(fraction, "accent"),
    ]);
  }
  barBody.replaceChildren(
    summary,
    el("div", { class: "image-actions" }, [
      button(t("downloads.viewQueue"), openQueue, { iconPaths: icons.library }),
      button(t("downloads.bandwidth"), openBandwidth, {
        variant: "primary",
        iconPaths: icons.activity,
      }),
    ]),
  );
}

/** Fila de una transferencia en la ventana de la cola. */
function jobRow(job: Job): HTMLElement {
  const fraction = job.totalBytes > 0 ? job.doneBytes / job.totalBytes : 0;
  const details: string[] = [];
  if (job.totalBytes > 0) {
    details.push(fmt.bytes(job.doneBytes) + " / " + fmt.bytes(job.totalBytes));
  }
  if (job.status === "running" && job.speedBps > 0) {
    details.push(fmt.rate(job.speedBps));
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
        badge(t("downloads.status." + job.status), STATUS_TONES[job.status]),
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

/** Abre la ventana con la cola de transferencias. */
function openQueue(): void {
  const jobs = [...snapshot.transfers].reverse();
  const clear = button(t("downloads.clear"), clearFinished, { iconPaths: icons.check });
  clear.classList.add("btn-small");
  openModal(
    t("downloads.queue"),
    [
      ...(jobs.length > 0
        ? jobs.map(jobRow)
        : [el("p", { class: "muted" }, [t("downloads.queueEmpty")])]),
      el("div", { class: "modal-actions" }, [clear]),
    ],
    "modal-wide",
  );
}

/** Velocidades máximas que se ofrecen (MB/s); 0 es sin límite. */
const SPEEDS = [0, 2, 5, 10, 25, 50];

/** Conexiones que se ofrecen. */
const CONNECTIONS = [1, 2, 4, 6, 8];

/** Abre la ventana del ancho de banda. */
async function openBandwidth(): Promise<void> {
  let settings: BandwidthSettings;
  try {
    settings = await getJSON<BandwidthSettings>("/api/transfers/settings");
  } catch (error) {
    message?.replaceChildren(errorBox(error));
    return;
  }
  const state = {
    speed: Math.round(settings.maxBytesPerSecond / 1e6),
    connections: settings.connections,
  };
  const body = el("div", { class: "bw-dialog" });
  const dialog = openModal(t("downloads.bandwidth"), [body], "modal-wide");

  /** Dibuja las opciones con la selección actual. */
  function draw(): void {
    const speedButtons = SPEEDS.map(function speedChoice(value) {
      const node = el(
        "button",
        {
          class: "segment-button",
          type: "button",
          role: "radio",
          "aria-checked": String(state.speed === value),
          "data-value": String(value),
        },
        [value === 0 ? t("downloads.noLimit") : value + " MB/s"],
      );
      node.addEventListener("click", pickSpeed);
      return node;
    });
    const connectionButtons = CONNECTIONS.map(function connectionChoice(value) {
      const node = el(
        "button",
        {
          class: "segment-button",
          type: "button",
          role: "radio",
          "aria-checked": String(state.connections === value),
          "data-value": String(value),
        },
        [String(value)],
      );
      node.addEventListener("click", pickConnections);
      return node;
    });
    body.replaceChildren(
      el("h3", {}, [t("downloads.bwSpeed")]),
      el("p", { class: "muted" }, [t("downloads.bwSpeedHint")]),
      el("div", { class: "segmented", role: "radiogroup" }, speedButtons),
      el("h3", {}, [t("downloads.bwConnections")]),
      el("p", { class: "muted" }, [t("downloads.bwConnectionsHint")]),
      el("div", { class: "segmented", role: "radiogroup" }, connectionButtons),
      el("div", { class: "modal-actions" }, [
        button(t("downloads.save"), save, { variant: "primary" }),
      ]),
    );
  }

  /** Elige un límite de velocidad. */
  function pickSpeed(event: MouseEvent): void {
    state.speed = Number((event.currentTarget as HTMLElement).dataset.value);
    draw();
  }

  /** Elige cuántas conexiones. */
  function pickConnections(event: MouseEvent): void {
    state.connections = Number((event.currentTarget as HTMLElement).dataset.value);
    draw();
  }

  /** Guarda los ajustes (se aplican a la descarga en marcha). */
  async function save(): Promise<void> {
    try {
      await postJSON("/api/transfers/settings", {
        maxBytesPerSecond: state.speed * 1_000_000,
        connections: state.connections,
      });
      dialog.close();
    } catch (error) {
      body.prepend(errorBox(error));
    }
  }

  draw();
}

/** Refresca el destino, la barra inferior y las tarjetas con las descargas. */
function update(): void {
  if (page === null || !page.isConnected || snapshot.vault === null) {
    return;
  }
  const signature = JSON.stringify([snapshot.vault, snapshot.transfers]);
  if (signature === lastQueue) {
    return;
  }
  lastQueue = signature;
  drawDestination();
  drawBar();
  // * Cuando termina una transferencia se recarga el catálogo para marcar "en tu biblioteca".
  const done = snapshot.transfers.filter(isDone).length;
  if (doneCount >= 0 && done > doneCount) {
    reloadCatalog();
  } else {
    renderList();
  }
  doneCount = done;
}

/** Indica si una transferencia ha terminado bien. */
function isDone(job: Job): boolean {
  return job.status === "done";
}

export const downloadsPage: Page = { render, update };

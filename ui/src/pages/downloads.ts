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
import { badge, button, errorBox, logoTile, progress, type Tone } from "../components.js";
import { openModal } from "../dialogs.js";
import { el, icon } from "../dom.js";
import * as fmt from "../format.js";
import { getLanguage, t } from "../i18n.js";
import { icons } from "../icons.js";
import { appMode } from "../mode.js";
import { navigate, type Page } from "../router.js";
import { refreshVault, snapshot } from "../state.js";
import { linkPanel } from "./linkpanel.js";

/** Pestañas de la lista. */
const TABS = ["all", "windows", "linux", "servers", "rescue", "other"] as const;
type Tab = (typeof TABS)[number];

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
/** Familia cuyo selector de versión está desplegado, si hay alguna. */
let openPicker = "";
let drive: Drive | null = null;
let lastQueue = "";
let doneCount = -1;
const picked = new Map<string, string>();
const resolved = new Map<string, Resolved | "error">();
const resolveQueue: string[] = [];
let resolving = 0;

/** Deja escrita la búsqueda con la que se abrirá la página (la usa el lanzador de la portada). */
export function presetSearch(text: string): void {
  query = text.trim().toLowerCase();
}

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
    el("header", { class: "page-header" }, [
      el("div", {}, [
        el("h1", { class: "page-title" }, [t("downloads.title")]),
        el("p", { class: "page-subtitle" }, [t("downloads.subtitle")]),
      ]),
      el("div", { class: "page-actions" }, [
        button(t("downloads.viewQueue"), openQueue, { iconPaths: icons.library }),
        button(t("downloads.bandwidth"), openBandwidth, { iconPaths: icons.activity }),
      ]),
    ]),
    destinationBody,
    message,
    el("div", { class: "dl-tools" }, [search, tabsBody]),
    listBody,
    linkPanel(),
    barBody,
  ]);
  root.append(page);
  lastQueue = "";
  openPicker = "";
  document.addEventListener("click", closePicker);
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

/** Elige la versión de una familia y cierra el selector. */
function chooseVersion(event: MouseEvent): void {
  const node = event.currentTarget as HTMLElement;
  picked.set(node.dataset.family ?? "", node.dataset.value ?? "");
  openPicker = "";
  renderList();
}

/** Despliega o recoge el selector de versión de una familia. */
function togglePicker(event: MouseEvent): void {
  event.stopPropagation();
  const family = (event.currentTarget as HTMLElement).dataset.family ?? "";
  openPicker = openPicker === family ? "" : family;
  renderList();
}

/** Recoge el selector al pulsar fuera de él (y deja de escuchar si la página ya no está). */
function closePicker(): void {
  if (page === null || !page.isConnected) {
    document.removeEventListener("click", closePicker);
    return;
  }
  if (openPicker !== "") {
    openPicker = "";
    renderList();
  }
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

/** Texto de una versión en el selector: edición y, si se sabe, tamaño. */
function versionLabel(entry: CatalogEntry): string {
  const size = sizeText(entry, false);
  return entry.edition + (size && size !== "—" ? " · " + size : "");
}

/** Una opción de la lista de versiones. */
function versionOption(entry: CatalogEntry, chosen: CatalogEntry): HTMLElement {
  const node = el(
    "button",
    {
      class: "ver-option",
      type: "button",
      role: "option",
      "aria-selected": String(entry.id === chosen.id),
      "data-family": entry.family,
      "data-value": entry.id,
    },
    [
      el("span", {}, [entry.edition]),
      el("small", {}, [sizeText(entry, false)]),
      entry.tags.includes("recommended") ? badge(t("downloads.tags.recommended"), "accent") : null,
    ],
  );
  node.addEventListener("click", chooseVersion);
  return node;
}

/** Selector de versión de una familia (solo si hay más de una): un botón que despliega la lista. */
function versionSelect(entries: CatalogEntry[], chosen: CatalogEntry): HTMLElement {
  if (entries.length < 2) {
    return el("span", { class: "fam-edition" }, [chosen.edition]);
  }
  const open = openPicker === chosen.family;
  const trigger = el(
    "button",
    {
      class: "ver-trigger",
      type: "button",
      "aria-haspopup": "listbox",
      "aria-expanded": String(open),
      "aria-label": chosen.name,
      "data-family": chosen.family,
    },
    [el("span", {}, [versionLabel(chosen)]), icon(icons.chevronRight, "icon ver-caret")],
  );
  trigger.addEventListener("click", togglePicker);
  const parts: HTMLElement[] = [trigger];
  if (open) {
    parts.push(
      el("div", { class: "ver-list", role: "listbox" }, [
        ...entries.map(function option(entry) {
          return versionOption(entry, chosen);
        }),
      ]),
    );
  }
  return el("div", { class: "ver-picker" }, parts);
}

/** Abre la ventana con los detalles de una versión: verificación, arranque y SHA-256. */
function openDetails(event: MouseEvent): void {
  const id = (event.currentTarget as HTMLElement).dataset.id ?? "";
  const entry = catalog?.find(function sameId(item) {
    return item.id === id;
  });
  if (!entry) {
    return;
  }
  const check = verification(entry);
  const info = resolved.get(entry.id);
  const rows: HTMLElement[] = [
    el("p", {}, [entry.notes[getLanguage()] ?? entry.notes.en]),
    el("div", { class: "fam-facts" }, [badge(check.text, check.tone), bootBadge(entry)]),
    el("p", { class: "muted" }, [check.hint]),
  ];
  if (info && info !== "error" && info.fileName) {
    rows.push(el("p", { class: "muted" }, [info.fileName]));
  }
  if (entry.resolver.type !== "manual") {
    const copy = button(t("downloads.copySha"), copySha, { iconPaths: icons.check });
    copy.dataset.id = entry.id;
    copy.disabled = !(info && info !== "error" && info.sha256);
    rows.push(el("div", { class: "modal-actions" }, [copy]));
  }
  openModal(entry.name + " " + entry.version, rows);
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
  let main: HTMLElement;
  if (entry.resolver.type === "manual") {
    main = button(t("downloads.openVendor"), openVendor, {
      variant: "primary",
      iconPaths: icons.globe,
    });
    main.dataset.url = entry.homepage;
  } else if (entry.inLibrary) {
    main = badge(t("downloads.inLibrary"), "ok");
  } else {
    main = button(t("downloads.download"), startDownload, {
      variant: "primary",
      iconPaths: icons.download,
    });
    main.dataset.id = entry.id;
    if (main instanceof HTMLButtonElement) {
      main.disabled = !snapshot.vault?.available;
    }
  }
  const details = button(t("downloads.details"), openDetails, { iconPaths: icons.info });
  details.classList.add("btn-small", "btn-quiet");
  details.dataset.id = entry.id;
  return el("div", { class: "fam-actions" }, [main, details]);
}

/** Tarjeta de una familia: nombre, una frase, la versión a elegir y un botón. */
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
  return el("article", { class: "fam-card" }, [
    el("header", { class: "fam-head" }, [
      logoTile(family.id, family.name),
      el("div", { class: "fam-title" }, [
        el("h3", {}, [family.name]),
        el("span", { class: "muted" }, [family.summary[getLanguage()] ?? family.summary.en]),
      ]),
    ]),
    versionSelect(entries, chosen),
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

/** Dibuja los botones de las pestañas. */
function drawTabs(): void {
  if (tabsBody === null) {
    return;
  }
  tabsBody.replaceChildren(
    ...TABS.map(function tabButton(name) {
      const node = el(
        "button",
        {
          class: "chip-button" + (tab === name ? " chip-active" : ""),
          type: "button",
          role: "tab",
          "aria-selected": String(tab === name),
          "data-tab": name,
        },
        [t("downloads.tabs." + name)],
      );
      node.addEventListener("click", chooseTab);
      return node;
    }),
  );
}

/** Abre Crear USB (Rebootera Desktop). */
function openCreateUsb(): void {
  navigate("create");
}

/**
 * Dónde se guardan las descargas, sin dudas: solo en el USB de Rebootera. Con USB, dice cuál y cuánto
 * espacio queda; sin USB, avisa de que hay que conectarlo (nunca se descarga al disco del PC).
 */
function drawDestination(): void {
  if (destinationBody === null) {
    return;
  }
  const vault = snapshot.vault;
  if (vault === null) {
    destinationBody.replaceChildren();
    destinationBody.className = "dest";
    return;
  }
  if (!vault.available) {
    const desktopMode = appMode.mode === "desktop";
    destinationBody.className = "dest dest-warn";
    destinationBody.replaceChildren(
      el("span", { class: "dest-icon" }, [icon(icons.lock)]),
      el("div", { class: "dest-text" }, [
        el("strong", {}, [t("downloads.noUsbTitle")]),
        el("span", {}, [desktopMode ? t("downloads.noUsbText") : t("downloads.noUsbLive")]),
      ]),
      ...(desktopMode
        ? [
            button(t("downloads.makeUsb"), openCreateUsb, {
              variant: "primary",
              iconPaths: icons.sparkle,
            }),
          ]
        : []),
    );
    return;
  }
  const total = vault.usedBytes + vault.freeBytes;
  const name = appMode.usb
    ? (drive?.label || "REBOOTERA") + " (" + appMode.usb + ")"
    : t("downloads.destLiveName");
  destinationBody.className = "dest dest-ok";
  destinationBody.replaceChildren(
    el("span", { class: "dest-icon" }, [icon(icons.usb)]),
    el("div", { class: "dest-text" }, [
      el("strong", {}, [t("downloads.destTitle")]),
      el("span", {}, [
        name +
          " · " +
          t("downloads.destFree", { free: fmt.bytes(vault.freeBytes), total: fmt.bytes(total) }),
      ]),
    ]),
    progress(vault.usedBytes / Math.max(1, total), "accent"),
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
  barBody.hidden = active.length === 0;
  barBody.replaceChildren(summary);
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

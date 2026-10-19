// * Página "Crear USB" de Rebootera Desktop: la única forma de escribir un USB.
// * Un pendrive nuevo se borra y se convierte en un USB de Rebootera (Windows);
// * a un USB que ya tiene cosas (por ejemplo uno hecho con Rufus) se le añade
// * Rebootera sin borrar nada; un USB de Rebootera que ya tienes se actualiza. En los dos
// * casos puedes meter ya, en el mismo paso, imágenes (por ejemplo tu ISO de
// * Windows 11 LTSC) y carpetas de drivers: salen dentro del USB sin mover nada a mano.
// ! Borrar un disco exige elegirlo entre los USB que no son de sistema y escribir
// ! el texto exacto de confirmación. Actualizar un USB de Rebootera no borra nada.

import { getJSON, postJSON } from "../api.js";
import { button, emptyState, errorBox, more, pageHeader, panel, progress } from "../components.js";
import { el, icon } from "../dom.js";
import * as fmt from "../format.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";
import { guessCategory } from "../importer.js";
import { appMode } from "../mode.js";
import { navigate, type Page } from "../router.js";
import { refreshVault } from "../state.js";

/** Volumen con letra dentro de un disco USB. */
interface USBVolume {
  letter: string;
  label: string;
  fileSystem: string;
  sizeBytes: number;
  freeBytes: number;
}

/** Disco USB de /api/desktop/disks. */
interface USBDisk {
  number: number;
  friendlyName: string;
  sizeBytes: number;
  volumes: USBVolume[];
  usedBytes: number;
  rebootera: boolean;
  /** Se puede añadir Rebootera sin borrar lo que tiene (NTFS o exFAT con datos). */
  adoptable: boolean;
}

/** Unidad de /api/desktop/drives. */
interface Drive {
  root: string;
  label: string;
  sizeBytes: number;
  freeBytes: number;
  rebootera: boolean;
  version: string;
}

/** Imagen que se copia al USB. */
interface ImageChoice {
  path: string;
  category: string;
}

/** A partir de este tamaño, un disco USB parece un disco externo y no un pendrive (256 GB). */
const LARGE_DISK = 256e9;

/** Categorías de isos/ donde se pueden dejar imágenes. */
const CATEGORIES = ["windows", "linux", "tools"];

/** Estado de la creación o actualización de /api/desktop/create. */
interface CreateStatus {
  running: boolean;
  phase: string;
  progress: number;
  item?: string;
  error?: string;
  done: boolean;
  drive?: string;
}

/** Qué se va a hacer: borrar un disco, añadirle Rebootera sin borrar, o actualizar un USB de Rebootera. */
type Target =
  | { kind: "erase"; disk: number }
  | { kind: "adopt"; disk: number }
  | { kind: "keep"; root: string }
  | null;

/** De dónde sale Rebootera OS. */
type Source = "download" | "file" | "keep";

let body: HTMLElement | null = null;
let disks: USBDisk[] = [];
let drives: Drive[] = [];
let target: Target = null;
let source: Source = "download";
let iso = "";
let images: ImageChoice[] = [];
let driverDirs: string[] = [];
let typed = "";
let status: CreateStatus | null = null;
let acknowledged = false;
let selectedAfter = false;
let timer = 0;

/** Texto que hay que escribir para confirmar el borrado o el cambio del disco. */
function confirmText(): string {
  if (target?.kind === "erase") {
    return "ERASE DISK " + target.disk;
  }
  return target?.kind === "adopt" ? "ADD TO DISK " + target.disk : "";
}

/** Indica si ya está claro de dónde sale Rebootera OS. */
function sourceReady(): boolean {
  return source !== "file" || iso !== "";
}

/** Indica si hay algo que hacer (al actualizar un USB hace falta cambiar algo). */
function hasWork(): boolean {
  if (target?.kind === "keep") {
    return source !== "keep" || images.length > 0 || driverDirs.length > 0;
  }
  return true;
}

/** Indica si el botón final puede pulsarse. */
function canGo(): boolean {
  if (target === null || !sourceReady() || !hasWork()) {
    return false;
  }
  return target.kind === "keep" || typed === confirmText();
}

/** Dibuja la página. */
function render(root: HTMLElement): void {
  body = el("div", { class: "create-body" });
  root.append(
    el("div", { class: "page" }, [
      pageHeader(t("desktop.createTitle"), t("desktop.createSubtitle")),
      body,
    ]),
  );
  acknowledged = false;
  loadTargets();
  window.clearInterval(timer);
  timer = window.setInterval(pollStatus, 1000);
}

/** Lee los discos USB (solo Windows) y los USB de Rebootera ya creados. */
async function loadTargets(): Promise<void> {
  try {
    drives = (await getJSON<Drive[]>("/api/desktop/drives")).filter(function isRebootera(drive) {
      return drive.rebootera;
    });
    disks = appMode.platform === "windows" ? await getJSON<USBDisk[]>("/api/desktop/disks") : [];
  } catch (error) {
    body?.replaceChildren(errorBox(error));
    return;
  }
  // * Un único USB de Rebootera conectado se propone solo: lo normal es actualizarlo.
  if (target === null && drives.length === 1 && disks.length === 0) {
    target = { kind: "keep", root: drives[0].root };
    source = "keep";
  }
  draw();
}

/** Lee el estado de la operación mientras está en marcha. */
async function pollStatus(): Promise<void> {
  if (body === null || !body.isConnected) {
    window.clearInterval(timer);
    return;
  }
  try {
    const next = await getJSON<CreateStatus>("/api/desktop/create");
    const changed = JSON.stringify(next) !== JSON.stringify(status);
    status = next;
    if (next.done && next.drive && !selectedAfter) {
      selectedAfter = true;
      await useCreatedDrive(next.drive);
    }
    if (changed) {
      draw();
    }
  } catch {
    // * Si falla una lectura, se reintenta en el siguiente segundo.
  }
}

/** Deja el USB recién creado o actualizado como el que se usa en el resto de la aplicación. */
async function useCreatedDrive(root: string): Promise<void> {
  try {
    await postJSON("/api/desktop/select", { root });
    appMode.usb = root;
    await refreshVault();
  } catch {
    // * No es grave: se puede elegir después en el Resumen.
  }
}

/** Elige la ISO de Rebootera OS con el diálogo nativo. */
async function pickIso(): Promise<void> {
  try {
    const picked = await postJSON<{ path: string }>("/api/desktop/pick", {
      title: t("desktop.stepSystem"),
    });
    if (picked.path) {
      iso = picked.path;
      draw();
    }
  } catch (error) {
    body?.prepend(errorBox(error));
  }
}

/** Añade imágenes del equipo (varias a la vez) con el diálogo nativo. */
async function pickImages(): Promise<void> {
  try {
    const picked = await postJSON<{ paths: string[] }>("/api/desktop/pick-files", {
      title: t("desktop.addImages"),
    });
    for (const path of picked.paths) {
      const known = images.some(function sameImage(image) {
        return image.path === path;
      });
      if (!known) {
        images.push({ path, category: guessCategory(baseName(path)) });
      }
    }
    draw();
  } catch (error) {
    body?.prepend(errorBox(error));
  }
}

/** Añade una carpeta de drivers con el diálogo nativo. */
async function pickDrivers(): Promise<void> {
  try {
    const picked = await postJSON<{ path: string }>("/api/desktop/pick-folder", {
      title: t("desktop.addDrivers"),
    });
    if (picked.path && !driverDirs.includes(picked.path)) {
      driverDirs.push(picked.path);
      draw();
    }
  } catch (error) {
    body?.prepend(errorBox(error));
  }
}

/** Último componente de una ruta de Windows o Linux. */
function baseName(path: string): string {
  return path.split(/[\\/]/).pop() ?? path;
}

/** Quita la imagen de la fila pulsada. */
function removeImage(event: MouseEvent): void {
  const path = (event.currentTarget as HTMLElement).dataset.path;
  images = images.filter(function other(image) {
    return image.path !== path;
  });
  draw();
}

/** Quita la carpeta de drivers de la fila pulsada. */
function removeDrivers(event: MouseEvent): void {
  const path = (event.currentTarget as HTMLElement).dataset.path;
  driverDirs = driverDirs.filter(function other(dir) {
    return dir !== path;
  });
  draw();
}

/** Cambia la categoría de una imagen. */
function changeCategory(event: Event): void {
  const select = event.currentTarget as HTMLSelectElement;
  const image = images.find(function same(item) {
    return item.path === select.dataset.path;
  });
  if (image) {
    image.category = select.value;
  }
}

/** Elige el origen de Rebootera OS. */
function chooseSource(event: Event): void {
  source = (event.currentTarget as HTMLInputElement).value as Source;
  typed = "";
  draw();
}

/** Elige el USB: "erase:3" borra el disco 3; "keep:E:\" actualiza esa unidad. */
function chooseTarget(event: Event): void {
  const value = (event.currentTarget as HTMLInputElement).value;
  const [kind, rest] = [value.slice(0, value.indexOf(":")), value.slice(value.indexOf(":") + 1)];
  const next: Exclude<Target, null> =
    kind === "erase" || kind === "adopt"
      ? { kind, disk: Number(rest) }
      : { kind: "keep", root: rest };
  target = next;
  if (next.kind !== "keep" && source === "keep") {
    source = "download";
  }
  if (next.kind === "keep" && source === "download" && iso === "") {
    source = "keep";
  }
  typed = "";
  draw();
}

/** Guarda el texto de confirmación escrito. */
function typeConfirm(event: Event): void {
  typed = (event.currentTarget as HTMLInputElement).value;
  const go = body?.querySelector<HTMLButtonElement>(".create-go");
  if (go) {
    go.disabled = !canGo();
  }
}

/** Empieza la creación o la actualización. */
async function start(): Promise<void> {
  if (target === null) {
    return;
  }
  acknowledged = false;
  selectedAfter = false;
  const extras = { images, driverDirs };
  try {
    if (target.kind === "erase" || target.kind === "adopt") {
      // * ISO vacía: Core descarga y verifica la última versión publicada.
      await postJSON("/api/desktop/create", {
        disk: target.disk,
        iso: source === "file" ? iso : "",
        mode: target.kind === "adopt" ? "keep" : "erase",
        confirm: typed,
        ...extras,
      });
    } else {
      await postJSON("/api/desktop/apply", {
        root: target.root,
        system: source,
        iso: source === "file" ? iso : "",
        ...extras,
      });
    }
    pollStatus();
  } catch (error) {
    body?.prepend(errorBox(error));
  }
}

/** Vuelve al formulario con todo vacío tras terminar. */
function startOver(): void {
  acknowledged = true;
  images = [];
  driverDirs = [];
  typed = "";
  target = null;
  source = "download";
  iso = "";
  loadTargets();
}

/** Abre el Resumen. */
function openOverview(): void {
  navigate("overview");
}

/** Tarjeta seleccionable con un botón de opción. */
function choiceCard(options: {
  name: string;
  value: string;
  selected: boolean;
  iconPaths: string;
  title: string;
  hint: string;
  lines?: (Node | null)[];
  onChange: (event: Event) => void;
}): HTMLElement {
  const input = el("input", {
    type: "radio",
    name: options.name,
    value: options.value,
    checked: options.selected,
  });
  input.addEventListener("change", options.onChange);
  return el("label", { class: "target-card" + (options.selected ? " target-selected" : "") }, [
    input,
    el("span", { class: "disk-icon" }, [icon(options.iconPaths)]),
    el("div", { class: "disk-title" }, [
      el("strong", {}, [options.title]),
      el("span", { class: "mono-small muted" }, [options.hint]),
      ...(options.lines ?? []),
    ]),
  ]);
}

/** Aviso con icono dentro de una tarjeta. */
function warning(text: string, soft = false): HTMLElement {
  return el("span", { class: "disk-warning" + (soft ? " disk-warning-soft" : "") }, [
    icon(soft ? icons.info : icons.alert),
    text,
  ]);
}

/** Tarjeta de un USB de Rebootera que ya existe (se actualiza sin borrarlo). */
function keepCard(drive: Drive): HTMLElement {
  const used = Math.max(0, drive.sizeBytes - drive.freeBytes);
  return choiceCard({
    name: "target",
    value: "keep:" + drive.root,
    selected: target?.kind === "keep" && target.root === drive.root,
    iconPaths: icons.usb,
    title: (drive.label || "Rebootera") + " (" + drive.root + ")",
    hint:
      fmt.bytes(drive.sizeBytes) +
      " · " +
      t("desktop.volumeUsed", { used: fmt.bytes(used), size: fmt.bytes(drive.sizeBytes) }),
    lines: [
      el("span", { class: "mono-small muted" }, [
        t("desktop.version") + ": " + (drive.version || t("common.unknown")),
      ]),
    ],
    onChange: chooseTarget,
  });
}

/** Tarjeta de un disco USB que se borraría para crear un USB nuevo. */
function eraseCard(item: USBDisk): HTMLElement {
  const lines: (Node | null)[] = item.volumes.map(function volumeLine(volume) {
    const used = Math.max(0, volume.sizeBytes - volume.freeBytes);
    return el("span", { class: "mono-small muted" }, [
      volume.letter +
        ": " +
        (volume.label || "—") +
        " · " +
        t("desktop.volumeUsed", { used: fmt.bytes(used), size: fmt.bytes(volume.sizeBytes) }),
    ]);
  });
  // ! Avisos visibles antes de borrar: datos dentro, disco grande o USB de Rebootera.
  if (item.usedBytes > 0) {
    lines.push(warning(t("desktop.hasData", { used: fmt.bytes(item.usedBytes) })));
  }
  if (item.sizeBytes >= LARGE_DISK) {
    lines.push(warning(t("desktop.largeDisk")));
  }
  if (item.rebootera) {
    lines.push(warning(t("desktop.isRebootera"), true));
  }
  return choiceCard({
    name: "target",
    value: "erase:" + item.number,
    selected: target?.kind === "erase" && target.disk === item.number,
    iconPaths: icons.usb,
    title: item.friendlyName,
    hint: "#" + item.number + " · " + fmt.bytes(item.sizeBytes),
    lines,
    onChange: chooseTarget,
  });
}

/** Tarjeta para añadir Rebootera a un disco que ya tiene cosas, sin borrarlas. */
function adoptCard(item: USBDisk): HTMLElement {
  const first = item.volumes[0];
  return choiceCard({
    name: "target",
    value: "adopt:" + item.number,
    selected: target?.kind === "adopt" && target.disk === item.number,
    iconPaths: icons.usb,
    title: t("desktop.adoptTitle", { name: item.friendlyName }),
    hint: t("desktop.adoptHint", { fs: first.fileSystem, used: fmt.bytes(item.usedBytes) }),
    lines: [warning(t("desktop.adoptNote"), true)],
    onChange: chooseTarget,
  });
}

/** Panel 1: qué USB. */
function usbPanel(): HTMLElement {
  const refresh = button(t("common.refresh"), loadTargets, { iconPaths: icons.refresh });
  refresh.classList.add("btn-small");
  const parts: (Node | null)[] = [];
  if (drives.length > 0) {
    parts.push(
      el("p", { class: "group-title" }, [t("desktop.groupKeep")]),
      el("p", { class: "muted small-note" }, [t("desktop.groupKeepHint")]),
      ...drives.map(keepCard),
    );
  }
  if (appMode.platform === "windows") {
    const adoptable = disks.filter(function canAdopt(item) {
      return item.adoptable;
    });
    if (adoptable.length > 0) {
      parts.push(
        el("p", { class: "group-title" }, [t("desktop.groupAdopt")]),
        el("p", { class: "muted small-note" }, [t("desktop.groupAdoptHint")]),
        ...adoptable.map(adoptCard),
      );
    }
    parts.push(
      el("p", { class: "group-title" }, [t("desktop.groupErase")]),
      ...(disks.length > 0
        ? disks.map(eraseCard)
        : [el("p", { class: "muted" }, [t("desktop.noDisks")])]),
    );
  } else if (drives.length === 0) {
    parts.push(el("p", { class: "muted" }, [t("desktop.onlyWindows")]));
  }
  return panel(t("desktop.stepUsb"), parts, { iconPaths: icons.usb, action: refresh });
}

/** Panel 2: de dónde sale Rebootera OS. */
function systemPanel(): HTMLElement {
  const cards: HTMLElement[] = [];
  if (target?.kind === "keep") {
    cards.push(
      choiceCard({
        name: "source",
        value: "keep",
        selected: source === "keep",
        iconPaths: icons.check,
        title: t("desktop.sourceKeep"),
        hint: t("desktop.sourceKeepHint"),
        onChange: chooseSource,
      }),
    );
  }
  cards.push(
    choiceCard({
      name: "source",
      value: "download",
      selected: source === "download",
      iconPaths: icons.download,
      title: t("desktop.sourceDownload"),
      hint: t("desktop.sourceDownloadHint"),
      onChange: chooseSource,
    }),
    choiceCard({
      name: "source",
      value: "file",
      selected: source === "file",
      iconPaths: icons.library,
      title: t("desktop.sourceFile"),
      hint: t("desktop.sourceFileHint"),
      lines: [
        source === "file"
          ? button(iso ? iso : t("desktop.chooseIso"), pickIso, { iconPaths: icons.library })
          : null,
      ],
      onChange: chooseSource,
    }),
  );
  const label =
    source === "keep"
      ? t("desktop.sourceKeep")
      : source === "file"
        ? t("desktop.sourceFile")
        : t("desktop.sourceDownload");
  return more(t("desktop.stepSystem") + ": " + label, cards, "create-system");
}

/** Fila de una imagen elegida, con su categoría y el botón de quitarla. */
function imageRow(image: ImageChoice): HTMLElement {
  const select = el("select", {
    class: "category-select",
    "aria-label": t("importer.category"),
    "data-path": image.path,
  });
  for (const category of CATEGORIES) {
    const option = el("option", { value: category }, [t("library.categories." + category)]);
    option.selected = category === image.category;
    select.append(option);
  }
  select.addEventListener("change", changeCategory);
  const remove = button("", removeImage, { iconPaths: icons.x });
  remove.classList.add("btn-small");
  remove.title = t("desktop.remove");
  remove.dataset.path = image.path;
  return el("div", { class: "browse-row browse-image" }, [
    icon(icons.library),
    el("div", { class: "device-main" }, [
      el("strong", {}, [baseName(image.path)]),
      el("span", { class: "muted mono-small" }, [image.path]),
    ]),
    select,
    remove,
  ]);
}

/** Fila de una carpeta de drivers elegida. */
function driversRow(dir: string): HTMLElement {
  const remove = button("", removeDrivers, { iconPaths: icons.x });
  remove.classList.add("btn-small");
  remove.title = t("desktop.remove");
  remove.dataset.path = dir;
  return el("div", { class: "browse-row browse-image" }, [
    icon(icons.drivers),
    el("div", { class: "device-main" }, [
      el("strong", {}, [baseName(dir)]),
      el("span", { class: "muted mono-small" }, [dir]),
    ]),
    remove,
  ]);
}

/** Panel 3: imágenes y drivers que salen ya dentro del USB. */
function extrasPanel(): HTMLElement {
  const addImages = button(t("desktop.addImages"), pickImages, {
    variant: "primary",
    iconPaths: icons.library,
  });
  const addDrivers = button(t("desktop.addDrivers"), pickDrivers, { iconPaths: icons.drivers });
  return more(
    t("desktop.stepAdd"),
    [
      el("p", { class: "muted small-note" }, [t("desktop.addHint")]),
      el("div", { class: "image-actions" }, [addImages, addDrivers]),
      ...images.map(imageRow),
      ...driverDirs.map(driversRow),
    ],
    "create-extras",
  );
}

/** Panel 4: confirmación y botón final. */
function goPanel(): HTMLElement {
  const erasing = target?.kind === "erase";
  const adopting = target?.kind === "adopt";
  const goLabel = erasing
    ? t("desktop.createButton")
    : adopting
      ? t("desktop.adoptButton")
      : t("desktop.updateButton");
  const go = button(goLabel, start, {
    variant: erasing ? "danger" : "primary",
    iconPaths: erasing ? icons.alert : icons.refresh,
  });
  go.classList.add("create-go");
  go.disabled = !canGo();
  const parts: (Node | null)[] = [];
  if (erasing || adopting) {
    const confirmInput = el("input", {
      class: "text-input",
      type: "text",
      placeholder: confirmText(),
    });
    confirmInput.value = typed;
    confirmInput.addEventListener("input", typeConfirm);
    parts.push(
      adopting
        ? el("div", { class: "notice" }, [
            icon(icons.shield),
            el("span", {}, [t("desktop.adoptWarn")]),
          ])
        : el("div", { class: "error-box" }, [
            icon(icons.alert),
            el("span", {}, [t("desktop.createWarn")]),
          ]),
      el("label", { class: "confirm-field" }, [
        el("span", {}, [t("desktop.confirmHint", { text: confirmText() })]),
        confirmInput,
      ]),
      el("p", { class: "muted small-note" }, [t("desktop.needsAdmin")]),
    );
  } else if (target?.kind === "keep") {
    parts.push(
      el("div", { class: "notice" }, [icon(icons.shield), el("span", {}, [t("desktop.keepNote")])]),
    );
    if (!hasWork()) {
      parts.push(el("p", { class: "muted small-note" }, [t("desktop.nothingToDo")]));
    }
  } else {
    parts.push(el("p", { class: "muted" }, [t("desktop.chooseUsbFirst")]));
  }
  parts.push(el("div", { class: "deploy-actions" }, [go]));
  return panel(t("desktop.stepGo"), parts, { iconPaths: icons.shield });
}

/** Dibuja el estado de la operación en curso o terminada. */
function drawStatus(current: CreateStatus): HTMLElement {
  if (current.done) {
    return panel(
      t("desktop.createdTitle"),
      [
        el("p", {}, [t("desktop.createdText", { drive: current.drive ?? "" })]),
        el("div", { class: "image-actions" }, [
          button(t("nav.overview"), openOverview, {
            variant: "primary",
            iconPaths: icons.overview,
          }),
          button(t("desktop.startOver"), startOver, { iconPaths: icons.refresh }),
        ]),
      ],
      { iconPaths: icons.check },
    );
  }
  const phase = t("desktop.phases." + current.phase, {
    p: Math.round(current.progress * 100),
    item: current.item ?? "",
  });
  return panel(
    t("desktop.createTitle"),
    [
      el("p", {}, [phase]),
      progress(current.progress, "accent"),
      current.error ? errorBox(new Error(current.error)) : null,
    ],
    { iconPaths: icons.sparkle },
  );
}

/** Dibuja la página entera. */
function draw(): void {
  if (body === null) {
    return;
  }
  if (status && (status.running || (status.done && !acknowledged))) {
    body.replaceChildren(drawStatus(status));
    return;
  }
  if (disks.length === 0 && drives.length === 0 && appMode.platform !== "windows") {
    body.replaceChildren(
      emptyState(icons.info, t("desktop.createTitle"), t("desktop.onlyWindows")),
    );
    return;
  }
  body.replaceChildren(
    status?.error && !status.running ? errorBox(new Error(status.error)) : el("span"),
    usbPanel(),
    systemPanel(),
    extrasPanel(),
    goPanel(),
  );
}

export const createPage: Page = { render };

// * Página "Crear USB" de Rebootera Desktop: la única forma de escribir un USB.
// * Cada pendrive sale una sola vez. Al elegirlo se ofrece lo que se puede hacer con él: si ya es un USB de
// * Rebootera, actualizarlo (sin borrar nada); si tiene cosas (por ejemplo, hecho con Rufus), añadirle Rebootera
// * sin borrarlas; y siempre, borrarlo para empezar limpio. Después se elige de dónde sale Rebootera: la última
// * versión (se descarga y se comprueba) o una ISO propia de Rebootera.
// ! Borrar o cambiar un disco exige elegirlo entre los USB que no son de sistema y escribir el texto exacto de
// ! confirmación. Actualizar un USB de Rebootera no borra nada.

import { getJSON, postJSON } from "../api.js";
import { button, emptyState, errorBox, pageHeader, panel, progress } from "../components.js";
import { el, icon } from "../dom.js";
import * as fmt from "../format.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";
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

/** Un USB tal como se enseña: el disco físico y, si ya es de Rebootera, su unidad. */
interface Device {
  key: string;
  name: string;
  sizeBytes: number;
  usedBytes: number;
  disk: USBDisk | null;
  drive: Drive | null;
}

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

/** Lo que se hace con el USB elegido. */
type Action = "update" | "adopt" | "erase";

/** De dónde sale Rebootera OS: la última versión publicada o una ISO propia. */
type Source = "download" | "file";

/** A partir de este tamaño, un disco USB parece un disco externo y no un pendrive (256 GB). */
const LARGE_DISK = 256e9;

let body: HTMLElement | null = null;
let devices: Device[] = [];
let selectedKey = "";
let action: Action = "update";
let source: Source = "download";
let iso = "";
let typed = "";
let status: CreateStatus | null = null;
let acknowledged = false;
let selectedAfter = false;
let timer = 0;

/** El USB elegido, si hay alguno. */
function selected(): Device | null {
  return (
    devices.find(function isSelected(device) {
      return device.key === selectedKey;
    }) ?? null
  );
}

/** Acciones posibles con un USB: actualizar si ya es de Rebootera, añadir si tiene cosas, y borrar siempre. */
function actionsFor(device: Device): Action[] {
  const list: Action[] = [];
  if (device.drive !== null) {
    list.push("update");
  } else if (device.disk?.adoptable) {
    list.push("adopt");
  }
  if (device.disk !== null) {
    list.push("erase");
  }
  return list;
}

/** Texto que hay que escribir para confirmar el borrado o el cambio del disco. */
function confirmText(device: Device): string {
  if (device.disk === null) {
    return "";
  }
  return (action === "erase" ? "ERASE DISK " : "ADD TO DISK ") + device.disk.number;
}

/** Indica si el botón final puede pulsarse. */
function canGo(): boolean {
  const device = selected();
  if (device === null || (source === "file" && iso === "")) {
    return false;
  }
  return action === "update" || typed === confirmText(device);
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
  // * Al volver a esta pestaña con una operación en marcha, el progreso sale al instante (con el último estado
  // * conocido) y se actualiza ya, sin esperar a leer los discos, que en Windows tarda unos segundos.
  const busy = status !== null && (status.running || (status.done && !acknowledged));
  if (!busy) {
    acknowledged = false;
  }
  void pollStatus();
  if (busy) {
    draw();
  } else {
    void loadTargets();
  }
  window.clearInterval(timer);
  timer = window.setInterval(pollStatus, 1000);
}

/** Junta cada disco USB con su unidad de Rebootera (si la tiene) para que salga una sola vez. */
function mergeDevices(disks: USBDisk[], drives: Drive[]): Device[] {
  const used = new Set<string>();
  const list: Device[] = disks.map(function fromDisk(disk) {
    const drive =
      drives.find(function sameDisk(item) {
        return disk.volumes.some(function sameLetter(volume) {
          return volume.letter.toUpperCase() === item.root.charAt(0).toUpperCase();
        });
      }) ?? null;
    if (drive !== null) {
      used.add(drive.root);
    }
    return {
      key: "disk:" + disk.number,
      name: disk.friendlyName,
      sizeBytes: disk.sizeBytes,
      usedBytes: disk.usedBytes,
      disk,
      drive,
    };
  });
  for (const drive of drives) {
    if (!used.has(drive.root)) {
      list.push({
        key: "drive:" + drive.root,
        name: (drive.label || "Rebootera") + " (" + drive.root + ")",
        sizeBytes: drive.sizeBytes,
        usedBytes: Math.max(0, drive.sizeBytes - drive.freeBytes),
        disk: null,
        drive,
      });
    }
  }
  return list;
}

/** Lee los discos USB (solo Windows) y los USB de Rebootera ya creados. */
async function loadTargets(): Promise<void> {
  try {
    const drives = (await getJSON<Drive[]>("/api/desktop/drives")).filter(
      function isRebootera(drive) {
        return drive.rebootera;
      },
    );
    const disks =
      appMode.platform === "windows" ? await getJSON<USBDisk[]>("/api/desktop/disks") : [];
    devices = mergeDevices(disks, drives);
  } catch (error) {
    body?.replaceChildren(errorBox(error));
    return;
  }
  // * Un único USB conectado se propone solo, con la primera acción posible.
  if (selected() === null) {
    selectedKey = "";
    if (devices.length === 1) {
      choose(devices[0].key);
      return;
    }
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
    // * No es grave: se puede elegir después en Inicio.
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

/** Elige un USB y su primera acción posible. */
function choose(key: string): void {
  selectedKey = key;
  const device = selected();
  action = device ? actionsFor(device)[0] : "update";
  typed = "";
  draw();
}

/** Atiende el clic en la tarjeta de un USB. */
function onDevice(event: MouseEvent): void {
  choose((event.currentTarget as HTMLElement).dataset.key ?? "");
}

/** Atiende el clic en una acción. */
function onAction(event: MouseEvent): void {
  action = (event.currentTarget as HTMLElement).dataset.value as Action;
  typed = "";
  draw();
}

/** Atiende el clic en el origen de Rebootera. */
function onSource(event: MouseEvent): void {
  source = (event.currentTarget as HTMLElement).dataset.value as Source;
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
  const device = selected();
  if (device === null) {
    return;
  }
  acknowledged = false;
  selectedAfter = false;
  const extras = { images: [], driverDirs: [] };
  try {
    if (action === "update" && device.drive !== null) {
      await postJSON("/api/desktop/apply", {
        root: device.drive.root,
        system: source,
        iso: source === "file" ? iso : "",
        ...extras,
      });
    } else if (device.disk !== null) {
      // * ISO vacía: Core descarga y verifica la última versión publicada.
      await postJSON("/api/desktop/create", {
        disk: device.disk.number,
        iso: source === "file" ? iso : "",
        mode: action === "adopt" ? "keep" : "erase",
        confirm: typed,
        ...extras,
      });
    }
    void pollStatus();
  } catch (error) {
    body?.prepend(errorBox(error));
  }
}

/** Vuelve al formulario con todo vacío tras terminar. */
function startOver(): void {
  acknowledged = true;
  typed = "";
  selectedKey = "";
  source = "download";
  iso = "";
  void loadTargets();
}

/** Abre Inicio. */
function openHome(): void {
  navigate("overview");
}

/** Aviso con icono dentro de una tarjeta. */
function warning(text: string, soft = false): HTMLElement {
  return el("span", { class: "disk-warning" + (soft ? " disk-warning-soft" : "") }, [
    icon(soft ? icons.info : icons.alert),
    text,
  ]);
}

/** Tarjeta de un USB: nombre, tamaño, lo que ocupa y si ya es de Rebootera. */
function deviceCard(device: Device): HTMLElement {
  const isSelected = device.key === selectedKey;
  const lines: HTMLElement[] = [
    el("span", { class: "mono-small muted" }, [
      fmt.bytes(device.sizeBytes) +
        " · " +
        t("desktop.volumeUsed", {
          used: fmt.bytes(device.usedBytes),
          size: fmt.bytes(device.sizeBytes),
        }),
    ]),
  ];
  if (device.drive !== null) {
    lines.push(
      el("span", { class: "mono-small muted" }, [
        t("desktop.version") + ": " + (device.drive.version || t("common.unknown")),
      ]),
    );
  }
  const card = el(
    "button",
    {
      class: "target-card" + (isSelected ? " target-selected" : ""),
      type: "button",
      "aria-pressed": String(isSelected),
      "data-key": device.key,
    },
    [
      el("span", { class: "disk-icon" }, [icon(icons.usb)]),
      el("div", { class: "disk-title" }, [
        el("strong", {}, [device.name]),
        ...lines,
        device.drive !== null ? warning(t("desktop.isRebootera"), true) : null,
      ]),
    ],
  );
  card.addEventListener("click", onDevice);
  return card;
}

/** Panel 1: qué USB. */
function usbPanel(): HTMLElement {
  const refresh = button(t("common.refresh"), loadTargets, { iconPaths: icons.refresh });
  refresh.classList.add("btn-small");
  const parts: HTMLElement[] =
    devices.length > 0
      ? devices.map(deviceCard)
      : [
          el("p", { class: "muted" }, [
            appMode.platform === "windows" ? t("desktop.noDisks") : t("desktop.onlyWindows"),
          ]),
        ];
  return panel(t("desktop.stepUsb"), parts, { iconPaths: icons.usb, action: refresh });
}

/** Una opción grande con título y explicación (acción u origen). */
function optionCard(
  value: string,
  chosen: boolean,
  iconPaths: string,
  title: string,
  hint: string,
  onPick: (event: MouseEvent) => void,
  extra: Node | null = null,
): HTMLElement {
  const card = el(
    "button",
    {
      class: "target-card" + (chosen ? " target-selected" : ""),
      type: "button",
      "aria-pressed": String(chosen),
      "data-value": value,
    },
    [
      el("span", { class: "disk-icon" }, [icon(iconPaths)]),
      el("div", { class: "disk-title" }, [
        el("strong", {}, [title]),
        el("span", { class: "mono-small muted" }, [hint]),
        extra,
      ]),
    ],
  );
  card.addEventListener("click", onPick);
  return card;
}

/** Panel 2: qué hacer con el USB elegido. */
function actionPanel(device: Device): HTMLElement {
  const cards = actionsFor(device).map(function actionCard(item) {
    const erase = item === "erase";
    const hint =
      item === "update"
        ? t("desktop.actUpdateHint")
        : item === "adopt"
          ? t("desktop.adoptNote")
          : t("desktop.actEraseHint");
    const extra =
      erase && action === "erase" && device.usedBytes > 0
        ? warning(t("desktop.hasData", { used: fmt.bytes(device.usedBytes) }))
        : null;
    return optionCard(
      item,
      action === item,
      erase ? icons.alert : item === "update" ? icons.refresh : icons.check,
      t("desktop.act" + item.charAt(0).toUpperCase() + item.slice(1)),
      hint,
      onAction,
      extra,
    );
  });
  return panel(t("desktop.actionTitle"), cards, { iconPaths: icons.sparkle });
}

/** Panel 3: de dónde sale Rebootera OS. La opción por defecto es la última versión. */
function systemPanel(device: Device): HTMLElement {
  const updating = device.drive !== null && action === "update";
  const fileButton =
    source === "file"
      ? button(iso ? iso : t("desktop.chooseIso"), pickIso, { iconPaths: icons.library })
      : null;
  return panel(
    t("desktop.stepSystem"),
    [
      optionCard(
        "download",
        source === "download",
        icons.download,
        updating ? t("desktop.srcUpdate") : t("desktop.srcDownload"),
        t("desktop.sourceDownloadHint"),
        onSource,
      ),
      optionCard(
        "file",
        source === "file",
        icons.library,
        t("desktop.srcMine"),
        t("desktop.sourceFileHint"),
        onSource,
        fileButton,
      ),
    ],
    { iconPaths: icons.download },
  );
}

/** Panel 4: confirmación y botón final. */
function goPanel(device: Device): HTMLElement {
  const erasing = action === "erase";
  const goLabel = erasing
    ? t("desktop.createButton")
    : action === "adopt"
      ? t("desktop.adoptButton")
      : t("desktop.updateButton");
  const go = button(goLabel, start, {
    variant: erasing ? "danger" : "primary",
    iconPaths: erasing ? icons.alert : icons.refresh,
  });
  go.classList.add("create-go");
  go.disabled = !canGo();
  const parts: (Node | null)[] = [];
  if (action === "update") {
    parts.push(
      el("div", { class: "notice" }, [icon(icons.shield), el("span", {}, [t("desktop.keepNote")])]),
    );
  } else {
    const confirmInput = el("input", {
      class: "text-input",
      type: "text",
      placeholder: confirmText(device),
    });
    confirmInput.value = typed;
    confirmInput.addEventListener("input", typeConfirm);
    parts.push(
      erasing
        ? el("div", { class: "error-box" }, [
            icon(icons.alert),
            el("span", {}, [t("desktop.createWarn")]),
          ])
        : el("div", { class: "notice" }, [
            icon(icons.shield),
            el("span", {}, [t("desktop.adoptWarn")]),
          ]),
      device.sizeBytes >= LARGE_DISK ? warning(t("desktop.largeDisk")) : null,
      el("label", { class: "confirm-field" }, [
        el("span", {}, [t("desktop.confirmHint", { text: confirmText(device) })]),
        confirmInput,
      ]),
      el("p", { class: "muted small-note" }, [t("desktop.needsAdmin")]),
    );
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
          button(t("nav.overview"), openHome, { variant: "primary", iconPaths: icons.overview }),
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
  if (devices.length === 0 && appMode.platform !== "windows") {
    body.replaceChildren(
      emptyState(icons.info, t("desktop.createTitle"), t("desktop.onlyWindows")),
    );
    return;
  }
  const device = selected();
  const parts: (HTMLElement | null)[] = [
    status?.error && !status.running ? errorBox(new Error(status.error)) : null,
    usbPanel(),
  ];
  if (device !== null) {
    parts.push(actionPanel(device), systemPanel(device), goPanel(device));
  }
  body.replaceChildren(
    ...parts.filter(function present(node): node is HTMLElement {
      return node !== null;
    }),
  );
}

export const createPage: Page = { render };

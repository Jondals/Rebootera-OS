// * Página "Crear USB" de Rebootera Desktop (Windows): convierte un pendrive en
// * un USB de Rebootera con el menú de arranque (Ventoy), Rebootera OS y las
// * carpetas para imágenes.
// ! Borra todo el pendrive elegido. Solo se listan discos USB que no son de
// ! sistema, y hay que escribir el texto exacto de confirmación.

import { getJSON, postJSON } from "../api.js";
import { button, emptyState, errorBox, pageHeader, panel, progress } from "../components.js";
import { el, icon } from "../dom.js";
import * as fmt from "../format.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";
import { appMode } from "../mode.js";
import { navigate, type Page } from "../router.js";

/** Volumen con letra dentro de un disco USB. */
interface USBVolume {
  letter: string;
  label: string;
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
}

/** A partir de este tamaño, un disco USB parece un disco externo y no un pendrive (256 GB). */
const LARGE_DISK = 256e9;

/** Estado de la creación de /api/desktop/create. */
interface CreateStatus {
  running: boolean;
  phase: string;
  progress: number;
  error?: string;
  done: boolean;
  drive?: string;
}

let body: HTMLElement | null = null;
let disks: USBDisk[] = [];
let iso = "";
let disk = -1;
let typed = "";
let status: CreateStatus | null = null;
let timer = 0;

/** Texto que hay que escribir para confirmar el borrado del disco. */
function confirmText(): string {
  return "ERASE DISK " + disk;
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
  if (appMode.platform !== "windows") {
    body.replaceChildren(
      emptyState(icons.info, t("desktop.createTitle"), t("desktop.onlyWindows")),
    );
    return;
  }
  loadDisks();
  window.clearInterval(timer);
  timer = window.setInterval(pollStatus, 1000);
}

/** Lee los discos USB disponibles. */
async function loadDisks(): Promise<void> {
  try {
    disks = await getJSON<USBDisk[]>("/api/desktop/disks");
  } catch (error) {
    body?.replaceChildren(errorBox(error));
    return;
  }
  draw();
}

/** Lee el estado de la creación mientras está en marcha. */
async function pollStatus(): Promise<void> {
  if (body === null || !body.isConnected) {
    window.clearInterval(timer);
    return;
  }
  try {
    const next = await getJSON<CreateStatus>("/api/desktop/create");
    const changed = JSON.stringify(next) !== JSON.stringify(status);
    status = next;
    if (changed) {
      draw();
    }
  } catch {
    // * Si falla una lectura, se reintenta en el siguiente segundo.
  }
}

/** Elige la ISO de Rebootera OS con el diálogo nativo. */
async function pickIso(): Promise<void> {
  try {
    const picked = await postJSON<{ path: string }>("/api/desktop/pick", {
      title: t("desktop.step1"),
    });
    if (picked.path) {
      iso = picked.path;
      draw();
    }
  } catch (error) {
    body?.prepend(errorBox(error));
  }
}

/** Anota el disco elegido. */
function chooseDisk(event: Event): void {
  disk = Number((event.currentTarget as HTMLInputElement).value);
  typed = "";
  draw();
}

/** Guarda el texto de confirmación escrito. */
function typeConfirm(event: Event): void {
  typed = (event.currentTarget as HTMLInputElement).value;
  const go = body?.querySelector<HTMLButtonElement>(".create-go");
  if (go) {
    go.disabled = typed !== confirmText() || iso === "";
  }
}

/** Empieza la creación. */
async function create(): Promise<void> {
  try {
    await postJSON("/api/desktop/create", { disk, iso, confirm: typed });
    pollStatus();
  } catch (error) {
    body?.prepend(errorBox(error));
  }
}

/** Vuelve a "Mi USB". */
function openUsb(): void {
  navigate("usb");
}

/** Dibuja el estado de la creación en curso o terminada. */
function drawStatus(current: CreateStatus): HTMLElement {
  if (current.done) {
    return panel(
      t("desktop.createdTitle"),
      [
        el("p", {}, [t("desktop.createdText", { drive: current.drive ?? "" })]),
        button(t("desktop.nav.usb"), openUsb, { variant: "primary", iconPaths: icons.usb }),
      ],
      { iconPaths: icons.check },
    );
  }
  const phase = t("desktop.phases." + current.phase, { p: Math.round(current.progress * 100) });
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

/** Dibuja el formulario. */
function draw(): void {
  if (body === null) {
    return;
  }
  if (status && (status.running || status.done)) {
    body.replaceChildren(drawStatus(status));
    return;
  }
  const isoButton = button(iso ? iso : t("desktop.chooseIso"), pickIso, {
    iconPaths: icons.library,
  });
  const refresh = button(t("common.refresh"), loadDisks, { iconPaths: icons.refresh });
  refresh.classList.add("btn-small");
  const diskList = disks.length
    ? disks.map(function diskCard(item) {
        const volumes = item.volumes.map(function volumeLine(volume) {
          const used = Math.max(0, volume.sizeBytes - volume.freeBytes);
          return el("span", { class: "mono-small muted" }, [
            volume.letter +
              ": " +
              (volume.label || "—") +
              " · " +
              t("desktop.volumeUsed", { used: fmt.bytes(used), size: fmt.bytes(volume.sizeBytes) }),
          ]);
        });
        const warnings: HTMLElement[] = [];
        // ! Avisos visibles antes de borrar: datos dentro, disco grande o USB de Rebootera.
        if (item.usedBytes > 0) {
          warnings.push(
            el("span", { class: "disk-warning" }, [
              icon(icons.alert),
              t("desktop.hasData", { used: fmt.bytes(item.usedBytes) }),
            ]),
          );
        }
        if (item.sizeBytes >= LARGE_DISK) {
          warnings.push(
            el("span", { class: "disk-warning" }, [icon(icons.alert), t("desktop.largeDisk")]),
          );
        }
        if (item.rebootera) {
          warnings.push(
            el("span", { class: "disk-warning disk-warning-soft" }, [
              icon(icons.info),
              t("desktop.isRebootera"),
            ]),
          );
        }
        const input = el("input", {
          type: "radio",
          name: "disk",
          value: String(item.number),
          checked: disk === item.number,
        });
        input.addEventListener("change", chooseDisk);
        return el(
          "label",
          { class: "target-card" + (disk === item.number ? " target-selected" : "") },
          [
            input,
            el("span", { class: "disk-icon" }, [icon(icons.usb)]),
            el("div", { class: "disk-title" }, [
              el("strong", {}, [item.friendlyName]),
              el("span", { class: "mono-small muted" }, [
                "#" + item.number + " · " + fmt.bytes(item.sizeBytes),
              ]),
              ...volumes,
              ...warnings,
            ]),
          ],
        );
      })
    : [el("p", { class: "muted" }, [t("desktop.noDisks")])];
  const confirmInput = el("input", {
    class: "text-input",
    type: "text",
    placeholder: disk >= 0 ? confirmText() : "",
    disabled: disk < 0,
  });
  confirmInput.value = typed;
  confirmInput.addEventListener("input", typeConfirm);
  const go = button(t("desktop.createButton"), create, {
    variant: "danger",
    iconPaths: icons.alert,
  });
  go.classList.add("create-go");
  go.disabled = typed !== confirmText() || iso === "" || disk < 0;
  body.replaceChildren(
    status?.error ? errorBox(new Error(status.error)) : el("span"),
    panel("1 · " + t("desktop.step1"), [isoButton], { iconPaths: icons.library }),
    panel("2 · " + t("desktop.step2"), diskList, { iconPaths: icons.usb, action: refresh }),
    panel(
      "3 · " + t("desktop.step3"),
      [
        el("div", { class: "error-box" }, [
          icon(icons.alert),
          el("span", {}, [t("desktop.createWarn")]),
        ]),
        el("label", { class: "confirm-field" }, [
          el("span", {}, [
            disk >= 0 ? t("desktop.confirmHint", { text: confirmText() }) : t("desktop.step2"),
          ]),
          confirmInput,
        ]),
        el("p", { class: "muted small-note" }, [t("desktop.needsAdmin")]),
        el("div", { class: "deploy-actions" }, [go]),
      ],
      { iconPaths: icons.shield },
    ),
  );
}

export const createPage: Page = { render };

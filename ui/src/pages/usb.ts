// * Página "Mi USB" de Rebootera Desktop: detecta los USB de Rebootera
// * conectados al equipo, muestra el elegido (versión, espacio, imágenes),
// * permite actualizar Rebootera OS en él y explica cómo arrancar desde el USB.

import { getJSON, postJSON } from "../api.js";
import { badge, button, emptyState, errorBox, pageHeader, panel, progress } from "../components.js";
import { el, icon } from "../dom.js";
import * as fmt from "../format.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";
import { appMode } from "../mode.js";
import { navigate, type Page } from "../router.js";
import { refreshVault, snapshot } from "../state.js";

/** Unidad que devuelve /api/desktop/drives. */
interface Drive {
  root: string;
  label: string;
  fileSystem: string;
  sizeBytes: number;
  freeBytes: number;
  removable: boolean;
  rebootera: boolean;
  version: string;
}

/** Estado de "Actualizar Rebootera OS". */
interface UpdateStatus {
  running: boolean;
  progress: number;
  error: string;
  done: boolean;
}

let body: HTMLElement | null = null;
let message: HTMLElement | null = null;
let drives: Drive[] = [];
let updateState: UpdateStatus | null = null;
let timer = 0;
let lastSignature = "";

/** Dibuja la estructura y empieza a vigilar las unidades. */
function render(root: HTMLElement): void {
  body = el("div", { class: "usb-body" });
  message = el("div", { class: "page-message", "aria-live": "polite" });
  lastSignature = "";
  root.append(
    el("div", { class: "page" }, [
      pageHeader(t("desktop.usbTitle"), t("desktop.usbSubtitle")),
      message,
      body,
    ]),
  );
  window.clearInterval(timer);
  timer = window.setInterval(poll, 3000);
  poll();
}

/** Lee las unidades y el estado de la actualización. */
async function poll(): Promise<void> {
  if (body === null || !body.isConnected) {
    window.clearInterval(timer);
    return;
  }
  try {
    drives = await getJSON<Drive[]>("/api/desktop/drives");
    updateState = await getJSON<UpdateStatus>("/api/desktop/update");
  } catch (error) {
    message?.replaceChildren(errorBox(error));
  }
  update();
}

/** Elige el USB pulsado. */
async function useDrive(event: MouseEvent): Promise<void> {
  try {
    await postJSON("/api/desktop/select", {
      root: (event.currentTarget as HTMLElement).dataset.root,
    });
    appMode.usb = (event.currentTarget as HTMLElement).dataset.root ?? "";
    await refreshVault();
    lastSignature = "";
    update();
  } catch (error) {
    message?.replaceChildren(errorBox(error));
  }
}

/** Elige una ISO nueva de Rebootera OS y la instala en el USB. */
async function updateSystem(): Promise<void> {
  try {
    const picked = await postJSON<{ path: string }>("/api/desktop/pick", {
      title: t("desktop.updateTitle"),
    });
    if (!picked.path) {
      return;
    }
    await postJSON("/api/desktop/update", { path: picked.path });
    poll();
  } catch (error) {
    message?.replaceChildren(errorBox(error));
  }
}

/** Abre la página para crear un USB nuevo. */
function openCreate(): void {
  navigate("create");
}

/** Abre la biblioteca. */
function openLibrary(): void {
  navigate("library");
}

/** Abre el centro de descargas. */
function openDownloads(): void {
  navigate("downloads");
}

/** Tarjeta del USB elegido. */
function selectedCard(drive: Drive): HTMLElement {
  const used = drive.sizeBytes - drive.freeBytes;
  const images = snapshot.library.length;
  const updateButton = button(t("desktop.chooseIso"), updateSystem, { iconPaths: icons.refresh });
  updateButton.disabled = updateState?.running ?? false;
  let updateLine: HTMLElement | null = null;
  if (updateState?.running) {
    updateLine = el("div", { class: "part-usage" }, [
      progress(updateState.progress, "accent"),
      el("span", { class: "mono-small" }, [
        t("desktop.updating", { p: Math.round(updateState.progress * 100) }),
      ]),
    ]);
  } else if (updateState?.done) {
    updateLine = el("div", { class: "notice notice-ok" }, [
      icon(icons.check),
      t("desktop.updated"),
    ]);
  } else if (updateState?.error) {
    updateLine = el("div", { class: "error-box" }, [icon(icons.alert), updateState.error]);
  }
  return el("div", { class: "usb-grid" }, [
    panel(
      drive.label || drive.root,
      [
        el("div", { class: "usb-summary" }, [
          el("span", { class: "install-icon" }, [icon(icons.usb)]),
          el("div", {}, [
            el("strong", { class: "usb-root" }, [drive.root]),
            el("span", { class: "muted" }, [
              t("desktop.version") + ": " + (drive.version || t("common.unknown")),
            ]),
            el("span", { class: "muted" }, [
              images === 1 ? t("library.countOne") : t("library.count", { n: images }),
            ]),
          ]),
        ]),
        progress(used / Math.max(1, drive.sizeBytes), "accent"),
        el("span", { class: "mono-small muted" }, [
          t("sidebar.usedFree", { used: fmt.bytes(used), free: fmt.bytes(drive.freeBytes) }),
        ]),
        el("div", { class: "image-actions usb-actions" }, [
          button(t("library.title"), openLibrary, { iconPaths: icons.library }),
          button(t("overview.downloadTitle"), openDownloads, {
            variant: "primary",
            iconPaths: icons.download,
          }),
        ]),
      ],
      { iconPaths: icons.usb },
    ),
    panel(
      t("desktop.updateTitle"),
      [el("p", { class: "muted" }, [t("desktop.updateText")]), updateButton, updateLine],
      {
        iconPaths: icons.refresh,
      },
    ),
    bootHelp(),
  ]);
}

/** Instrucciones para arrancar desde el USB. */
function bootHelp(): HTMLElement {
  return panel(
    t("desktop.bootTitle"),
    [
      el("ol", { class: "boot-steps" }, [
        el("li", {}, [t("desktop.bootStep1")]),
        el("li", {}, [t("desktop.bootStep2")]),
        el("li", {}, [t("desktop.bootStep3")]),
      ]),
      el("div", { class: "notice" }, [
        icon(icons.shield),
        el("span", {}, [t("desktop.secureBootNote")]),
      ]),
    ],
    { iconPaths: icons.info, className: "usb-help" },
  );
}

/** Fila de una unidad del equipo. */
function driveRow(drive: Drive): HTMLElement {
  const action = drive.rebootera
    ? drive.root === appMode.usb
      ? badge(t("desktop.inUse"), "ok")
      : button(t("desktop.use"), useDrive, { variant: "primary" })
    : badge(drive.fileSystem || "—", "muted");
  if (action instanceof HTMLButtonElement) {
    action.classList.add("btn-small");
    action.dataset.root = drive.root;
  }
  return el("div", { class: "source-row" }, [
    el("span", { class: "device-icon" }, [icon(drive.removable ? icons.usb : icons.disk)]),
    el("div", { class: "device-main" }, [
      el("strong", {}, [(drive.label || drive.root) + " (" + drive.root + ")"]),
      el("span", { class: "muted" }, [
        fmt.bytes(drive.sizeBytes) + (drive.rebootera ? " · " + t("desktop.rebooteraDrive") : ""),
      ]),
    ]),
    drive.rebootera ? badge("Rebootera", "accent") : el("span"),
    action,
  ]);
}

/** Refresca la página si cambian los datos. */
function update(): void {
  if (body === null || !body.isConnected) {
    return;
  }
  const signature = JSON.stringify([drives, updateState, appMode.usb, snapshot.library.length]);
  if (signature === lastSignature) {
    return;
  }
  lastSignature = signature;
  const selected = drives.find(function isSelected(drive) {
    return drive.rebootera && drive.root === appMode.usb;
  });
  const create = button(t("desktop.createCta"), openCreate, { iconPaths: icons.sparkle });
  const list = panel(
    t("desktop.drivesTitle"),
    drives.length > 0
      ? drives.map(driveRow)
      : [el("p", { class: "muted" }, [t("desktop.noDrives")])],
    {
      iconPaths: icons.disk,
      action: create,
    },
  );
  if (selected) {
    body.replaceChildren(selectedCard(selected), list);
  } else {
    body.replaceChildren(
      emptyState(
        icons.usb,
        t("desktop.noUsbTitle"),
        t("desktop.noUsbText"),
        button(t("desktop.createCta"), openCreate, {
          variant: "primary",
          iconPaths: icons.sparkle,
        }),
      ),
      list,
      bootHelp(),
    );
  }
}

export const usbPage: Page = { render, update };

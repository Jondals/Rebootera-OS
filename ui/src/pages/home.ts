// * Página "Resumen" de Rebootera Desktop: el estado del USB de Rebootera
// * conectado (versión, espacio, imágenes, drivers), atajos a lo que se puede
// * hacer con él y cómo arrancar desde él. No escribe nada: crear o actualizar
// * el USB se hace solo desde "Crear USB".

import { getJSON, postJSON } from "../api.js";
import { badge, button, errorBox, more, progress, tile } from "../components.js";
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

let body: HTMLElement | null = null;
let message: HTMLElement | null = null;
let drives: Drive[] = [];
let driverCount = 0;
let timer = 0;
let lastSignature = "";

/** Dibuja la estructura y empieza a vigilar las unidades. */
function render(root: HTMLElement): void {
  body = el("div", { class: "usb-body" });
  message = el("div", { class: "page-message", "aria-live": "polite" });
  lastSignature = "";
  root.append(
    el("div", { class: "page home" }, [
      el("header", { class: "home-head" }, [
        el("h1", { class: "home-title" }, [t("home.title")]),
        el("p", { class: "home-subtitle" }, [t("home.subtitle")]),
      ]),
      message,
      body,
    ]),
  );
  window.clearInterval(timer);
  timer = window.setInterval(poll, 3000);
  poll();
}

/** Lee las unidades y cuántos paquetes de drivers hay en el USB elegido. */
async function poll(): Promise<void> {
  if (body === null || !body.isConnected) {
    window.clearInterval(timer);
    return;
  }
  try {
    drives = await getJSON<Drive[]>("/api/desktop/drives");
    const found = await getJSON<{ packages: unknown[] }>("/api/desktop/drivers");
    driverCount = found.packages.length;
  } catch (error) {
    message?.replaceChildren(errorBox(error));
  }
  update();
}

/** Elige el USB pulsado. */
async function useDrive(event: MouseEvent): Promise<void> {
  const root = (event.currentTarget as HTMLElement).dataset.root ?? "";
  try {
    await postJSON("/api/desktop/select", { root });
    appMode.usb = root;
    await refreshVault();
    lastSignature = "";
    poll();
  } catch (error) {
    message?.replaceChildren(errorBox(error));
  }
}

/** Abre "Crear USB". */
function openCreate(): void {
  navigate("create");
}

/** Abre las imágenes guardadas. */
function openLibrary(): void {
  navigate("library");
}

/** Abre el centro de descargas. */
function openDownloads(): void {
  navigate("downloads");
}

/** Abre el asistente de drivers. */
function openDrivers(): void {
  navigate("drivers");
}

/** Tarjeta del USB elegido: nombre, espacio y cuántas imágenes tiene. */
function usbCard(drive: Drive): HTMLElement {
  const used = drive.sizeBytes - drive.freeBytes;
  const images = snapshot.library.length;
  return el("section", { class: "usb-hero" }, [
    el("span", { class: "usb-hero-icon" }, [icon(icons.usb)]),
    el("div", { class: "usb-hero-main" }, [
      el("strong", { class: "usb-hero-title" }, [
        (drive.label || t("home.usbTitle")) + " (" + drive.root + ")",
      ]),
      el("span", { class: "muted" }, [
        t("home.usbText", { used: fmt.bytes(used), free: fmt.bytes(drive.freeBytes) }) +
          " · " +
          (images === 1 ? t("home.usbImagesOne") : t("home.usbImages", { n: images })),
      ]),
      progress(used / Math.max(1, drive.sizeBytes), "accent"),
    ]),
  ]);
}

/** Instrucciones para arrancar desde el USB. */
function bootHelp(): HTMLElement {
  return more(t("home.bootHelp"), [
    el("ol", { class: "boot-steps" }, [
      el("li", {}, [t("desktop.bootStep1")]),
      el("li", {}, [t("desktop.bootStep2")]),
      el("li", {}, [t("desktop.bootStep3")]),
    ]),
    el("div", { class: "notice" }, [
      icon(icons.shield),
      el("span", {}, [t("desktop.secureBootNote")]),
    ]),
  ]);
}

/** Fila de un USB de Rebootera conectado. */
function driveRow(drive: Drive): HTMLElement {
  const inUse = drive.root === appMode.usb;
  const action = inUse
    ? badge(t("desktop.inUse"), "ok")
    : button(t("desktop.use"), useDrive, { variant: "primary" });
  if (action instanceof HTMLButtonElement) {
    action.classList.add("btn-small");
    action.dataset.root = drive.root;
  }
  return el("div", { class: "source-row" }, [
    el("span", { class: "device-icon" }, [icon(icons.usb)]),
    el("div", { class: "device-main" }, [
      el("strong", {}, [(drive.label || drive.root) + " (" + drive.root + ")"]),
      el("span", { class: "muted" }, [fmt.bytes(drive.sizeBytes)]),
    ]),
    action,
  ]);
}

/** Refresca la página si cambian los datos. */
function update(): void {
  if (body === null || !body.isConnected) {
    return;
  }
  const signature = JSON.stringify([drives, driverCount, appMode.usb, snapshot.library.length]);
  if (signature === lastSignature) {
    return;
  }
  lastSignature = signature;
  const found = drives.filter(function isRebootera(drive) {
    return drive.rebootera;
  });
  const selected = found.find(function isSelected(drive) {
    return drive.root === appMode.usb;
  });
  const parts: HTMLElement[] = [];
  if (selected) {
    parts.push(
      usbCard(selected),
      el("div", { class: "tile-grid tile-grid-2" }, [
        tile(icons.sparkle, t("home.make"), t("home.makeText"), openCreate, { primary: true }),
        tile(icons.download, t("home.download"), t("home.downloadText"), openDownloads),
        tile(icons.library, t("home.library"), t("home.libraryText"), openLibrary),
        tile(icons.drivers, t("home.drivers"), t("home.driversText"), openDrivers),
        tile(icons.library, t("home.rufus"), t("home.rufusText"), openLibrary),
      ]),
    );
  } else {
    parts.push(
      el("div", { class: "tile-grid" }, [
        tile(icons.usb, t("home.noUsb"), t("home.noUsbText"), openCreate, { primary: true }),
      ]),
    );
  }
  // * La lista solo hace falta si hay que elegir entre varios USB o ninguno está en uso.
  if (found.length > 1 || (found.length === 1 && !selected)) {
    parts.push(more(t("home.switchUsb"), found.map(driveRow)));
  }
  parts.push(bootHelp());
  body.replaceChildren(...parts);
}

export const homePage: Page = { render, update };

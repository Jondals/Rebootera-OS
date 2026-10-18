// * Página "Biblioteca": imágenes del Vault agrupadas por categoría, con su
// * estado de verificación y acciones (comprobar integridad, instalar, borrar).
// * Las imágenes que se copian al USB desde otro ordenador aparecen solas.
// ! Una imagen dañada (hash distinto del oficial) no ofrece "Instalar".

import { postJSON, type LibraryItem } from "../api.js";
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
import { confirmAction } from "../dialogs.js";
import { el, icon } from "../dom.js";
import * as fmt from "../format.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";
import { guessCategory, openImporter } from "../importer.js";
import { installersSection } from "./installers.js";
import { isDesktop } from "../mode.js";
import { navigate, type Page } from "../router.js";
import { refreshVault, snapshot } from "../state.js";
import { chooseImage } from "./wizard.js";

/** Tono de cada estado de verificación. */
const STATE_TONES: Record<LibraryItem["state"], Tone> = {
  verified: "ok",
  unchecked: "muted",
  hashing: "accent",
  imported: "violet",
  mismatch: "danger",
};

/** Orden de las categorías en la página. */
const CATEGORIES: LibraryItem["category"][] = ["windows", "linux", "tools", "other"];

let body: HTMLElement | null = null;
let message: HTMLElement | null = null;
let lastSignature = "";

/**
 * Añade imágenes: en Rebootera OS abre el importador de discos; en Rebootera
 * Desktop abre el diálogo nativo del sistema para elegir un archivo.
 */
async function addImages(): Promise<void> {
  if (!isDesktop()) {
    openImporter();
    return;
  }
  try {
    const picked = await postJSON<{ path: string }>("/api/desktop/pick", {
      title: t("desktop.addTitle"),
    });
    if (!picked.path) {
      return;
    }
    const name = picked.path.split(/[\/]/).pop() ?? "";
    await postJSON("/api/desktop/add", { path: picked.path, category: guessCategory(name) });
    message?.replaceChildren(
      el("div", { class: "notice notice-ok" }, [icon(icons.check), t("downloads.added", { name })]),
    );
    await refreshVault();
  } catch (error) {
    showError(error);
  }
}

/** Abre el centro de descargas. */
function openDownloads(): void {
  navigate("downloads");
}

/** Dibuja la estructura fija de la página. */
function render(root: HTMLElement): void {
  body = el("div", { class: "library-body" });
  message = el("div", { class: "page-message", "aria-live": "polite" });
  lastSignature = "";
  const add = button(t("library.add"), addImages, { variant: "primary", iconPaths: icons.usb });
  const more = button(t("library.download"), openDownloads, { iconPaths: icons.download });
  root.append(
    el("div", { class: "page" }, [
      pageHeader(t("library.title"), t("library.subtitle"), [more, add]),
      message,
      body,
      installersSection(function showMessage(node) {
        message?.replaceChildren(...(node ? [node] : []));
      }),
    ]),
  );
}

/** Busca la imagen del botón pulsado. */
function itemFor(event: MouseEvent): LibraryItem | undefined {
  const path = (event.currentTarget as HTMLElement).dataset.path;
  return snapshot.library.find(function samePath(item) {
    return item.path === path;
  });
}

/** Muestra un error en la cabecera de la página. */
function showError(error: unknown): void {
  message?.replaceChildren(errorBox(error));
}

/** Calcula el hash de la imagen pulsada. */
async function verifyItem(event: MouseEvent): Promise<void> {
  const item = itemFor(event);
  if (!item) {
    return;
  }
  try {
    await postJSON("/api/library/verify", { path: item.path });
    message?.replaceChildren();
    await refreshVault();
  } catch (error) {
    showError(error);
  }
}

/** Lleva la imagen pulsada al asistente de instalación. */
function installItem(event: MouseEvent): void {
  const item = itemFor(event);
  if (item) {
    chooseImage(item.path);
    navigate("install");
  }
}

/** Borra la imagen pulsada tras confirmarlo. */
async function deleteItem(event: MouseEvent): Promise<void> {
  const item = itemFor(event);
  if (!item) {
    return;
  }
  const ok = await confirmAction(
    t("library.deleteTitle"),
    t("library.deleteText", { name: item.name }),
    t("library.deleteConfirm"),
  );
  if (!ok) {
    return;
  }
  try {
    await postJSON("/api/library/delete", { path: item.path });
    message?.replaceChildren();
    await refreshVault();
  } catch (error) {
    showError(error);
  }
}

/** Texto del origen de una imagen. */
function sourceText(item: LibraryItem): string {
  if (item.source === "manual") {
    return t("library.sourceManual");
  }
  if (item.source === "import") {
    return t("library.sourceImport");
  }
  return t("library.sourceOfficial");
}

/** Fila de una imagen. */
function itemRow(item: LibraryItem): HTMLElement {
  const stateText =
    item.state === "hashing"
      ? t("library.states.hashing", { p: Math.round(item.progress * 100) })
      : t("library.states." + item.state);
  const actions: HTMLElement[] = [];
  if (item.state === "unchecked" || item.state === "imported") {
    actions.push(button(t("library.verify"), verifyItem, { iconPaths: icons.shield }));
  }
  // * Arrancar un instalador solo es posible dentro de Rebootera OS.
  if (
    !isDesktop() &&
    item.state !== "mismatch" &&
    item.state !== "hashing" &&
    item.bootStatus !== "incompatible"
  ) {
    actions.push(
      button(t("library.install"), installItem, { variant: "primary", iconPaths: icons.install }),
    );
  }
  actions.push(button(t("library.delete"), deleteItem, { iconPaths: icons.x }));
  for (const action of actions) {
    action.classList.add("btn-small");
    action.dataset.path = item.path;
  }
  const next = snapshot.vault?.nextBoot === "/" + item.path;
  return el(
    "article",
    { class: "image-row" + (item.state === "mismatch" ? " image-damaged" : "") },
    [
      el("span", { class: "device-icon" }, [
        icon(item.kind === "iso" ? icons.library : icons.disk),
      ]),
      el("div", { class: "device-main" }, [
        el("strong", { title: item.name }, [item.name]),
        el("span", { class: "muted" }, [fmt.bytes(item.sizeBytes) + " · " + sourceText(item)]),
        item.state === "hashing" ? progress(item.progress, "accent") : null,
      ]),
      el("div", { class: "image-state", title: t("library.stateHelp." + item.state) }, [
        badge(stateText, STATE_TONES[item.state]),
        next ? badge(t("library.nextBoot"), "amber") : null,
        item.bootStatus === "incompatible"
          ? badge(t("downloads.bootIncompatible"), "danger")
          : null,
      ]),
      el("div", { class: "image-actions" }, actions),
    ],
  );
}

/** Refresca la página si cambian los datos. */
function update(): void {
  if (body === null || !body.isConnected || snapshot.vault === null) {
    return;
  }
  const signature = JSON.stringify([snapshot.vault, snapshot.library]);
  if (signature === lastSignature) {
    return;
  }
  lastSignature = signature;
  if (!snapshot.vault.available) {
    body.replaceChildren(emptyState(icons.lock, t("needsVault.title"), t("needsVault.text")));
    return;
  }
  const items = snapshot.library;
  const total = items.reduce(function sum(acc, item) {
    return acc + item.sizeBytes;
  }, 0);
  const stats = el("div", { class: "library-stats" }, [
    el("span", {}, [
      items.length === 1 ? t("library.countOne") : t("library.count", { n: items.length }),
    ]),
    el("span", {}, [t("library.totalSize", { size: fmt.bytes(total) })]),
    el("span", { class: "accent" }, [
      t("library.free", { size: fmt.bytes(snapshot.vault.freeBytes) }),
    ]),
  ]);
  if (items.length === 0) {
    body.replaceChildren(
      stats,
      emptyState(
        icons.library,
        t("library.empty"),
        t("library.emptyHint"),
        button(t("library.add"), addImages, { variant: "primary", iconPaths: icons.usb }),
      ),
    );
    return;
  }
  const groups: HTMLElement[] = [];
  for (const category of CATEGORIES) {
    const inCategory = items.filter(function matches(item) {
      return item.category === category;
    });
    if (inCategory.length > 0) {
      groups.push(
        panel(t("library.categories." + category), inCategory.map(itemRow), {
          className: "image-group",
        }),
      );
    }
  }
  body.replaceChildren(stats, ...groups);
}

export const libraryPage: Page = { render, update };

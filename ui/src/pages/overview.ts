// * Página "Inicio" de Rebootera OS: una pregunta ("¿qué quieres hacer?"), una barra de búsqueda y la lista de
// * acciones (ver launcher.ts). Lo técnico (procesador, discos, firmware) está en la pestaña "Este equipo".
// ! Todos los valores salen de state.ts; si un dato no existe en este equipo se dice, nunca se
// ! rellena con un ejemplo.

import type { SystemInfo } from "../api.js";
import { badge } from "../components.js";
import { browserAvailable, openBrowser } from "../bridge.js";
import { el } from "../dom.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";
import { onlineInterface } from "../insights.js";
import { navigate, type Page } from "../router.js";
import { snapshot } from "../state.js";
import { launcher, type LauncherAction } from "./launcher.js";

/** Contenedores que se refrescan con cada actualización de datos. */
let refs: { status: HTMLElement } | null = null;

/** Abre el asistente de instalación. */
function openInstall(): void {
  navigate("install");
}

/** Abre el Centro de descargas. */
function openDownloads(): void {
  navigate("downloads");
}

/** Abre las imágenes guardadas. */
function openLibrary(): void {
  navigate("library");
}

/** Abre los discos y particiones. */
function openDisks(): void {
  navigate("storage");
}

/** Abre los drivers. */
function openDrivers(): void {
  navigate("drivers");
}

/** Abre los ajustes. */
function openSettings(): void {
  navigate("settings");
}

/** Acciones de la portada de Rebootera OS (el navegador solo si la sesión lo permite). */
function actions(): LauncherAction[] {
  const list: LauncherAction[] = [
    {
      icon: icons.install,
      title: t("home.install"),
      text: t("home.installText"),
      keywords: "windows linux instalar install",
      primary: true,
      run: openInstall,
    },
    {
      icon: icons.download,
      title: t("home.download"),
      text: t("home.downloadText"),
      keywords: "iso descargar download",
      run: openDownloads,
    },
    {
      icon: icons.library,
      title: t("home.library"),
      text: t("home.libraryText"),
      keywords: "iso imagenes images",
      run: openLibrary,
    },
    {
      icon: icons.fileDisc,
      title: t("home.media"),
      text: t("home.mediaText"),
      keywords: "rufus usb carpeta folder ventoy medio",
      run: openLibrary,
    },
    {
      icon: icons.disk,
      title: t("home.disks"),
      text: t("home.disksText"),
      keywords: "particiones partitions disco disk",
      run: openDisks,
    },
    {
      icon: icons.drivers,
      title: t("home.drivers"),
      text: t("home.driversText"),
      keywords: "controladores hardware",
      run: openDrivers,
    },
  ];
  if (browserAvailable()) {
    list.push({
      icon: icons.browser,
      title: t("home.browser"),
      text: t("home.browserText"),
      keywords: "firefox internet web navegar",
      run: openBrowser,
    });
  }
  list.push({
    icon: icons.settings,
    title: t("nav.settings"),
    text: t("home.settingsText"),
    keywords: "ajustes tema idioma teclado wifi raton",
    run: openSettings,
  });
  return list;
}

/** Dibuja la estructura fija de la página. */
function render(root: HTMLElement): void {
  const status = el("div", { class: "home-status" });
  refs = { status };

  root.append(
    el("div", { class: "page home" }, [
      launcher({
        title: t("home.title"),
        subtitle: t("home.subtitle"),
        actions: actions(),
        status,
      }),
    ]),
  );
}

/** Refresca todas las partes dinámicas. */
function update(): void {
  if (refs === null || !refs.status.isConnected) {
    return;
  }
  renderStatus(snapshot.system);
}

/** Línea de estado bajo el título: red y modo de arranque, sin jerga. */
function renderStatus(system: SystemInfo | null): void {
  if (refs === null || system === null) {
    return;
  }
  const online = onlineInterface(system) !== null;
  const mode = system.firmware === "uefi" ? t("home.uefi") : t("home.bios");
  refs.status.replaceChildren(
    badge(online ? t("home.online") : t("home.offline"), online ? "ok" : "warn"),
    badge(mode + (system.secureBoot === "on" ? " · Secure Boot" : ""), "muted"),
  );
}

export const overviewPage: Page = { render, update };

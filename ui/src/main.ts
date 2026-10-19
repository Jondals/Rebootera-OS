// * Punto de entrada de Rebootera UI. Monta el marco de la aplicación (menú
// * lateral, barra superior y tarjeta del USB), registra las páginas, arranca
// * el sondeo de datos y reproduce la intro mientras llegan los primeros.
// * La misma interfaz sirve a Rebootera OS (modo live) y a Rebootera Desktop
// * (modo escritorio), con secciones distintas en cada modo.

import { getJSON } from "./api.js";
import { progress } from "./components.js";
import { openPowerDialog } from "./dialogs.js";
import { el, icon } from "./dom.js";
import * as fmt from "./format.js";
import { t, translateStatic } from "./i18n.js";
import { icons } from "./icons.js";
import { liveMedia, onlineInterface } from "./insights.js";
import { playIntro } from "./intro.js";
import { appMode, isDesktop, loadMode } from "./mode.js";
import { createPage } from "./pages/create.js";
import { desktopDriversPage } from "./pages/desktop-drivers.js";
import { homePage } from "./pages/home.js";
import { driversPage } from "./pages/drivers.js";
import { overviewPage } from "./pages/overview.js";
import { settingsPage } from "./pages/settings.js";
import { storagePage } from "./pages/storage.js";
import { downloadsPage } from "./pages/downloads.js";
import { libraryPage } from "./pages/library.js";
import { wizardPage } from "./pages/wizard.js";
import { forceLite, loadPrefs, loadVaultPrefs, prefs, savePrefs } from "./prefs.js";
import {
  activeRoute,
  desktopRoutes,
  navigate,
  register,
  renderCurrent,
  routes,
  setRoutes,
  startRouter,
  type Route,
} from "./router.js";
import { snapshot, start, subscribe } from "./state.js";

/** Icono de cada entrada del menú lateral. */
const NAV_ICONS: Record<string, string> = {
  overview: icons.overview,
  library: icons.library,
  install: icons.install,
  drivers: icons.drivers,
  downloads: icons.download,
  storage: icons.usb,
  create: icons.sparkle,
};

/** Clave de traducción del nombre de una ruta del menú. */
function navLabel(route: string): string {
  return route === "create" ? t("desktop.nav.create") : t("nav." + route);
}

/** Busca un elemento por id (todos existen en index.html). */
function byId(id: string): HTMLElement {
  return document.getElementById(id) as HTMLElement;
}

/** Navega a la ruta del enlace pulsado sin recargar. */
function onNavClick(event: MouseEvent): void {
  event.preventDefault();
  navigate((event.currentTarget as HTMLElement).dataset.route as Route);
}

/** Dibuja el menú lateral con la ruta activa marcada. */
function renderNav(): void {
  const nav = byId("nav");
  nav.replaceChildren(
    ...routes.map(function navItem(route) {
      const active = activeRoute() === route;
      const link = el(
        "a",
        {
          class: "nav-item" + (active ? " nav-active" : ""),
          href: "#/" + route,
          "data-route": route,
          "aria-current": active ? "page" : "false",
        },
        [icon(NAV_ICONS[route]), el("span", {}, [navLabel(route)])],
      );
      link.addEventListener("click", onNavClick);
      return link;
    }),
  );
}

/** Aviso de la barra superior con icono y texto. */
function chip(iconPaths: string, text: string, tone: string): HTMLElement {
  return el("span", { class: "chip chip-" + tone }, [icon(iconPaths), el("span", {}, [text])]);
}

/**
 * Refresca la barra superior y la tarjeta del USB. La barra solo avisa cuando algo falla (sin red,
 * sin USB): lo demás no hace falta para usar la aplicación y está en Ajustes.
 */
function renderShell(): void {
  let notice: HTMLElement | null = null;
  if (isDesktop()) {
    if (!appMode.usb && !appMode.library) {
      notice = chip(icons.usb, t("desktop.noUsbChip"), "warn");
    }
  } else if (snapshot.system !== null && onlineInterface(snapshot.system) === null) {
    notice = chip(icons.offline, t("topbar.offline"), "warn");
  }
  byId("status-chip").replaceChildren(...(notice ? [notice] : []));
  renderDriveCard();
}

/** Tarjeta inferior del menú lateral con el uso del USB de Rebootera. */
function renderDriveCard(): void {
  const card = byId("drive-card");
  const media = liveMedia(snapshot.disks);
  const store = snapshot.vault;
  // * Rebootera Desktop sin USB: las imágenes viven en una carpeta del equipo.
  if (isDesktop() && appMode.library) {
    card.replaceChildren(
      el("div", { class: "drive-head" }, [
        icon(icons.disk),
        el("strong", {}, [t("desktop.localChip")]),
      ]),
      el("p", { class: "drive-hint mono-small" }, [appMode.library]),
    );
    return;
  }
  if (store === null || !store.available) {
    card.replaceChildren(
      el("div", { class: "drive-head" }, [
        icon(icons.lock),
        el("strong", {}, [t("sidebar.readOnly")]),
      ]),
      el("p", { class: "drive-hint" }, [t("sidebar.readOnlyHint")]),
    );
    return;
  }
  const total = store.usedBytes + store.freeBytes;
  card.replaceChildren(
    el("div", { class: "drive-head" }, [
      icon(icons.usb),
      el("strong", {}, [isDesktop() ? t("desktop.rebooteraDrive") : t("sidebar.liveDrive")]),
      el("span", { class: "mono-small accent" }, [fmt.bytes(media?.sizeBytes ?? total)]),
    ]),
    progress(store.usedBytes / Math.max(1, total), "accent"),
    el("p", { class: "drive-hint mono-small" }, [
      t("sidebar.usedFree", { used: fmt.bytes(store.usedBytes), free: fmt.bytes(store.freeBytes) }),
    ]),
  );
}

/** Alterna entre inglés y español desde la barra superior. */
function toggleLanguage(): void {
  prefs.language = prefs.language === "en" ? "es" : "en";
  savePrefs();
  relabel();
  renderCurrent();
}

/** Vuelve a traducir el marco tras un cambio de idioma. */
function relabel(): void {
  translateStatic(document);
  byId("lang-code").textContent = prefs.language.toUpperCase();
  renderNav();
  renderShell();
}

/** Abre los ajustes. */
function openSettings(): void {
  navigate("settings");
}

/** Muestra la versión instalada. */
async function loadVersion(): Promise<void> {
  try {
    const data = await getJSON<{ version: string }>("/api/version");
    byId("version").textContent = "v" + data.version;
    const hero = document.getElementById("hero-version");
    if (hero) {
      hero.textContent = "v" + data.version;
    }
  } catch {
    // * Sin versión no se bloquea nada; la barra superior ya avisa del fallo.
  }
}

/** Aplica los ajustes guardados en el USB (si los hay) y redibuja. */
async function applyVaultPrefs(): Promise<void> {
  if (await loadVaultPrefs()) {
    relabel();
    renderCurrent();
  }
}

/** Cancela la acción por defecto de un evento. */
function preventDefault(event: Event): void {
  event.preventDefault();
}

/** Inicializa la interfaz. */
async function init(): Promise<void> {
  loadPrefs();
  translateStatic(document);
  byId("lang-code").textContent = prefs.language.toUpperCase();
  byId("power-button").addEventListener("click", openPowerDialog);
  byId("settings-button").addEventListener("click", openSettings);
  byId("lang-button").addEventListener("click", toggleLanguage);
  document.addEventListener("rebootera:language", relabel);
  // * El kiosco no ofrece menú contextual: no hay nada útil que mostrar en él.
  document.addEventListener("contextmenu", preventDefault);

  await loadMode();
  if (isDesktop()) {
    // * En el escritorio no hay agente: ni apagado ni secciones del equipo.
    setRoutes(desktopRoutes);
    byId("power-button").hidden = true;
    forceLite();
    register("create", createPage);
  }

  // * Resumen y asistente de drivers son distintos en el escritorio: allí se ve y se gestiona el USB.
  register("overview", isDesktop() ? homePage : overviewPage);
  register("library", libraryPage);
  register("install", wizardPage);
  register("drivers", isDesktop() ? desktopDriversPage : driversPage);
  register("downloads", downloadsPage);
  register("storage", storagePage);
  register("settings", settingsPage);

  subscribe(renderShell);
  const ready = Promise.all([loadVersion(), start(isDesktop()), applyVaultPrefs()]);
  startRouter(byId("view"), renderNav);
  playIntro(ready);
}

init();

// * Punto de entrada de Rebootera UI. Monta el marco de la aplicación (menú
// * lateral, barra superior y tarjeta del USB), registra las páginas, arranca
// * el sondeo de datos y reproduce la intro mientras llegan los primeros.
// * La misma interfaz sirve a Rebootera OS (modo live) y a Rebootera Desktop
// * (modo escritorio), con secciones distintas en cada modo.

import { getJSON, postJSON } from "./api.js";
import { openPowerDialog } from "./dialogs.js";
import { el, icon } from "./dom.js";
import * as fmt from "./format.js";
import { t, translateStatic } from "./i18n.js";
import { icons } from "./icons.js";
import { reportReady } from "./bridge.js";
import { onlineInterface } from "./insights.js";
import { playIntro } from "./intro.js";
import { appMode, isDesktop, loadMode } from "./mode.js";
import { homePage } from "./pages/home.js";
import { overviewPage } from "./pages/overview.js";
import { forceLite, loadPrefs, loadVaultPrefs } from "./prefs.js";
import {
  activeRoute,
  desktopRoutes,
  navigate,
  register,
  registerLazy,
  renderCurrent,
  routes,
  setRoutes,
  type Page,
  startRouter,
  type Route,
} from "./router.js";
import { refreshVault, snapshot, start, subscribe } from "./state.js";

/** Icono de cada entrada del menú lateral. */
const NAV_ICONS: Record<string, string> = {
  overview: icons.overview,
  library: icons.library,
  install: icons.install,
  drivers: icons.drivers,
  downloads: icons.download,
  storage: icons.disk,
  computer: icons.cpu,
  create: icons.usb,
  settings: icons.settings,
};

/** Clave de traducción del nombre de una ruta del menú. */
function navLabel(route: string): string {
  return route === "create" ? t("desktop.nav.create") : t("nav." + route);
}

/** Busca un elemento por id (todos existen en index.html). */
function byId(id: string): HTMLElement {
  return document.getElementById(id) as HTMLElement;
}

/** Navega a la ruta del botón pulsado sin recargar. */
function onNavClick(event: MouseEvent): void {
  navigate((event.currentTarget as HTMLElement).dataset.route as Route);
}

/** Pestaña de una sección en la cabecera. */
function navButton(route: string): HTMLElement {
  const active = activeRoute() === route;
  // * Botones y no enlaces: sin href, WebView2 no enseña la dirección abajo a la izquierda al pasar el ratón.
  const link = el(
    "button",
    {
      class: "tab" + (active ? " tab-active" : ""),
      type: "button",
      "data-route": route,
      "aria-current": active ? "page" : "false",
    },
    [icon(NAV_ICONS[route]), el("span", {}, [navLabel(route)])],
  );
  link.title = navLabel(route);
  link.addEventListener("click", onNavClick);
  return link;
}

/** Dibuja las pestañas de la cabecera con la sección activa marcada (Ajustes es la última). */
function renderNav(): void {
  byId("nav").replaceChildren(...[...routes, "settings"].map(navButton));
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
    if (!appMode.usb) {
      notice = chip(icons.usb, t("desktop.noUsbChip"), "warn");
    }
  } else if (snapshot.system !== null && onlineInterface(snapshot.system) === null) {
    notice = chip(icons.offline, t("topbar.offline"), "warn");
  }
  byId("status-chip").replaceChildren(...(notice ? [notice] : []));
  renderDriveCard();
}

/** Indicador del USB en la cabecera: el USB de Rebootera con su espacio libre o, en el sistema, "sesión live". */
function renderDriveCard(): void {
  const chipNode = byId("drive-card");
  const vault = snapshot.vault;
  let text: string;
  if (isDesktop()) {
    text =
      appMode.usb && vault && vault.available
        ? t("sidebar.free", { size: fmt.bytes(vault.freeBytes) })
        : t("sidebar.noUsb");
  } else if (vault && vault.available) {
    text = t("sidebar.free", { size: fmt.bytes(vault.freeBytes) });
  } else {
    text = t("sidebar.live");
  }
  chipNode.replaceChildren(icon(icons.usb), el("span", {}, [text]));
  chipNode.title = text;
}

/** Vuelve a traducir el marco tras un cambio de idioma. */
function relabel(): void {
  translateStatic(document);
  renderNav();
  renderShell();
}

/** Unidad de /api/desktop/drives (solo se mira si es un USB de Rebootera). */
interface DriveInfo {
  root: string;
  rebootera: boolean;
}

/**
 * Rebootera Desktop sin USB elegido: en cuanto se conecta un USB de Rebootera (y solo hay uno) se
 * elige solo, sin pasos. Las descargas solo van a un USB, así que no hay que hacer esperar al usuario.
 */
async function autoSelectUsb(): Promise<void> {
  if (appMode.usb !== "") {
    return;
  }
  try {
    const found = (await getJSON<DriveInfo[]>("/api/desktop/drives")).filter(
      function isRebootera(drive) {
        return drive.rebootera;
      },
    );
    if (found.length === 1) {
      await postJSON("/api/desktop/select", { root: found[0].root });
      appMode.usb = found[0].root;
      await refreshVault();
      renderShell();
      void renderCurrent();
    }
  } catch {
    // * Sin respuesta se vuelve a intentar en el siguiente ciclo.
  }
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
    void renderCurrent();
  }
}

/** Cancela la acción por defecto de un evento. */
function preventDefault(event: Event): void {
  event.preventDefault();
}

/** Carga de cada página al abrirla por primera vez (cada una es un archivo aparte). */
async function loadCreate(): Promise<Page> {
  return (await import("./pages/create.js")).createPage;
}
async function loadLibrary(): Promise<Page> {
  return (await import("./pages/library.js")).libraryPage;
}
async function loadWizard(): Promise<Page> {
  return (await import("./pages/wizard.js")).wizardPage;
}
async function loadDesktopDrivers(): Promise<Page> {
  return (await import("./pages/desktop-drivers.js")).desktopDriversPage;
}
async function loadDrivers(): Promise<Page> {
  return (await import("./pages/drivers.js")).driversPage;
}
async function loadDownloads(): Promise<Page> {
  return (await import("./pages/downloads.js")).downloadsPage;
}
async function loadStorage(): Promise<Page> {
  return (await import("./pages/storage.js")).storagePage;
}
async function loadComputer(): Promise<Page> {
  return (await import("./pages/computer.js")).computerPage;
}
async function loadSettings(): Promise<Page> {
  return (await import("./pages/settings.js")).settingsPage;
}

/** Inicializa la interfaz. */
async function init(): Promise<void> {
  loadPrefs();
  translateStatic(document);
  byId("power-button").addEventListener("click", openPowerDialog);
  document.addEventListener("rebootera:language", relabel);
  byId("power-icon").innerHTML = icons.power;
  // * El kiosco no ofrece menú contextual: no hay nada útil que mostrar en él.
  document.addEventListener("contextmenu", preventDefault);

  await loadMode();
  if (isDesktop()) {
    // * En el escritorio no hay agente: ni apagado ni secciones del equipo.
    setRoutes(desktopRoutes);
    byId("power-button").hidden = true;
    forceLite();
    registerLazy("create", loadCreate);
    window.setInterval(autoSelectUsb, 4000);
  }

  // * Resumen y asistente de drivers son distintos en el escritorio: allí se ve y se gestiona el USB.
  register("overview", isDesktop() ? homePage : overviewPage);
  // * Solo la portada se carga de entrada; las demás páginas se leen del USB la primera vez que se abren.
  registerLazy("library", loadLibrary);
  registerLazy("install", loadWizard);
  registerLazy("drivers", isDesktop() ? loadDesktopDrivers : loadDrivers);
  registerLazy("downloads", loadDownloads);
  registerLazy("storage", loadStorage);
  registerLazy("computer", loadComputer);
  registerLazy("settings", loadSettings);

  subscribe(renderShell);
  const ready = Promise.all([loadVersion(), start(isDesktop()), applyVaultPrefs()]);
  startRouter(byId("view"), renderNav);
  playIntro(ready);
  // * start-kiosk espera este aviso: la interfaz ya dibujó (dos fotogramas) y la sesión es buena.
  window.requestAnimationFrame(function firstFrame() {
    window.requestAnimationFrame(reportReady);
  });
}

init();

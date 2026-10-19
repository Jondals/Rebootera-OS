// * Punto de entrada de Rebootera UI. Monta el marco de la aplicación (menú
// * lateral, barra superior y tarjeta del USB), registra las páginas, arranca
// * el sondeo de datos y reproduce la intro mientras llegan los primeros.
// * La misma interfaz sirve a Rebootera OS (modo live) y a Rebootera Desktop
// * (modo escritorio), con secciones distintas en cada modo.

import { getJSON, postJSON } from "./api.js";
import { miniTheme, progress } from "./components.js";
import { openPowerDialog } from "./dialogs.js";
import { el, icon } from "./dom.js";
import * as fmt from "./format.js";
import { t, translateStatic } from "./i18n.js";
import { icons } from "./icons.js";
import { reportReady } from "./bridge.js";
import { onlineInterface } from "./insights.js";
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
import {
  forceLite,
  loadPrefs,
  loadVaultPrefs,
  prefs,
  savePrefs,
  THEMES,
  type Theme,
} from "./prefs.js";
import {
  activeRoute,
  desktopRoutes,
  navigate,
  register,
  refreshCurrent,
  renderCurrent,
  routes,
  setRoutes,
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

/** Botón de una sección del menú lateral. */
function navButton(route: string): HTMLElement {
  const active = activeRoute() === route;
  // * Botones y no enlaces: sin href, WebView2 no enseña la dirección abajo a la izquierda al pasar el ratón.
  const link = el(
    "button",
    {
      class: "nav-item" + (active ? " nav-active" : ""),
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

/** Dibuja el menú lateral con la ruta activa marcada: las secciones arriba y Ajustes al pie. */
function renderNav(): void {
  byId("nav").replaceChildren(...routes.map(navButton));
  byId("nav-foot").replaceChildren(navButton("settings"));
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

/** Tarjeta de estado al pie del menú lateral: el USB de Rebootera (con su espacio) o, en el sistema, el medio de arranque. */
function renderDriveCard(): void {
  const card = byId("drive-card");
  const vault = snapshot.vault;
  let title = "";
  let detail = "";
  let ratio = -1;
  if (isDesktop()) {
    title = appMode.usb ? t("sidebar.usbReady") : t("sidebar.noUsb");
    if (appMode.usb && vault && vault.available) {
      const total = vault.usedBytes + vault.freeBytes;
      detail = t("sidebar.free", { size: fmt.bytes(vault.freeBytes) });
      ratio = total > 0 ? vault.usedBytes / total : 0;
    } else if (!appMode.usb) {
      detail = t("sidebar.noUsbHint");
    }
  } else if (vault && vault.available) {
    const total = vault.usedBytes + vault.freeBytes;
    title = t("sidebar.usbReady");
    detail = t("sidebar.free", { size: fmt.bytes(vault.freeBytes) });
    ratio = total > 0 ? vault.usedBytes / total : 0;
  } else {
    title = t("sidebar.live");
    detail = t("sidebar.readOnly");
  }
  const parts: Node[] = [
    icon(icons.usb),
    el("div", { class: "drive-text" }, [el("strong", {}, [title]), el("span", {}, [detail])]),
  ];
  const children: Node[] = [el("div", { class: "drive-row" }, parts)];
  if (ratio >= 0) {
    children.push(progress(ratio, ratio > 0.9 ? "warn" : "accent"));
  }
  card.replaceChildren(...children);
  card.title = title + (detail ? " · " + detail : "");
}

/** Alterna entre inglés y español desde la barra superior. */
function toggleLanguage(): void {
  prefs.language = prefs.language === "en" ? "es" : "en";
  savePrefs();
  relabel();
  refreshCurrent();
}

/** Pliega o despliega el menú lateral. */
function toggleSidebar(): void {
  prefs.sidebar = prefs.sidebar === "full" ? "compact" : "full";
  savePrefs();
  drawSideToggle();
}

/** Dibuja el botón que pliega el menú lateral (su icono y su texto dependen del estado). */
function drawSideToggle(): void {
  const compact = prefs.sidebar === "compact";
  byId("side-toggle-icon").innerHTML = compact ? icons.panelOpen : icons.panelClose;
  byId("side-toggle-text").textContent = compact ? t("nav.expand") : t("nav.collapse");
  byId("side-toggle").title = compact ? t("nav.expand") : t("nav.collapse");
}

/** Vuelve a traducir el marco tras un cambio de idioma. */
function relabel(): void {
  translateStatic(document);
  byId("lang-code").textContent = prefs.language.toUpperCase();
  renderNav();
  renderShell();
  drawThemeSwitch();
  drawSideToggle();
}

/** Indica si el selector de tema está desplegado. */
let themeOpen = false;

/**
 * Dibuja el selector de tema de la barra superior: un botón con la miniatura del tema actual que despliega
 * las tres opciones, cada una con su vista previa y su nombre.
 */
function drawThemeSwitch(): void {
  const current = el(
    "button",
    {
      class: "theme-current",
      type: "button",
      title: t("theme.title"),
      "aria-haspopup": "true",
      "aria-expanded": String(themeOpen),
    },
    [
      icon(icons.palette),
      el("span", { class: "theme-name" }, [t("theme." + prefs.theme)]),
      icon(icons.chevronRight, "icon theme-caret"),
    ],
  );
  current.addEventListener("click", toggleThemeMenu);
  const parts: HTMLElement[] = [current];
  if (themeOpen) {
    parts.push(
      el(
        "div",
        { class: "theme-popover", role: "menu" },
        THEMES.map(function themeOption(name) {
          const option = el(
            "button",
            {
              class: "theme-option",
              type: "button",
              role: "menuitemradio",
              "aria-checked": String(prefs.theme === name),
              "data-theme": name,
            },
            [
              miniTheme(name),
              el("span", { class: "theme-option-text" }, [
                el("strong", {}, [t("theme." + name)]),
                el("span", {}, [t("theme." + name + "Hint")]),
              ]),
            ],
          );
          option.addEventListener("click", pickTheme);
          return option;
        }),
      ),
    );
  }
  byId("theme-switch").replaceChildren(...parts);
}

/** Despliega o recoge el selector de tema. */
function toggleThemeMenu(): void {
  themeOpen = !themeOpen;
  drawThemeSwitch();
}

/** Recoge el selector al pulsar fuera de él o Escape. */
function closeThemeMenu(event: Event): void {
  if (!themeOpen) {
    return;
  }
  if (event instanceof KeyboardEvent && event.key !== "Escape") {
    return;
  }
  if (event instanceof MouseEvent && byId("theme-switch").contains(event.target as Node)) {
    return;
  }
  themeOpen = false;
  drawThemeSwitch();
}

/** Aplica el tema de la opción pulsada y lo recuerda. */
function pickTheme(event: MouseEvent): void {
  prefs.theme = (event.currentTarget as HTMLElement).dataset.theme as Theme;
  themeOpen = false;
  savePrefs();
  drawThemeSwitch();
  document.dispatchEvent(new CustomEvent("rebootera:theme"));
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
      renderCurrent();
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
  byId("lang-button").addEventListener("click", toggleLanguage);
  byId("side-toggle").addEventListener("click", toggleSidebar);
  drawSideToggle();
  document.addEventListener("rebootera:language", relabel);
  document.addEventListener("rebootera:theme", drawThemeSwitch);
  document.addEventListener("click", closeThemeMenu);
  document.addEventListener("keydown", closeThemeMenu);
  byId("power-icon").innerHTML = icons.power;
  drawThemeSwitch();
  // * El kiosco no ofrece menú contextual: no hay nada útil que mostrar en él.
  document.addEventListener("contextmenu", preventDefault);

  await loadMode();
  if (isDesktop()) {
    // * En el escritorio no hay agente: ni apagado ni secciones del equipo.
    setRoutes(desktopRoutes);
    byId("power-button").hidden = true;
    forceLite();
    register("create", createPage);
    window.setInterval(autoSelectUsb, 4000);
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
  // * start-kiosk espera este aviso: la interfaz ya dibujó (dos fotogramas) y la sesión es buena.
  window.requestAnimationFrame(function firstFrame() {
    window.requestAnimationFrame(reportReady);
  });
}

init();

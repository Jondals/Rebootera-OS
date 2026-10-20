// * Enrutador por hash (#/overview, #/drivers...). Cada sección es una Page
// * que se dibuja en <main> y que puede actualizarse cuando llegan datos
// * nuevos de state.ts sin volver a dibujarse entera (así no se pierde el
// * foco ni la posición de desplazamiento).

import { subscribe } from "./state.js";

/** Una página de la interfaz. */
export interface Page {
  /** Dibuja la página dentro de root. */
  render(root: HTMLElement): void;
  /** Refresca las partes dinámicas con los datos actuales. */
  update?(): void;
}

/** Rutas del menú lateral en Rebootera OS. */
const liveRoutes = [
  "overview",
  "install",
  "library",
  "drivers",
  "downloads",
  "storage",
  "computer",
];

/** Rutas del menú lateral en Rebootera Desktop. */
export const desktopRoutes = ["overview", "create", "drivers", "library", "downloads"];

/** Rutas del menú lateral activas (cambian con el modo). */
export let routes: string[] = liveRoutes;

/** Nombre de una ruta. */
export type Route = string;

/** Cambia las rutas del menú lateral (al saber el modo). */
export function setRoutes(list: string[]): void {
  routes = list;
}

const pages = new Map<Route, Page>();
/** Cómo cargar las páginas que se cargan al abrirlas (cada una es un archivo aparte: se lee del USB solo si se usa). */
const loaders = new Map<Route, () => Promise<Page>>();
let currentRoute: Route = "overview";
let currentPage: Page | null = null;
let mainElement: HTMLElement | null = null;
let onChange: ((route: Route) => void) | null = null;

/** Registra la página de una ruta. */
export function register(route: Route, page: Page): void {
  pages.set(route, page);
}

/** Registra una ruta cuya página se carga la primera vez que se abre. */
export function registerLazy(route: Route, load: () => Promise<Page>): void {
  loaders.set(route, load);
}

/** Devuelve la página de una ruta, cargándola si hace falta. */
async function pageFor(route: Route): Promise<Page | null> {
  const known = pages.get(route);
  if (known) {
    return known;
  }
  const load = loaders.get(route);
  if (!load) {
    return null;
  }
  const page = await load();
  pages.set(route, page);
  return page;
}

/** Lee la ruta actual del hash; por defecto, la primera del menú. */
function routeFromHash(): Route {
  const name = window.location.hash.replace(/^#\/?/, "");
  if (name === "settings" || routes.includes(name)) {
    return name;
  }
  return routes[0];
}

/** Dibuja la página de la ruta actual (si hay que cargarla, la pestaña ya se marca mientras tanto). */
export async function renderCurrent(): Promise<void> {
  if (mainElement === null) {
    return;
  }
  const route = routeFromHash();
  currentRoute = route;
  onChange?.(route);
  const page = await pageFor(route);
  // * Si mientras se cargaba se pidió otra ruta, esa otra llamada es la que dibuja.
  if (route !== currentRoute || mainElement === null) {
    return;
  }
  currentPage = page;
  mainElement.replaceChildren();
  mainElement.scrollTop = 0;
  page?.render(mainElement);
  page?.update?.();
  mainElement.focus({ preventScroll: true });
}

/**
 * Vuelve a dibujar la página actual sin que se note: conserva el desplazamiento y no repite la animación de
 * entrada. Se usa al cambiar de idioma o de tema, donde la página es la misma y solo cambian los textos.
 */
export async function refreshCurrent(): Promise<void> {
  if (mainElement === null) {
    return;
  }
  const top = mainElement.scrollTop;
  mainElement.classList.add("keep");
  await renderCurrent();
  mainElement.scrollTop = top;
  window.requestAnimationFrame(function release() {
    mainElement?.classList.remove("keep");
  });
}

/** Navega a una ruta. */
export function navigate(route: Route): void {
  if (window.location.hash === "#/" + route) {
    void renderCurrent();
  } else {
    window.location.hash = "#/" + route;
  }
}

/** Devuelve la ruta activa. */
export function activeRoute(): Route {
  return currentRoute;
}

/** Actualiza la página activa con los datos nuevos. */
function updateCurrent(): void {
  currentPage?.update?.();
}

/** Arranca el enrutador sobre el elemento principal. */
export function startRouter(main: HTMLElement, changed: (route: Route) => void): void {
  mainElement = main;
  onChange = changed;
  window.addEventListener("hashchange", function onHash() {
    void renderCurrent();
  });
  subscribe(updateCurrent);
  void renderCurrent();
}

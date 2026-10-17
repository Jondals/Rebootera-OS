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
export const liveRoutes = ["overview", "install", "library", "drivers", "downloads", "storage"];

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
let currentRoute: Route = "overview";
let currentPage: Page | null = null;
let mainElement: HTMLElement | null = null;
let onChange: ((route: Route) => void) | null = null;

/** Registra la página de una ruta. */
export function register(route: Route, page: Page): void {
  pages.set(route, page);
}

/** Lee la ruta actual del hash; por defecto, la primera del menú. */
function routeFromHash(): Route {
  const name = window.location.hash.replace(/^#\/?/, "");
  if (name === "settings" || routes.includes(name)) {
    return name;
  }
  return routes[0];
}

/** Dibuja la página de la ruta actual. */
export function renderCurrent(): void {
  if (mainElement === null) {
    return;
  }
  currentRoute = routeFromHash();
  currentPage = pages.get(currentRoute) ?? null;
  mainElement.replaceChildren();
  mainElement.scrollTop = 0;
  currentPage?.render(mainElement);
  currentPage?.update?.();
  mainElement.focus({ preventScroll: true });
  onChange?.(currentRoute);
}

/** Navega a una ruta. */
export function navigate(route: Route): void {
  if (window.location.hash === "#/" + route) {
    renderCurrent();
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
  window.addEventListener("hashchange", renderCurrent);
  subscribe(updateCurrent);
  renderCurrent();
}

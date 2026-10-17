// * Modo de funcionamiento: "live" dentro de Rebootera OS o "desktop" en
// * Rebootera Desktop (Windows o Linux). La misma interfaz muestra secciones
// * distintas en cada uno.

import { getJSON } from "./api.js";

/** Información del modo que devuelve /api/mode. */
interface ModeInfo {
  mode: "live" | "desktop";
  platform: string;
  usb: string;
  /** Carpeta de la biblioteca local del equipo (escritorio sin USB), o "". */
  library: string;
}

export const appMode: ModeInfo = { mode: "live", platform: "linux", usb: "", library: "" };

/** Lee el modo de Core (por defecto, sistema live). */
export async function loadMode(): Promise<void> {
  try {
    Object.assign(appMode, await getJSON<ModeInfo>("/api/mode"));
  } catch {
    // * Sin respuesta se asume el sistema live.
  }
  document.documentElement.dataset.mode = appMode.mode;
}

/** Indica si la interfaz corre en Rebootera Desktop. */
export function isDesktop(): boolean {
  return appMode.mode === "desktop";
}

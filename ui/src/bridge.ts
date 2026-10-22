// * Puente con la sesión del kiosco de Rebootera OS: rebootera-shell (la ventana que muestra la interfaz) escucha
// * un único canal de mensajes con acciones cerradas. Hoy solo conoce "browser" (abrir el navegador del sistema).
// * Fuera del kiosco (Rebootera Desktop, un navegador normal) el canal no existe y las funciones lo dicen.

/** Forma mínima del canal que WebKitGTK expone a la página. */
interface ShellChannel {
  postMessage(message: string): void;
}

/** Devuelve el canal del kiosco, o null si esta página no corre dentro de él. */
function channel(): ShellChannel | null {
  const bridge = (
    window as unknown as { webkit?: { messageHandlers?: { rebootera?: ShellChannel } } }
  ).webkit;
  return bridge?.messageHandlers?.rebootera ?? null;
}

/** Indica si la sesión puede abrir el navegador del sistema (solo dentro del kiosco de Rebootera OS). */
export function browserAvailable(): boolean {
  return channel() !== null;
}

/** Avisa a la sesión de que la interfaz ya dibujó su primer fotograma (start-kiosk espera este aviso). */
export function reportReady(): void {
  channel()?.postMessage(JSON.stringify({ action: "ready" }));
}

/**
 * Pide a la sesión que abra el navegador del sistema (navegador ligero de la sesión, en modo oscuro, sin telemetría y
 * sin cookies ni historial), si se indica en una página https concreta.
 */
export function openBrowser(url?: string): void {
  channel()?.postMessage(
    JSON.stringify({ action: "browser", url: typeof url === "string" ? url : undefined }),
  );
}

/**
 * Avisa a la ventana de Rebootera Desktop del tema puesto, para que cambie a juego el icono y la barra de título.
 * Fuera del escritorio la función no existe y no pasa nada.
 */
export function reportTheme(theme: string): void {
  const host = window as unknown as { rebooteraTheme?: (name: string) => unknown };
  try {
    host.rebooteraTheme?.(theme);
  } catch {
    // * Un icono sin cambiar no merece un error.
  }
}

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

/** Pide a la sesión que abra el navegador del sistema (Firefox, solo para navegar, con uBlock Origin y Dark Reader). */
export function openBrowser(): void {
  channel()?.postMessage(JSON.stringify({ action: "browser" }));
}

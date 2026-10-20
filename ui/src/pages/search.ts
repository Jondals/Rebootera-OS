// * Búsqueda con la que se abrirá el Centro de descargas cuando el lanzador de la portada la deje escrita. Vive aparte para que
// * la portada no cargue el Centro de descargas entero solo por esto (cada página se carga cuando se abre).

let pending: string | null = null;

/** Deja escrita la búsqueda con la que se abrirá el Centro de descargas. */
export function presetSearch(text: string): void {
  pending = text.trim().toLowerCase();
}

/** Devuelve (y olvida) la búsqueda pendiente, o null si no hay ninguna. */
export function takePresetSearch(): string | null {
  const text = pending;
  pending = null;
  return text;
}

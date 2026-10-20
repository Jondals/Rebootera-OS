// * Panel del puntero de la página de Ajustes: sensibilidad del ratón y del panel táctil, toque para hacer clic y
// * desplazamiento natural. Solo existe en Rebootera OS: Core se lo pide al agente, que recarga la configuración
// * del compositor sin reiniciar la pantalla; el cambio se envía al soltar el control.

import { getJSON, postJSON } from "../api.js";
import { errorBox, panel } from "../components.js";
import { el } from "../dom.js";
import { t } from "../i18n.js";

/** Ajustes del puntero, tal como los devuelve /api/input. */
interface PointerSettings {
  speed: number;
  tap: boolean;
  natural: boolean;
}

/** Respuesta de GET /api/input. */
interface PointerState {
  input: PointerSettings;
  persistent: boolean;
}

let body: HTMLElement | null = null;
let settings: PointerSettings | null = null;
let failure: unknown = null;
let applying = false;

/** Lee los ajustes en uso. */
async function load(): Promise<void> {
  try {
    settings = (await getJSON<PointerState>("/api/input")).input;
    failure = null;
  } catch (error) {
    failure = error;
  }
  draw();
}

/** Envía los ajustes nuevos y los aplica al momento. */
async function apply(next: PointerSettings): Promise<void> {
  if (applying) {
    return;
  }
  applying = true;
  failure = null;
  draw();
  try {
    await postJSON("/api/input", next);
    settings = next;
  } catch (error) {
    failure = error;
  }
  applying = false;
  draw();
}

/** Suelta la barra de sensibilidad: se aplica el valor elegido. */
function onSpeed(event: Event): void {
  if (settings === null) {
    return;
  }
  void apply({ ...settings, speed: Number((event.currentTarget as HTMLInputElement).value) });
}

/** Cambia el toque para hacer clic. */
function onTap(event: Event): void {
  if (settings !== null) {
    void apply({ ...settings, tap: (event.currentTarget as HTMLInputElement).checked });
  }
}

/** Cambia el desplazamiento natural. */
function onNatural(event: Event): void {
  if (settings !== null) {
    void apply({ ...settings, natural: (event.currentTarget as HTMLInputElement).checked });
  }
}

/** Fila con un interruptor. */
function switchRow(label: string, checked: boolean, onChange: (event: Event) => void): HTMLElement {
  const input = el("input", {
    type: "checkbox",
    class: "switch-input",
    checked,
    disabled: applying,
  });
  input.addEventListener("change", onChange);
  return el("label", { class: "setting-row" }, [el("span", {}, [label]), input]);
}

/** Redibuja el contenido del panel. */
function draw(): void {
  if (body === null || !body.isConnected) {
    return;
  }
  const children: HTMLElement[] = [];
  if (failure !== null) {
    children.push(errorBox(failure));
  }
  if (settings !== null) {
    const slider = el("input", {
      class: "range-input",
      type: "range",
      min: "0",
      max: "100",
      step: "5",
      value: String(settings.speed),
      disabled: applying,
      "aria-label": t("pointer.speed"),
    });
    slider.addEventListener("change", onSpeed);
    children.push(
      el("div", { class: "range-row" }, [
        el("span", {}, [t("pointer.speed")]),
        el("div", { class: "range-control" }, [
          el("small", {}, [t("pointer.slow")]),
          slider,
          el("small", {}, [t("pointer.fast")]),
        ]),
      ]),
      switchRow(t("pointer.tap"), settings.tap, onTap),
      switchRow(t("pointer.natural"), settings.natural, onNatural),
      el("p", { class: "muted" }, [applying ? t("pointer.applying") : t("pointer.hint")]),
    );
  }
  body.replaceChildren(...children);
}

/** Panel del puntero de Ajustes. */
export function pointerPanel(): HTMLElement {
  body = el("div", { class: "pointer-body" });
  void load();
  return panel(t("pointer.title"), [body]);
}

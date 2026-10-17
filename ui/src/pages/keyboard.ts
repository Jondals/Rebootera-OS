// * Panel de teclado de la página de Ajustes: elige la distribución del
// * teclado de la interfaz. Solo existe en Rebootera OS: Core se lo pide al
// * agente, que reinicia el kiosco (la pantalla parpadea un par de segundos).

import { getJSON, postJSON } from "../api.js";
import { errorBox, panel } from "../components.js";
import { el } from "../dom.js";
import { t } from "../i18n.js";

/** Respuesta de GET /api/keyboard. */
interface KeyboardState {
  layout: string;
  layouts: string[];
  persistent: boolean;
}

/** Nombre con el que se muestra cada distribución (en su propio idioma). */
const LAYOUT_NAMES: Record<string, string> = {
  es: "Español",
  us: "English",
};

let body: HTMLElement | null = null;
let state: KeyboardState | null = null;
let failure: unknown = null;
let applying = false;

/** Lee la distribución actual. */
async function load(): Promise<void> {
  try {
    state = await getJSON<KeyboardState>("/api/keyboard");
    failure = null;
  } catch (error) {
    failure = error;
  }
  draw();
}

/** Pide el cambio de distribución. */
async function choose(event: MouseEvent): Promise<void> {
  const layout = (event.currentTarget as HTMLElement).dataset.value ?? "";
  if (applying || state === null || layout === state.layout) {
    return;
  }
  applying = true;
  failure = null;
  draw();
  try {
    await postJSON("/api/keyboard", { layout });
    state.layout = layout;
  } catch (error) {
    failure = error;
  }
  applying = false;
  draw();
}

/** Redibuja el contenido del panel. */
function draw(): void {
  if (body === null || !body.isConnected) {
    return;
  }
  const children: (HTMLElement | null)[] = [];
  if (failure !== null) {
    children.push(errorBox(failure));
  }
  if (state !== null) {
    const group = el("div", {
      class: "segmented",
      role: "radiogroup",
      "aria-label": t("keyboard.title"),
    });
    for (const layout of state.layouts) {
      const node = el(
        "button",
        {
          class: "segment-button",
          type: "button",
          role: "radio",
          "aria-checked": String(layout === state.layout),
          "data-value": layout,
          disabled: applying,
        },
        [LAYOUT_NAMES[layout] ?? layout],
      );
      node.addEventListener("click", choose);
      group.append(node);
    }
    children.push(group);
    children.push(
      el("p", { class: "muted" }, [applying ? t("keyboard.applying") : t("keyboard.hint")]),
    );
  }
  body.replaceChildren(...children.filter(isElement));
}

/** Filtro de tipo para quitar los huecos de la lista. */
function isElement(node: HTMLElement | null): node is HTMLElement {
  return node !== null;
}

/** Panel de teclado de Ajustes. */
export function keyboardPanel(): HTMLElement {
  body = el("div", { class: "keyboard-body" });
  void load();
  return panel(t("keyboard.title"), [body]);
}

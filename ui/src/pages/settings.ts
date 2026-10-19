// * Página "Ajustes": idioma, color de acento, intro al arrancar, movimiento
// * reducido y datos del sistema (versión, kernel, tiempo encendido).

import { more, pageHeader, panel } from "../components.js";
import { el } from "../dom.js";
import * as fmt from "../format.js";
import { t, type Language } from "../i18n.js";
import { prefs, savePrefs } from "../prefs.js";
import { renderCurrent, type Page } from "../router.js";
import { isDesktop } from "../mode.js";
import { snapshot } from "../state.js";
import { keyboardPanel } from "./keyboard.js";
import { wifiPanel, wifiUpdate } from "./wifi.js";

let about: HTMLElement | null = null;

/** Grupo de botones de opción excluyentes. */
function choice(
  name: string,
  options: [string, string][],
  selected: string,
  onSelect: (value: string) => void,
): HTMLElement {
  const group = el("div", { class: "segmented", role: "radiogroup", "aria-label": name });
  for (const [value, label] of options) {
    const node = el(
      "button",
      {
        class: "segment-button",
        type: "button",
        role: "radio",
        "aria-checked": String(value === selected),
        "data-value": value,
      },
      [label],
    );
    node.addEventListener("click", handleClick);
    group.append(node);
  }

  /** Aplica la opción pulsada. */
  function handleClick(event: MouseEvent): void {
    onSelect((event.currentTarget as HTMLElement).dataset.value ?? "");
  }

  return group;
}

/** Interruptor de activado/desactivado. */
function toggle(
  label: string,
  checked: boolean,
  onChange: (checked: boolean) => void,
): HTMLElement {
  const input = el("input", { type: "checkbox", class: "switch-input", checked });
  input.addEventListener("change", handleChange);

  /** Aplica el nuevo valor. */
  function handleChange(): void {
    onChange(input.checked);
  }

  return el("label", { class: "setting-row" }, [el("span", {}, [label]), input]);
}

/** Cambia el idioma y redibuja toda la interfaz. */
function setLanguage(value: string): void {
  prefs.language = value as Language;
  savePrefs();
  document.dispatchEvent(new CustomEvent("rebootera:language"));
  renderCurrent();
}

/** Cambia el color de acento. */
function setAccent(value: string): void {
  prefs.accent = value as "aurora" | "nebula";
  savePrefs();
  renderCurrent();
}

/** Activa o desactiva la intro. */
function setIntro(checked: boolean): void {
  prefs.intro = checked;
  savePrefs();
}

/** Activa o desactiva el movimiento reducido. */
function setMotion(checked: boolean): void {
  prefs.reduceMotion = checked;
  savePrefs();
}

/** Dibuja la página. */
function render(root: HTMLElement): void {
  about = el("dl", { class: "about-list" });
  root.append(
    el("div", { class: "page" }, [
      pageHeader(t("settings.title"), t("settings.subtitle")),
      el("div", { class: "settings-grid" }, [
        panel(t("settings.language"), [
          choice(
            t("settings.language"),
            [
              ["en", "English"],
              ["es", "Español"],
            ],
            prefs.language,
            setLanguage,
          ),
        ]),
        isDesktop() ? null : keyboardPanel(),
        isDesktop() ? null : wifiPanel(),
        // * Lo que casi nadie toca queda plegado.
        more(t("settings.advanced"), [
          panel(t("settings.accent"), [
            choice(
              t("settings.accent"),
              [
                ["aurora", t("settings.accentAurora")],
                ["nebula", t("settings.accentNebula")],
              ],
              prefs.accent,
              setAccent,
            ),
          ]),
          panel(t("settings.title"), [
            toggle(t("settings.intro"), prefs.intro, setIntro),
            toggle(t("settings.motion"), prefs.reduceMotion, setMotion),
          ]),
          panel(t("settings.about"), [about]),
        ]),
      ]),
    ]),
  );
}

/** Refresca los datos del sistema. */
function update(): void {
  if (about === null || !about.isConnected) {
    return;
  }
  wifiUpdate();
  const system = snapshot.system;
  const rows: [string, string][] = [
    [t("settings.version"), document.getElementById("version")?.textContent ?? ""],
    [t("settings.kernel"), system?.kernel ?? "—"],
    [t("settings.uptime"), system ? fmt.duration(system.uptimeSeconds) : "—"],
  ];
  about.replaceChildren(
    ...rows.flatMap(function row([name, value]) {
      return [el("dt", {}, [name]), el("dd", {}, [value])];
    }),
  );
}

export const settingsPage: Page = { render, update };

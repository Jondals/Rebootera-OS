// * Página "Ajustes": idioma, color de acento, intro al arrancar, movimiento
// * reducido y datos del sistema (versión, kernel, tiempo encendido).

import { miniTheme, pageHeader, panel } from "../components.js";
import { el } from "../dom.js";
import * as fmt from "../format.js";
import { t, type Language } from "../i18n.js";
import { applyPrefs, prefs, savePrefs, THEMES, type Theme } from "../prefs.js";
import { refreshCurrent, type Page } from "../router.js";
import { isDesktop } from "../mode.js";
import { snapshot } from "../state.js";
import { keyboardPanel } from "./keyboard.js";
import { pointerPanel } from "./pointer.js";
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

  /** Marca la opción pulsada (sin esperar a que se redibuje la página) y la aplica. */
  function handleClick(event: MouseEvent): void {
    const chosen = event.currentTarget as HTMLElement;
    for (const other of group.querySelectorAll<HTMLElement>(".segment-button")) {
      other.setAttribute("aria-checked", String(other === chosen));
    }
    onSelect(chosen.dataset.value ?? "");
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
  refreshCurrent();
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

/** Cambia el fondo animado (automático, siempre o nunca) y lo aplica al momento. */
function setBackground(value: string): void {
  prefs.background = value as typeof prefs.background;
  savePrefs();
  applyPrefs();
}

/** Cambia el desenfoque del fondo y lo aplica al momento. */
function setBlur(value: string): void {
  prefs.blur = value as typeof prefs.blur;
  savePrefs();
}

/** Cambia el tamaño de la interfaz y lo aplica al momento. */
function setScale(value: string): void {
  prefs.scale = Number(value);
  savePrefs();
  applyPrefs();
}

/** Panel de aspecto: fondo animado y tamaño de la interfaz. */
function appearancePanel(): HTMLElement {
  return panel(t("settings.appearance"), [
    el("div", { class: "setting-block" }, [
      el("span", {}, [t("settings.background")]),
      choice(
        t("settings.background"),
        [
          ["auto", t("settings.backgroundAuto")],
          ["on", t("settings.backgroundOn")],
          ["off", t("settings.backgroundOff")],
        ],
        prefs.background,
        setBackground,
      ),
    ]),
    el("div", { class: "setting-block" }, [
      el("span", {}, [t("settings.blur")]),
      choice(
        t("settings.blur"),
        [
          ["off", t("settings.blurOff")],
          ["soft", t("settings.blurSoft")],
          ["medium", t("settings.blurMedium")],
          ["strong", t("settings.blurStrong")],
        ],
        prefs.blur,
        setBlur,
      ),
    ]),
    el("div", { class: "setting-block" }, [
      el("span", {}, [t("settings.size")]),
      choice(
        t("settings.size"),
        [
          ["90", "90%"],
          ["100", "100%"],
          ["115", "115%"],
          ["130", "130%"],
        ],
        String(prefs.scale),
        setScale,
      ),
    ]),
  ]);
}

/** Cambia el tema, lo recuerda y avisa al marco para que actualice sus puntos. */
function setTheme(event: MouseEvent): void {
  prefs.theme = (event.currentTarget as HTMLElement).dataset.theme as Theme;
  savePrefs();
  document.dispatchEvent(new CustomEvent("rebootera:theme"));
  refreshCurrent();
}

/** Panel con los tres temas, cada uno con su muestra. */
function themePanel(): HTMLElement {
  return panel(t("theme.title"), [
    el(
      "div",
      { class: "theme-cards" },
      THEMES.map(function themeCard(name) {
        const card = el(
          "button",
          {
            class: "theme-card",
            type: "button",
            "aria-pressed": String(prefs.theme === name),
            "data-theme": name,
          },
          [
            miniTheme(name),
            el("strong", {}, [t("theme." + name)]),
            el("span", { class: "muted" }, [t("theme." + name + "Hint")]),
          ],
        );
        card.addEventListener("click", setTheme);
        return card;
      }),
    ),
  ]);
}

/** Dibuja la página. */
function render(root: HTMLElement): void {
  about = el("dl", { class: "about-list" });
  root.append(
    el("div", { class: "page" }, [
      pageHeader(t("settings.title"), t("settings.subtitle")),
      el("div", { class: "settings-grid" }, [
        isDesktop() ? null : wifiPanel(),
        themePanel(),
        appearancePanel(),
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
        isDesktop() ? null : pointerPanel(),
        isDesktop() ? null : keyboardPanel(),
        panel(t("settings.more"), [
          toggle(t("settings.intro"), prefs.intro, setIntro),
          toggle(t("settings.motion"), prefs.reduceMotion, setMotion),
        ]),
        panel(t("settings.about"), [about]),
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

// * Preferencias de la interfaz: idioma, color de acento, intro al arrancar y
// * movimiento reducido. Se guardan en localStorage (dura la sesión) y, si el
// * USB de Rebootera tiene Vault, también en él para que sobrevivan al reinicio.

import { getJSON, postJSON } from "./api.js";
import { setLanguage, type Language } from "./i18n.js";

/** Preferencias disponibles. */
export interface Prefs {
  language: Language;
  accent: "aurora" | "nebula";
  intro: boolean;
  reduceMotion: boolean;
}

const KEY = "rebootera.prefs";

export const prefs: Prefs = {
  language: "en",
  accent: "aurora",
  intro: true,
  reduceMotion: false,
};

/** Carga las preferencias guardadas, si las hay. */
export function loadPrefs(): void {
  try {
    const saved = JSON.parse(localStorage.getItem(KEY) ?? "{}") as Partial<Prefs>;
    if (saved.language === "en" || saved.language === "es") {
      prefs.language = saved.language;
    }
    if (saved.accent === "aurora" || saved.accent === "nebula") {
      prefs.accent = saved.accent;
    }
    if (typeof saved.intro === "boolean") {
      prefs.intro = saved.intro;
    }
    if (typeof saved.reduceMotion === "boolean") {
      prefs.reduceMotion = saved.reduceMotion;
    }
  } catch {
    // * Sin almacenamiento disponible se usan los valores por defecto.
  }
  applyPrefs();
}

/** Guarda las preferencias y las aplica. */
export function savePrefs(): void {
  try {
    localStorage.setItem(KEY, JSON.stringify(prefs));
  } catch {
    // * Si no se puede guardar, siguen aplicadas durante esta sesión.
  }
  applyPrefs();
  postJSON("/api/settings", prefs).catch(ignore);
}

/** Lee las preferencias guardadas en el Vault, si las hay, y las aplica. */
export async function loadVaultPrefs(): Promise<boolean> {
  try {
    const data = await getJSON<{ persistent: boolean; settings: Prefs }>("/api/settings");
    if (!data.persistent) {
      return false;
    }
    Object.assign(prefs, data.settings);
    applyPrefs();
    return true;
  } catch {
    return false;
  }
}

/** No hace nada: guardar en el Vault es opcional (sesión de solo lectura). */
function ignore(): void {}

/** Aplica idioma, acento y movimiento al documento. */
export function applyPrefs(): void {
  setLanguage(prefs.language);
  document.documentElement.dataset.accent = prefs.accent;
  document.documentElement.dataset.motion = prefs.reduceMotion ? "reduced" : "full";
}

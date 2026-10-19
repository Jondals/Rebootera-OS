// * Preferencias de la interfaz: idioma, color de acento, intro al arrancar y
// * movimiento reducido. Se guardan en localStorage (dura la sesión) y, si el
// * USB de Rebootera tiene Vault, también en él para que sobrevivan al reinicio.

import { getJSON, postJSON } from "./api.js";
import { setLanguage, type Language } from "./i18n.js";

/** Temas visuales: cielo (claro), noche (oscuro) y bosque (claro, verde). */
export type Theme = "sky" | "night" | "forest";

/** Todos los temas, en el orden en que se ofrecen. */
export const THEMES: Theme[] = ["sky", "night", "forest"];

/** Preferencias disponibles. */
export interface Prefs {
  language: Language;
  accent: "aurora" | "nebula";
  theme: Theme;
  intro: boolean;
  reduceMotion: boolean;
}

const KEY = "rebootera.prefs";

/** Renderizadores que dibujan por software o por una GPU virtual lenta. */
const SLOW_RENDERERS =
  /llvmpipe|softpipe|swiftshader|software|basic render|virtualbox|vmware|svga|virgl|bochs|cirrus/i;

let lite: boolean | null = null;
let keepMotion = false;

/**
 * Indica si hay que usar el modo ligero: sin GPU real (renderizado por
 * software, máquinas virtuales) o con muy pocas CPU, los desenfoques y las
 * animaciones continuas hacen la interfaz lenta. Se decide una vez.
 */
export function isLite(): boolean {
  if (lite !== null) {
    return lite;
  }
  lite = false;
  // * El kiosco sabe si dibuja por software (start-kiosk): es la señal más fiable.
  if ((window as unknown as { REBOOTERA_LITE?: boolean }).REBOOTERA_LITE === true) {
    lite = true;
    return lite;
  }
  try {
    const canvas = document.createElement("canvas");
    const gl = (canvas.getContext("webgl") ??
      canvas.getContext("experimental-webgl")) as WebGLRenderingContext | null;
    if (!gl) {
      lite = true;
    } else {
      const info = gl.getExtension("WEBGL_debug_renderer_info");
      const renderer = info ? String(gl.getParameter(info.UNMASKED_RENDERER_WEBGL)) : "";
      lite = SLOW_RENDERERS.test(renderer);
    }
  } catch {
    lite = true;
  }
  if (navigator.hardwareConcurrency > 0 && navigator.hardwareConcurrency <= 2) {
    lite = true;
  }
  return lite;
}

/**
 * Fuerza el modo ligero. Rebootera Desktop lo usa siempre: en reposo, el fondo
 * animado y los desenfoques mantenían ocupado a WebView2 (más de un tercio de un núcleo).
 */
export function forceLite(): void {
  lite = true;
  // * Sin desenfoques ni brillos, pero el fondo con burbujas sigue animado: solo mueve capas y cuesta poco.
  keepMotion = true;
  applyPrefs();
}

export const prefs: Prefs = {
  language: "en",
  accent: "aurora",
  theme: "sky",
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
    if (saved.theme === "sky" || saved.theme === "night" || saved.theme === "forest") {
      prefs.theme = saved.theme;
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
    if (!THEMES.includes(prefs.theme)) {
      prefs.theme = "sky";
    }
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
  document.documentElement.dataset.theme = THEMES.includes(prefs.theme) ? prefs.theme : "sky";
  document.documentElement.dataset.gfx = isLite() ? "lite" : "full";
  document.documentElement.dataset.motion =
    prefs.reduceMotion || (isLite() && !keepMotion) ? "reduced" : "full";
}

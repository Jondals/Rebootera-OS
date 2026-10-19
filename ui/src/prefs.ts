// * Preferencias de la interfaz: idioma, color de acento, intro al arrancar y
// * movimiento reducido. Se guardan en localStorage (dura la sesión) y, si el
// * USB de Rebootera tiene Vault, también en él para que sobrevivan al reinicio.

import { getJSON, postJSON } from "./api.js";
import { setLanguage, type Language } from "./i18n.js";

/** Temas visuales: siete oscuros (obsidiana, zafiro, esmeralda, amatista, brasa, grafito y amoled) y marfil (claro). */
export type Theme =
  | "obsidian"
  | "sapphire"
  | "emerald"
  | "amethyst"
  | "ember"
  | "graphite"
  | "amoled"
  | "ivory";

/** Todos los temas, en el orden en que se ofrecen. */
export const THEMES: Theme[] = [
  "obsidian",
  "sapphire",
  "emerald",
  "amethyst",
  "ember",
  "graphite",
  "amoled",
  "ivory",
];

/** Preferencias disponibles. */
export interface Prefs {
  language: Language;
  theme: Theme;
  /** Fondo animado: automático (se apaga en equipos modestos), siempre o nunca. */
  background: "auto" | "on" | "off";
  /** Tamaño de la interfaz en porcentaje. */
  scale: number;
  /** Desenfoque del fondo animado. */
  blur: "off" | "soft" | "medium" | "strong";
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
  theme: "obsidian",
  background: "auto",
  scale: 100,
  blur: "soft",
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
    if (THEMES.includes(saved.theme as Theme)) {
      prefs.theme = saved.theme as Theme;
    }
    if (saved.background === "auto" || saved.background === "on" || saved.background === "off") {
      prefs.background = saved.background;
    }
    if (
      saved.blur === "off" ||
      saved.blur === "soft" ||
      saved.blur === "medium" ||
      saved.blur === "strong"
    ) {
      prefs.blur = saved.blur;
    }
    if (typeof saved.scale === "number" && saved.scale >= 90 && saved.scale <= 130) {
      prefs.scale = saved.scale;
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
      prefs.theme = "obsidian";
    }
    applyPrefs();
    return true;
  } catch {
    return false;
  }
}

/** No hace nada: guardar en el Vault es opcional (sesión de solo lectura). */
function ignore(): void {}

/**
 * Indica si el equipo es modesto: pocos núcleos o poca memoria. Ahí el fondo animado se queda quieto en el modo
 * automático, porque cada fotograma compite con la propia interfaz.
 */
function isModest(): boolean {
  const memory = (navigator as unknown as { deviceMemory?: number }).deviceMemory;
  return (
    (navigator.hardwareConcurrency > 0 && navigator.hardwareConcurrency <= 4) ||
    (memory !== undefined && memory <= 4)
  );
}

/** Estado del fondo animado: "on" (se mueve), "still" (quieto) u "off" (sin fondo animado). */
function backgroundState(): "on" | "still" | "off" {
  if (prefs.background === "off") {
    return "off";
  }
  if (prefs.background === "on") {
    return "on";
  }
  // * Automático: quieto en equipos modestos o sin GPU; en el escritorio forzado a "ligero" (sin brillos) sí se mueve.
  if ((isLite() && !keepMotion) || isModest()) {
    return "still";
  }
  return "on";
}

/** Aplica idioma, tema, fondo, tamaño y movimiento al documento. */
export function applyPrefs(): void {
  setLanguage(prefs.language);
  const root = document.documentElement;
  root.dataset.theme = THEMES.includes(prefs.theme) ? prefs.theme : "obsidian";
  root.dataset.gfx = isLite() ? "lite" : "full";
  root.dataset.motion = prefs.reduceMotion || (isLite() && !keepMotion) ? "reduced" : "full";
  root.dataset.bg = backgroundState();
  // * El desenfoque del fondo cuesta en equipos modestos: allí no se aplica nunca.
  root.dataset.blur = isLite() && !keepMotion ? "off" : prefs.blur;
  if (document.body) {
    document.body.style.zoom = String(prefs.scale / 100);
  }
}

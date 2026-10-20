// * Los ocho temas de la página: los mismos (nombres y colores) que Rebootera OS y Rebootera Desktop, todos oscuros.
// * Esta lista es la única fuente: de ella salen el CSS de cada tema (themeCss) y las tarjetas del selector.

/** Un tema: colores base, de acento y el texto que lo describe en los dos idiomas. */
export interface Theme {
  id: string;
  name: { en: string; es: string };
  hint: { en: string; es: string };
  bg: string;
  surface: string;
  /** Color (r, g, b) con el que se tiñen los bordes y las superficies. */
  tint: string;
  text: string;
  muted: string;
  dim: string;
  accent: string;
  strong: string;
  /** Color del texto sobre el acento (botones principales). */
  contrast: string;
}

export const THEMES: Theme[] = [
  { id: "amoled", name: { en: "Amoled", es: "Amoled" }, hint: { en: "Pure black, cyan", es: "Negro puro, cian" }, bg: "#000000", surface: "#0a0b0d", tint: "120, 220, 240", text: "#e8f4f7", muted: "#8fb0b8", dim: "#5a7a82", accent: "#4fd8ee", strong: "#28bcd6", contrast: "#021418" },
  { id: "obsidian", name: { en: "Obsidian", es: "Obsidiana" }, hint: { en: "Dark, with brass", es: "Oscuro, con latón" }, bg: "#0b0d11", surface: "#14181f", tint: "255, 255, 255", text: "#ece7db", muted: "#a39e8f", dim: "#6f6b60", accent: "#d8ae62", strong: "#c2953f", contrast: "#1c1406" },
  { id: "sapphire", name: { en: "Sapphire", es: "Zafiro" }, hint: { en: "Deep blue", es: "Azul profundo" }, bg: "#070b17", surface: "#101730", tint: "150, 175, 255", text: "#e6ebf8", muted: "#97a3c4", dim: "#66719a", accent: "#8eaaff", strong: "#6a8cf0", contrast: "#08122e" },
  { id: "emerald", name: { en: "Emerald", es: "Esmeralda" }, hint: { en: "Deep green", es: "Verde profundo" }, bg: "#060d0b", surface: "#0f1a17", tint: "150, 230, 200", text: "#e4f2ec", muted: "#93b3a6", dim: "#62806f", accent: "#5fd3a0", strong: "#3fb985", contrast: "#04140d" },
  { id: "amethyst", name: { en: "Amethyst", es: "Amatista" }, hint: { en: "Violet night", es: "Noche violeta" }, bg: "#0b0814", surface: "#181226", tint: "190, 160, 255", text: "#ece6fa", muted: "#a398c4", dim: "#6f6694", accent: "#b79cff", strong: "#9a78f0", contrast: "#12082e" },
  { id: "ember", name: { en: "Ember", es: "Brasa" }, hint: { en: "Dark, warm red", es: "Oscuro, rojo cálido" }, bg: "#0f0807", surface: "#1d1210", tint: "255, 170, 140", text: "#f4e6e0", muted: "#b9998d", dim: "#82665c", accent: "#ff9a6b", strong: "#f0773f", contrast: "#1c0a02" },
  { id: "orange", name: { en: "Orange", es: "Naranja" }, hint: { en: "Dark, vivid orange", es: "Oscuro, naranja intenso" }, bg: "#0d0905", surface: "#1a130b", tint: "255, 170, 90", text: "#f6ece0", muted: "#b9a28a", dim: "#82705c", accent: "#ff8a1f", strong: "#e5700a", contrast: "#1f0e00" },
  { id: "graphite", name: { en: "Graphite", es: "Grafito" }, hint: { en: "Neutral grey", es: "Gris neutro" }, bg: "#0d0e10", surface: "#17191c", tint: "200, 205, 215", text: "#eceef1", muted: "#9ca2ab", dim: "#6a7078", accent: "#e8ebf0", strong: "#c5cad3", contrast: "#101214" },
];

/** Convierte "#rrggbb" en "r, g, b". */
function rgb(hex: string): string {
  const n = parseInt(hex.slice(1), 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255].join(", ");
}

/** Variables CSS de un tema (las que usa Tailwind a través de @theme en global.css). */
function variables(theme: Theme): string {
  const a = rgb(theme.accent);
  return [
    `--bg: ${theme.bg}`,
    `--panel: ${theme.surface}`,
    `--panel-2: rgba(${theme.tint}, 0.06)`,
    `--border: rgba(${theme.tint}, 0.12)`,
    `--border-strong: rgba(${theme.tint}, 0.25)`,
    `--text: ${theme.text}`,
    `--muted: ${theme.muted}`,
    `--dim: color-mix(in srgb, ${theme.muted} 62%, ${theme.dim})`,
    `--accent: ${theme.accent}`,
    `--accent-strong: ${theme.strong}`,
    `--accent-rgb: ${a}`,
    `--on-accent: ${theme.contrast}`,
  ].join("; ");
}

/** CSS de todos los temas: el primero (Amoled) también es el de por defecto. */
export function themeCss(): string {
  return THEMES.map(function block(theme, index) {
    const selector = index === 0 ? `:root, :root[data-theme="${theme.id}"]` : `:root[data-theme="${theme.id}"]`;
    return `${selector} { ${variables(theme)}; color-scheme: dark; }`;
  }).join("\n");
}

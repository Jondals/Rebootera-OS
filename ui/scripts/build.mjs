// * Genera dist/ con todo lo que sirve rebootera-core: el JavaScript (todo el código en un solo archivo, minimizado),
// * una sola hoja de estilos (las ocho de src/styles unidas y minimizadas) y un archivo por página (se leen al abrir cada
// * una), el HTML, el logotipo y las fuentes.
// * Menos archivos y menos bytes importan mucho cuando la interfaz se lee desde un pendrive: cada archivo es una
// * lectura más del USB. tsc solo comprueba los tipos (npm run check-types); aquí no genera nada.
import { build, transform } from "esbuild";
import { cpSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { copyFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const uiDir = join(dirname(fileURLToPath(import.meta.url)), "..");
const distDir = join(uiDir, "dist");

/** Hojas de estilo en el orden en que se aplican. */
const STYLES = ["tokens", "base", "layout", "components", "pages", "lux", "intro", "topnav"];

rmSync(distDir, { recursive: true, force: true });
mkdirSync(join(distDir, "js"), { recursive: true });
mkdirSync(join(distDir, "styles"), { recursive: true });

await build({
  entryPoints: [join(uiDir, "src/main.ts")],
  outdir: join(distDir, "js"),
  // * Cada página es un archivo aparte (import() en main.ts) y el código que comparten va en uno común.
  splitting: true,
  chunkNames: "[name]-[hash]",
  bundle: true,
  minify: true,
  format: "esm",
  target: ["es2022", "safari16"],
  legalComments: "none",
});

const css = STYLES.map(function read(name) {
  return readFileSync(join(uiDir, "src/styles", name + ".css"), "utf8");
}).join("\n");
const sheet = await transform(css, { loader: "css", minify: true, target: ["safari16"] });
writeFileSync(join(distDir, "styles/app.css"), sheet.code);

// * El HTML avisa de los archivos de JavaScript que main.js necesita nada más empezar (modulepreload): sin eso, el navegador
// * los descubre uno tras otro (main.js, luego sus archivos comunes, luego los de estos...) y cada vuelta es una lectura del USB.
const entry = readFileSync(join(distDir, "js/main.js"), "utf8");
const preloads = [...entry.matchAll(/from"\.\/(chunk-[A-Za-z0-9]+\.js)"/g)].map(
  function name(match) {
    return match[1];
  },
);
const script = '<script type="module" src="js/main.js"></script>';
const tags = [...new Set(preloads)].map(function tag(name) {
  return '<link rel="modulepreload" href="js/' + name + '" />';
});
const source = readFileSync(join(uiDir, "src/index.html"), "utf8");
if (!source.includes(script)) {
  throw new Error("index.html no tiene la etiqueta del script principal");
}
writeFileSync(
  join(distDir, "index.html"),
  source.replace(script, [...tags, script].join("\n    ")),
);

copyFileSync(join(uiDir, "../branding/logo.svg"), join(distDir, "logo.svg"));
cpSync(join(uiDir, "src/fonts"), join(distDir, "fonts"), { recursive: true });
console.log("Interfaz lista en " + distDir);

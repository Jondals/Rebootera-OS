// * Copia los archivos estáticos de la interfaz (HTML, hojas de estilo y
// * logotipo) junto al JavaScript que genera tsc, dejando en dist/ todo lo
// * que sirve rebootera-core.
import { copyFileSync, cpSync, mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const uiDir = join(dirname(fileURLToPath(import.meta.url)), "..");
const distDir = join(uiDir, "dist");

mkdirSync(distDir, { recursive: true });
copyFileSync(join(uiDir, "src/index.html"), join(distDir, "index.html"));
copyFileSync(join(uiDir, "../branding/logo.svg"), join(distDir, "logo.svg"));
cpSync(join(uiDir, "src/styles"), join(distDir, "styles"), { recursive: true });
cpSync(join(uiDir, "src/fonts"), join(distDir, "fonts"), { recursive: true });
console.log("Interfaz lista en " + distDir);

// * Configuración de Astro para la landing de Rebootera OS: sitio estático con Tailwind 4 (plugin de Vite).
// * Todo el CSS va dentro del HTML y las rutas son relativas, para que la página funcione servida desde
// * cualquier carpeta (GitHub Pages, un USB, el servidor local de scripts/dev/site.ps1).
import { defineConfig } from "astro/config";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  build: { inlineStylesheets: "always", assets: "assets" },
  vite: { plugins: [tailwindcss()] },
});

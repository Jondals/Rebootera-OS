// * Copia a public/ el logotipo del proyecto (branding/logo.svg). Las capturas de la aplicación ya están en
// * public/images/<tema>/ (una por pantalla y tema, en WebP). Se ejecuta antes de `astro dev` y de `astro build`.
import { copyFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const site = join(dirname(fileURLToPath(import.meta.url)), "..");
copyFileSync(join(site, "..", "branding", "logo.svg"), join(site, "public", "logo.svg"));
console.log("Logotipo copiado a public/");

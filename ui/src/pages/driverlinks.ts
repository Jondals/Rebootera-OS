// * Enlaces para conseguir un driver que falta, de lo más oficial a lo más general, para abrirlos en el navegador de
// * Rebootera OS: la página del fabricante del chip (Intel, Realtek...), el soporte del fabricante del equipo con su modelo
// * (Lenovo, HP, Dell...) y el Catálogo de Microsoft Update con el identificador de hardware exacto. Cada búsqueda va
// * limitada al dominio oficial (site:) para que los primeros resultados sean del fabricante y no de webs de descargas.
// ! Los enlaces se construyen solo con datos conocidos (dominios fijos y texto codificado): nunca se abre una dirección
// ! que venga de fuera.

import type { Device, SystemInfo } from "../api.js";

/** Un enlace con su texto ya traducible (clave y variables) y su dirección https. */
export interface DriverLink {
  /** Clave de traducción del texto del botón. */
  labelKey: string;
  /** Nombre del fabricante, para el texto del botón. */
  maker: string;
  url: string;
}

/** Fabricantes de chips por identificador PCI/USB: nombre, dominio oficial y página de descargas. */
const CHIP_MAKERS: Record<string, { name: string; site: string; home: string }> = {
  "8086": {
    name: "Intel",
    site: "intel.com",
    home: "https://www.intel.com/content/www/us/en/download-center/home.html",
  },
  "8087": {
    name: "Intel",
    site: "intel.com",
    home: "https://www.intel.com/content/www/us/en/download-center/home.html",
  },
  "10ec": { name: "Realtek", site: "realtek.com", home: "https://www.realtek.com/Download" },
  "0bda": { name: "Realtek", site: "realtek.com", home: "https://www.realtek.com/Download" },
  "10de": {
    name: "NVIDIA",
    site: "nvidia.com",
    home: "https://www.nvidia.com/Download/index.aspx",
  },
  "1002": {
    name: "AMD",
    site: "amd.com",
    home: "https://www.amd.com/en/support/download/drivers.html",
  },
  "1022": {
    name: "AMD",
    site: "amd.com",
    home: "https://www.amd.com/en/support/download/drivers.html",
  },
  "14e4": {
    name: "Broadcom",
    site: "broadcom.com",
    home: "https://www.broadcom.com/support/download-search",
  },
  "168c": { name: "Qualcomm", site: "qualcomm.com", home: "https://www.qualcomm.com/support" },
  "17cb": { name: "Qualcomm", site: "qualcomm.com", home: "https://www.qualcomm.com/support" },
  "14c3": { name: "MediaTek", site: "mediatek.com", home: "https://www.mediatek.com/products" },
  "0e8d": { name: "MediaTek", site: "mediatek.com", home: "https://www.mediatek.com/products" },
  "1b21": { name: "ASMedia", site: "asmedia.com.tw", home: "https://www.asmedia.com.tw/" },
  "1912": {
    name: "Renesas",
    site: "renesas.com",
    home: "https://www.renesas.com/en/support/downloads",
  },
  "144d": {
    name: "Samsung",
    site: "samsung.com",
    home: "https://semiconductor.samsung.com/consumer-storage/support/tools/",
  },
  "15b7": {
    name: "SanDisk",
    site: "sandisk.com",
    home: "https://support-en.sandisk.com/app/answers/detail/a_id/17210",
  },
  "1987": { name: "Phison", site: "phison.com", home: "https://www.phison.com/" },
  "1344": { name: "Micron", site: "crucial.com", home: "https://www.crucial.com/support" },
};

/** Fabricantes de equipos: palabra que sale en el nombre del fabricante de la BIOS y su soporte oficial. */
const PC_MAKERS: { match: RegExp; name: string; site: string; home: string }[] = [
  {
    match: /lenovo/i,
    name: "Lenovo",
    site: "support.lenovo.com",
    home: "https://pcsupport.lenovo.com/",
  },
  {
    match: /hewlett|^hp\b|\bhp\b/i,
    name: "HP",
    site: "support.hp.com",
    home: "https://support.hp.com/drivers",
  },
  {
    match: /dell|alienware/i,
    name: "Dell",
    site: "dell.com/support",
    home: "https://www.dell.com/support/home/",
  },
  {
    match: /asus/i,
    name: "ASUS",
    site: "asus.com/support",
    home: "https://www.asus.com/support/download-center/",
  },
  { match: /acer|packard/i, name: "Acer", site: "acer.com", home: "https://www.acer.com/support" },
  {
    match: /micro-star|msi/i,
    name: "MSI",
    site: "msi.com/support",
    home: "https://www.msi.com/support",
  },
  {
    match: /gigabyte|aorus/i,
    name: "Gigabyte",
    site: "gigabyte.com/Support",
    home: "https://www.gigabyte.com/Support",
  },
  {
    match: /samsung/i,
    name: "Samsung",
    site: "samsung.com/support",
    home: "https://www.samsung.com/support/",
  },
  {
    match: /toshiba|dynabook/i,
    name: "Dynabook",
    site: "support.dynabook.com",
    home: "https://support.dynabook.com/",
  },
  {
    match: /huawei|honor/i,
    name: "Huawei",
    site: "consumer.huawei.com/en/support",
    home: "https://consumer.huawei.com/en/support/",
  },
  {
    match: /microsoft/i,
    name: "Microsoft",
    site: "support.microsoft.com",
    home: "https://support.microsoft.com/surface",
  },
  {
    match: /medion/i,
    name: "Medion",
    site: "medion.com",
    home: "https://www.medion.com/en/service/start/",
  },
  {
    match: /lg\b|lg electronics/i,
    name: "LG",
    site: "lg.com/support",
    home: "https://www.lg.com/support",
  },
];

/** Dirección de una búsqueda limitada a un dominio oficial. */
function siteSearch(site: string, words: string): string {
  return "https://duckduckgo.com/?q=" + encodeURIComponent("site:" + site + " " + words);
}

/** Identificador de fabricante (4 cifras hexadecimales en minúsculas) de un dispositivo, sacado de su hardware ID. */
function vendorIdOf(device: Device): string {
  const id = device.hardwareIds[0] ?? "";
  const found = /(?:VEN|VID)_([0-9A-Fa-f]{4})/.exec(id);
  return found ? found[1].toLowerCase() : device.vendorId.toLowerCase();
}

/** Nombre del dispositivo para buscarlo: se quita lo que no ayuda (corchetes de PCI, números de revisión). */
function searchName(device: Device): string {
  return (device.name || device.className || "")
    .replace(/\(rev [0-9a-f]+\)/i, "")
    .replace(/[[\]]/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

/**
 * Enlaces para el driver de Windows de un dispositivo, del más oficial al más general:
 *  1. la página de descargas del fabricante del chip (con una búsqueda de su modelo limitada a su dominio);
 *  2. el soporte del fabricante del equipo, con el modelo exacto de este equipo;
 *  3. el Catálogo de Microsoft Update, con el identificador de hardware del dispositivo.
 */
export function driverLinks(device: Device, system: SystemInfo | null): DriverLink[] {
  const links: DriverLink[] = [];
  const name = searchName(device);
  const chip = CHIP_MAKERS[vendorIdOf(device)];
  if (chip) {
    links.push({
      labelKey: "drivers.linkChip",
      maker: chip.name,
      url: name ? siteSearch(chip.site, name + " driver Windows") : chip.home,
    });
  }
  if (system) {
    const owner = [system.vendor, system.biosVendor].join(" ");
    const maker = PC_MAKERS.find(function matches(item) {
      return item.match.test(owner);
    });
    if (maker) {
      const model = [system.model, name].filter(Boolean).join(" ");
      links.push({
        labelKey: "drivers.linkPc",
        maker: maker.name,
        url: system.model ? siteSearch(maker.site, model + " driver") : maker.home,
      });
    }
  }
  const hardwareId = device.hardwareIds.find(function usable(id) {
    return id.includes("VEN_") || id.includes("VID_");
  });
  if (hardwareId) {
    // * El catálogo busca por el identificador sin la revisión ni el subsistema: el más general que casa con el chip.
    const short = hardwareId.split("&").slice(0, 2).join("&");
    links.push({
      labelKey: "drivers.linkCatalog",
      maker: "Microsoft",
      url: "https://www.catalog.update.microsoft.com/Search.aspx?q=" + encodeURIComponent(short),
    });
  }
  if (links.length === 0) {
    links.push({
      labelKey: "drivers.linkSearch",
      maker: "",
      url:
        "https://duckduckgo.com/?q=" +
        encodeURIComponent([device.vendor, name, "Windows driver"].filter(Boolean).join(" ")),
    });
  }
  return links;
}

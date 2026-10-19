// * Formato de cantidades para la interfaz: tamaños, velocidades, tiempos y
// * temperaturas, respetando el idioma activo.

import { getLanguage } from "./i18n.js";

/** Locale de Intl según el idioma de la interfaz. */
function locale(): string {
  return getLanguage() === "es" ? "es-ES" : "en-US";
}

/** Formatea un número con como mucho `digits` decimales. */
function number(value: number, digits = 1): string {
  return value.toLocaleString(locale(), { maximumFractionDigits: digits });
}

/** Formatea bytes en la unidad decimal más adecuada (como los fabricantes). */
export function bytes(value: number): string {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = value;
  let unit = 0;
  while (size >= 1000 && unit < units.length - 1) {
    size /= 1000;
    unit++;
  }
  return number(size, size < 10 ? 1 : 0) + " " + units[unit];
}

/** Formatea bytes por segundo. */
export function rate(bytesPerSecond: number): string {
  return bytes(bytesPerSecond) + "/s";
}

/** Formatea una velocidad de enlace en Mbps o Gbps. */
/** Formatea una temperatura en grados Celsius. */
/** Formatea un intervalo pasado ("hace 5 s", "5s ago"). */
/** Formatea una duración larga en horas y minutos. */
export function duration(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  return hours > 0 ? hours + " h " + minutes + " min" : minutes + " min";
}

/** Formatea la hora actual en formato 24 h. */

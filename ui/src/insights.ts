// * Consultas sobre los datos de state.ts que comparten varias páginas:
// * medio de arranque, discos internos, Vault, sensor principal e interfaz
// * de red conectada.

import type { Disk, NetIf, Partition, SystemInfo } from "./api.js";

/** Devuelve el disco desde el que arrancó Rebootera, si se conoce. */
export function liveMedia(disks: Disk[] | null): Disk | null {
  return disks?.find(isLiveMedia) ?? null;
}

/** Indica si un disco es el medio de arranque. */
function isLiveMedia(disk: Disk): boolean {
  return disk.role === "live-media";
}

/** Devuelve los discos internos del equipo. */
export function internalDisks(disks: Disk[] | null): Disk[] {
  return disks?.filter(isInternal) ?? [];
}

/** Indica si un disco es interno. */
function isInternal(disk: Disk): boolean {
  return disk.role === "internal";
}

/** Devuelve la partición Vault montada del medio de arranque, si existe. */
export function vault(disks: Disk[] | null): Partition | null {
  const media = liveMedia(disks);
  return media?.partitions.find(isMountedVault) ?? null;
}

/** Indica si una partición es un Vault montado. */
function isMountedVault(part: Partition): boolean {
  return part.role === "vault" && part.mountPoint !== "" && part.fsType !== "iso9660";
}

/** Tipo de arranque para la barra superior: usb, optical u other. */
/** Devuelve la primera interfaz conectada con dirección IP. */
export function onlineInterface(system: SystemInfo | null): NetIf | null {
  return system?.interfaces.find(isOnline) ?? null;
}

/** Indica si una interfaz está conectada y tiene dirección. */
function isOnline(iface: NetIf): boolean {
  return iface.state === "up" && iface.addresses.length > 0;
}

/** Indica si una línea del registro del kernel menciona un problema. */
export function isProblemLine(line: string): boolean {
  return /\b(error|fail(ed|ure)?|warn(ing)?|timeout|corrupt)\b/i.test(line);
}

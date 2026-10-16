// * Consultas sobre los datos de state.ts que comparten varias páginas:
// * medio de arranque, discos internos, Vault, sensor principal e interfaz
// * de red conectada.

import type { Disk, NetIf, Partition, Reading, SystemInfo } from "./api.js";

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
export function bootSource(disks: Disk[] | null): "usb" | "optical" | "other" {
  const media = liveMedia(disks);
  if (media?.transport === "usb") {
    return "usb";
  }
  if (media?.transport === "optical") {
    return "optical";
  }
  return "other";
}

/** Devuelve la primera interfaz conectada con dirección IP. */
export function onlineInterface(system: SystemInfo | null): NetIf | null {
  return system?.interfaces.find(isOnline) ?? null;
}

/** Indica si una interfaz está conectada y tiene dirección. */
function isOnline(iface: NetIf): boolean {
  return iface.state === "up" && iface.addresses.length > 0;
}

/** Elige el sensor más representativo: CPU, luego disco, luego cualquiera. */
export function primarySensor(sensors: Reading[] | null): Reading | null {
  if (!sensors || sensors.length === 0) {
    return null;
  }
  return sensors.find(isCpuPackage) ?? sensors.find(isCpu) ?? sensors.find(isStorage) ?? sensors[0];
}

/** Indica si es la temperatura del encapsulado de la CPU. */
function isCpuPackage(reading: Reading): boolean {
  return reading.kind === "cpu" && /package|tctl|tdie/i.test(reading.label);
}

/** Indica si es un sensor de CPU. */
function isCpu(reading: Reading): boolean {
  return reading.kind === "cpu";
}

/** Indica si es un sensor de disco. */
function isStorage(reading: Reading): boolean {
  return reading.kind === "storage";
}

/** Clave de historial de un sensor (igual que en state.ts). */
export function sensorKey(reading: Reading): string {
  return reading.chip + "/" + reading.label;
}

/** Indica si una línea del registro del kernel menciona un problema. */
export function isProblemLine(line: string): boolean {
  return /\b(error|fail(ed|ure)?|warn(ing)?|timeout|corrupt)\b/i.test(line);
}

// * Estado compartido de la interfaz. Sondea la API de Core a intervalos,
// * guarda un historial corto de temperaturas y tráfico de red para las
// * gráficas y avisa a quien se haya suscrito cuando llegan datos nuevos.
// ! Todo lo que se muestra sale de aquí: no hay datos inventados.

import { isLite } from "./prefs.js";
import {
  getJSON,
  type Device,
  type Disk,
  type Job,
  type LibraryItem,
  type NetIf,
  type Reading,
  type SystemInfo,
  type VaultStatus,
} from "./api.js";

/** Intervalo de sondeo del Vault en Rebootera Desktop (no hay nada más que vigilar). */
const DESKTOP_MS = 5000;

/** Datos actuales y su historial. */
export interface Snapshot {
  system: SystemInfo | null;
  disks: Disk[] | null;
  devices: Device[] | null;
  sensors: Reading[] | null;
  /** Estado del Vault del USB (null hasta la primera lectura). */
  vault: VaultStatus | null;
  /** Imágenes de la biblioteca (vacía si no hay Vault). */
  library: LibraryItem[];
  /** Cola de descargas e importaciones. */
  transfers: Job[];
  /** Momento del último análisis de hardware (ms). */
  hardwareScannedAt: number;
  /** Último error de conexión con Core, si lo hay. */
  error: unknown;
}

export const snapshot: Snapshot = {
  system: null,
  disks: null,
  devices: null,
  sensors: null,
  vault: null,
  library: [],
  transfers: [],
  hardwareScannedAt: 0,
  error: null,
};

/** Función a la que se avisa cuando cambian los datos. */
export type Listener = () => void;

const listeners = new Set<Listener>();
/** Huella de lo último que se avisó a las páginas: si no cambia nada visible, no se redibuja nada. */
let lastFingerprint = "";

/** Suscribe una función a los cambios de datos. */
export function subscribe(listener: Listener): void {
  listeners.add(listener);
}

/**
 * Resume lo que se ve de los datos. Se dejan fuera lo que cambia siempre sin que importe (tiempo encendido, hora,
 * contadores de red) y las temperaturas se redondean a 5 grados: así un sondeo que no trae nada nuevo no hace
 * que las páginas se redibujen (en un USB lento cada redibujado se nota).
 */
function fingerprint(): string {
  const system = snapshot.system;
  return JSON.stringify([
    system && {
      ...system,
      uptimeSeconds: 0,
      time: "",
      interfaces: system.interfaces.map(withoutCounters),
    },
    snapshot.sensors?.map(function rounded(reading) {
      return [reading.chip, reading.label, Math.round(reading.celsius / 5)];
    }),
    snapshot.disks,
    snapshot.devices,
    snapshot.vault,
    snapshot.library,
    snapshot.transfers,
    snapshot.hardwareScannedAt,
    String(snapshot.error),
  ]);
}

/** Interfaz de red sin sus contadores de tráfico. */
function withoutCounters(iface: NetIf): NetIf {
  return { ...iface, rxBytes: 0, txBytes: 0 };
}

/** Avisa a todos los suscriptores, solo si algo visible cambió desde la última vez. */
function notify(): void {
  const current = fingerprint();
  if (current === lastFingerprint) {
    return;
  }
  lastFingerprint = current;
  for (const listener of listeners) {
    listener();
  }
}

/** Lee el sistema y los sensores (sondeo rápido). */
async function pollFast(): Promise<void> {
  try {
    const [system, sensors] = await Promise.all([
      getJSON<SystemInfo>("/api/system"),
      getJSON<Reading[]>("/api/sensors"),
    ]);
    snapshot.system = system;
    snapshot.sensors = sensors;
    snapshot.error = null;
  } catch (error) {
    snapshot.error = error;
  }
  notify();
}

/** Lee el Vault, la biblioteca y las transferencias (sondeo rápido). */
async function pollVault(): Promise<void> {
  try {
    snapshot.vault = await getJSON<VaultStatus>("/api/vault");
    if (snapshot.vault.available) {
      const [library, transfers] = await Promise.all([
        getJSON<LibraryItem[]>("/api/library"),
        getJSON<Job[]>("/api/transfers"),
      ]);
      snapshot.library = library;
      snapshot.transfers = transfers;
    } else {
      snapshot.library = [];
      snapshot.transfers = [];
    }
  } catch {
    // * Un fallo puntual no borra lo último conocido.
  }
  notify();
}

/** Lee los discos (sondeo lento). */
async function pollSlow(): Promise<void> {
  try {
    snapshot.disks = await getJSON<Disk[]>("/api/storage");
  } catch (error) {
    snapshot.error = error;
  }
  notify();
}

/** Vuelve a analizar el hardware (al arrancar y con "Volver a analizar"). */
export async function scanHardware(): Promise<void> {
  try {
    snapshot.devices = await getJSON<Device[]>("/api/hardware");
    snapshot.hardwareScannedAt = Date.now();
  } catch (error) {
    snapshot.error = error;
  }
  notify();
}

/** Fuerza una lectura completa inmediata. */
async function refreshAll(): Promise<void> {
  await Promise.all([pollFast(), pollSlow(), pollVault(), scanHardware()]);
}

/** Vuelve a leer la biblioteca y las transferencias tras un cambio. */
export async function refreshVault(): Promise<void> {
  await pollVault();
}

/** Ejecuta un sondeo solo si la ventana se ve: minimizada u oculta no gasta CPU ni red. */
function whenVisible(poll: () => Promise<void>): void {
  if (!document.hidden) {
    void poll();
  }
}

/**
 * Sondea el Vault a un ritmo que depende de lo que pasa: rápido mientras haya una descarga en marcha (para ver su
 * progreso) y tranquilo el resto del tiempo, porque cada lectura recorre la biblioteca del USB.
 */
function scheduleVaultPoll(idleMs: number): void {
  const busy = snapshot.transfers.some(function active(job) {
    return job.status === "running" || job.status === "verifying" || job.status === "queued";
  });
  window.setTimeout(
    async function again(): Promise<void> {
      if (!document.hidden) {
        await pollVault();
      }
      scheduleVaultPoll(idleMs);
    },
    busy ? 1500 : idleMs,
  );
}

/** Arranca el sondeo periódico y hace la primera lectura completa. */
export async function start(desktop = false): Promise<void> {
  // * En el escritorio solo cambian el USB, la biblioteca y las descargas: no se sondean
  // * ni sensores, ni discos, ni hardware (no existen en Windows).
  if (desktop) {
    scheduleVaultPoll(DESKTOP_MS);
    await pollVault();
    return;
  }
  // * Equipos modestos: sondeos más espaciados. Con la ventana oculta no se sondea nada (ver whenVisible).
  const fast = isLite() ? 10000 : 6000;
  const slow = isLite() ? 45000 : 30000;
  window.setInterval(whenVisible, fast, pollFast);
  scheduleVaultPoll(isLite() ? 10000 : 6000);
  window.setInterval(whenVisible, slow, pollSlow);
  await refreshAll();
}

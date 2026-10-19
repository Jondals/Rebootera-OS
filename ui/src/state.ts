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
  type Reading,
  type SystemInfo,
  type VaultStatus,
} from "./api.js";

/** Número de muestras que se guardan para las gráficas. */
const HISTORY = 40;

/** Intervalo de sondeo del Vault en Rebootera Desktop (no hay nada más que vigilar). */
const DESKTOP_MS = 5000;

/** Datos actuales y su historial. */
export interface Snapshot {
  system: SystemInfo | null;
  disks: Disk[] | null;
  devices: Device[] | null;
  sensors: Reading[] | null;
  kernelLog: string[] | null;
  /** Estado del Vault del USB (null hasta la primera lectura). */
  vault: VaultStatus | null;
  /** Imágenes de la biblioteca (vacía si no hay Vault). */
  library: LibraryItem[];
  /** Cola de descargas e importaciones. */
  transfers: Job[];
  /** Historial por sensor: clave "chip/etiqueta". */
  tempHistory: Map<string, number[]>;
  /** Bytes recibidos por segundo en todas las interfaces. */
  rxHistory: number[];
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
  kernelLog: null,
  vault: null,
  library: [],
  transfers: [],
  tempHistory: new Map(),
  rxHistory: [],
  hardwareScannedAt: 0,
  error: null,
};

/** Función a la que se avisa cuando cambian los datos. */
export type Listener = () => void;

const listeners = new Set<Listener>();
let lastRx: { bytes: number; at: number } | null = null;

/** Suscribe una función a los cambios de datos. */
export function subscribe(listener: Listener): void {
  listeners.add(listener);
}

/** Avisa a todos los suscriptores. */
function notify(): void {
  for (const listener of listeners) {
    listener();
  }
}

/** Añade una muestra a un historial acotado. */
function push(list: number[], value: number): void {
  list.push(value);
  if (list.length > HISTORY) {
    list.shift();
  }
}

/** Suma los bytes recibidos de todas las interfaces conectadas. */
function totalRx(system: SystemInfo): number {
  let total = 0;
  for (const iface of system.interfaces) {
    total += iface.rxBytes;
  }
  return total;
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

    const now = performance.now();
    const rx = totalRx(system);
    if (lastRx !== null && now > lastRx.at) {
      push(snapshot.rxHistory, Math.max(0, ((rx - lastRx.bytes) * 1000) / (now - lastRx.at)));
    }
    lastRx = { bytes: rx, at: now };

    for (const reading of sensors) {
      const key = reading.chip + "/" + reading.label;
      const history = snapshot.tempHistory.get(key) ?? [];
      push(history, reading.celsius);
      snapshot.tempHistory.set(key, history);
    }
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

/** Lee los discos y el registro del kernel (sondeo lento). */
async function pollSlow(): Promise<void> {
  try {
    snapshot.disks = await getJSON<Disk[]>("/api/storage");
  } catch (error) {
    snapshot.error = error;
  }
  try {
    const log = await getJSON<{ lines: string[] }>("/api/logs/kernel");
    snapshot.kernelLog = log.lines;
  } catch {
    // * El registro es opcional: si el agente no responde se deja el último.
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

/** Arranca el sondeo periódico y hace la primera lectura completa. */
export async function start(desktop = false): Promise<void> {
  // * En el escritorio solo cambian el USB, la biblioteca y las descargas: no se sondean
  // * ni sensores, ni discos, ni registro del kernel, ni hardware (no existen en Windows).
  if (desktop) {
    window.setInterval(whenVisible, DESKTOP_MS, pollVault);
    await pollVault();
    return;
  }
  // * Equipos modestos: sondeos más espaciados. Con la ventana oculta no se sondea nada (ver whenVisible).
  const fast = isLite() ? 6000 : 4000;
  const slow = isLite() ? 30000 : 20000;
  window.setInterval(whenVisible, fast, pollFast);
  window.setInterval(whenVisible, isLite() ? 4000 : 2500, pollVault);
  window.setInterval(whenVisible, slow, pollSlow);
  await refreshAll();
}

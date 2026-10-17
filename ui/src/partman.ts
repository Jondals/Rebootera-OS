// * Tipos y utilidades del gestor de particiones de la interfaz. Los números salen
// * de Core (que usa el paquete partman): aquí solo se dibujan y se convierten.

import { getJSON, postJSON } from "./api.js";

/** Sectores de 512 bytes en 1 MiB. */
export const SECTORS_PER_MIB = 2048;

/** Trozo de un disco: partición (number > 0) o hueco libre. */
export interface Segment {
  number: number;
  name?: string;
  start: number;
  sectors: number;
  fsType?: string;
  label?: string;
  role?: string;
  mounted: boolean;
  usedBytes: number;
  resizable: boolean;
}

/** Distribución de un disco. */
export interface Layout {
  disk: string;
  table: string;
  totalSectors: number;
  limit: number;
  segments: Segment[];
  locked?: string;
}

/** Disco de /api/partitions. */
export interface PartDisk {
  name: string;
  model: string;
  vendor: string;
  transport: string;
  role: string;
  sizeBytes: number;
  layout: Layout;
}

/** Operación que se pide al gestor. */
export interface Op {
  kind: "resize" | "boundary" | "merge" | "delete" | "create";
  disk: string;
  part: number;
  other: number;
  sizeMiB: number;
  startMiB: number;
  fs: string;
  label: string;
}

/** Paso del plan. */
export interface PlanStep {
  kind: string;
  part: number;
  length?: number;
  fs?: string;
}

/** Plan de una operación, con el texto que hay que escribir para aplicarla. */
export interface Plan {
  steps: PlanStep[];
  destructive: boolean;
  movesBytes: number;
  affected: number[];
  confirm: string;
}

/** Estado de la operación en curso. */
export interface PartStatus {
  running: boolean;
  done: boolean;
  error?: string;
  disk: string;
  step: number;
  total: number;
  kind: string;
  part: number;
  progress: number;
  seconds: number;
}

/** Operación vacía con valores por defecto. */
export function newOp(kind: Op["kind"], disk: string): Op {
  return { kind, disk, part: 0, other: 0, sizeMiB: 0, startMiB: 0, fs: "", label: "" };
}

/** Tamaño en MiB de un segmento. */
export function mib(segment: Segment): number {
  return Math.floor(segment.sectors / SECTORS_PER_MIB);
}

/** Bytes de un número de MiB. */
export function mibBytes(value: number): number {
  return value * 1048576;
}

/** Hueco libre que empieza justo donde acaba una partición (0 si no hay). */
export function freeAfter(layout: Layout, segment: Segment): number {
  for (const other of layout.segments) {
    if (other.number === 0 && other.start === segment.start + segment.sectors) {
      return Math.floor(other.sectors / SECTORS_PER_MIB);
    }
  }
  return 0;
}

/** Vecina pegada (sin hueco) a la izquierda o a la derecha, si es una partición. */
export function neighbour(
  layout: Layout,
  segment: Segment,
  side: "left" | "right",
): Segment | null {
  for (const other of layout.segments) {
    const adjacent =
      side === "right"
        ? other.start === segment.start + segment.sectors
        : other.start + other.sectors === segment.start;
    if (adjacent && other.number > 0) {
      return other;
    }
  }
  return null;
}

/** Mínimo (MiB, redondeado hacia arriba) al que se puede encoger una partición, o 0 si no se sabe. */
export async function minimumMiB(disk: string, part: number): Promise<number> {
  const info = await getJSON<{ minBytes: number }>(
    "/api/partitions/info?disk=" + encodeURIComponent(disk) + "&part=" + part,
  );
  return Math.ceil(info.minBytes / 1048576);
}

/** Pide la vista previa de una operación. */
export function planOp(op: Op): Promise<Plan> {
  return postJSON<Plan>("/api/partitions/plan", { op });
}

/** Aplica una operación con el texto de confirmación escrito. */
export function applyOp(op: Op, confirm: string): Promise<unknown> {
  return postJSON("/api/partitions/apply", { op, confirm });
}

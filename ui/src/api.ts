// * Cliente de la API local de rebootera-core. Todas las peticiones van al
// * mismo origen con la cookie de sesión; las que modifican algo añaden la
// * cabecera anti-CSRF que exige el servidor. Los errores se traducen aquí
// * según el código HTTP para que la interfaz los muestre en su idioma.

import { t } from "./i18n.js";

/** Interfaz de red tal como la describe /api/system. */
export interface NetIf {
  name: string;
  kind: "ethernet" | "wifi";
  state: string;
  mac: string;
  addresses: string[];
  speedMbps: number;
  rxBytes: number;
  txBytes: number;
}

/** Resumen del equipo devuelto por /api/system. */
export interface SystemInfo {
  vendor: string;
  model: string;
  biosVendor: string;
  biosVersion: string;
  firmware: "uefi" | "bios";
  secureBoot: "on" | "off" | "unknown" | "n/a";
  tpm: string;
  cpu: string;
  cpuThreads: number;
  memoryBytes: number;
  kernel: string;
  hostname: string;
  uptimeSeconds: number;
  time: string;
  interfaces: NetIf[];
  isVirtual: boolean;
}

/** Partición de /api/storage. */
export interface Partition {
  name: string;
  path: string;
  number: number;
  sizeBytes: number;
  fsType: string;
  label: string;
  role: string;
  mountPoint: string;
  usedBytes: number;
  freeBytes: number;
}

/** Disco de /api/storage. */
export interface Disk {
  name: string;
  path: string;
  model: string;
  vendor: string;
  sizeBytes: number;
  transport: string;
  removable: boolean;
  rotational: boolean;
  readOnly: boolean;
  role: "internal" | "removable" | "optical" | "live-media";
  protected: boolean;
  partTable: string;
  partitions: Partition[];
}

/** Dispositivo de /api/hardware. */
export interface Device {
  bus: "pci" | "usb";
  address: string;
  vendorId: string;
  deviceId: string;
  vendor: string;
  name: string;
  category: string;
  className: string;
  driver: string;
  hardwareIds: string[];
  critical: boolean;
}

/** Lectura de temperatura de /api/sensors. */
export interface Reading {
  chip: string;
  label: string;
  kind: "cpu" | "storage" | "gpu" | "other";
  celsius: number;
}

/** Error de la API con un mensaje ya traducido. */
export class ApiError extends Error {
  status: number;
  code: string;

  /** Crea el error con su código HTTP (0 si no hubo respuesta) y su código propio. */
  constructor(status: number, message: string, code = "") {
    super(message);
    this.status = status;
    this.code = code;
  }
}

/** Construye el error de una respuesta, traduciendo su código si lo trae. */
async function errorFrom(response: Response): Promise<ApiError> {
  let code = "";
  try {
    const body = (await response.json()) as { code?: string };
    code = body.code ?? "";
  } catch {
    // * Respuesta sin JSON: solo se usa el código HTTP.
  }
  const key = "errors.codes." + code;
  const translated = code ? t(key) : "";
  const message = translated && translated !== key ? translated : messageFor(response.status);
  return new ApiError(response.status, message, code);
}

/** Traduce un código HTTP a un mensaje comprensible. */
function messageFor(status: number): string {
  switch (status) {
    case 0:
      return t("errors.service");
    case 401:
      return t("errors.session");
    case 503:
      return t("errors.agent");
    default:
      return t("errors.generic", { code: status });
  }
}

/** Hace un GET y devuelve el JSON. */
export async function getJSON<T>(path: string): Promise<T> {
  let response: Response;
  try {
    response = await fetch(path, { credentials: "same-origin" });
  } catch {
    throw new ApiError(0, messageFor(0));
  }
  if (!response.ok) {
    throw await errorFrom(response);
  }
  return (await response.json()) as T;
}

/** Hace un POST con cuerpo JSON y devuelve el JSON de la respuesta. */
export async function postJSON<T>(path: string, body: unknown): Promise<T> {
  let response: Response;
  try {
    response = await fetch(path, {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json", "X-Rebootera": "1" },
      body: JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, messageFor(0));
  }
  if (!response.ok) {
    throw await errorFrom(response);
  }
  return (await response.json()) as T;
}

/** Estado del Vault de /api/vault. */
export interface VaultStatus {
  available: boolean;
  usedBytes: number;
  freeBytes: number;
  nextBoot: string;
}

/** Imagen de la biblioteca de /api/library. */
export interface LibraryItem {
  path: string;
  name: string;
  category: "windows" | "linux" | "tools" | "other";
  kind: string;
  sizeBytes: number;
  modified: string;
  sha256: string;
  expected?: string;
  state: "unchecked" | "hashing" | "verified" | "imported" | "mismatch";
  source: string;
  progress: number;
  bootStatus?: "verified" | "incompatible" | "untested";
}

/** Entrada del catálogo de /api/catalog. */
export interface CatalogEntry {
  id: string;
  name: string;
  vendor: string;
  version: string;
  arch: string;
  category: "windows" | "linux" | "tools";
  homepage: string;
  resolver: { type: string; signature?: { type: string }; isoSignature?: { type: string } };
  secureBoot: "signed" | "unsigned" | "unknown";
  notes: Record<string, string>;
  inLibrary: boolean;
  boot: {
    status: "verified" | "incompatible" | "untested";
    tested: string;
    notes: Record<string, string>;
  };
}

/** Transferencia de /api/transfers. */
export interface Job {
  id: string;
  kind: "download" | "import";
  name: string;
  catalogId?: string;
  fileName: string;
  category: string;
  totalBytes: number;
  doneBytes: number;
  speedBps: number;
  status: "queued" | "running" | "verifying" | "paused" | "done" | "failed" | "cancelled";
  error?: string;
  errorCode?: string;
}

/** Elemento de una carpeta de un medio externo. */
export interface BrowseEntry {
  name: string;
  path: string;
  dir: boolean;
  sizeBytes: number;
}

/** Paquete de drivers de Windows de /api/drivers/plan. */
export interface DriverPackage {
  dir: string;
  inf: string;
  provider: string;
  class: string;
  version: string;
  date: string;
  catalog: string;
  signature: "present" | "missing" | "invalid";
}

/** Dispositivo con su paquete de drivers (o sin él). */
export interface DriverMatch {
  address: string;
  name: string;
  vendor: string;
  category: string;
  critical: boolean;
  hardwareId: string;
  specific: boolean;
  description: string;
  package: DriverPackage | null;
}

/** Casado de los drivers del Vault con el hardware. */
export interface DriverPlan {
  available: boolean;
  packages: number;
  matches: DriverMatch[];
  missing: DriverMatch[];
}

/** Estado del adaptador Wi-Fi de /api/wifi. */
export interface WifiStatus {
  adapter: boolean;
  device: string;
  state: string;
  connection: string;
  enabled: boolean;
}

/** Red Wi-Fi de /api/wifi/scan. */
export interface WifiNetwork {
  ssid: string;
  signal: number;
  security: "open" | "wpa" | "wpa3" | "enterprise" | "unsupported";
  inUse: boolean;
}

/** Respuestas automáticas del instalador (idioma, zona horaria y cuenta). */
export interface Unattended {
  language: string;
  timeZone: string;
  userName: string;
  fullName: string;
  password: string;
}

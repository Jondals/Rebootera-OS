// * Página "Este equipo": todo lo que se sabe del ordenador en un solo sitio, con datos reales leídos al arrancar
// * (nada inventado): placa y firmware, procesador, memoria, gráficos, discos, red, resto de dispositivos con su
// * driver y temperaturas. Sustituye al antiguo registro del kernel, que solo servía a quien sabe leerlo.

import type { Device, Disk, NetIf, Reading, SystemInfo } from "../api.js";
import { badge, button, emptyState, pageHeader, panel } from "../components.js";
import { el } from "../dom.js";
import * as fmt from "../format.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";
import type { Page } from "../router.js";
import { scanHardware, snapshot } from "../state.js";

let body: HTMLElement | null = null;

/** Una fila "nombre: valor" (se omite si no hay valor). */
function row(name: string, value: string | number | undefined): Node[] {
  if (value === undefined || value === "" || value === 0) {
    return [];
  }
  return [el("dt", {}, [name]), el("dd", {}, [String(value)])];
}

/** Lista de pares nombre y valor. */
function facts(rows: Node[][]): HTMLElement {
  return el("dl", { class: "about-list" }, rows.flat());
}

/** Texto de Secure Boot. */
function secureBootText(system: SystemInfo): string {
  switch (system.secureBoot) {
    case "on":
      return t("topbar.secureBootOn");
    case "off":
      return t("topbar.secureBootOff");
    case "n/a":
      return t("topbar.legacy");
    default:
      return t("topbar.secureBootUnknown");
  }
}

/** Panel de la placa, el firmware y el sistema que arrancó. */
function machinePanel(system: SystemInfo): HTMLElement {
  return panel(
    t("computer.machine"),
    [
      facts([
        row(t("computer.manufacturer"), system.vendor),
        row(t("computer.model"), system.model),
        row(t("computer.bios"), [system.biosVendor, system.biosVersion].filter(Boolean).join(" ")),
        row(t("computer.boot"), system.firmware === "uefi" ? t("home.uefi") : t("home.bios")),
        row(t("computer.secureBoot"), secureBootText(system)),
        row("TPM", system.tpm || t("overview.noTpm")),
        row(t("computer.hostname"), system.hostname),
        row(t("settings.kernel"), system.kernel),
        row(t("settings.uptime"), fmt.duration(system.uptimeSeconds)),
        row(t("computer.virtual"), system.isVirtual ? t("computer.yes") : ""),
      ]),
    ],
    { iconPaths: icons.monitor },
  );
}

/** Panel del procesador, la memoria y las tarjetas gráficas. */
function cpuPanel(system: SystemInfo, sensors: Reading[], devices: Device[]): HTMLElement {
  const cards = devices
    .filter(function isDisplay(device) {
      return device.category === "display";
    })
    .map(function name(device) {
      return device.name;
    });
  const cpuTemps = sensors.filter(function isCpu(reading) {
    return reading.kind === "cpu";
  });
  const hottest = cpuTemps.length > 0 ? Math.max(...cpuTemps.map(celsius)) : 0;
  return panel(
    t("computer.processor"),
    [
      facts([
        row(t("computer.cpu"), system.cpu),
        row(t("computer.threads"), system.cpuThreads),
        row(t("computer.memory"), fmt.bytes(system.memoryBytes)),
        row(t("computer.graphics"), cards.join(" · ")),
        row(t("computer.temperature"), hottest > 0 ? Math.round(hottest) + " °C" : ""),
      ]),
    ],
    { iconPaths: icons.cpu },
  );
}

/** Temperatura de una lectura. */
function celsius(reading: Reading): number {
  return reading.celsius;
}

/** Una tarjeta pequeña con título, detalle y una insignia opcional. */
function item(title: string, detail: string, tag: HTMLElement | null): HTMLElement {
  return el("div", { class: "computer-item" }, [
    el("div", { class: "computer-item-text" }, [
      el("strong", {}, [title]),
      el("span", { class: "muted" }, [detail]),
    ]),
    tag,
  ]);
}

/** Panel de los discos. */
function disksPanel(disks: Disk[]): HTMLElement {
  const own = disks.filter(function isNotOptical(disk) {
    return disk.role !== "optical";
  });
  return panel(
    t("computer.storage"),
    own.length > 0
      ? own.map(function card(disk) {
          const kind = disk.rotational ? "HDD" : disk.transport === "nvme" ? "NVMe" : "SSD";
          const where =
            disk.role === "live-media"
              ? t("overview.liveMedia")
              : disk.role === "removable"
                ? t("computer.removable")
                : t("computer.internal");
          return item(
            disk.model || disk.name,
            [
              fmt.disk(disk.sizeBytes),
              kind,
              disk.partTable.toUpperCase(),
              t("overview.partitions", { n: disk.partitions.length }),
            ]
              .filter(Boolean)
              .join(" · "),
            badge(where, disk.role === "internal" ? "muted" : "accent"),
          );
        })
      : [el("p", { class: "muted" }, [t("overview.noInternal")])],
    { iconPaths: icons.disk },
  );
}

/** Texto de una interfaz de red. */
function interfaceDetail(iface: NetIf): string {
  const parts = [iface.mac, ...iface.addresses];
  if (iface.speedMbps > 0) {
    parts.push(iface.speedMbps + " Mb/s");
  }
  return parts.filter(Boolean).join(" · ");
}

/** Panel de la red: interfaces con su estado y los adaptadores con el driver que usan. */
function networkPanel(system: SystemInfo, devices: Device[]): HTMLElement {
  const adapters = devices.filter(function isNetwork(device) {
    return device.category === "network" || device.category === "wireless";
  });
  const parts: HTMLElement[] = system.interfaces.map(function card(iface) {
    const up = iface.state === "up" && iface.addresses.length > 0;
    return item(
      iface.name + " · " + (iface.kind === "wifi" ? "Wi-Fi" : "Ethernet"),
      interfaceDetail(iface),
      badge(up ? t("home.online") : t("home.offline"), up ? "ok" : "muted"),
    );
  });
  for (const device of adapters) {
    parts.push(
      item(
        device.name,
        [device.vendor, t("computer.driver") + ": " + (device.driver || t("computer.noDriver"))]
          .filter(Boolean)
          .join(" · "),
        device.driver ? null : badge(t("computer.noDriver"), "danger"),
      ),
    );
  }
  return panel(
    t("computer.network"),
    parts.length > 0 ? parts : [el("p", { class: "muted" }, [t("computer.none")])],
    { iconPaths: icons.ethernet },
  );
}

/** Vuelve a analizar el hardware. */
async function rescan(): Promise<void> {
  await scanHardware();
}

/** Dibuja la página. */
function render(root: HTMLElement): void {
  body = el("div", { class: "computer-grid" });
  root.append(
    el("div", { class: "page" }, [
      pageHeader(t("computer.title"), t("computer.subtitle"), [
        button(t("common.rescan"), rescan, { iconPaths: icons.refresh }),
      ]),
      body,
    ]),
  );
}

/** Refresca los datos con lo último leído. */
function update(): void {
  if (body === null || !body.isConnected) {
    return;
  }
  const system = snapshot.system;
  if (system === null) {
    body.replaceChildren(emptyState(icons.info, t("computer.title"), t("computer.loading")));
    return;
  }
  const devices = snapshot.devices ?? [];
  body.replaceChildren(
    machinePanel(system),
    cpuPanel(system, snapshot.sensors ?? [], devices),
    disksPanel(snapshot.disks ?? []),
    networkPanel(system, devices),
  );
}

export const computerPage: Page = { render, update };

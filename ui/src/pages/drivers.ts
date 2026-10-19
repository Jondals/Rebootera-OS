// * Página "Asistente de drivers": inventario real de dispositivos PCI y USB
// * con el driver que usa Linux y sus Hardware ID de Windows. Filtra por
// * dispositivos necesarios para instalar y por dispositivos sin driver.
// * También casa esos Hardware ID con los paquetes INF de la carpeta drivers/
// * del USB de Rebootera (los que el asistente puede llevar al instalador de Windows).

import { getJSON, type Device, type DriverMatch, type DriverPlan } from "../api.js";
import { badge, button, emptyState, more, pageHeader } from "../components.js";
import { el, icon } from "../dom.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";
import type { Page } from "../router.js";
import { scanHardware, snapshot } from "../state.js";

/** Filtros disponibles. */
type Filter = "all" | "critical" | "missing";

/** Icono de cada categoría. */
const CATEGORY_ICONS: Record<string, string> = {
  storage: icons.disk,
  network: icons.ethernet,
  wireless: icons.wifi,
  display: icons.monitor,
  audio: icons.activity,
  bluetooth: icons.sparkle,
  "usb-controller": icons.usb,
  input: icons.overview,
  bridge: icons.cpu,
  other: icons.drivers,
};

let filter: Filter = "all";
let list: HTMLElement | null = null;
let tabs: HTMLElement | null = null;
let usbBox: HTMLElement | null = null;
let plan: DriverPlan | null = null;

/** Pide el casado de drivers del USB con este equipo. */
async function loadPlan(): Promise<void> {
  try {
    plan = await getJSON<DriverPlan>("/api/drivers/plan");
  } catch {
    plan = null;
  }
  drawUsb();
}

/** Fila de un dispositivo con su paquete de drivers. */
function packageRow(match: DriverMatch): HTMLElement {
  const pack = match.package;
  return el("article", { class: "driver-match" }, [
    el("div", {}, [
      el("strong", {}, [match.name || match.vendor || match.address]),
      el("div", { class: "mono-small muted" }, [
        pack ? pack.inf + " · " + pack.provider + " " + pack.version + " · " + pack.date : "",
      ]),
      el("div", { class: "mono-small muted" }, [
        t("drivers.usbMatchedBy", { id: match.hardwareId }),
      ]),
    ]),
    pack
      ? badge(
          t("wizard.driversSig" + pack.signature.charAt(0).toUpperCase() + pack.signature.slice(1)),
          pack.signature === "present" ? "ok" : pack.signature === "invalid" ? "danger" : "warn",
        )
      : null,
  ]);
}

/** Fila de un dispositivo crítico sin paquete. */
function missingPackageRow(match: DriverMatch): HTMLElement {
  return el("article", { class: "driver-match" }, [
    el("span", {}, [match.name || match.vendor || match.address]),
    badge(t("drivers.noDriver"), "warn"),
  ]);
}

/** Dibuja el panel de drivers de Windows del USB. */
function drawUsb(): void {
  if (usbBox === null || !usbBox.isConnected) {
    return;
  }
  const body: (Node | null)[] = [];
  if (plan === null) {
    body.push(el("p", { class: "muted" }, [t("common.loading")]));
  } else if (!plan.available) {
    body.push(el("p", { class: "muted" }, [t("drivers.usbNoVault")]));
  } else {
    body.push(
      el("p", { class: "muted small-note" }, [t("drivers.usbPackages", { n: plan.packages })]),
    );
    if (plan.matches.length > 0) {
      body.push(...plan.matches.map(packageRow));
    } else {
      body.push(el("p", { class: "muted" }, [t("drivers.usbNone")]));
    }
    if (plan.missing.length > 0) {
      body.push(
        el("strong", {}, [t("drivers.usbMissing")]),
        ...plan.missing.map(missingPackageRow),
      );
    }
  }
  usbBox.replaceChildren(more(t("drivers.usbTitle"), body));
}

/** Vuelve a analizar el hardware. */
function rescan(): void {
  scanHardware();
  loadPlan();
}

/** Cambia el filtro activo según el botón pulsado. */
function selectFilter(event: MouseEvent): void {
  filter = (event.currentTarget as HTMLElement).dataset.filter as Filter;
  update();
}

/** Indica si un dispositivo pasa el filtro activo. */
function matches(device: Device): boolean {
  if (filter === "critical") {
    return device.critical;
  }
  if (filter === "missing") {
    return device.driver === "";
  }
  return true;
}

/** Dibuja la estructura fija de la página. */
function render(root: HTMLElement): void {
  tabs = el("div", { class: "segmented", role: "tablist" });
  list = el("div", { class: "device-list" });
  usbBox = el("div", {});
  plan = null;
  loadPlan();
  root.append(
    el("div", { class: "page" }, [
      pageHeader(t("drivers.title"), t("drivers.subtitle"), [
        button(t("common.rescan"), rescan, { iconPaths: icons.refresh }),
      ]),
      usbBox,
      tabs,
      list,
    ]),
  );
}

/** Fila de un dispositivo. */
function deviceRow(device: Device): HTMLElement {
  const title = device.name || device.className || t("common.unknown");
  return el("article", { class: "device-row" + (device.critical ? " device-critical" : "") }, [
    el("span", { class: "device-icon" }, [icon(CATEGORY_ICONS[device.category] ?? icons.drivers)]),
    el("div", { class: "device-main" }, [
      el("strong", {}, [title]),
      // * Los identificadores de hardware ya no se enseñan: salen al pasar el ratón.
      el("span", { class: "muted", title: device.hardwareIds.join("\n") }, [
        t("categories." + device.category) + (device.vendor ? " · " + device.vendor : ""),
      ]),
    ]),
    device.critical ? badge(t("drivers.critical"), "amber") : null,
    device.driver ? badge(t("drivers.hasDriver"), "ok") : badge(t("drivers.noDriver"), "warn"),
  ]);
}

/** Refresca los filtros y la lista. */
function update(): void {
  if (list === null || tabs === null || !list.isConnected) {
    return;
  }
  const devices = snapshot.devices;
  if (devices === null) {
    list.replaceChildren(el("p", { class: "muted" }, [t("common.loading")]));
    return;
  }
  const counts: Record<Filter, number> = {
    all: devices.length,
    critical: devices.filter(isCritical).length,
    missing: devices.filter(isMissing).length,
  };
  const filters: [Filter, string][] = [
    ["all", t("drivers.filterAll")],
    ["critical", t("drivers.filterCritical")],
    ["missing", t("drivers.filterMissing")],
  ];
  tabs.replaceChildren(
    ...filters.map(function tab([name, label]) {
      const node = el(
        "button",
        {
          class: "segment-button",
          type: "button",
          role: "tab",
          "data-filter": name,
          "aria-selected": String(filter === name),
        },
        [label, el("span", { class: "segment-count" }, [String(counts[name])])],
      );
      node.addEventListener("click", selectFilter);
      return node;
    }),
  );
  const visible = devices.filter(matches);
  list.replaceChildren(
    ...(visible.length > 0
      ? visible.map(deviceRow)
      : [emptyState(icons.check, t("drivers.empty"), "")]),
  );
}

/** Indica si un dispositivo es crítico. */
function isCritical(device: Device): boolean {
  return device.critical;
}

/** Indica si un dispositivo no tiene driver. */
function isMissing(device: Device): boolean {
  return device.driver === "";
}

export const driversPage: Page = { render, update };

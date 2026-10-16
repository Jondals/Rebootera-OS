// * Página "Asistente de drivers": inventario real de dispositivos PCI y USB
// * con el driver que usa Linux y sus Hardware ID de Windows. Filtra por
// * dispositivos necesarios para instalar y por dispositivos sin driver.
// TODO: casar los Hardware ID con paquetes INF del Vault (hito 5).

import type { Device } from "../api.js";
import { badge, button, emptyState, pageHeader } from "../components.js";
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

/** Vuelve a analizar el hardware. */
function rescan(): void {
  scanHardware();
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
  root.append(
    el("div", { class: "page" }, [
      pageHeader(t("drivers.title"), t("drivers.subtitle"), [
        button(t("common.rescan"), rescan, { iconPaths: icons.refresh }),
      ]),
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
      el("span", { class: "muted" }, [
        [device.vendor, device.bus.toUpperCase() + " " + device.address]
          .filter(Boolean)
          .join(" · "),
      ]),
    ]),
    el("div", { class: "device-meta" }, [
      el("span", { class: "device-category" }, [t("categories." + device.category)]),
      device.critical ? badge(t("drivers.critical"), "amber") : null,
    ]),
    el("div", { class: "device-driver" }, [
      device.driver ? badge(device.driver, "ok") : badge(t("drivers.noDriver"), "warn"),
    ]),
    // * Se muestra el Hardware ID genérico (el último); el completo va en el título.
    el("code", { class: "device-id", title: device.hardwareIds.join("\n") }, [
      device.hardwareIds[device.hardwareIds.length - 1] ?? "",
    ]),
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

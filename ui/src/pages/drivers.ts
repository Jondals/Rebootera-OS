// * Página "Drivers" de Rebootera OS: una respuesta clara arriba ("todo listo" o "faltan N") y debajo lo que
// * importa para instalar (red y discos), cada pieza con su estado: Listo (hay driver para Windows en el USB, en una
// * partición del equipo o en un Windows instalado) o Falta (con ayuda para conseguirlo). El resto de dispositivos y
// * de dónde se buscan los drivers quedan plegados.

import {
  getJSON,
  postJSON,
  type Device,
  type DriverMatch,
  type DriverPlan,
  type DriverSource,
} from "../api.js";
import { badge, button, emptyState, more, pageHeader, panel } from "../components.js";
import { browserAvailable, openBrowser } from "../bridge.js";
import { el, icon } from "../dom.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";
import { onlineInterface } from "../insights.js";
import type { Page } from "../router.js";
import { scanHardware, snapshot } from "../state.js";
import { driverLinks } from "./driverlinks.js";

/** Estado de un dispositivo respecto a los drivers de Windows. */
type DeviceState = "ready" | "missing" | "fine";

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

let body: HTMLElement | null = null;
let plan: DriverPlan | null = null;
let searching = false;

/** Pide el casado de drivers del USB con este equipo. */
async function loadPlan(): Promise<void> {
  try {
    plan = await getJSON<DriverPlan>("/api/drivers/plan");
  } catch {
    plan = null;
  }
  update();
}

/** Vuelve a mirar el hardware y a buscar drivers en las particiones del equipo. */
async function searchAgain(): Promise<void> {
  searching = true;
  update();
  scanHardware();
  try {
    await postJSON("/api/drivers/scan", {});
  } catch {
    // * Si la búsqueda falla, se enseña lo que ya había.
  }
  searching = false;
  await loadPlan();
}

/** Nombre legible de un origen de drivers. */
function sourceName(source: DriverSource): string {
  if (source.kind === "usb") {
    return t("drivers.sourceUsb");
  }
  const where = (source.device ?? "") + (source.folder ? " / " + source.folder : "");
  return t(source.kind === "windows" ? "drivers.sourceWindows" : "drivers.sourcePartition", {
    where,
  });
}

/** Origen de un paquete, por su identificador. */
function sourceOf(id: string): DriverSource | undefined {
  return plan?.sources.find(function same(source) {
    return source.id === id;
  });
}

/** Paquete casado con un dispositivo, si lo hay. */
function matchOf(device: Device): DriverMatch | undefined {
  return plan?.matches.find(function same(match) {
    return match.address === device.address;
  });
}

/** Estado de un dispositivo: listo (hay paquete), falta (importante y sin paquete) o sin problema. */
function stateOf(device: Device): DeviceState {
  if (matchOf(device)?.package) {
    return "ready";
  }
  const listedMissing = plan?.missing.some(function same(match) {
    return match.address === device.address;
  });
  if (listedMissing || (device.critical && device.driver === "")) {
    return "missing";
  }
  return "fine";
}

/** Nombre legible de un dispositivo. */
function nameOf(device: Device): string {
  return device.name || device.className || t("common.unknown");
}

/**
 * Botones para conseguir el driver de un dispositivo en las webs oficiales, abiertas en el navegador de Rebootera: el
 * fabricante del chip, el soporte del fabricante del equipo (con su modelo) y el Catálogo de Microsoft.
 */
function webButtons(device: Device): HTMLElement {
  const buttons = driverLinks(device, snapshot.system).map(function linkButton(link) {
    const node = button(
      t(link.labelKey, { maker: link.maker }),
      function open(): void {
        openBrowser(link.url);
      },
      { iconPaths: icons.globe },
    );
    node.classList.add("btn-small");
    node.title = link.url;
    return node;
  });
  return el("div", { class: "driver-links" }, buttons);
}

/** Indica si se puede ofrecer buscar en la web (navegador de la sesión y red). */
function canSearchWeb(): boolean {
  return (
    browserAvailable() && snapshot.system !== null && onlineInterface(snapshot.system) !== null
  );
}

/** Fila de un dispositivo importante, con su estado y qué hacer. */
function importantRow(device: Device): HTMLElement {
  const state = stateOf(device);
  const found = matchOf(device)?.package;
  const source = found ? sourceOf(found.sourceId) : undefined;
  let note = "";
  if (state === "ready" && source) {
    note = t("drivers.readyFrom", { source: sourceName(source) });
  } else if (state === "missing") {
    note = t("drivers.missingHint");
  } else {
    note = t("drivers.fineHint");
  }
  const search = state === "missing" && canSearchWeb() ? webButtons(device) : null;
  return el("article", { class: "device-row device-" + state }, [
    el("span", { class: "device-icon" }, [icon(CATEGORY_ICONS[device.category] ?? icons.drivers)]),
    el("div", { class: "device-main" }, [
      el("strong", {}, [nameOf(device)]),
      // * Los identificadores de hardware salen al pasar el ratón: casi nunca hacen falta.
      el("span", { class: "muted", title: device.hardwareIds.join("\n") }, [
        t("categories." + device.category) + (device.vendor ? " · " + device.vendor : ""),
        " — ",
        note,
      ]),
    ]),
    search,
    badge(
      t("drivers.state." + state),
      state === "ready" ? "ok" : state === "missing" ? "warn" : "muted",
    ),
  ]);
}

/** Fila sencilla de un dispositivo que no hace falta para instalar. */
function otherRow(device: Device): HTMLElement {
  return el("article", { class: "device-row" }, [
    el("span", { class: "device-icon" }, [icon(CATEGORY_ICONS[device.category] ?? icons.drivers)]),
    el("div", { class: "device-main" }, [
      el("strong", {}, [nameOf(device)]),
      el("span", { class: "muted", title: device.hardwareIds.join("\n") }, [
        t("categories." + device.category) + (device.vendor ? " · " + device.vendor : ""),
      ]),
    ]),
    badge(
      device.driver ? t("drivers.hasDriver") : t("drivers.optional"),
      device.driver ? "ok" : "muted",
    ),
  ]);
}

/** Tarjeta grande con la respuesta: todo listo, o cuántos drivers faltan. */
function statusCard(missing: Device[], ready: number): HTMLElement {
  if (plan === null) {
    return el("div", { class: "driver-hero" }, [
      el("span", { class: "driver-hero-icon" }, [icon(icons.drivers)]),
      el("div", {}, [el("strong", {}, [t("common.loading")])]),
    ]);
  }
  const good = missing.length === 0;
  const names = missing.slice(0, 3).map(nameOf).join(", ");
  const again = button(searching ? t("drivers.searching") : t("drivers.searchAgain"), searchAgain, {
    variant: good ? "ghost" : "primary",
    iconPaths: icons.refresh,
  });
  again.disabled = searching;
  again.classList.toggle("is-spinning", searching);
  return el("div", { class: "driver-hero " + (good ? "driver-hero-ok" : "driver-hero-warn") }, [
    el("span", { class: "driver-hero-icon" }, [icon(good ? icons.check : icons.alert)]),
    el("div", { class: "driver-hero-text" }, [
      el("strong", {}, [
        good ? t("drivers.okTitle") : t("drivers.missTitle", { n: missing.length }),
      ]),
      el("span", {}, [
        good
          ? ready > 0
            ? t("drivers.okReady", { n: ready })
            : t("drivers.okNone")
          : t("drivers.missText", { names }),
      ]),
    ]),
    again,
  ]);
}

/** Pasos para conseguir un driver que falta. */
function howTo(): HTMLElement {
  return el("ol", { class: "boot-steps" }, [
    el("li", {}, [t("drivers.how1")]),
    el("li", {}, [t("drivers.how2")]),
    el("li", {}, [t("drivers.how3")]),
  ]);
}

/** De dónde busca drivers Rebootera, con cuántos paquetes aportó cada sitio. */
function sourcesBlock(): Node[] {
  if (plan === null) {
    return [];
  }
  const rows =
    plan.sources.length > 0
      ? plan.sources.map(function sourceRow(source) {
          const sourceIcon =
            source.kind === "usb"
              ? icons.usb
              : source.kind === "windows"
                ? icons.monitor
                : icons.disk;
          return el("div", { class: "driver-source" }, [
            icon(sourceIcon),
            el("div", { class: "driver-source-text" }, [
              el("span", {}, [sourceName(source)]),
              source.error
                ? el("small", { class: "muted" }, [t("drivers.sourceError", { why: source.error })])
                : source.packages === 0
                  ? el("small", { class: "muted" }, [t("drivers.sourceEmpty")])
                  : null,
            ]),
            source.error
              ? badge(t("drivers.sourceNotOpened"), "warn")
              : badge(t("drivers.sourcePackages", { n: source.packages }), "muted"),
          ]);
        })
      : [el("p", { class: "muted" }, [t("drivers.noSources")])];
  return [el("p", { class: "muted small-note" }, [t("drivers.autoText")]), ...rows];
}

/** Dibuja la estructura fija de la página. */
function render(root: HTMLElement): void {
  body = el("div", { class: "drivers-body" });
  plan = null;
  void loadPlan();
  root.append(
    el("div", { class: "page" }, [pageHeader(t("drivers.title"), t("drivers.subtitle")), body]),
  );
}

/** Refresca la página con los datos actuales. */
function update(): void {
  if (body === null || !body.isConnected) {
    return;
  }
  const devices = snapshot.devices;
  if (devices === null) {
    body.replaceChildren(el("p", { class: "muted" }, [t("common.loading")]));
    return;
  }
  const important = devices.filter(isCritical);
  const others = devices.filter(isNotCritical);
  const missing = important.filter(isMissing);
  const ready = devices.filter(isReady).length;
  const parts: (Node | null)[] = [statusCard(missing, ready)];
  if (important.length > 0) {
    parts.push(
      panel(t("drivers.importantTitle"), important.map(importantRow), {
        subtitle: t("drivers.importantText"),
        iconPaths: icons.drivers,
      }),
    );
  } else {
    parts.push(emptyState(icons.check, t("drivers.empty"), ""));
  }
  if (missing.length > 0) {
    parts.push(more(t("drivers.howTitle"), [howTo()], "drivers-how"));
  }
  if (others.length > 0) {
    parts.push(
      more(t("drivers.othersTitle", { n: others.length }), others.map(otherRow), "drivers-others"),
    );
  }
  parts.push(more(t("drivers.sourcesTitle"), sourcesBlock(), "drivers-sources"));
  body.replaceChildren(...parts.filter(isNode));
}

/** Indica si un dispositivo es importante para instalar. */
function isCritical(device: Device): boolean {
  return device.critical;
}

/** Indica si un dispositivo no hace falta para instalar. */
function isNotCritical(device: Device): boolean {
  return !device.critical;
}

/** Indica si un dispositivo importante no tiene driver para Windows. */
function isMissing(device: Device): boolean {
  return stateOf(device) === "missing";
}

/** Indica si hay un paquete de drivers para el dispositivo. */
function isReady(device: Device): boolean {
  return stateOf(device) === "ready";
}

/** Filtro de tipo para quitar los huecos de la lista. */
function isNode(node: Node | null): node is Node {
  return node !== null;
}

export const driversPage: Page = { render, update };

// * "Drivers" de Rebootera Desktop: los drivers que lleva el USB (carpeta drivers/) para instalar Windows en
// * cualquier equipo. La página explica en tres pasos cómo funciona, deja añadir la carpeta que se bajó del
// * fabricante y enseña lo que hay, agrupado por carpeta y con una pista clara si algún paquete no tiene firma.
// ! Los paquetes solo se leen: nunca se ejecuta nada de lo que contienen.

import { getJSON, postJSON } from "../api.js";
import {
  badge,
  button,
  emptyState,
  errorBox,
  more,
  pageHeader,
  panel,
  type Tone,
} from "../components.js";
import { confirmAction } from "../dialogs.js";
import { el, icon } from "../dom.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";
import type { Page } from "../router.js";

/** Paquete de drivers de /api/desktop/drivers. */
interface Pack {
  dir: string;
  inf: string;
  provider: string;
  class: string;
  version: string;
  date: string;
  signature: "present" | "missing" | "invalid";
}

/** Respuesta de /api/desktop/drivers. */
interface DriversView {
  available: boolean;
  root: string;
  packages: Pack[];
}

/** Carpeta de primer nivel de drivers/ con sus paquetes. */
interface Group {
  name: string;
  packs: Pack[];
}

/** Clase INF de Windows → clave del tipo en lenguaje normal. */
const CLASS_KINDS: Record<string, string> = {
  net: "network",
  scsiadapter: "storage",
  hdc: "storage",
  diskdrive: "storage",
  display: "display",
  media: "audio",
  bluetooth: "bluetooth",
  usb: "usb",
};

let body: HTMLElement | null = null;
let message: HTMLElement | null = null;
let view: DriversView | null = null;

/** Dibuja la página. */
function render(root: HTMLElement): void {
  body = el("div", { class: "drivers-body" });
  message = el("div", { class: "page-message", "aria-live": "polite" });
  root.append(
    el("div", { class: "page" }, [
      pageHeader(t("nav.drivers"), t("desktop.driversSubtitle")),
      message,
      body,
    ]),
  );
  void load();
}

/** Lee los paquetes del USB. */
async function load(): Promise<void> {
  try {
    view = await getJSON<DriversView>("/api/desktop/drivers");
  } catch (error) {
    message?.replaceChildren(errorBox(error));
    return;
  }
  draw();
}

/** Elige una carpeta de drivers y la copia al USB. */
async function addFolder(): Promise<void> {
  message?.replaceChildren();
  try {
    const picked = await postJSON<{ path: string }>("/api/desktop/pick-folder", {
      title: t("desktop.addDrivers"),
    });
    if (!picked.path) {
      return;
    }
    view = await postJSON<DriversView>("/api/desktop/drivers/add", { path: picked.path });
    message?.replaceChildren(
      el("div", { class: "notice notice-ok" }, [icon(icons.check), t("desktop.driversAdded")]),
    );
    draw();
  } catch (error) {
    message?.replaceChildren(errorBox(error));
  }
}

/** Quita del USB la carpeta de la fila pulsada, tras confirmar. */
async function removeFolder(event: MouseEvent): Promise<void> {
  const name = (event.currentTarget as HTMLElement).dataset.name ?? "";
  if (!(await confirmAction(name, t("desktop.driversRemoveText"), t("desktop.remove")))) {
    return;
  }
  try {
    view = await postJSON<DriversView>("/api/desktop/drivers/remove", { name });
    draw();
  } catch (error) {
    message?.replaceChildren(errorBox(error));
  }
}

/** Agrupa los paquetes por la carpeta de primer nivel de drivers/. */
function groups(packs: Pack[]): Group[] {
  const byName = new Map<string, Pack[]>();
  for (const pack of packs) {
    const name = pack.dir.split("/")[0] || pack.inf.split("/")[0];
    byName.set(name, [...(byName.get(name) ?? []), pack]);
  }
  return [...byName.entries()].map(function toGroup([name, list]) {
    return { name, packs: list };
  });
}

/** Tipo en lenguaje normal de un paquete ("Red", "Almacenamiento"...), o su clase INF si no se conoce. */
function kindOf(pack: Pack): string {
  const kind = CLASS_KINDS[pack.class.toLowerCase()];
  return kind ? t("desktop.kinds." + kind) : pack.class;
}

/** Tipos distintos que aporta una carpeta, para la línea de resumen. */
function kindsOf(packs: Pack[]): string {
  const kinds = new Set<string>();
  for (const pack of packs) {
    if (pack.class) {
      kinds.add(kindOf(pack));
    }
  }
  return [...kinds].slice(0, 4).join(", ");
}

/** Tono y texto de la firma de un paquete. */
function signatureBadge(signature: Pack["signature"]): HTMLElement {
  const tone: Tone = signature === "present" ? "ok" : signature === "invalid" ? "danger" : "warn";
  const key = "wizard.driversSig" + signature.charAt(0).toUpperCase() + signature.slice(1);
  return badge(t(key), tone);
}

/** Paquetes sin firma válida de una lista. */
function unsigned(packs: Pack[]): number {
  return packs.filter(function isUnsigned(pack) {
    return pack.signature !== "present";
  }).length;
}

/** Los tres pasos de cómo se usan los drivers. */
function stepsPanel(): HTMLElement {
  const steps = ["one", "two", "three"].map(function step(key, index) {
    return el("li", { class: "how-step" }, [
      el("span", { class: "how-num" }, [String(index + 1)]),
      el("div", {}, [
        el("strong", {}, [t("desktop.how." + key)]),
        el("span", { class: "muted" }, [t("desktop.how." + key + "Text")]),
      ]),
    ]);
  });
  return el("ol", { class: "how-steps" }, steps);
}

/** Botón grande de añadir carpeta. */
function addButton(): HTMLButtonElement {
  return button(t("desktop.addDrivers"), addFolder, {
    variant: "primary",
    iconPaths: icons.drivers,
  });
}

/** Tarjeta con lo que hay en el USB: cuántos drivers y si alguno no tiene firma. */
function summaryCard(list: Group[], total: number, noSignature: number): HTMLElement {
  return el("div", { class: "driver-hero " + (noSignature > 0 ? "" : "driver-hero-ok") }, [
    el("span", { class: "driver-hero-icon" }, [icon(icons.drivers)]),
    el("div", { class: "driver-hero-text" }, [
      el("strong", {}, [t("desktop.summary", { n: total, folders: list.length })]),
      el("span", {}, [
        noSignature > 0 ? t("desktop.summaryUnsigned", { n: noSignature }) : t("desktop.summaryOk"),
      ]),
    ]),
    addButton(),
  ]);
}

/** Fila de una carpeta de drivers: resumen, quitar y, plegados, sus paquetes. */
function groupPanel(group: Group): HTMLElement {
  const remove = button(t("desktop.remove"), removeFolder, { iconPaths: icons.x });
  remove.classList.add("btn-small");
  remove.dataset.name = group.name;
  const rows = group.packs.map(function packRow(pack) {
    return el("div", { class: "source-row" }, [
      el("span", { class: "device-icon" }, [icon(icons.drivers)]),
      el("div", { class: "device-main" }, [
        el("strong", {}, [pack.inf.split("/").pop() ?? pack.inf]),
        el("span", { class: "muted" }, [
          [pack.provider, kindOf(pack), pack.version].filter(Boolean).join(" · "),
        ]),
      ]),
      signatureBadge(pack.signature),
    ]);
  });
  const bad = unsigned(group.packs);
  return panel(group.name, [more(t("desktop.showDrivers"), rows, "dd-" + group.name)], {
    iconPaths: icons.drivers,
    subtitle:
      t("desktop.groupCount", { n: group.packs.length }) +
      (kindsOf(group.packs) ? " · " + kindsOf(group.packs) : "") +
      (bad > 0 ? " · " + t("desktop.unsignedCount", { n: bad }) : ""),
    action: remove,
  });
}

/** Dibuja la página según haya USB, drivers o ninguna de las dos cosas. */
function draw(): void {
  if (body === null || view === null) {
    return;
  }
  if (!view.available) {
    body.replaceChildren(emptyState(icons.usb, t("desktop.noUsbTitle"), t("desktop.noUsbText")));
    return;
  }
  const list = groups(view.packages);
  const help = more(t("desktop.whichTitle"), [el("p", {}, [t("desktop.whichText")])], "dd-which");
  if (list.length === 0) {
    body.replaceChildren(
      emptyState(
        icons.drivers,
        t("desktop.noDriversTitle"),
        t("desktop.noDriversText"),
        addButton(),
      ),
      panel(t("desktop.howTitle"), [stepsPanel()], { iconPaths: icons.info }),
      help,
    );
    return;
  }
  body.replaceChildren(
    summaryCard(list, view.packages.length, unsigned(view.packages)),
    ...list.map(groupPanel),
    panel(t("desktop.howTitle"), [stepsPanel()], { iconPaths: icons.info }),
    help,
  );
}

export const desktopDriversPage: Page = { render };

// * "Asistente de drivers" de Rebootera Desktop: los paquetes de drivers que
// * lleva el USB de Rebootera (carpeta drivers/). Aquí se añaden carpetas de
// * drivers del equipo, se ven agrupadas con el estado de su firma y se quitan.
// * Rebootera OS los casa luego con el hardware del equipo que arranca y puede
// * llevarlos al instalador de Windows.
// ! Los paquetes solo se leen: nunca se ejecuta nada de lo que contienen.

import { getJSON, postJSON } from "../api.js";
import {
  badge,
  button,
  emptyState,
  errorBox,
  pageHeader,
  panel,
  type Tone,
} from "../components.js";
import { confirmAction } from "../dialogs.js";
import { el, icon } from "../dom.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";
import { navigate, type Page } from "../router.js";

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

let body: HTMLElement | null = null;
let message: HTMLElement | null = null;
let view: DriversView | null = null;

/** Dibuja la página. */
function render(root: HTMLElement): void {
  body = el("div", { class: "drivers-body" });
  message = el("div", { class: "page-message", "aria-live": "polite" });
  const add = button(t("desktop.addDrivers"), addFolder, {
    variant: "primary",
    iconPaths: icons.drivers,
  });
  root.append(
    el("div", { class: "page" }, [
      pageHeader(t("nav.drivers"), t("desktop.driversSubtitle"), [add]),
      message,
      body,
    ]),
  );
  load();
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

/** Abre "Crear USB" para meter drivers junto con otras cosas. */
function openCreate(): void {
  navigate("create");
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

/** Tono y texto de la firma de un paquete. */
function signatureBadge(signature: Pack["signature"]): HTMLElement {
  const tone: Tone = signature === "present" ? "ok" : signature === "invalid" ? "danger" : "warn";
  const key = "wizard.driversSig" + signature.charAt(0).toUpperCase() + signature.slice(1);
  return badge(t(key), tone);
}

/** Panel de una carpeta de drivers. */
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
          [pack.provider, pack.class, pack.version].filter(Boolean).join(" · "),
        ]),
      ]),
      signatureBadge(pack.signature),
    ]);
  });
  return panel(group.name, rows, {
    iconPaths: icons.drivers,
    subtitle: t("desktop.driverCount", { n: group.packs.length }),
    action: remove,
  });
}

/** Dibuja la lista de carpetas. */
function draw(): void {
  if (body === null || view === null) {
    return;
  }
  if (!view.available) {
    body.replaceChildren(emptyState(icons.usb, t("desktop.noUsbTitle"), t("desktop.noUsbText")));
    return;
  }
  const list = groups(view.packages);
  if (list.length === 0) {
    const create = button(t("desktop.nav.create"), openCreate, { iconPaths: icons.sparkle });
    const empty = emptyState(
      icons.drivers,
      t("desktop.noDriversTitle"),
      t("desktop.noDriversText"),
    );
    empty.append(create);
    body.replaceChildren(empty);
    return;
  }
  body.replaceChildren(...list.map(groupPanel));
}

export const desktopDriversPage: Page = { render };

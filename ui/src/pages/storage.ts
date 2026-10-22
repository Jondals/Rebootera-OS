// * Página "Discos": cada disco como una barra visual (una pieza por partición y por hueco libre). Se pulsa una
// * pieza y salen, con palabras sencillas, las cosas que se pueden hacer con ella: hacerla más grande o pequeña,
// * unirla con la vecina, borrarla o, si es un hueco, crear una partición. Cada acción enseña antes cómo
// * quedará el disco y pide confirmación escrita.
// ! Rebootera nunca toca el USB desde el que ha arrancado ni un disco con particiones
// ! montadas; el resto de discos se editan con la confirmación "CHANGE <disco>".

import { getJSON } from "../api.js";
import { button, emptyState, errorBox, pageHeader, progress, spinning } from "../components.js";
import { el, icon } from "../dom.js";
import * as fmt from "../format.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";
import {
  neighbour,
  type Layout,
  type PartDisk,
  type PartStatus,
  type Segment,
} from "../partman.js";
import type { Page } from "../router.js";
import {
  diskBar,
  diskHeader,
  findPiece,
  legend,
  runningPanel,
  sizeOf,
  startedStatus,
  titleOf,
} from "./diskmap.js";
import {
  openCreate,
  openDelete,
  openFormat,
  openMerge,
  openRename,
  openResize,
} from "./partdialogs.js";

/** Cada cuántos ms se vuelve a leer el estado de la operación y, en reposo, los discos. */
const STATUS_MS = 2000;
const RELOAD_EVERY = 8;

let body: HTMLElement | null = null;
let message: HTMLElement | null = null;
let disks: PartDisk[] | null = null;
let selected = "";
let status: PartStatus | null = null;
let timer = 0;
let ticks = 0;
let lastSignature = "";

/** Dibuja la estructura fija de la página y empieza a vigilar. */
function render(root: HTMLElement): void {
  body = el("div", { class: "disk-list" });
  message = el("div", { class: "page-message", "aria-live": "polite" });
  lastSignature = "";
  const refresh = button(t("common.refresh"), spinning(reload), { iconPaths: icons.refresh });
  root.append(
    el("div", { class: "page" }, [
      pageHeader(t("storage.title"), t("storage.subtitle"), [refresh]),
      message,
      body,
    ]),
  );
  window.clearInterval(timer);
  timer = window.setInterval(tick, STATUS_MS);
  // * Se enseña "Cargando…" al momento y, si ya se leyeron los discos antes, esos mismos mientras llegan los nuevos.
  draw();
  reload();
}

/** Lee la distribución de todos los discos. */
async function reload(): Promise<void> {
  try {
    disks = await getJSON<PartDisk[]>("/api/partitions");
    message?.replaceChildren();
  } catch (error) {
    message?.replaceChildren(errorBox(error));
  }
  draw();
}

/** Vigila la operación en curso; en reposo, refresca los discos de vez en cuando. */
async function tick(): Promise<void> {
  if (body === null || !body.isConnected) {
    window.clearInterval(timer);
    return;
  }
  ticks += 1;
  let next: PartStatus | null = null;
  try {
    next = await getJSON<PartStatus>("/api/partitions/status");
  } catch {
    // * Sin respuesta del agente se reintenta en el siguiente ciclo.
  }
  const wasRunning = status?.running === true;
  const changed = JSON.stringify(next) !== JSON.stringify(status);
  status = next;
  if (wasRunning && next && !next.running) {
    // * Terminó: se enseña el resultado y se vuelven a leer los discos.
    await reload();
    return;
  }
  if (changed && next?.running) {
    draw();
  } else if (
    !next?.running &&
    ticks % RELOAD_EVERY === 0 &&
    document.querySelector("dialog.modal") === null
  ) {
    reload();
  }
}

/** Elige una pieza de un disco. */
function select(event: MouseEvent): void {
  selected = (event.currentTarget as HTMLElement).dataset.key ?? "";
  lastSignature = "";
  draw();
}

/** Vuelve a leer los discos tras una operación aceptada: el progreso lo enseña el estado. */
function started(): void {
  status = startedStatus();
  draw();
  tick();
}

/** Tarjeta de acción del panel de una pieza (desactivada, con su motivo, si no se puede). */
function action(
  label: string,
  text: string,
  onClick: () => void,
  allowed: boolean,
  why: string,
  iconPaths: string,
  danger = false,
): HTMLButtonElement {
  const node = el(
    "button",
    { class: "pm-action" + (danger ? " pm-action-danger" : ""), type: "button" },
    [
      el("span", { class: "pm-action-icon" }, [icon(iconPaths)]),
      el("span", { class: "pm-action-text" }, [
        el("strong", {}, [label]),
        el("span", {}, [allowed ? text : why]),
      ]),
    ],
  );
  node.disabled = !allowed;
  node.addEventListener("click", onClick);
  return node;
}

/** Partición pegada por la izquierda a un hueco libre (la que puede crecer hacia él). */
function leftOfFree(layout: Layout, free: Segment): Segment | null {
  return neighbour(layout, free, "left");
}

/** Motivo por el que una partición no se puede cambiar de tamaño ahora. */
function resizeWhy(segment: Segment): string {
  return segment.resizable
    ? t("pm.resizeMounted")
    : t("pm.cannotResize", { fs: segment.fsType || "?" });
}

/** Acciones de un hueco libre: crear una partición o añadírselo a la de al lado. */
function freeActions(layout: Layout, segment: Segment): HTMLElement[] {
  const buttons = [
    action(
      t("pm.create"),
      t("pm.actCreate"),
      function create(): void {
        openCreate(layout, segment, started);
      },
      true,
      "",
      icons.sparkle,
    ),
  ];
  const left = leftOfFree(layout, segment);
  if (left) {
    const usable = left.resizable && !left.mounted;
    buttons.push(
      action(
        t("pm.addTo", { name: titleOf(left) }),
        t("pm.actAddTo"),
        function addTo(): void {
          void openResize(layout, left, started, { grow: true });
        },
        usable,
        resizeWhy(left),
        icons.refresh,
      ),
    );
  }
  return buttons;
}

/** Unir con la vecina está oculto: todavía no es fiable. El diálogo y el plan siguen en el código. */
const MERGE_ENABLED = false;

/** Acciones de una partición: tamaño, nombre, formatear, unir con la vecina (oculto) y borrar. */
function partitionActions(layout: Layout, segment: Segment): HTMLElement[] {
  const left = neighbour(layout, segment, "left");
  const right = neighbour(layout, segment, "right");
  const buttons = [
    action(
      t("pm.resize"),
      t("pm.actResize"),
      function resize(): void {
        void openResize(layout, segment, started);
      },
      segment.resizable && !segment.mounted,
      resizeWhy(segment),
      icons.refresh,
    ),
  ];
  buttons.push(
    action(
      t("pm.rename"),
      t("pm.actRename"),
      function rename(): void {
        openRename(layout, segment, started);
      },
      !segment.mounted &&
        ["ext2", "ext3", "ext4", "ntfs", "exfat", "vfat"].includes(segment.fsType ?? ""),
      segment.mounted ? t("pm.mounted") : t("pm.renameNo"),
      icons.settings,
    ),
  );
  buttons.push(
    action(
      t("pm.format"),
      t("pm.actFormat"),
      function format(): void {
        openFormat(layout, segment, started);
      },
      !segment.mounted,
      t("pm.mounted"),
      icons.refresh,
    ),
  );
  if (MERGE_ENABLED && (left || right)) {
    buttons.push(
      action(
        t("pm.merge"),
        t("pm.actMerge"),
        function merge(): void {
          openMerge(layout, segment, started);
        },
        true,
        "",
        icons.library,
      ),
    );
  }
  buttons.push(
    action(
      t("pm.delete"),
      t("pm.actDelete"),
      function remove(): void {
        openDelete(layout, segment, started);
      },
      true,
      "",
      icons.x,
      true,
    ),
  );
  return buttons;
}

/** Panel de la pieza elegida: qué es, cuánto está lleno y qué se puede hacer con ella. */
function actionsPanel(layout: Layout, segment: Segment): HTMLElement {
  const size = sizeOf(segment);
  const isFree = segment.number === 0;
  const used = !isFree && size > 0 ? Math.min(1, segment.usedBytes / size) : 0;
  const summary: HTMLElement[] = [
    el("strong", { class: "pm-name" }, [titleOf(segment)]),
    el("span", { class: "muted" }, [
      [
        isFree ? t("pm.free") : (segment.fsType || "—").toUpperCase(),
        fmt.disk(size),
        segment.mounted ? t("pm.mounted") : "",
      ]
        .filter(Boolean)
        .join(" · "),
    ]),
  ];
  if (!isFree && segment.usedBytes > 0) {
    summary.push(
      progress(used, used > 0.9 ? "warn" : "accent"),
      el("span", { class: "muted" }, [
        t("pm.usedFree", {
          used: fmt.disk(segment.usedBytes),
          free: fmt.disk(Math.max(0, size - segment.usedBytes)),
        }),
      ]),
    );
  }
  return el("div", { class: "pm-panel" }, [
    el("div", { class: "pm-info" }, summary),
    el("div", { class: "pm-actions" }, [
      ...(isFree ? freeActions(layout, segment) : partitionActions(layout, segment)),
    ]),
  ]);
}

/** Texto del bloqueo de un disco. */
function lockedText(code: string): string {
  return t("pm.locked." + code);
}

/** Tarjeta de un disco. */
function diskCard(disk: PartDisk): HTMLElement {
  const locked = disk.layout.locked ?? "";
  const picked = findPiece([disk], selected)?.segment;
  return el("article", { class: "disk-card" }, [
    diskHeader(disk, [
      locked
        ? el("span", { class: "guard-chip" }, [icon(icons.lock), t("common.protected")])
        : null,
    ]),
    disk.layout.segments.length > 0
      ? el("div", {}, [diskBar(disk, selected, select), legend(disk)])
      : el("p", { class: "muted" }, [t("storage.noTable")]),
    locked
      ? el("p", { class: "muted small-note" }, [lockedText(locked)])
      : picked
        ? actionsPanel(disk.layout, picked)
        : el("p", { class: "muted small-note" }, [t("pm.pick")]),
  ]);
}

/** Dibuja la página entera. */
function draw(): void {
  if (body === null || !body.isConnected) {
    return;
  }
  const finished = status && !status.running && (status.done || status.error);
  const signature = JSON.stringify([disks, selected, status]);
  if (signature === lastSignature) {
    return;
  }
  lastSignature = signature;
  const parts: (Node | null)[] = [];
  if (status?.running) {
    parts.push(runningPanel(status));
  } else if (finished && status?.error) {
    parts.push(el("div", { class: "error-box" }, [t("pm.failed", { error: status.error })]));
  } else if (finished && status?.done) {
    parts.push(el("div", { class: "notice notice-ok" }, [icon(icons.check), t("pm.done")]));
  }
  if (disks === null) {
    parts.push(el("p", { class: "muted" }, [t("common.loading")]));
  } else if (disks.length === 0) {
    parts.push(emptyState(icons.disk, t("storage.noDisks"), ""));
  } else {
    // * Mientras se ejecuta algo no se puede elegir otra cosa: los discos se enseñan sin acciones.
    parts.push(
      ...disks.map(function card(disk) {
        return status?.running ? disabledCard(disk) : diskCard(disk);
      }),
    );
  }
  body.replaceChildren(...parts.filter(isNode));
}

/** Tarjeta de un disco sin acciones (hay una operación en marcha). */
function disabledCard(disk: PartDisk): HTMLElement {
  const card = diskCard({ ...disk, layout: { ...disk.layout, locked: "busy" } });
  card.classList.add("pm-busy");
  return card;
}

/** Filtro de tipo para quitar los huecos de la lista. */
function isNode(node: Node | null): node is Node {
  return node !== null;
}

export const storagePage: Page = { render };

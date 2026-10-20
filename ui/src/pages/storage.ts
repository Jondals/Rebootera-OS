// * Página "Discos y particiones": cada disco como una barra visual (una pieza por
// * partición y por hueco libre). Se elige una pieza y se puede cambiar su tamaño,
// * mover el límite con su vecina, fusionarla, borrarla o crear una partición en un
// * hueco. Cada acción enseña antes cómo quedará el disco y pide confirmación escrita.
// ! Rebootera nunca toca el USB desde el que ha arrancado ni un disco con particiones
// ! montadas; el resto de discos se editan con la confirmación "CHANGE <disco>".

import { getJSON } from "../api.js";
import {
  badge,
  button,
  emptyState,
  errorBox,
  pageHeader,
  panel,
  progress,
  type Tone,
} from "../components.js";
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
import { openCreate, openDelete, openMerge, openResize } from "./partdialogs.js";

/** Tono de cada función de disco. */
const ROLE_TONES: Record<string, Tone> = {
  "live-media": "accent",
  internal: "violet",
  removable: "amber",
  optical: "muted",
};

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

/** Clave de selección de una pieza. */
function keyOf(disk: string, segment: Segment): string {
  return disk + ":" + (segment.number > 0 ? "p" + segment.number : "f" + segment.start);
}

/** Dibuja la estructura fija de la página y empieza a vigilar. */
function render(root: HTMLElement): void {
  body = el("div", { class: "disk-list" });
  message = el("div", { class: "page-message", "aria-live": "polite" });
  lastSignature = "";
  const refresh = button(t("common.refresh"), reload, { iconPaths: icons.refresh });
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

/** Color y clase de una pieza por su sistema de archivos. */
function pieceClass(segment: Segment): string {
  if (segment.number === 0) {
    return "pm-free";
  }
  switch (segment.fsType) {
    case "ntfs":
      return "pm-fs-ntfs";
    case "ext2":
    case "ext3":
    case "ext4":
      return "pm-fs-ext";
    case "exfat":
    case "vfat":
      return "pm-fs-fat";
    case "swap":
      return "pm-fs-swap";
  }
  return "pm-fs-other";
}

/** Nombre que se muestra de una pieza. */
function titleOf(segment: Segment): string {
  if (segment.number === 0) {
    return t("pm.free");
  }
  return segment.label || segment.name || "#" + segment.number;
}

/** Barra del disco: una pieza clicable por partición y por hueco, con cuánto está lleno dentro. */
function diskBar(disk: PartDisk): HTMLElement {
  const bar = el("div", {
    class: "pm-bar",
    role: "group",
    "aria-label": t("pm.barLabel", { disk: disk.name }),
  });
  for (const segment of disk.layout.segments) {
    const key = keyOf(disk.name, segment);
    const size = segment.sectors * 512;
    const used = segment.number > 0 && size > 0 ? Math.min(1, segment.usedBytes / size) : 0;
    const meter = el("i", { class: "pm-use" });
    meter.style.width = Math.round(used * 100) + "%";
    const piece = el(
      "button",
      {
        class: "pm-seg " + pieceClass(segment) + (selected === key ? " pm-selected" : ""),
        type: "button",
        "data-key": key,
        "aria-pressed": String(selected === key),
        title:
          titleOf(segment) +
          " · " +
          fmt.bytes(size) +
          (segment.number > 0 ? " · " + t("pm.usedPercent", { p: Math.round(used * 100) }) : ""),
      },
      [
        el("strong", {}, [titleOf(segment)]),
        el("span", { class: "pm-size-text" }, [fmt.bytes(size)]),
        segment.number > 0 ? el("span", { class: "pm-meter" }, [meter]) : null,
      ],
    );
    piece.style.flexGrow = String(Math.max(segment.sectors, disk.layout.totalSectors * 0.05));
    piece.addEventListener("click", select);
    bar.append(piece);
  }
  return bar;
}

/** Leyenda de colores de la barra: solo los tipos que hay en el disco. */
function legend(disk: PartDisk): HTMLElement {
  const kinds = new Map<string, string>();
  for (const segment of disk.layout.segments) {
    kinds.set(pieceClass(segment), segment.number === 0 ? "free" : pieceClass(segment));
  }
  const names: Record<string, string> = {
    "pm-free": t("pm.legendFree"),
    "pm-fs-ntfs": t("pm.legendNtfs"),
    "pm-fs-ext": t("pm.legendExt"),
    "pm-fs-fat": t("pm.legendFat"),
    "pm-fs-swap": t("pm.legendOther"),
    "pm-fs-other": t("pm.legendOther"),
  };
  const shown = new Set<string>();
  const items: HTMLElement[] = [];
  for (const cls of kinds.keys()) {
    const name = names[cls];
    if (name && !shown.has(name)) {
      shown.add(name);
      items.push(el("span", { class: "pm-key" }, [el("i", { class: "pm-dot " + cls }), name]));
    }
  }
  return el("div", { class: "pm-legend" }, items);
}

/** Vuelve a leer los discos tras una operación aceptada: el progreso lo enseña el estado. */
function started(): void {
  status = {
    running: true,
    done: false,
    disk: "",
    step: 0,
    total: 1,
    kind: "",
    part: 0,
    progress: 0,
    seconds: 0,
  };
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

/** Panel de la pieza elegida: qué es, cuánto está lleno y qué se puede hacer con ella. */
function actionsPanel(layout: Layout, segment: Segment): HTMLElement {
  const size = segment.sectors * 512;
  const isFree = segment.number === 0;
  const used = !isFree && size > 0 ? Math.min(1, segment.usedBytes / size) : 0;
  const summary: HTMLElement[] = [
    el("strong", { class: "pm-name" }, [titleOf(segment)]),
    el("span", { class: "muted" }, [
      [
        isFree ? t("pm.free") : (segment.fsType || "—").toUpperCase(),
        fmt.bytes(size),
        segment.mounted ? t("pm.mounted") : "",
      ]
        .filter(Boolean)
        .join(" · "),
    ]),
  ];
  if (!isFree && segment.usedBytes > 0) {
    const meter = progress(used, used > 0.9 ? "warn" : "accent");
    summary.push(
      meter,
      el("span", { class: "muted" }, [
        t("pm.usedFree", {
          used: fmt.bytes(segment.usedBytes),
          free: fmt.bytes(Math.max(0, size - segment.usedBytes)),
        }),
      ]),
    );
  }
  const buttons: HTMLElement[] = [];
  if (isFree) {
    buttons.push(
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
    );
  } else {
    const left = neighbour(layout, segment, "left");
    const right = neighbour(layout, segment, "right");
    const resizeWhy = t("pm.cannotResize", { fs: segment.fsType || "?" });
    buttons.push(
      action(
        t("pm.resize"),
        t("pm.actResize"),
        function resize(): void {
          openResize(layout, segment, started);
        },
        segment.resizable,
        resizeWhy,
        icons.refresh,
      ),
    );
    if (left || right) {
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
  }
  return el("div", { class: "pm-panel" }, [
    el("div", { class: "pm-info" }, summary),
    el("div", { class: "pm-actions" }, buttons),
  ]);
}

/** Texto del bloqueo de un disco. */
function lockedText(code: string): string {
  return t("pm.locked." + code);
}

/** Tarjeta de un disco. */
function diskCard(disk: PartDisk): HTMLElement {
  const locked = disk.layout.locked ?? "";
  const picked = disk.layout.segments.find(function isPicked(segment) {
    return keyOf(disk.name, segment) === selected;
  });
  return el("article", { class: "disk-card" }, [
    el("header", { class: "disk-header" }, [
      el("span", { class: "disk-icon" }, [icon(disk.transport === "usb" ? icons.usb : icons.disk)]),
      el("div", { class: "disk-title" }, [
        el("strong", {}, [[disk.vendor, disk.model].filter(Boolean).join(" ") || disk.name]),
        el("span", { class: "muted" }, [
          [
            fmt.bytes(disk.sizeBytes),
            disk.transport.toUpperCase(),
            disk.layout.table.toUpperCase(),
            "/dev/" + disk.name,
          ]
            .filter(Boolean)
            .join(" · "),
        ]),
      ]),
      badge(t("storage.roles." + disk.role), ROLE_TONES[disk.role] ?? "muted"),
      locked
        ? el("span", { class: "guard-chip" }, [icon(icons.lock), t("common.protected")])
        : null,
    ]),
    disk.layout.segments.length > 0
      ? el("div", {}, [diskBar(disk), legend(disk)])
      : el("p", { class: "muted" }, [t("storage.noTable")]),
    locked
      ? el("p", { class: "muted small-note" }, [lockedText(locked)])
      : picked
        ? actionsPanel(disk.layout, picked)
        : el("p", { class: "muted small-note" }, [t("pm.pick")]),
  ]);
}

/** Panel con el avance de la operación en curso. */
function runningPanel(current: PartStatus): HTMLElement {
  const overall = current.total > 0 ? (current.step + current.progress) / current.total : 0;
  return panel(
    t("pm.running"),
    [
      el("p", {}, [
        current.kind
          ? t("pm.steps." + current.kind, { n: current.part, size: "" }) +
            " (" +
            (current.step + 1) +
            "/" +
            current.total +
            ")"
          : t("pm.starting"),
      ]),
      progress(overall, "accent"),
      el("p", { class: "mono-small muted" }, [fmt.duration(current.seconds)]),
      el("div", { class: "error-box" }, [t("pm.dontUnplug")]),
    ],
    { iconPaths: icons.disk },
  );
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

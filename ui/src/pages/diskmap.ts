// * Piezas compartidas por "Discos" y por el asistente de instalación: el mapa de un disco (una pieza pulsable
// * por partición y por hueco libre), su leyenda y el panel de avance de una operación en curso.

import { badge, panel, progress, type Tone } from "../components.js";
import { el, icon } from "../dom.js";
import * as fmt from "../format.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";
import type { PartDisk, PartStatus, Segment } from "../partman.js";

/** Tono de cada función de disco. */
const ROLE_TONES: Record<string, Tone> = {
  "live-media": "accent",
  internal: "violet",
  removable: "amber",
  optical: "muted",
};

/** Clave de selección de una pieza. */
export function keyOf(disk: string, segment: Segment): string {
  return disk + ":" + (segment.number > 0 ? "p" + segment.number : "f" + segment.start);
}

/** Encuentra la pieza de una clave en los discos. */
export function findPiece(
  disks: PartDisk[] | null,
  key: string,
): { disk: PartDisk; segment: Segment } | null {
  for (const disk of disks ?? []) {
    for (const segment of disk.layout.segments) {
      if (keyOf(disk.name, segment) === key) {
        return { disk, segment };
      }
    }
  }
  return null;
}

/** Tamaño de una pieza en bytes. */
export function sizeOf(segment: Segment): number {
  return segment.sectors * 512;
}

/** Color y clase de una pieza por su sistema de archivos. */
export function pieceClass(segment: Segment): string {
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
export function titleOf(segment: Segment): string {
  if (segment.number === 0) {
    return t("pm.free");
  }
  return segment.label || segment.name || "#" + segment.number;
}

/** Nombre del disco con su marca y modelo. */
export function diskName(disk: PartDisk): string {
  return [disk.vendor, disk.model].filter(Boolean).join(" ") || disk.name;
}

/** Insignia con la función del disco (interno, extraíble...). */
export function roleBadge(disk: PartDisk): HTMLElement {
  return badge(t("storage.roles." + disk.role), ROLE_TONES[disk.role] ?? "muted");
}

/** Cabecera de un disco: icono, nombre, datos y función. */
export function diskHeader(disk: PartDisk, extra: (Node | null)[] = []): HTMLElement {
  return el("header", { class: "disk-header" }, [
    el("span", { class: "disk-icon" }, [icon(disk.transport === "usb" ? icons.usb : icons.disk)]),
    el("div", { class: "disk-title" }, [
      el("strong", {}, [diskName(disk)]),
      el("span", { class: "muted" }, [
        [fmt.disk(disk.sizeBytes), disk.transport.toUpperCase(), "/dev/" + disk.name]
          .filter(Boolean)
          .join(" · "),
      ]),
    ]),
    roleBadge(disk),
    ...extra,
  ]);
}

/**
 * Barra del disco: una pieza pulsable por partición y por hueco, con cuánto está lleno dentro. `onPick` recibe el
 * clic (la clave de la pieza va en data-key); `dimmed` atenúa las piezas que no sirven en ese momento.
 */
export function diskBar(
  disk: PartDisk,
  selectedKey: string,
  onPick: (event: MouseEvent) => void,
  dimmed?: (segment: Segment) => boolean,
): HTMLElement {
  const bar = el("div", {
    class: "pm-bar",
    role: "group",
    "aria-label": t("pm.barLabel", { disk: disk.name }),
  });
  for (const segment of disk.layout.segments) {
    const key = keyOf(disk.name, segment);
    const size = sizeOf(segment);
    const used = segment.number > 0 && size > 0 ? Math.min(1, segment.usedBytes / size) : 0;
    const meter = el("i", { class: "pm-use" });
    meter.style.width = Math.round(used * 100) + "%";
    const piece = el(
      "button",
      {
        class:
          "pm-seg " +
          pieceClass(segment) +
          (selectedKey === key ? " pm-selected" : "") +
          (dimmed?.(segment) ? " pm-dim" : ""),
        type: "button",
        "data-key": key,
        "aria-pressed": String(selectedKey === key),
        title:
          titleOf(segment) +
          " · " +
          fmt.disk(size) +
          (segment.number > 0 ? " · " + t("pm.usedPercent", { p: Math.round(used * 100) }) : ""),
      },
      [
        el("strong", {}, [titleOf(segment)]),
        el("span", { class: "pm-size-text" }, [fmt.disk(size)]),
        segment.number > 0 ? el("span", { class: "pm-meter" }, [meter]) : null,
      ],
    );
    piece.style.flexGrow = String(Math.max(segment.sectors, disk.layout.totalSectors * 0.05));
    piece.addEventListener("click", onPick);
    bar.append(piece);
  }
  return bar;
}

/** Leyenda de colores de la barra: solo los tipos que hay en el disco. */
export function legend(disk: PartDisk): HTMLElement {
  const names: Record<string, string> = {
    "pm-free": t("pm.legendFree"),
    "pm-fs-ntfs": t("pm.legendNtfs"),
    "pm-fs-ext": t("pm.legendExt"),
    "pm-fs-fat": t("pm.legendFat"),
    "pm-fs-swap": t("pm.legendOther"),
    "pm-fs-other": t("pm.legendOther"),
  };
  const kinds = new Map<string, string>();
  for (const segment of disk.layout.segments) {
    const cls = pieceClass(segment);
    kinds.set(names[cls] ?? "", cls);
  }
  const items: HTMLElement[] = [];
  for (const [name, cls] of kinds) {
    if (name !== "") {
      items.push(el("span", { class: "pm-key" }, [el("i", { class: "pm-dot " + cls }), name]));
    }
  }
  return el("div", { class: "pm-legend" }, items);
}

/** Panel con el avance de la operación en curso. */
export function runningPanel(current: PartStatus): HTMLElement {
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

/** Estado de "operación empezada" que se usa mientras llega el primer estado real. */
export function startedStatus(): PartStatus {
  return {
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
}

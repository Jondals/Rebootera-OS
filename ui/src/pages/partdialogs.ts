// * Diálogos del gestor de particiones. Cada uno enseña, con una barra, cómo
// * quedará el disco, deja elegir el tamaño con un deslizador y, antes de tocar
// * nada, pide al sistema la vista previa del plan (los pasos exactos, lo que se
// * borra y lo que se copia) y exige escribir "CHANGE <disco>".
// ! Aplicar solo se habilita con el texto exacto; el agente lo comprueba otra vez.

import { ApiError } from "../api.js";
import { button, errorBox } from "../components.js";
import { openModal } from "../dialogs.js";
import { el } from "../dom.js";
import * as fmt from "../format.js";
import { t } from "../i18n.js";
import {
  applyOp,
  freeAfter,
  mib,
  mibBytes,
  minimumMiB,
  neighbour,
  newOp,
  planOp,
  type Layout,
  type Op,
  type Plan,
  type Segment,
} from "../partman.js";

/** Trozo de una barra de vista previa. */
interface Piece {
  label: string;
  mib: number;
  tone: string;
}

/** Función que se llama al aplicar con éxito (la página vuelve a leer los discos). */
type Done = () => void;

/** Nombre corto de una partición para los textos. */
function nameOf(segment: Segment): string {
  return segment.label || segment.name || "#" + segment.number;
}

/** Barra de vista previa: cada trozo ocupa su proporción del total. */
function previewBar(pieces: Piece[]): HTMLElement {
  const bar = el("div", { class: "pm-preview", role: "img" });
  for (const piece of pieces) {
    const node = el("span", { class: "pm-piece pm-tone-" + piece.tone }, [
      el("strong", {}, [piece.label]),
      el("span", { class: "mono-small" }, [fmt.disk(mibBytes(piece.mib))]),
    ]);
    node.style.flexGrow = String(Math.max(piece.mib, 1));
    bar.append(node);
  }
  return bar;
}

/** Tono de una partición por su sistema de archivos. */
function toneOf(segment: Segment): string {
  switch (segment.fsType) {
    case "ntfs":
      return "accent";
    case "ext4":
    case "ext3":
    case "ext2":
      return "ok";
    case "exfat":
    case "vfat":
      return "amber";
  }
  return "muted";
}

/** MiB que tiene un GB de los que enseñan Windows y los instaladores (1 GB = 1024 MiB). */
const MIB_PER_GB = 1024;

/** Un tamaño en MiB como número de GB, sin decimales de más (50 si son 51200 MiB justos). */
function gbText(valueMiB: number): string {
  return String(Math.round((valueMiB / MIB_PER_GB) * 10) / 10);
}

/**
 * Deslizador con campo numérico sincronizado; avisa con cada cambio en MiB. Si hay sitio de sobra va en pasos de 1 GB
 * (un novato no tiene por qué acertar con valores raros) y el campo también está en GB: escribir 50 son 50 GB justos,
 * los mismos que verá el instalador. Los extremos del rango son exactos aunque no sean GB enteros. Con poco sitio
 * (menos de 2 GB) va en MiB.
 */
function sizeInput(
  min: number,
  max: number,
  value: number,
  onChange: (value: number) => void,
): HTMLElement {
  const inGB = max >= 2 * MIB_PER_GB;
  const unit = inGB ? MIB_PER_GB : 1;
  const clamp = function clamp(valueMiB: number): number {
    return Math.min(max, Math.max(min, Math.round(valueMiB)));
  };
  const range = el("input", {
    type: "range",
    class: "pm-range",
    min: String(Math.floor(min / unit)),
    max: String(Math.ceil(max / unit)),
    step: "1",
    value: String(Math.round(value / unit)),
    "aria-label": t("pm.size"),
  });
  const number = el("input", {
    type: "number",
    class: "text-input pm-number",
    min: inGB ? String(Math.ceil((min / unit) * 10) / 10) : String(min),
    max: inGB ? String(Math.floor((max / unit) * 10) / 10) : String(max),
    step: inGB ? "1" : "any",
    value: inGB ? gbText(value) : String(value),
    "aria-label": t("pm.sizeUnit"),
  });

  /** El deslizador manda: salto de 1 GB (o el extremo exacto) y se copia al campo. */
  function fromRange(): void {
    const next = clamp(Number(range.value) * unit);
    number.value = inGB ? gbText(next) : String(next);
    onChange(next);
  }

  /** El campo manda: se limita al rango y se copia al deslizador. */
  function fromNumber(): void {
    const typed = Number(number.value.replace(",", ".")) || 0;
    const next = clamp(typed * unit);
    range.value = String(Math.round(next / unit));
    number.value = inGB ? gbText(next) : String(next);
    onChange(next);
  }

  range.addEventListener("input", fromRange);
  number.addEventListener("change", fromNumber);
  return el("div", { class: "pm-size" }, [
    range,
    el("label", { class: "pm-number-label" }, [number, el("span", {}, [inGB ? "GB" : "MiB"])]),
  ]);
}

/** Tiempo aproximado para copiar unos bytes (a unos 60 MB/s, lo que da un USB normal). */
function copyTime(bytes: number): string {
  return fmt.duration(bytes / 60e6);
}

/** Texto de un paso del plan. */
function stepText(step: Plan["steps"][number]): string {
  return t("pm.steps." + step.kind, {
    n: step.part,
    size: step.length ? fmt.disk(step.length * 512) : "",
  });
}

/** Segunda mitad de todos los diálogos: la vista previa del plan y la confirmación. */
async function review(
  dialog: HTMLDialogElement,
  body: HTMLElement,
  op: Op,
  done: Done,
): Promise<void> {
  body.replaceChildren(el("p", { class: "muted" }, [t("pm.planning")]));
  let plan: Plan;
  try {
    plan = await planOp(op);
  } catch (error) {
    body.replaceChildren(errorBox(error));
    return;
  }
  const apply = button(
    t("pm.apply"),
    async function applyClick(): Promise<void> {
      try {
        await applyOp(op, plan.confirm);
        dialog.close();
        done();
      } catch (error) {
        body.prepend(errorBox(error));
      }
    },
    { variant: "danger" },
  );
  const warnings: HTMLElement[] = [];
  if (plan.destructive) {
    warnings.push(el("div", { class: "error-box" }, [t("pm.warnDestructive")]));
  }
  if (plan.movesBytes > 0) {
    warnings.push(
      el("div", { class: "notice" }, [
        t("pm.warnMove", { size: fmt.disk(plan.movesBytes), time: copyTime(plan.movesBytes) }),
      ]),
    );
  }
  warnings.push(el("div", { class: "notice" }, [t("pm.warnBackup")]));
  body.replaceChildren(
    el("h3", {}, [t("pm.reviewTitle")]),
    el(
      "ol",
      { class: "pm-steps" },
      plan.steps.map(function item(step) {
        return el("li", {}, [stepText(step)]);
      }),
    ),
    ...warnings,
    el("div", { class: "modal-actions" }, [apply]),
  );
  apply.focus();
}

/** Muestra un error de medición del mínimo. */
function measureError(body: HTMLElement, error: unknown): void {
  const message = error instanceof ApiError ? error.message : t("pm.noInfo");
  body.replaceChildren(errorBox(new Error(message)));
}

/** De dónde sale o a dónde va el espacio al cambiar el tamaño: libre, o compartido con una vecina. */
interface SpaceMode {
  /** "free", "left" o "right" (la vecina con la que se reparte). */
  kind: "free" | "left" | "right";
  other: Segment | null;
}

/** Modos posibles de una partición: el hueco libre de detrás y cada vecina pegada con la que se puede repartir. */
function spaceModes(layout: Layout, segment: Segment): SpaceMode[] {
  const modes: SpaceMode[] = [{ kind: "free", other: null }];
  const right = neighbour(layout, segment, "right");
  const left = neighbour(layout, segment, "left");
  if (right && right.resizable && segment.resizable) {
    modes.push({ kind: "right", other: right });
  }
  if (left && left.resizable && segment.resizable) {
    modes.push({ kind: "left", other: left });
  }
  return modes;
}

/** Tamaño con el que arranca el deslizador: encoger para dejar sitio, o crecer al máximo. */
interface ResizePreset {
  /** MiB que se quieren liberar (la partición se encoge esa cantidad, o lo que pueda). */
  freeMiB?: number;
  /** Empezar con la partición lo más grande posible. */
  grow?: boolean;
}

/**
 * Cambiar el tamaño de una partición. Un solo deslizador con el tamaño nuevo y, si tiene vecinas, un
 * selector de qué pasa con la diferencia: queda libre (cambia el final) o se reparte con la vecina (se mueve
 * el límite entre las dos: una gana lo que la otra cede).
 */
export async function openResize(
  layout: Layout,
  segment: Segment,
  done: Done,
  preset: ResizePreset = {},
): Promise<void> {
  const body = el("div", { class: "pm-dialog" }, [
    el("p", { class: "muted" }, [t("pm.measuring")]),
  ]);
  const dialog = openModal(t("pm.resizeTitle", { name: nameOf(segment) }), [body], "modal-wide");
  const modes = spaceModes(layout, segment);
  const floors = new Map<number, number>();
  try {
    floors.set(segment.number, await minimumMiB(layout.disk, segment.number));
  } catch (error) {
    measureError(body, error);
    return;
  }
  let mode = modes[0];
  let value = mib(segment);
  if (preset.freeMiB !== undefined) {
    value = Math.max(floors.get(segment.number) ?? 1, mib(segment) - preset.freeMiB);
  } else if (preset.grow) {
    value = Number.MAX_SAFE_INTEGER;
  }
  const preview = el("div", {});
  const info = el("p", { class: "muted" });
  const slider = el("div", {});
  const choices = el("div", { class: "pm-modes", role: "radiogroup" });

  /** Límites del deslizador en el modo actual (MiB de la partición elegida). */
  function limits(): [number, number] {
    const floor = floors.get(segment.number) ?? 1;
    if (mode.kind === "free") {
      return [floor, Math.max(mib(segment) + freeAfter(layout, segment), floor)];
    }
    const otherFloor = floors.get((mode.other as Segment).number) ?? 1;
    return [floor, Math.max(mib(segment) + mib(mode.other as Segment) - otherFloor, floor)];
  }

  /** Operación que corresponde al modo y al tamaño elegidos. */
  function buildOp(): Op {
    if (mode.kind === "free") {
      const op = newOp("resize", layout.disk);
      op.part = segment.number;
      op.sizeMiB = value;
      return op;
    }
    const other = mode.other as Segment;
    const op = newOp("boundary", layout.disk);
    const total = mib(segment) + mib(other);
    // * boundary mide el tamaño de la partición de la izquierda.
    if (mode.kind === "right") {
      op.part = segment.number;
      op.other = other.number;
      op.sizeMiB = value;
    } else {
      op.part = other.number;
      op.other = segment.number;
      op.sizeMiB = total - value;
    }
    return op;
  }

  /** Redibuja la barra de vista previa y el texto con el tamaño elegido. */
  function draw(next: number): void {
    value = next;
    const delta = value - mib(segment);
    let pieces: Piece[];
    if (mode.kind === "free") {
      const ceiling = mib(segment) + freeAfter(layout, segment);
      pieces = [
        { label: nameOf(segment), mib: value, tone: toneOf(segment) },
        { label: t("pm.free"), mib: Math.max(ceiling - value, 0), tone: "free" },
      ];
    } else {
      const other = mode.other as Segment;
      const total = mib(segment) + mib(other);
      const mine = { label: nameOf(segment), mib: value, tone: toneOf(segment) };
      const theirs = { label: nameOf(other), mib: total - value, tone: toneOf(other) };
      pieces = mode.kind === "right" ? [mine, theirs] : [theirs, mine];
    }
    preview.replaceChildren(previewBar(pieces));
    if (delta === 0) {
      info.textContent = t("pm.noChange");
    } else if (mode.kind === "free") {
      info.textContent =
        delta > 0
          ? t("pm.grows", { size: fmt.disk(mibBytes(delta)) })
          : t("pm.shrinks", { size: fmt.disk(mibBytes(-delta)) });
    } else {
      const other = mode.other as Segment;
      const moved =
        mode.kind === "right"
          ? delta > 0
            ? mib(other) - delta
            : mib(other)
          : mib(segment) + delta;
      info.textContent = t("pm.shareInfo", {
        to: delta > 0 ? nameOf(segment) : nameOf(other),
        from: delta > 0 ? nameOf(other) : nameOf(segment),
        size: fmt.disk(mibBytes(Math.abs(delta))),
        move: copyTime(mibBytes(Math.max(moved, 0))),
      });
    }
  }

  /** Dibuja el deslizador con los límites del modo actual. */
  function drawSlider(): void {
    const [low, high] = limits();
    value = Math.min(Math.max(value, low), high);
    slider.replaceChildren(sizeInput(low, high, value, draw));
    draw(value);
  }

  /** Cambia de modo (midiendo antes el mínimo de la vecina si hace falta). */
  async function chooseMode(next: SpaceMode): Promise<void> {
    if (next.other && !floors.has(next.other.number)) {
      try {
        floors.set(next.other.number, await minimumMiB(layout.disk, next.other.number));
      } catch (error) {
        measureError(body, error);
        return;
      }
    }
    mode = next;
    value = mib(segment);
    drawChoices();
    drawSlider();
  }

  /** Dibuja las opciones de qué hacer con el espacio (solo si hay más de una). */
  function drawChoices(): void {
    choices.replaceChildren(
      ...modes.map(function choice(candidate) {
        const label =
          candidate.kind === "free"
            ? t("pm.spaceFree")
            : t("pm.spaceShared", { name: nameOf(candidate.other as Segment) });
        const node = el(
          "button",
          {
            class: "pm-mode",
            type: "button",
            role: "radio",
            "aria-checked": String(candidate === mode),
          },
          [label],
        );
        node.addEventListener("click", function pick(): void {
          void chooseMode(candidate);
        });
        return node;
      }),
    );
  }

  const next = button(
    t("pm.continue"),
    function go(): void {
      review(dialog, body, buildOp(), done);
    },
    { variant: "primary" },
  );
  body.replaceChildren(
    el("p", { class: "muted" }, [t("pm.resizeIntro")]),
    ...(modes.length > 1
      ? [el("strong", { class: "pm-modes-title" }, [t("pm.spaceTitle")]), choices]
      : []),
    preview,
    slider,
    info,
    el("div", { class: "modal-actions" }, [next]),
  );
  drawChoices();
  drawSlider();
}

/** Fusionar dos particiones vecinas en una: se elige cuál se queda; la otra se borra y su espacio pasa a la que se queda. */
export function openMerge(layout: Layout, segment: Segment, done: Done): void {
  const sides = [neighbour(layout, segment, "left"), neighbour(layout, segment, "right")].filter(
    function exists(other): other is Segment {
      return other !== null;
    },
  );
  const body = el("div", { class: "pm-dialog" });
  const dialog = openModal(t("pm.mergeTitle", { name: nameOf(segment) }), [body], "modal-wide");
  const choices: HTMLElement[] = [];
  for (const other of sides) {
    for (const [keep, gone] of [
      [segment, other],
      [other, segment],
    ] as [Segment, Segment][]) {
      if (!keep.resizable) {
        continue;
      }
      const op = newOp("merge", layout.disk);
      op.part = keep.number;
      op.other = gone.number;
      // * Si la que se queda está a la derecha hay que copiar todos sus datos hacia atrás: tarda mucho más.
      const slow = keep.start > gone.start;
      const total = fmt.disk(mibBytes(mib(keep) + mib(gone)));
      const choose = button(
        t("pm.mergeKeep", { keep: nameOf(keep) }),
        function pick(): void {
          review(dialog, body, op, done);
        },
        { variant: "primary" },
      );
      choices.push(
        el("div", { class: "pm-choice pm-merge-choice" }, [
          previewBar([{ label: nameOf(keep), mib: mib(keep) + mib(gone), tone: toneOf(keep) }]),
          el("p", { class: "pm-merge-line" }, [
            el("strong", {}, [t("pm.mergeStays", { keep: nameOf(keep), size: total })]),
          ]),
          el("p", { class: "pm-merge-loss" }, [t("pm.mergeLoses", { gone: nameOf(gone) })]),
          slow
            ? el("p", { class: "muted" }, [
                t("pm.mergeSlow", {
                  size: fmt.disk(keep.usedBytes > 0 ? keep.usedBytes : mibBytes(mib(keep))),
                  time: copyTime(keep.usedBytes > 0 ? keep.usedBytes : mibBytes(mib(keep))),
                }),
              ])
            : el("p", { class: "muted" }, [t("pm.mergeFast")]),
          choose,
        ]),
      );
    }
  }
  body.append(
    el("p", { class: "muted" }, [t("pm.mergeText")]),
    ...(choices.length > 0 ? choices : [el("p", { class: "muted" }, [t("pm.mergeNone")])]),
  );
}

/** Cambiar el nombre (etiqueta) de una partición. */
export function openRename(layout: Layout, segment: Segment, done: Done): void {
  const body = el("div", { class: "pm-dialog" });
  const dialog = openModal(t("pm.renameTitle", { name: nameOf(segment) }), [body], "modal-wide");
  const input = el("input", {
    class: "text-input",
    type: "text",
    maxlength: "16",
    value: segment.label ?? "",
    placeholder: t("pm.renamePlaceholder"),
  });
  const go = function go(): void {
    const label = input.value.trim();
    if (label === "") {
      return;
    }
    const op = newOp("rename", layout.disk);
    op.part = segment.number;
    op.label = label;
    review(dialog, body, op, done);
  };
  input.addEventListener("keydown", function onEnter(event: KeyboardEvent): void {
    if (event.key === "Enter") {
      go();
    }
  });
  body.append(
    el("p", { class: "muted" }, [t("pm.renameText")]),
    input,
    el("div", { class: "modal-actions" }, [button(t("pm.continue"), go, { variant: "primary" })]),
  );
  input.focus();
}

/** Máximo de caracteres de la etiqueta según el sistema de archivos. */
const LABEL_LIMITS: Record<string, number> = { exfat: 15, ntfs: 32, ext4: 16, fat32: 11 };

/** Formatear una partición que ya existe: se borra todo lo que tiene y se le pone un sistema de archivos nuevo. */
export function openFormat(layout: Layout, segment: Segment, done: Done): void {
  const body = el("div", { class: "pm-dialog" });
  const dialog = openModal(t("pm.formatTitle", { name: nameOf(segment) }), [body], "modal-wide");
  const op = newOp("format", layout.disk);
  op.part = segment.number;
  op.fs = Object.hasOwn(LABEL_LIMITS, segment.fsType ?? "") ? (segment.fsType as string) : "exfat";
  const fsSelect = el("select", { class: "text-input", "aria-label": t("pm.fs") });
  for (const fs of ["exfat", "ntfs", "ext4", "fat32"]) {
    fsSelect.append(el("option", { value: fs }, [t("pm.fsNames." + fs)]));
  }
  fsSelect.value = op.fs;
  const label = el("input", {
    class: "text-input",
    type: "text",
    maxlength: String(LABEL_LIMITS[op.fs]),
    value: segment.label ?? "",
    placeholder: t("pm.labelHintFormat"),
  });
  op.label = label.value;
  fsSelect.addEventListener("change", function changeFS(): void {
    op.fs = fsSelect.value;
    label.maxLength = LABEL_LIMITS[op.fs];
    label.value = label.value.slice(0, LABEL_LIMITS[op.fs]);
    op.label = label.value;
  });
  label.addEventListener("input", function changeLabel(): void {
    op.label = label.value;
  });
  const next = button(
    t("pm.continue"),
    function go(): void {
      review(dialog, body, op, done);
    },
    { variant: "primary" },
  );
  body.append(
    el("div", { class: "error-box" }, [
      t("pm.formatText", {
        size: fmt.disk(segment.sectors * 512),
        used: fmt.disk(segment.usedBytes),
      }),
    ]),
    el("label", { class: "confirm-field" }, [el("span", {}, [t("pm.fs")]), fsSelect]),
    el("label", { class: "confirm-field" }, [el("span", {}, [t("pm.label")]), label]),
    el("div", { class: "modal-actions" }, [next]),
  );
}

/** Borrar una partición. */
export function openDelete(layout: Layout, segment: Segment, done: Done): void {
  const body = el("div", { class: "pm-dialog" });
  const dialog = openModal(t("pm.deleteTitle", { name: nameOf(segment) }), [body], "modal-wide");
  const op = newOp("delete", layout.disk);
  op.part = segment.number;
  review(dialog, body, op, done);
}

/** Crear una partición nueva en un hueco libre. */
export function openCreate(layout: Layout, free: Segment, done: Done): void {
  const body = el("div", { class: "pm-dialog" });
  const dialog = openModal(t("pm.createTitle"), [body], "modal-wide");
  const space = mib(free);
  const startMiB = Math.floor(free.start / 2048);
  const op = newOp("create", layout.disk);
  op.startMiB = startMiB;
  op.fs = "exfat";
  const preview = el("div", {});
  function draw(value: number): void {
    op.sizeMiB = value;
    preview.replaceChildren(
      previewBar([
        { label: t("pm.newPartition"), mib: value, tone: "accent" },
        { label: t("pm.free"), mib: Math.max(space - value, 0), tone: "free" },
      ]),
    );
  }
  const size = sizeInput(1, space, space, draw);
  const fsSelect = el("select", { class: "text-input", "aria-label": t("pm.fs") });
  for (const fs of ["exfat", "ntfs", "ext4", "fat32"]) {
    fsSelect.append(el("option", { value: fs }, [t("pm.fsNames." + fs)]));
  }
  fsSelect.addEventListener("change", function changeFS(): void {
    op.fs = fsSelect.value;
  });
  const label = el("input", {
    class: "text-input",
    type: "text",
    maxlength: "11",
    placeholder: t("pm.labelHint"),
  });
  label.addEventListener("input", function changeLabel(): void {
    op.label = label.value;
  });
  const next = button(
    t("pm.continue"),
    function go(): void {
      review(dialog, body, op, done);
    },
    { variant: "primary" },
  );
  body.append(
    el("p", { class: "muted" }, [t("pm.createText", { size: fmt.disk(mibBytes(space)) })]),
    preview,
    size,
    el("label", { class: "confirm-field" }, [el("span", {}, [t("pm.fs")]), fsSelect]),
    el("label", { class: "confirm-field" }, [el("span", {}, [t("pm.label")]), label]),
    el("div", { class: "modal-actions" }, [next]),
  );
  draw(space);
}

/** Una partición que se puede encoger para dejar sitio. */
interface Shrinkable {
  segment: Segment;
  /** Lo que se podría liberar, a ojo (lo exacto se mide al abrir el diálogo de tamaño). */
  roomBytes: number;
}

/** Margen que se deja a una partición al calcular lo que se puede liberar (10 % más 1 GiB). */
function usableRoom(segment: Segment): number {
  const size = segment.sectors * 512;
  return Math.max(0, size - segment.usedBytes * 1.1 - 1073741824);
}

/**
 * "Hacer sitio": enseña qué particiones del disco se pueden encoger para conseguir `needBytes` libres y, al
 * elegir una, abre el cambio de tamaño ya con la cantidad puesta. `free` es el hueco libre elegido, si hay.
 */
export function openMakeRoom(
  layout: Layout,
  needBytes: number,
  free: Segment | null,
  done: Done,
): void {
  const body = el("div", { class: "pm-dialog" });
  const dialog = openModal(t("pm.roomTitle"), [body], "modal-wide");
  const alreadyFree = free ? free.sectors * 512 : 0;
  const lack = Math.max(0, needBytes - alreadyFree);
  const candidates: Shrinkable[] = layout.segments
    .filter(function canShrink(segment) {
      return (
        segment.number > 0 &&
        segment.resizable &&
        !segment.mounted &&
        usableRoom(segment) >= 1073741824
      );
    })
    .map(function toShrinkable(segment) {
      return { segment, roomBytes: usableRoom(segment) };
    })
    .sort(function biggestFirst(a, b) {
      return b.roomBytes - a.roomBytes;
    });
  const rows = candidates.map(function row(candidate) {
    const enough = candidate.roomBytes >= lack;
    const pick = button(
      t("pm.roomPick"),
      function choose(): void {
        dialog.close();
        void openResize(layout, candidate.segment, done, {
          freeMiB: Math.ceil(lack / 1048576) + 64,
        });
      },
      { variant: enough ? "primary" : "ghost" },
    );
    pick.disabled = !enough;
    return el("div", { class: "pm-room-row" }, [
      el("div", {}, [
        el("strong", {}, [nameOf(candidate.segment)]),
        el("span", { class: "muted" }, [
          t("pm.roomInfo", {
            size: fmt.disk(candidate.segment.sectors * 512),
            used: fmt.disk(candidate.segment.usedBytes),
            room: fmt.disk(candidate.roomBytes),
          }),
        ]),
      ]),
      pick,
    ]);
  });
  body.append(
    el("p", {}, [t("pm.roomText", { size: fmt.disk(lack) })]),
    ...(rows.length > 0 ? rows : [el("p", { class: "muted" }, [t("pm.roomNone")])]),
  );
}

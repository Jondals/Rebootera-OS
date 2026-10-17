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
      el("span", { class: "mono-small" }, [fmt.bytes(mibBytes(piece.mib))]),
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

/** Deslizador con campo numérico sincronizado (en MiB); avisa con cada cambio. */
function sizeInput(
  min: number,
  max: number,
  value: number,
  onChange: (value: number) => void,
): HTMLElement {
  const range = el("input", {
    type: "range",
    class: "pm-range",
    min: String(min),
    max: String(max),
    step: "1",
    value: String(value),
    "aria-label": t("pm.size"),
  });
  const number = el("input", {
    type: "number",
    class: "text-input pm-number",
    min: String(min),
    max: String(max),
    value: String(value),
    "aria-label": t("pm.sizeMiB"),
  });

  /** El deslizador manda: se copia al campo. */
  function fromRange(): void {
    number.value = range.value;
    onChange(Number(range.value));
  }

  /** El campo manda: se limita al rango y se copia al deslizador. */
  function fromNumber(): void {
    const clamped = Math.min(max, Math.max(min, Number(number.value) || 0));
    range.value = String(clamped);
    number.value = String(clamped);
    onChange(clamped);
  }

  range.addEventListener("input", fromRange);
  number.addEventListener("change", fromNumber);
  return el("div", { class: "pm-size" }, [
    range,
    el("label", { class: "pm-number-label" }, [number, el("span", {}, ["MiB"])]),
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
    size: step.length ? fmt.bytes(step.length * 512) : "",
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
  const typed = el("input", { class: "text-input", type: "text", placeholder: plan.confirm });
  const apply = button(
    t("pm.apply"),
    async function applyClick(): Promise<void> {
      try {
        await applyOp(op, typed.value);
        dialog.close();
        done();
      } catch (error) {
        body.prepend(errorBox(error));
      }
    },
    { variant: "danger" },
  );
  apply.disabled = true;
  typed.addEventListener("input", function check(): void {
    apply.disabled = typed.value !== plan.confirm;
  });
  const warnings: HTMLElement[] = [];
  if (plan.destructive) {
    warnings.push(el("div", { class: "error-box" }, [t("pm.warnDestructive")]));
  }
  if (plan.movesBytes > 0) {
    warnings.push(
      el("div", { class: "notice" }, [
        t("pm.warnMove", { size: fmt.bytes(plan.movesBytes), time: copyTime(plan.movesBytes) }),
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
    el("label", { class: "confirm-field" }, [
      el("span", {}, [t("pm.typeToConfirm", { text: plan.confirm })]),
      typed,
    ]),
    el("div", { class: "modal-actions" }, [apply]),
  );
  typed.focus();
}

/** Muestra un error de medición del mínimo. */
function measureError(body: HTMLElement, error: unknown): void {
  const message = error instanceof ApiError ? error.message : t("pm.noInfo");
  body.replaceChildren(errorBox(new Error(message)));
}

/** Cambiar el tamaño de una partición por su final. */
export async function openResize(layout: Layout, segment: Segment, done: Done): Promise<void> {
  const body = el("div", { class: "pm-dialog" }, [
    el("p", { class: "muted" }, [t("pm.measuring")]),
  ]);
  const dialog = openModal(t("pm.resizeTitle", { name: nameOf(segment) }), [body], "modal-wide");
  let floor: number;
  try {
    floor = await minimumMiB(layout.disk, segment.number);
  } catch (error) {
    measureError(body, error);
    return;
  }
  const current = mib(segment);
  const ceiling = current + freeAfter(layout, segment);
  const op = newOp("resize", layout.disk);
  op.part = segment.number;
  const preview = el("div", {});
  const info = el("p", { class: "muted" });
  function draw(value: number): void {
    op.sizeMiB = value;
    preview.replaceChildren(
      previewBar([
        { label: nameOf(segment), mib: value, tone: toneOf(segment) },
        { label: t("pm.free"), mib: Math.max(ceiling - value, 0), tone: "free" },
      ]),
    );
    const delta = value - current;
    info.textContent =
      delta === 0
        ? t("pm.noChange")
        : delta > 0
          ? t("pm.grows", { size: fmt.bytes(mibBytes(delta)) })
          : t("pm.shrinks", { size: fmt.bytes(mibBytes(-delta)) });
  }
  const size = sizeInput(floor, Math.max(ceiling, floor), current, draw);
  const next = button(
    t("pm.continue"),
    function go(): void {
      review(dialog, body, op, done);
    },
    { variant: "primary" },
  );
  body.replaceChildren(
    el("p", { class: "muted" }, [
      t("pm.resizeText", { min: fmt.bytes(mibBytes(floor)), max: fmt.bytes(mibBytes(ceiling)) }),
    ]),
    preview,
    size,
    info,
    el("div", { class: "modal-actions" }, [next]),
  );
  draw(current);
}

/** Mover el límite entre dos particiones vecinas. */
export async function openBoundary(
  layout: Layout,
  left: Segment,
  right: Segment,
  done: Done,
): Promise<void> {
  const body = el("div", { class: "pm-dialog" }, [
    el("p", { class: "muted" }, [t("pm.measuring")]),
  ]);
  const dialog = openModal(
    t("pm.boundaryTitle", { a: nameOf(left), b: nameOf(right) }),
    [body],
    "modal-wide",
  );
  let minLeft: number;
  let minRight: number;
  try {
    [minLeft, minRight] = await Promise.all([
      minimumMiB(layout.disk, left.number),
      minimumMiB(layout.disk, right.number),
    ]);
  } catch (error) {
    measureError(body, error);
    return;
  }
  const total = mib(left) + mib(right);
  const op = newOp("boundary", layout.disk);
  op.part = left.number;
  op.other = right.number;
  const preview = el("div", {});
  const info = el("p", { class: "muted" });
  function draw(value: number): void {
    op.sizeMiB = value;
    preview.replaceChildren(
      previewBar([
        { label: nameOf(left), mib: value, tone: toneOf(left) },
        { label: nameOf(right), mib: total - value, tone: toneOf(right) },
      ]),
    );
    const delta = value - mib(left);
    const moved = delta > 0 ? mibBytes(total - value) : mibBytes(mib(right));
    info.textContent =
      delta === 0
        ? t("pm.noChange")
        : t("pm.boundaryInfo", {
            to: delta > 0 ? nameOf(left) : nameOf(right),
            from: delta > 0 ? nameOf(right) : nameOf(left),
            size: fmt.bytes(mibBytes(Math.abs(delta))),
            move: copyTime(moved),
          });
  }
  const size = sizeInput(minLeft, Math.max(total - minRight, minLeft), mib(left), draw);
  const next = button(
    t("pm.continue"),
    function go(): void {
      review(dialog, body, op, done);
    },
    { variant: "primary" },
  );
  body.replaceChildren(
    el("p", { class: "muted" }, [t("pm.boundaryText", { a: nameOf(left), b: nameOf(right) })]),
    preview,
    size,
    info,
    el("div", { class: "modal-actions" }, [next]),
  );
  draw(mib(left));
}

/** Fusionar dos particiones vecinas en una. */
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
    // * Se conserva la que puede crecer; la otra se absorbe y pierde sus datos.
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
      const choose = button(
        t("pm.mergeKeep", { keep: nameOf(keep), gone: nameOf(gone) }),
        function pick(): void {
          review(dialog, body, op, done);
        },
        { variant: "ghost" },
      );
      choices.push(
        el("div", { class: "pm-choice" }, [
          previewBar([{ label: nameOf(keep), mib: mib(keep) + mib(gone), tone: toneOf(keep) }]),
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
    el("p", { class: "muted" }, [t("pm.createText", { size: fmt.bytes(mibBytes(space)) })]),
    preview,
    size,
    el("label", { class: "confirm-field" }, [el("span", {}, [t("pm.fs")]), fsSelect]),
    el("label", { class: "confirm-field" }, [el("span", {}, [t("pm.label")]), label]),
    el("div", { class: "modal-actions" }, [next]),
  );
  draw(space);
}

// * Página "USB y almacenamiento": todos los discos del equipo con su
// * distribución de particiones, sistema de archivos, función y uso.
// ! Solo lectura: esta página no ofrece ninguna acción que escriba en disco.

import type { Disk, Partition } from "../api.js";
import { badge, emptyState, pageHeader, progress, type Tone } from "../components.js";
import { el, icon } from "../dom.js";
import * as fmt from "../format.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";
import type { Page } from "../router.js";
import { snapshot } from "../state.js";

/** Tono de cada función de partición (coincide con la barra del resumen). */
const PART_TONES: Record<string, Tone> = {
  efi: "amber",
  msr: "muted",
  recovery: "violet",
  windows: "accent",
  linux: "ok",
  vault: "accent",
  "boot-tools": "amber",
  swap: "muted",
  data: "muted",
  unknown: "muted",
};

/** Tono de cada función de disco. */
const ROLE_TONES: Record<string, Tone> = {
  "live-media": "accent",
  internal: "violet",
  removable: "amber",
  optical: "muted",
};

let list: HTMLElement | null = null;
let lastSignature = "";

/** Dibuja la estructura fija de la página. */
function render(root: HTMLElement): void {
  list = el("div", { class: "disk-list" });
  lastSignature = "";
  root.append(
    el("div", { class: "page" }, [pageHeader(t("storage.title"), t("storage.subtitle")), list]),
  );
}

/** Fila de una partición. */
function partitionRow(part: Partition): HTMLElement {
  const usage =
    part.mountPoint && part.usedBytes + part.freeBytes > 0
      ? el("div", { class: "part-usage" }, [
          progress(
            part.usedBytes / (part.usedBytes + part.freeBytes),
            PART_TONES[part.role] ?? "accent",
          ),
          el("span", { class: "mono-small" }, [
            t("sidebar.usedFree", {
              used: fmt.bytes(part.usedBytes),
              free: fmt.bytes(part.freeBytes),
            }),
          ]),
        ])
      : el("span", { class: "muted" }, [
          part.mountPoint
            ? t("storage.mountedAt", { path: part.mountPoint })
            : t("storage.notMounted"),
        ]);
  return el("div", { class: "part-row" }, [
    el("span", { class: "part-swatch swatch-" + (PART_TONES[part.role] ?? "muted") }),
    el("div", { class: "part-name" }, [
      el("strong", {}, [part.label || part.name]),
      el("span", { class: "mono-small muted" }, [part.path]),
    ]),
    badge(t("storage.parts." + part.role), PART_TONES[part.role] ?? "muted"),
    el("span", { class: "mono-small" }, [part.fsType || "—"]),
    el("span", { class: "mono-small part-size" }, [fmt.bytes(part.sizeBytes)]),
    usage,
  ]);
}

/** Tarjeta de un disco. */
function diskCard(disk: Disk): HTMLElement {
  const barNode = el("div", { class: "partition-bar partition-bar-large", role: "presentation" });
  for (const part of disk.partitions) {
    const segment = el("span", { class: "segment segment-" + (PART_TONES[part.role] ?? "muted") });
    segment.style.flexGrow = String(Math.max(part.sizeBytes / Math.max(1, disk.sizeBytes), 0.01));
    barNode.append(segment);
  }
  return el("article", { class: "disk-card" }, [
    el("header", { class: "disk-header" }, [
      el("span", { class: "disk-icon" }, [icon(disk.transport === "usb" ? icons.usb : icons.disk)]),
      el("div", { class: "disk-title" }, [
        el("strong", {}, [[disk.vendor, disk.model].filter(Boolean).join(" ") || disk.name]),
        el("span", { class: "mono-small muted" }, [
          [
            disk.path,
            fmt.bytes(disk.sizeBytes),
            disk.transport.toUpperCase(),
            disk.partTable.toUpperCase() || t("storage.noTable"),
          ]
            .filter(Boolean)
            .join(" · "),
        ]),
      ]),
      badge(t("storage.roles." + disk.role), ROLE_TONES[disk.role] ?? "muted"),
      disk.protected
        ? el("span", { class: "guard-chip" }, [icon(icons.lock), t("common.protected")])
        : null,
    ]),
    disk.partitions.length > 0 ? barNode : null,
    el("div", { class: "part-table" }, disk.partitions.map(partitionRow)),
  ]);
}

/** Refresca la lista solo si los discos han cambiado. */
function update(): void {
  if (list === null || !list.isConnected) {
    return;
  }
  const disks = snapshot.disks;
  if (disks === null) {
    list.replaceChildren(el("p", { class: "muted" }, [t("common.loading")]));
    return;
  }
  // * Se evita redibujar si no ha cambiado nada (el sondeo es periódico).
  const signature = JSON.stringify(disks);
  if (signature === lastSignature) {
    return;
  }
  lastSignature = signature;
  list.replaceChildren(
    ...(disks.length > 0
      ? disks.map(diskCard)
      : [emptyState(icons.disk, t("storage.noDisks"), "")]),
  );
}

export const storagePage: Page = { render, update };

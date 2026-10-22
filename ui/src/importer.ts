// * Diálogo "Añadir imágenes": importa ISO desde otro disco (un USB, el disco
// * interno con Windows, otra unidad...). Lista las particiones legibles,
// * abre la elegida en SOLO LECTURA mediante el agente, permite navegar por
// * sus carpetas (solo se muestran carpetas e imágenes) y pone en cola la
// * copia hacia la biblioteca. Al cerrar, intenta desmontar el disco; si hay
// * una copia en curso, el agente no lo desmonta hasta que termine.

import { getJSON, postJSON, type BrowseEntry, type Disk, type Partition } from "./api.js";
import { badge, button, emptyState, errorBox } from "./components.js";
import { openModal } from "./dialogs.js";
import { el, icon } from "./dom.js";
import * as fmt from "./format.js";
import { t } from "./i18n.js";
import { icons } from "./icons.js";
import { refreshVault, snapshot } from "./state.js";

/** Sistemas de archivos que el agente sabe abrir en solo lectura. */
const READABLE = [
  "vfat",
  "exfat",
  "ntfs",
  "ext2",
  "ext3",
  "ext4",
  "btrfs",
  "xfs",
  "iso9660",
  "udf",
];

/** Funciones de partición que nunca contienen imágenes del usuario. */
const SKIP_ROLES = ["msr", "swap", "efi", "boot-tools", "vault"];

/** Estado del diálogo abierto. */
let body: HTMLElement | null = null;
let device = "";
let folder = "";

/** Fuente posible: una partición con su disco. */
interface Source {
  disk: Disk;
  part: Partition;
}

/** Devuelve las particiones desde las que se puede importar. */
function sources(): Source[] {
  const result: Source[] = [];
  for (const disk of snapshot.disks ?? []) {
    if (disk.role === "live-media") {
      continue;
    }
    for (const part of disk.partitions) {
      if (READABLE.includes(part.fsType) && !SKIP_ROLES.includes(part.role)) {
        result.push({ disk, part });
      }
    }
  }
  return result;
}

/** Abre el diálogo de importación. */
export function openImporter(): void {
  body = el("div", { class: "importer" });
  device = "";
  folder = "";
  const dialog = openModal(t("importer.title"), [body], "modal-wide");
  dialog.addEventListener("close", closeMedia);
  showSources();
}

/** Muestra la lista de discos. */
function showSources(): void {
  if (body === null) {
    return;
  }
  const list = sources();
  if (list.length === 0) {
    body.replaceChildren(emptyState(icons.usb, t("importer.title"), t("importer.noSources")));
    return;
  }
  body.replaceChildren(
    el("p", { class: "muted" }, [t("importer.text")]),
    ...list.map(function sourceRow(source) {
      const open = button(t("importer.open"), openSource, {
        variant: "primary",
        iconPaths: icons.chevronRight,
      });
      open.classList.add("btn-small");
      open.dataset.device = source.part.path;
      return el("div", { class: "source-row" }, [
        el("span", { class: "device-icon" }, [
          icon(source.disk.transport === "usb" ? icons.usb : icons.disk),
        ]),
        el("div", { class: "device-main" }, [
          el("strong", {}, [source.part.label || source.part.name]),
          el("span", { class: "muted" }, [
            [
              source.disk.model || source.disk.name,
              source.part.fsType.toUpperCase(),
              fmt.bytes(source.part.sizeBytes),
            ].join(" · "),
          ]),
        ]),
        badge(
          t("storage.roles." + source.disk.role),
          source.disk.role === "internal" ? "violet" : "amber",
        ),
        open,
      ]);
    }),
  );
}

/** Abre (monta en solo lectura) el disco pulsado. */
async function openSource(event: MouseEvent): Promise<void> {
  const target = event.currentTarget as HTMLButtonElement;
  target.disabled = true;
  try {
    await postJSON("/api/media/mount", { device: target.dataset.device });
    device = target.dataset.device ?? "";
    folder = "";
    await showFolder();
  } catch (error) {
    target.disabled = false;
    body?.prepend(errorBox(error));
  }
}

/** Entra en la carpeta pulsada. */
function enterFolder(event: MouseEvent): void {
  folder = (event.currentTarget as HTMLElement).dataset.path ?? "";
  showFolder();
}

/** Sube una carpeta (o vuelve a la lista de discos desde la raíz). */
function goUp(): void {
  if (folder === "") {
    closeMedia();
    device = "";
    showSources();
    return;
  }
  folder = folder.includes("/") ? folder.slice(0, folder.lastIndexOf("/")) : "";
  showFolder();
}

/** Muestra el contenido de la carpeta actual. */
async function showFolder(): Promise<void> {
  if (body === null) {
    return;
  }
  let entries: BrowseEntry[];
  try {
    entries = await getJSON<BrowseEntry[]>(
      "/api/media/browse?device=" +
        encodeURIComponent(device) +
        "&dir=" +
        encodeURIComponent(folder),
    );
  } catch (error) {
    body.replaceChildren(errorBox(error));
    return;
  }
  const up = button(t("importer.up"), goUp, { iconPaths: icons.chevronLeft });
  up.classList.add("btn-small");
  const rows = entries.map(function entryRow(entry) {
    if (entry.dir) {
      const node = el(
        "button",
        { class: "browse-row browse-dir", type: "button", "data-path": entry.path },
        [icon(icons.chevronRight), el("span", {}, [entry.name])],
      );
      node.addEventListener("click", enterFolder);
      return node;
    }
    return imageRow(entry);
  });
  body.replaceChildren(
    el("div", { class: "browse-head" }, [
      up,
      el("code", { class: "browse-path" }, [device + "/" + folder]),
    ]),
    ...(rows.length > 0 ? rows : [el("p", { class: "muted" }, [t("importer.empty")])]),
  );
}

/** Adivina la categoría de una imagen por su nombre. */
export function guessCategory(name: string): string {
  if (/win/i.test(name)) {
    return "windows";
  }
  if (
    /ubuntu|debian|fedora|mint|arch|manjaro|suse|linux|pop|elementary|zorin|kali|alpine|centos|rocky|alma|mx/i.test(
      name,
    )
  ) {
    return "linux";
  }
  return "tools";
}

/** Fila de una imagen con su categoría y el botón "Añadir". */
function imageRow(entry: BrowseEntry): HTMLElement {
  const select = el("select", { class: "category-select", "aria-label": t("importer.category") });
  for (const category of ["windows", "linux", "tools"]) {
    const option = el("option", { value: category }, [t("library.categories." + category)]);
    option.selected = category === guessCategory(entry.name);
    select.append(option);
  }
  const add = button(t("importer.add"), importImage, {
    variant: "primary",
    iconPaths: icons.download,
  });
  add.classList.add("btn-small");
  add.dataset.path = entry.path;
  return el("div", { class: "browse-row browse-image" }, [
    icon(icons.fileDisc),
    el("div", { class: "device-main" }, [
      el("strong", {}, [entry.name]),
      el("span", { class: "muted" }, [fmt.bytes(entry.sizeBytes)]),
    ]),
    select,
    add,
  ]);
}

/** Pone en cola la copia de la imagen pulsada. */
async function importImage(event: MouseEvent): Promise<void> {
  const target = event.currentTarget as HTMLButtonElement;
  const select = target.parentElement?.querySelector<HTMLSelectElement>("select");
  target.disabled = true;
  try {
    await postJSON("/api/transfers/import", {
      device,
      path: target.dataset.path,
      category: select?.value ?? "tools",
    });
    target.replaceWith(badge(t("importer.added"), "ok"));
    await refreshVault();
  } catch (error) {
    target.disabled = false;
    target.after(errorBox(error));
  }
}

/** Desmonta el disco abierto, si lo hay (el agente lo impide si está en uso). */
function closeMedia(): void {
  if (device !== "") {
    postJSON("/api/media/unmount", { device }).catch(ignore);
  }
}

/** No hace nada: si el disco está en uso se desmontará más tarde. */
function ignore(): void {}

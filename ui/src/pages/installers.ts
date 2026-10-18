// * Instaladores en carpetas: lo que crea Rufus (y otras herramientas) cuando "extrae" Windows en un
// * USB en vez de copiar la ISO. Se buscan en los discos del equipo y se pueden empaquetar como una
// * ISO arrancable (BIOS y UEFI) que aparece en la biblioteca y en el menú de Rebootera.
// ! Solo se lee la carpeta de origen: nunca se modifica.

import { getJSON, postJSON, type InstallerFolder } from "../api.js";
import { badge, button, errorBox, panel } from "../components.js";
import { el, icon } from "../dom.js";
import * as fmt from "../format.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";
import { isDesktop } from "../mode.js";
import { navigate } from "../router.js";
import { refreshVault } from "../state.js";

/** Carpeta enviada a crear la ISO desde el botón pulsado. */
interface Target {
  device: string;
  path: string;
}

/** Calcula qué carpeta pide la API: ruta local en el escritorio, medio abierto en Rebootera OS. */
function targetOf(folder: InstallerFolder): Target {
  if (isDesktop()) {
    return { device: "", path: folder.root };
  }
  return { device: folder.device ?? "", path: folder.rel ?? "" };
}

/**
 * Crea la sección "Instaladores en carpetas": busca al abrir y ofrece volver a buscar y elegir una
 * carpeta a mano (solo en Rebootera Desktop, donde existe el diálogo nativo).
 */
export function installersSection(onMessage: (node: Node | null) => void): HTMLElement {
  const list = el("div", { class: "installer-list" });
  const root = el("div", { class: "installer-section" });

  /** Pone en cola la ISO de una carpeta y avisa. */
  async function make(target: Target, name: string): Promise<void> {
    try {
      await postJSON("/api/transfers/iso", target);
      onMessage(
        el("div", { class: "notice notice-ok" }, [
          icon(icons.check),
          el("span", {}, [t("installers.queued", { name })]),
          button(t("downloads.viewQueue"), openQueue, { iconPaths: icons.download }),
        ]),
      );
      await refreshVault();
    } catch (error) {
      onMessage(errorBox(error));
    }
  }

  /** Lleva al Centro de descargas, donde está la cola. */
  function openQueue(): void {
    navigate("downloads");
  }

  /** Fila de un instalador encontrado. */
  function row(folder: InstallerFolder): HTMLElement {
    const where = folder.rel ? folder.drive + " / " + folder.rel : folder.drive;
    const action = button(
      t("installers.make"),
      function onMake() {
        void make(targetOf(folder), folder.name);
      },
      { variant: "primary", iconPaths: icons.library },
    );
    action.classList.add("btn-small");
    const modes: string[] = [];
    if (folder.biosBoot) {
      modes.push("BIOS");
    }
    if (folder.uefiBoot) {
      modes.push("UEFI");
    }
    return el("article", { class: "image-row" }, [
      el("span", { class: "device-icon" }, [icon(icons.disk)]),
      el("div", { class: "device-main" }, [
        el("strong", { title: folder.root }, [folder.name]),
        el("span", { class: "muted" }, [where + " · " + fmt.bytes(folder.bytes)]),
      ]),
      el("div", { class: "image-state" }, [
        badge(
          modes.length > 0 ? modes.join(" + ") : t("installers.notBootable"),
          modes.length > 0 ? "ok" : "danger",
        ),
        folder.largeFile ? badge(t("installers.udf"), "accent") : null,
      ]),
      el("div", { class: "image-actions" }, [action]),
    ]);
  }

  /** Busca instaladores en los discos del equipo. */
  async function scan(): Promise<void> {
    list.replaceChildren(el("p", { class: "muted" }, [t("installers.scanning")]));
    try {
      const found = await getJSON<InstallerFolder[]>("/api/installer/found");
      if (found.length === 0) {
        list.replaceChildren(el("p", { class: "muted" }, [t("installers.none")]));
        return;
      }
      list.replaceChildren(...found.map(row));
    } catch (error) {
      list.replaceChildren(errorBox(error));
    }
  }

  /** Elige una carpeta con el diálogo del sistema y crea su ISO. */
  async function pickFolder(): Promise<void> {
    try {
      const picked = await postJSON<{ path: string }>("/api/desktop/pick-folder", {
        title: t("installers.pickTitle"),
      });
      if (!picked.path) {
        return;
      }
      const info = await postJSON<InstallerFolder>("/api/installer/inspect", {
        device: "",
        path: picked.path,
      });
      if (info.kind === "") {
        onMessage(errorBox(new Error(t("installers.notInstaller"))));
        return;
      }
      await make({ device: "", path: picked.path }, info.name);
    } catch (error) {
      onMessage(errorBox(error));
    }
  }

  const actions: HTMLElement[] = [
    button(
      t("installers.rescan"),
      function onRescan() {
        void scan();
      },
      { iconPaths: icons.refresh },
    ),
  ];
  if (isDesktop()) {
    actions.push(
      button(
        t("installers.pick"),
        function onPick() {
          void pickFolder();
        },
        { iconPaths: icons.library },
      ),
    );
  }
  root.append(
    panel(t("installers.title"), [el("p", { class: "muted" }, [t("installers.hint")]), list], {
      action: el("div", { class: "panel-actions" }, actions),
      iconPaths: icons.library,
    }),
  );
  void scan();
  return root;
}

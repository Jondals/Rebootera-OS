// * Página "Inicio" de Rebootera OS: una pregunta ("¿qué quieres hacer?") y cinco tarjetas grandes.
// * Lo técnico (procesador, discos, firmware) queda plegado en "Sobre este equipo".
// ! Todos los valores salen de state.ts; si un dato no existe en este equipo se dice, nunca se
// ! rellena con un ejemplo.

import type { Disk, SystemInfo } from "../api.js";
import { badge, button, more, progress, tile } from "../components.js";
import { openKernelLog } from "../dialogs.js";
import { el, icon } from "../dom.js";
import * as fmt from "../format.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";
import { internalDisks, liveMedia, onlineInterface, vault } from "../insights.js";
import { navigate, type Page } from "../router.js";
import { scanHardware, snapshot } from "../state.js";

/** Contenedores que se refrescan con cada actualización de datos. */
let refs: { status: HTMLElement; profile: HTMLElement; driversBadge: HTMLElement } | null = null;

/** Abre el asistente de instalación. */
function openInstall(): void {
  navigate("install");
}

/** Abre el Centro de descargas. */
function openDownloads(): void {
  navigate("downloads");
}

/** Abre las imágenes guardadas. */
function openLibrary(): void {
  navigate("library");
}

/** Abre los discos y particiones. */
function openDisks(): void {
  navigate("storage");
}

/** Abre los drivers. */
function openDrivers(): void {
  navigate("drivers");
}

/** Vuelve a analizar el hardware. */
function rescan(): void {
  scanHardware();
}

/** Dibuja la estructura fija de la página. */
function render(root: HTMLElement): void {
  const status = el("div", { class: "home-status" });
  const profile = el("div", { class: "profile-grid" });
  const driversBadge = el("span", { class: "tile-badge" });
  refs = { status, profile, driversBadge };

  const rescanButton = button(t("common.rescan"), rescan, { iconPaths: icons.refresh });
  rescanButton.classList.add("btn-small");
  const logButton = button(t("overview.kernelTitle"), openKernelLog, { iconPaths: icons.terminal });
  logButton.classList.add("btn-small");

  root.append(
    el("div", { class: "page home" }, [
      el("header", { class: "home-head" }, [
        el("h1", { class: "home-title" }, [t("home.title")]),
        el("p", { class: "home-subtitle" }, [t("home.subtitle")]),
        status,
      ]),
      el("div", { class: "tile-grid tile-grid-2" }, [
        tile(icons.install, t("home.install"), t("home.installText"), openInstall, {
          primary: true,
        }),
        tile(icons.download, t("home.download"), t("home.downloadText"), openDownloads),
        tile(icons.library, t("home.library"), t("home.libraryText"), openLibrary),
        tile(icons.usb, t("home.disks"), t("home.disksText"), openDisks),
        tile(icons.drivers, t("home.drivers"), t("home.driversText"), openDrivers, {
          badge: driversBadge,
        }),
      ]),
      more(t("home.thisComputer"), [
        profile,
        el("div", { class: "more-actions" }, [rescanButton, logButton]),
      ]),
    ]),
  );
}

/** Refresca todas las partes dinámicas. */
function update(): void {
  if (refs === null || !refs.profile.isConnected) {
    return;
  }
  const system = snapshot.system;
  renderStatus(system);
  renderProfile(system, snapshot.disks);
  const count = snapshot.devices?.length;
  refs.driversBadge.textContent = count === undefined ? "" : String(count);
  refs.driversBadge.hidden = count === undefined;
}

/** Línea de estado bajo el título: red y modo de arranque, sin jerga. */
function renderStatus(system: SystemInfo | null): void {
  if (refs === null || system === null) {
    return;
  }
  const online = onlineInterface(system) !== null;
  const mode = system.firmware === "uefi" ? t("home.uefi") : t("home.bios");
  refs.status.replaceChildren(
    badge(online ? t("home.online") : t("home.offline"), online ? "ok" : "warn"),
    badge(mode + (system.secureBoot === "on" ? " · Secure Boot" : ""), "muted"),
  );
}

/** Subtarjeta del perfil de hardware. */
function profileCard(iconPaths: string, label: string, children: (Node | null)[]): HTMLElement {
  return el("div", { class: "profile-card" }, [
    el("div", { class: "profile-card-head" }, [
      el("span", { class: "profile-icon" }, [icon(iconPaths)]),
      el("span", { class: "profile-label" }, [label]),
    ]),
    ...children,
  ]);
}

/** Dibuja las cuatro subtarjetas de "Sobre este equipo". */
function renderProfile(system: SystemInfo | null, disks: Disk[] | null): void {
  if (refs === null || system === null) {
    return;
  }
  const cards: HTMLElement[] = [];
  cards.push(
    profileCard(icons.monitor, t("overview.hostMachine"), [
      el("strong", { class: "profile-title" }, [
        [system.vendor, system.model].filter(Boolean).join(" ") || t("common.unknown"),
      ]),
      el("span", { class: "profile-line" }, [system.cpu || t("common.unknown")]),
      el("span", { class: "profile-line" }, [
        t("overview.threads", { n: system.cpuThreads }) + " · " + fmt.bytes(system.memoryBytes),
      ]),
    ]),
  );
  const internal = internalDisks(disks)[0];
  cards.push(
    profileCard(
      icons.disk,
      t("overview.internalStorage"),
      internal
        ? [
            el("strong", { class: "profile-title" }, [internal.model || internal.name]),
            el("span", { class: "profile-line" }, [
              fmt.bytes(internal.sizeBytes) +
                " · " +
                (internal.partitions.length > 0
                  ? t("overview.partitions", { n: internal.partitions.length })
                  : t("overview.noPartitions")),
            ]),
          ]
        : [el("span", { class: "profile-line" }, [t("overview.noInternal")])],
    ),
  );
  const media = liveMedia(disks);
  const store = vault(disks);
  cards.push(
    profileCard(icons.usb, t("overview.liveMedia"), [
      el("strong", { class: "profile-title" }, [
        media ? media.model || media.name : t("common.unknown"),
      ]),
      store
        ? progress(store.usedBytes / Math.max(1, store.usedBytes + store.freeBytes), "accent")
        : el("span", { class: "profile-line muted" }, [t("sidebar.readOnly")]),
    ]),
  );
  cards.push(
    profileCard(icons.shield, t("overview.firmwareSecurity"), [
      el("strong", { class: "profile-title" }, [secureBootText(system)]),
      el("span", { class: "profile-line" }, [
        system.tpm ? t("overview.tpm", { v: system.tpm }) : t("overview.noTpm"),
      ]),
    ]),
  );
  refs.profile.replaceChildren(...cards);
}

/** Texto del estado de Secure Boot. */
function secureBootText(system: SystemInfo): string {
  switch (system.secureBoot) {
    case "on":
      return t("topbar.secureBootOn");
    case "off":
      return t("topbar.secureBootOff");
    case "n/a":
      return t("topbar.legacy");
    default:
      return t("topbar.secureBootUnknown");
  }
}

export const overviewPage: Page = { render, update };

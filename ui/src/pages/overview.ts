// * Página "Resumen": portada de Rebootera OS. Saludo, tarjeta principal de
// * instalación, accesos a las secciones, perfil de hardware detectado y tres
// * widgets en vivo (temperaturas, tráfico de red y registro del kernel).
// ! Todos los valores salen de state.ts; si un dato no existe en este equipo
// ! se dice, nunca se rellena con un ejemplo.

import type { Disk, SystemInfo } from "../api.js";
import { bars, sparkline } from "../charts.js";
import { badge, button, monoLabel, panel, progress } from "../components.js";
import { el, icon } from "../dom.js";
import * as fmt from "../format.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";
import {
  internalDisks,
  isProblemLine,
  liveMedia,
  onlineInterface,
  primarySensor,
  sensorKey,
  vault,
} from "../insights.js";
import { openKernelLog } from "../dialogs.js";
import { navigate, type Page, type Route } from "../router.js";
import { scanHardware, snapshot } from "../state.js";

/** Colores de las particiones en la barra de distribución del disco. */
const PARTITION_TONES: Record<string, string> = {
  efi: "amber",
  msr: "muted",
  recovery: "violet",
  windows: "accent",
  linux: "ok",
  vault: "accent",
  "boot-tools": "amber",
};

/** Contenedores que se refrescan con cada actualización de datos. */
let refs: {
  bootStatus: HTMLElement;
  installBadges: HTMLElement;
  hardwareFooter: HTMLElement;
  profileMeta: HTMLElement;
  profile: HTMLElement;
  temps: HTMLElement;
  network: HTMLElement;
  kernel: HTMLElement;
} | null = null;

/** Navega a la ruta indicada en data-route del elemento pulsado. */
function goToRoute(event: MouseEvent): void {
  const target = event.currentTarget as HTMLElement;
  navigate(target.dataset.route as Route);
}

/** Abre el asistente de instalación. */
function openWizard(): void {
  navigate("install");
}

/** Abre la biblioteca. */
function openLibrary(): void {
  navigate("library");
}

/** Vuelve a analizar el hardware. */
function rescan(): void {
  scanHardware();
}

/** Tarjeta de acceso a una sección. */
function featureCard(
  route: Route,
  iconPaths: string,
  title: string,
  text: string,
  footer: Node,
): HTMLElement {
  const card = el("button", { class: "feature-card", type: "button", "data-route": route }, [
    el("span", { class: "feature-icon" }, [icon(iconPaths)]),
    el("span", { class: "feature-title" }, [title]),
    el("span", { class: "feature-text" }, [text]),
    el("span", { class: "feature-footer" }, [footer, icon(icons.chevronRight)]),
  ]);
  card.addEventListener("click", goToRoute);
  return card;
}

/** Dibuja la estructura fija de la página. */
function render(root: HTMLElement): void {
  const version = el("span", { class: "hero-version", id: "hero-version" });
  version.textContent = document.getElementById("version")?.textContent ?? "";

  const bootStatus = el("div", { class: "boot-status-value" });
  const installBadges = el("div", { class: "install-badges" });
  const hardwareFooter = el("span", { class: "mono-small accent" });
  const profileMeta = el("span", { class: "mono-small" });
  const profile = el("div", { class: "profile-grid" });
  const temps = el("div", { class: "widget-body" });
  const network = el("div", { class: "widget-body" });
  const kernel = el("div", { class: "widget-body" });
  refs = {
    bootStatus,
    installBadges,
    hardwareFooter,
    profileMeta,
    profile,
    temps,
    network,
    kernel,
  };

  const rescanButton = button(t("common.rescan"), rescan, { iconPaths: icons.refresh });
  rescanButton.classList.add("btn-small");

  const viewLog = button(t("overview.viewAll"), openKernelLog, {
    trailingIcon: icons.chevronRight,
  });
  viewLog.classList.add("btn-link");

  root.append(
    el("div", { class: "page overview" }, [
      el("section", { class: "hero" }, [
        el("div", { class: "hero-text" }, [
          el("div", { class: "hero-tag" }, [
            el("span", { class: "tag-pill" }, [
              el("span", { class: "pulse-dot" }),
              t("overview.tag"),
            ]),
            version,
          ]),
          el("h1", { class: "hero-title" }, [t("overview.title")]),
          el("p", { class: "hero-subtitle" }, [t("overview.subtitle")]),
        ]),
        el("div", { class: "boot-status" }, [
          el("span", { class: "boot-status-icon" }, [icon(icons.activity)]),
          el("div", {}, [monoLabel(t("overview.bootStatus")), bootStatus]),
        ]),
      ]),
      el("section", { class: "install-card" }, [
        el("span", { class: "install-icon" }, [icon(icons.install)]),
        el("div", { class: "install-text" }, [
          installBadges,
          el("h2", {}, [t("overview.installTitle")]),
          el("p", {}, [t("overview.installText")]),
        ]),
        el("div", { class: "install-actions" }, [
          button(t("overview.startWizard"), openWizard, {
            variant: "primary",
            trailingIcon: icons.arrowRight,
          }),
          button(t("overview.openLibrary"), openLibrary, { iconPaths: icons.library }),
        ]),
      ]),
      el("div", { class: "feature-grid" }, [
        featureCard(
          "downloads",
          icons.download,
          t("overview.downloadTitle"),
          t("overview.downloadText"),
          el("span", { class: "mono-small" }, [t("overview.downloadFooter")]),
        ),
        featureCard(
          "library",
          icons.library,
          t("overview.libraryTitle"),
          t("overview.libraryText"),
          el("span", { class: "mono-small ok" }, [t("overview.libraryFooter")]),
        ),
        featureCard(
          "storage",
          icons.recover,
          t("overview.recoverTitle"),
          t("overview.recoverText"),
          el("span", { class: "mono-small amber" }, [t("overview.recoverFooter")]),
        ),
        featureCard(
          "drivers",
          icons.drivers,
          t("overview.hardwareTitle"),
          t("overview.hardwareText"),
          hardwareFooter,
        ),
      ]),
      panel(t("overview.profileTitle"), [profile], {
        iconPaths: icons.cpu,
        action: el("div", { class: "panel-meta" }, [
          el("span", { class: "pulse-dot" }),
          profileMeta,
          rescanButton,
        ]),
      }),
      el("div", { class: "widget-grid" }, [
        panel(t("overview.tempsTitle"), [temps], {
          subtitle: t("overview.tempsSub"),
          className: "widget",
        }),
        panel(t("overview.netTitle"), [network], {
          subtitle: t("overview.netSub"),
          className: "widget",
        }),
        panel(t("overview.kernelTitle"), [kernel], {
          subtitle: t("overview.kernelSub"),
          className: "widget",
          action: icon(icons.terminal, "icon panel-icon"),
        }),
      ]),
    ]),
  );
  // * El enlace "Ver todo" va al pie del widget del kernel.
  kernel.after(el("div", { class: "widget-footer" }, [viewLog]));
}

/** Refresca todas las partes dinámicas. */
function update(): void {
  if (refs === null || !refs.profile.isConnected) {
    return;
  }
  const system = snapshot.system;
  renderBootStatus(system);
  renderProfile(system, snapshot.disks);
  renderTemps();
  renderNetwork(system);
  renderKernel();
  const count = snapshot.devices?.length;
  refs.hardwareFooter.textContent =
    count === undefined ? t("common.loading") : t("overview.hardwareFooter", { n: count });
  refs.profileMeta.textContent = snapshot.hardwareScannedAt
    ? t("overview.scanned", { time: fmt.ago((Date.now() - snapshot.hardwareScannedAt) / 1000) })
    : t("common.loading");
}

/** Estado del arranque y etiquetas de la tarjeta de instalación. */
function renderBootStatus(system: SystemInfo | null): void {
  if (refs === null || system === null) {
    return;
  }
  const mode = system.firmware === "uefi" ? "UEFI" : "BIOS";
  const secure = system.secureBoot === "on" ? " · Secure Boot" : "";
  refs.bootStatus.replaceChildren(
    el("strong", {}, [t("overview.liveReady")]),
    el("span", {}, [mode + secure]),
  );
  refs.installBadges.replaceChildren(
    badge(t("overview.recommended"), "accent"),
    monoLabel(system.firmware === "uefi" ? t("overview.uefiNative") : t("overview.legacyMode")),
  );
}

/** Subtarjeta del perfil de hardware. */
function profileCard(
  iconPaths: string,
  label: string,
  children: (Node | null)[],
  aside?: Node,
): HTMLElement {
  return el("div", { class: "profile-card" }, [
    el("div", { class: "profile-card-head" }, [
      el("span", { class: "profile-icon" }, [icon(iconPaths)]),
      monoLabel(label),
      aside ?? null,
    ]),
    ...children,
  ]);
}

/** Barra con la distribución de particiones de un disco. */
function partitionBar(disk: Disk): HTMLElement {
  const barNode = el("div", { class: "partition-bar", role: "presentation" });
  for (const part of disk.partitions) {
    const segment = el("span", {
      class: "segment segment-" + (PARTITION_TONES[part.role] ?? "muted"),
    });
    segment.style.flexGrow = String(Math.max(part.sizeBytes / disk.sizeBytes, 0.01));
    segment.title = part.name + " · " + fmt.bytes(part.sizeBytes);
    barNode.append(segment);
  }
  return barNode;
}

/** Dibuja las cuatro subtarjetas del perfil de hardware. */
function renderProfile(system: SystemInfo | null, disks: Disk[] | null): void {
  if (refs === null || system === null) {
    return;
  }
  const cards: HTMLElement[] = [];

  cards.push(
    profileCard(
      icons.monitor,
      t("overview.hostMachine"),
      [
        el("strong", { class: "profile-title" }, [
          [system.vendor, system.model].filter(Boolean).join(" ") || t("common.unknown"),
        ]),
        el("span", { class: "profile-line" }, [system.cpu || t("common.unknown")]),
        el("span", { class: "profile-line mono-small" }, [
          t("overview.threads", { n: system.cpuThreads }) + " · " + fmt.bytes(system.memoryBytes),
        ]),
      ],
      system.isVirtual ? badge("VM", "violet") : undefined,
    ),
  );

  const internal = internalDisks(disks)[0];
  cards.push(
    internal
      ? profileCard(
          icons.disk,
          t("overview.internalStorage"),
          [
            el("strong", { class: "profile-title" }, [internal.model || internal.name]),
            el("span", { class: "profile-line mono-small" }, [
              fmt.bytes(internal.sizeBytes) +
                " · " +
                internal.transport.toUpperCase() +
                " · " +
                (internal.partitions.length > 0
                  ? t("overview.partitions", { n: internal.partitions.length })
                  : t("overview.noPartitions")),
            ]),
            partitionBar(internal),
          ],
          badge(t("common.protected"), "ok"),
        )
      : profileCard(icons.disk, t("overview.internalStorage"), [
          el("span", { class: "profile-line" }, [t("overview.noInternal")]),
        ]),
  );

  const media = liveMedia(disks);
  const store = vault(disks);
  cards.push(
    profileCard(
      icons.usb,
      t("overview.liveMedia"),
      [
        el("strong", { class: "profile-title" }, [
          media ? media.model || media.name : t("common.unknown"),
        ]),
        el("span", { class: "profile-line mono-small" }, [
          media ? fmt.bytes(media.sizeBytes) + " · " + media.transport.toUpperCase() : "",
        ]),
        store
          ? progress(store.usedBytes / Math.max(1, store.usedBytes + store.freeBytes), "amber")
          : el("span", { class: "profile-line muted" }, [t("sidebar.readOnly")]),
      ],
      media ? badge(media.transport.toUpperCase(), "muted") : undefined,
    ),
  );

  const secureTone =
    system.secureBoot === "on" ? "ok" : system.secureBoot === "n/a" ? "muted" : "warn";
  cards.push(
    profileCard(
      icons.shield,
      t("overview.firmwareSecurity"),
      [
        el("strong", { class: "profile-title" }, [secureBootText(system)]),
        el("span", { class: "profile-line" }, [
          t("overview.firmwareMode", { mode: system.firmware === "uefi" ? "UEFI" : "BIOS" }),
        ]),
        el("span", { class: "profile-line mono-small" }, [
          [system.biosVendor, system.biosVersion].filter(Boolean).join(" "),
        ]),
      ],
      badge(
        system.tpm ? t("overview.tpm", { v: system.tpm }) : t("overview.noTpm"),
        system.tpm ? secureTone : "muted",
      ),
    ),
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

/** Widget de temperaturas: sensor principal con su gráfica y otros dos. */
function renderTemps(): void {
  if (refs === null) {
    return;
  }
  const main = primarySensor(snapshot.sensors);
  if (main === null) {
    refs.temps.replaceChildren(el("p", { class: "widget-empty" }, [t("overview.noSensors")]));
    return;
  }
  const others = (snapshot.sensors ?? []).filter(isNotSame).slice(0, 2);

  /** Descarta el sensor principal de la lista de otros. */
  function isNotSame(reading: typeof main): boolean {
    return reading !== null && main !== null && sensorKey(reading) !== sensorKey(main);
  }

  refs.temps.replaceChildren(
    el("div", { class: "widget-headline" }, [
      el("span", { class: "widget-caption" }, [main.label]),
      badge(
        fmt.celsius(main.celsius),
        main.celsius >= 80 ? "danger" : main.celsius >= 65 ? "warn" : "accent",
      ),
    ]),
    sparkline(snapshot.tempHistory.get(sensorKey(main)) ?? []),
    el(
      "div",
      { class: "widget-stats" },
      others.map(function sensorStat(reading) {
        return el("span", {}, [
          el("span", { class: "muted" }, [reading.label + " "]),
          el("strong", {}, [fmt.celsius(reading.celsius)]),
        ]);
      }),
    ),
  );
}

/** Widget de red: tráfico recibido por segundo y datos de la conexión. */
function renderNetwork(system: SystemInfo | null): void {
  if (refs === null) {
    return;
  }
  const iface = onlineInterface(system);
  if (iface === null) {
    refs.network.replaceChildren(el("p", { class: "widget-empty" }, [t("overview.noNetwork")]));
    return;
  }
  const current = snapshot.rxHistory[snapshot.rxHistory.length - 1] ?? 0;
  refs.network.replaceChildren(
    el("div", { class: "widget-headline" }, [
      el("span", { class: "widget-caption" }, [iface.kind === "wifi" ? "Wi-Fi" : "Ethernet"]),
      badge(fmt.rate(current), "accent"),
    ]),
    bars(snapshot.rxHistory),
    el("div", { class: "widget-stats" }, [
      el("span", {}, [
        el("span", { class: "muted" }, [iface.name + " "]),
        el("strong", {}, [iface.addresses[0]]),
      ]),
      iface.speedMbps > 0 ? el("strong", {}, [fmt.linkSpeed(iface.speedMbps)]) : null,
    ]),
  );
}

/** Widget del registro del kernel: últimas líneas y recuento de problemas. */
function renderKernel(): void {
  if (refs === null) {
    return;
  }
  const lines = snapshot.kernelLog;
  if (lines === null) {
    refs.kernel.replaceChildren(el("p", { class: "widget-empty" }, [t("common.loading")]));
    return;
  }
  const problems = lines.filter(isProblemLine).length;
  refs.kernel.replaceChildren(
    el(
      "pre",
      { class: "log-preview" },
      lines.slice(-5).map(function logLine(line) {
        return el("span", { class: isProblemLine(line) ? "log-line log-problem" : "log-line" }, [
          line + "\n",
        ]);
      }),
    ),
    el("div", { class: "widget-stats" }, [
      el("span", { class: problems > 0 ? "warn" : "ok" }, [
        problems > 0 ? t("overview.kernelErrors", { n: problems }) : t("overview.kernelClean"),
      ]),
    ]),
  );
}

export const overviewPage: Page = { render, update };

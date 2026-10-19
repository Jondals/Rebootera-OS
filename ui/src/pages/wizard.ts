// * Página "Asistente de instalación": seis pasos para instalar un sistema.
// * Elegir sistema, revisar hardware, drivers críticos, disco de destino,
// * ajustes e instalar. Los pasos de revisión usan datos reales del equipo.
// ! Nada de esta página escribe en los discos del equipo. Elegir un destino
// ! solo lo anota en el plan. El último paso preselecciona la imagen en el
// ! menú de Ventoy (un archivo del USB) y reinicia, tras una confirmación
// ! explícita; el particionado lo hace el instalador original, que pregunta.

import {
  getJSON,
  postJSON,
  type Disk,
  type DriverMatch,
  type DriverPlan,
  type LibraryItem,
  type SystemInfo,
} from "../api.js";
import {
  badge,
  button,
  emptyState,
  errorBox,
  more,
  pageHeader,
  panel,
  type Tone,
} from "../components.js";
import { el, icon } from "../dom.js";
import * as fmt from "../format.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";
import { prefs } from "../prefs.js";
import { navigate, type Page } from "../router.js";
import { snapshot } from "../state.js";

/** Claves de los pasos, en orden. */
const STEPS = ["stepOs", "stepTarget", "stepSettings", "stepDeploy"];

/** Tamaño mínimo de disco para Windows 11 (64 GB). */
const MIN_DISK = 64e9;
/** Memoria mínima para Windows 11 (4 GB, con margen por la memoria reservada). */
const MIN_RAM = 3.8e9;

/** Idiomas con respuestas automáticas (claves de wizard.languages). */
const ANSWER_LANGUAGES = ["es", "en", "gb", "de", "fr", "it", "pt"];

/** Zonas horarias que ofrece el asistente (las mismas que conoce Core). */
const TIME_ZONES = [
  "UTC",
  "Europe/Madrid",
  "Atlantic/Canary",
  "Europe/London",
  "Europe/Lisbon",
  "Europe/Paris",
  "Europe/Berlin",
  "Europe/Rome",
  "Europe/Amsterdam",
  "Europe/Athens",
  "Europe/Moscow",
  "America/New_York",
  "America/Chicago",
  "America/Denver",
  "America/Los_Angeles",
  "America/Mexico_City",
  "America/Bogota",
  "America/Lima",
  "America/Santiago",
  "America/Argentina/Buenos_Aires",
  "America/Sao_Paulo",
  "Asia/Tokyo",
  "Asia/Shanghai",
  "Asia/Kolkata",
  "Australia/Sydney",
];

/** Respuestas automáticas que el usuario ha escrito. */
interface Answers {
  enabled: boolean;
  language: string;
  timeZone: string;
  userName: string;
  fullName: string;
  password: string;
}

let step = 0;
let selectedDisk = "";
let selectedImage = "";
let understood = false;
let driverPlan: DriverPlan | null = null;
let planRequested = false;
let takeDrivers = false;
let winBypass = false;
let privacy = false;
let bootOnce = false;
const answers: Answers = {
  enabled: false,
  language: prefs.language === "es" ? "es" : "en",
  timeZone: "",
  userName: "",
  fullName: "",
  password: "",
};
let stepper: HTMLElement | null = null;
let content: HTMLElement | null = null;
let footer: HTMLElement | null = null;
let lastSignature = "";

/** Va al paso indicado en data-step. */
function goToStep(event: MouseEvent): void {
  step = Number((event.currentTarget as HTMLElement).dataset.step);
  update();
}

/** Paso anterior. */
function previousStep(): void {
  step = Math.max(0, step - 1);
  update();
}

/** Paso siguiente. */
function nextStep(): void {
  step = Math.min(STEPS.length - 1, step + 1);
  update();
}

/** Abre la biblioteca. */
function openLibrary(): void {
  navigate("library");
}

/** Elige la imagen que se instalará (también desde la biblioteca). */
export function chooseImage(path: string): void {
  selectedImage = path;
  understood = false;
  step = 1;
}

/** Anota la imagen pulsada en el paso 1. */
function selectImage(event: Event): void {
  selectedImage = (event.currentTarget as HTMLInputElement).value;
  understood = false;
  update();
}

/** Marca o desmarca la confirmación del último paso. */
function toggleUnderstood(event: Event): void {
  understood = (event.currentTarget as HTMLInputElement).checked;
  lastSignature = "";
  update();
}

/** Devuelve la imagen elegida si sigue en la biblioteca. */
function chosenItem(): LibraryItem | undefined {
  return snapshot.library.find(function samePath(item) {
    return item.path === selectedImage;
  });
}

/** Sistema de una imagen: "windows", "debian" u "other" (como lo deduce Core). */
function systemOf(item: LibraryItem): "windows" | "debian" | "other" {
  const name = item.name.toLowerCase();
  if (item.category === "windows" || name.startsWith("win")) {
    return "windows";
  }
  if (item.source.startsWith("debian") || name.startsWith("debian-")) {
    return "debian";
  }
  return "other";
}

/** Construye el cuerpo de la petición de instalar, o un error si falta algo. */
function installRequest(item: LibraryItem): Record<string, unknown> | Error {
  const system = systemOf(item);
  const request: Record<string, unknown> = {
    path: item.path,
    drivers: takeDrivers && system === "windows",
    winBypass: winBypass && system === "windows",
    privacy: privacy && system === "windows",
    bootOnce: bootOnce && snapshot.system?.firmware === "uefi",
  };
  if (answers.enabled && system !== "other") {
    if (answers.userName !== "" && answers.password === "") {
      return new Error(t("wizard.answersNeedPassword"));
    }
    request.unattended = {
      language: answers.language,
      timeZone: answers.timeZone,
      userName: answers.userName,
      fullName: answers.fullName,
      password: answers.password,
    };
  }
  return request;
}

/** Pide a Core el casado de drivers del Vault con este equipo (una vez). */
async function loadDriverPlan(): Promise<void> {
  if (planRequested) {
    return;
  }
  planRequested = true;
  try {
    driverPlan = await getJSON<DriverPlan>("/api/drivers/plan");
  } catch {
    driverPlan = null;
  }
  lastSignature = "";
  update();
}

/** Preselecciona la imagen en Ventoy y reinicia el equipo. */
async function deploy(event: MouseEvent): Promise<void> {
  const item = chosenItem();
  const target = event.currentTarget as HTMLButtonElement;
  if (!item || !understood) {
    return;
  }
  const request = installRequest(item);
  if (request instanceof Error) {
    content?.prepend(errorBox(request));
    return;
  }
  target.disabled = true;
  try {
    await postJSON("/api/boot/install", request);
    const overlay = document.getElementById("overlay") as HTMLElement;
    (document.getElementById("overlay-text") as HTMLElement).textContent = t(
      "wizard.restartingInto",
      { name: item.name },
    );
    overlay.hidden = false;
  } catch (error) {
    target.disabled = false;
    content?.prepend(errorBox(error));
  }
}

/** Anota el disco elegido como destino. */
function selectDisk(event: Event): void {
  selectedDisk = (event.currentTarget as HTMLInputElement).value;
  update();
}

/** Dibuja la estructura fija de la página. */
function render(root: HTMLElement): void {
  // * Cada vez que se abre el asistente se vuelve a mirar el Vault (puede haber cambiado).
  planRequested = false;
  driverPlan = null;
  stepper = el("ol", { class: "stepper" });
  content = el("div", { class: "wizard-content" });
  footer = el("footer", { class: "wizard-footer" });
  lastSignature = "";
  root.append(
    el("div", { class: "page" }, [
      pageHeader(t("wizard.title"), t("wizard.subtitle")),
      stepper,
      content,
      footer,
    ]),
  );
}

/** Dibuja la barra de pasos. */
function renderStepper(): void {
  if (stepper === null) {
    return;
  }
  stepper.replaceChildren(
    ...STEPS.map(function stepItem(key, index) {
      const state = index < step ? "done" : index === step ? "current" : "pending";
      const node = el("li", {}, [
        el(
          "button",
          {
            class: "step step-" + state,
            type: "button",
            "data-step": String(index),
            "aria-current": index === step ? "step" : "false",
          },
          [
            el("span", { class: "step-index" }, [
              index < step ? icon(icons.check) : String(index + 1),
            ]),
            el("span", { class: "step-text" }, [el("span", {}, [t("wizard." + key)])]),
          ],
        ),
      ]);
      node.querySelector("button")?.addEventListener("click", goToStep);
      return node;
    }),
  );
}

/** Fila de requisito con su estado. */
function requirement(label: string, state: "ok" | "warn" | "fail", detail: string): HTMLElement {
  const tones: Record<string, Tone> = { ok: "ok", warn: "warn", fail: "danger" };
  return el("div", { class: "req-row" }, [
    el("span", { class: "req-icon req-" + state }, [
      icon(state === "ok" ? icons.check : icons.alert),
    ]),
    el("div", {}, [el("strong", {}, [label]), el("span", { class: "muted" }, [detail])]),
    badge(t("wizard." + state), tones[state]),
  ]);
}

/** Paso 1: elegir sistema entre las imágenes utilizables de la biblioteca. */
function stepChooseOs(): Node[] {
  const usable = snapshot.library.filter(isInstallable);
  const incompatible = snapshot.library.filter(isIncompatible);
  // * Las imágenes que se sabe que no arrancan se muestran aparte, con la alternativa.
  const incompatibleNote =
    incompatible.length > 0
      ? el("div", { class: "notice" }, [
          icon(icons.alert),
          el("span", {}, [
            incompatible.map(itemName).join(", ") + ": " + t("wizard.incompatibleNote"),
          ]),
        ])
      : null;
  if (usable.length > 0) {
    return [
      panel(t("wizard.chooseOsTitle"), [...usable.map(imageCard), incompatibleNote], {
        subtitle: t("wizard.chooseOsHint"),
        iconPaths: icons.library,
      }),
    ];
  }
  if (incompatibleNote) {
    return [panel(t("wizard.chooseOsTitle"), [incompatibleNote], { iconPaths: icons.library })];
  }
  return [
    panel(t("wizard.chooseOsTitle"), [
      emptyState(
        icons.library,
        t("wizard.chooseOsEmpty"),
        t("wizard.chooseOsEmptyHint"),
        button(t("wizard.goLibrary"), openLibrary, {
          variant: "primary",
          trailingIcon: icons.arrowRight,
        }),
      ),
    ]),
  ];
}

/** Paso 2: requisitos de hardware con datos reales. */
function stepInspect(system: SystemInfo | null, disks: Disk[] | null): Node[] {
  if (system === null) {
    return [el("p", { class: "muted" }, [t("common.loading")])];
  }
  const biggest = Math.max(0, ...(disks ?? []).filter(isCandidate).map(diskSize));
  const devices = snapshot.devices ?? [];
  const storageOk = devices.filter(isStorageController).every(hasDriver);
  const netDevices = devices.filter(isNetworkDevice);
  return [
    panel(
      t("wizard.inspectTitle"),
      [
        requirement(
          t("wizard.reqUefi"),
          system.firmware === "uefi" ? "ok" : "fail",
          system.firmware.toUpperCase(),
        ),
        requirement(
          t("wizard.reqSecureBoot"),
          system.secureBoot === "on" ? "ok" : system.firmware === "uefi" ? "warn" : "fail",
          system.secureBoot === "on" ? t("topbar.secureBootOn") : t("topbar.secureBootOff"),
        ),
        requirement(
          t("wizard.reqTpm"),
          system.tpm === "2.0" ? "ok" : "fail",
          system.tpm ? "TPM " + system.tpm : t("overview.noTpm"),
        ),
        requirement(
          t("wizard.reqRam"),
          system.memoryBytes >= MIN_RAM ? "ok" : "fail",
          fmt.bytes(system.memoryBytes),
        ),
        requirement(
          t("wizard.reqDisk"),
          biggest >= MIN_DISK ? "ok" : "fail",
          biggest > 0 ? fmt.bytes(biggest) : t("storage.noDisks"),
        ),
        requirement(
          t("wizard.reqStorage"),
          storageOk ? "ok" : "warn",
          storageOk ? t("wizard.ok") : t("drivers.noDriver"),
        ),
        requirement(
          t("wizard.reqNet"),
          netDevices.some(hasDriver) ? "ok" : "warn",
          netDevices.filter(hasDriver).map(deviceName).join(", ") || t("drivers.noDriver"),
        ),
      ],
      { subtitle: t("wizard.inspectText"), iconPaths: icons.cpu },
    ),
  ];
}

/** Paso 3: dispositivos críticos y su driver. */
function stepDrivers(): Node[] {
  const critical = (snapshot.devices ?? []).filter(isCritical);
  const missing = critical.filter(isWithoutDriver);
  return [
    panel(
      t("wizard.driversTitle"),
      [
        missing.length === 0 ? el("p", { class: "ok" }, [t("wizard.driversAllGood")]) : null,
        ...critical.map(function criticalRow(device) {
          return requirement(
            deviceName(device),
            device.driver ? "ok" : "warn",
            device.driver || device.hardwareIds[device.hardwareIds.length - 1],
          );
        }),
        el("div", { class: "notice" }, [
          icon(icons.alert),
          el("span", {}, [t("wizard.driversWindowsNote")]),
        ]),
      ],
      { subtitle: t("wizard.driversText"), iconPaths: icons.drivers },
    ),
    ...stepUsbDrivers(),
  ];
}

/** Etiqueta del estado del catálogo de firma de un paquete de drivers. */
function signatureBadge(signature: string): HTMLElement {
  const tone: Tone = signature === "present" ? "ok" : signature === "invalid" ? "danger" : "warn";
  const key = "wizard.driversSig" + signature.charAt(0).toUpperCase() + signature.slice(1);
  return badge(t(key), tone);
}

/** Fila de un dispositivo con el paquete de drivers que le corresponde. */
function driverMatchRow(match: DriverMatch): HTMLElement {
  const pack = match.package;
  return el("div", { class: "driver-match" }, [
    el("div", {}, [
      el("strong", {}, [match.name || match.vendor || match.address]),
      el("div", { class: "mono-small muted" }, [
        pack ? pack.inf + " · " + pack.provider + " " + pack.version : "",
      ]),
      el("div", { class: "mono-small muted" }, [
        t("wizard.driversMatchedBy", { id: match.hardwareId }),
      ]),
    ]),
    pack ? signatureBadge(pack.signature) : null,
  ]);
}

/** Drivers del USB que casan con este equipo y opción de llevarlos a Windows. */
function stepUsbDrivers(): Node[] {
  if (driverPlan === null) {
    return [];
  }
  const item = chosenItem();
  const windows = item !== undefined && systemOf(item) === "windows";
  const nodes: (Node | null)[] = [];
  if (driverPlan.matches.length > 0) {
    nodes.push(...driverPlan.matches.map(driverMatchRow));
  } else {
    nodes.push(el("p", { class: "muted" }, [t("wizard.driversNone")]));
  }
  if (driverPlan.missing.length > 0) {
    nodes.push(
      el("strong", {}, [t("wizard.driversMissingTitle")]),
      ...driverPlan.missing.map(missingRow),
    );
  }
  if (driverPlan.matches.length > 0) {
    if (windows) {
      const check = el("input", { type: "checkbox", checked: takeDrivers });
      check.addEventListener("change", toggleTakeDrivers);
      nodes.push(
        el("label", { class: "confirm-row" }, [check, el("span", {}, [t("wizard.driversTake")])]),
        el("p", { class: "muted small-note" }, [t("wizard.driversTakeHint")]),
      );
    } else {
      nodes.push(el("p", { class: "muted small-note" }, [t("wizard.driversOnlyWindows")]));
    }
  }
  return [
    panel(t("wizard.driversMatchTitle"), nodes, {
      subtitle: t("wizard.driversMatchText"),
      iconPaths: icons.usb,
    }),
  ];
}

/** Fila de un dispositivo crítico sin paquete. */
function missingRow(match: DriverMatch): HTMLElement {
  return el("div", { class: "driver-match" }, [
    el("span", {}, [match.name || match.vendor || match.address]),
    badge(t("drivers.noDriver"), "warn"),
  ]);
}

/** Marca o desmarca llevar los drivers al instalador. */
function toggleTakeDrivers(event: Event): void {
  takeDrivers = (event.currentTarget as HTMLInputElement).checked;
  lastSignature = "";
  update();
}

/** Paso 4: disco de destino. */
function stepTarget(disks: Disk[] | null): Node[] {
  const candidates = (disks ?? []).filter(isCandidate);
  if (candidates.length === 0) {
    return [panel(t("wizard.targetTitle"), [emptyState(icons.disk, t("wizard.noTargets"), "")])];
  }
  return [
    panel(t("wizard.targetTitle"), candidates.map(targetCard), {
      subtitle: t("wizard.targetText"),
      iconPaths: icons.disk,
    }),
  ];
}

/** Tarjeta seleccionable de un disco de destino. */
function targetCard(disk: Disk): HTMLElement {
  const input = el("input", {
    type: "radio",
    name: "target",
    value: disk.name,
    checked: selectedDisk === disk.name,
  });
  input.addEventListener("change", selectDisk);
  return el(
    "label",
    { class: "target-card" + (selectedDisk === disk.name ? " target-selected" : "") },
    [
      input,
      el("span", { class: "disk-icon" }, [icon(icons.disk)]),
      el("div", { class: "disk-title" }, [
        el("strong", {}, [[disk.vendor, disk.model].filter(Boolean).join(" ") || disk.name]),
        el("span", { class: "mono-small muted" }, [
          [
            disk.path,
            fmt.bytes(disk.sizeBytes),
            disk.transport.toUpperCase(),
            t("overview.partitions", { n: disk.partitions.length }),
          ].join(" · "),
        ]),
      ]),
      badge(t("storage.roles." + disk.role), disk.role === "internal" ? "violet" : "amber"),
      disk.protected
        ? el("span", { class: "guard-chip" }, [icon(icons.lock), t("common.protected")])
        : null,
    ],
  );
}

/** Tarjeta seleccionable de una imagen de la biblioteca. */
function imageCard(item: LibraryItem): HTMLElement {
  const input = el("input", {
    type: "radio",
    name: "image",
    value: item.path,
    checked: selectedImage === item.path,
  });
  input.addEventListener("change", selectImage);
  return el(
    "label",
    { class: "target-card" + (selectedImage === item.path ? " target-selected" : "") },
    [
      input,
      el("span", { class: "disk-icon" }, [icon(icons.library)]),
      el("div", { class: "disk-title" }, [
        el("strong", {}, [item.title || item.name]),
        el("span", { class: "mono-small muted" }, [
          fmt.bytes(item.sizeBytes) + " · " + t("library.categories." + item.category),
        ]),
      ]),
      badge(
        t("library.states." + item.state),
        item.state === "verified" ? "ok" : item.state === "imported" ? "violet" : "muted",
      ),
    ],
  );
}

/** Paso 6: confirmación explícita y despliegue. */
function stepDeploy(): Node[] {
  const item = chosenItem();
  if (!snapshot.vault?.available) {
    return [
      panel(t("wizard.deployTitle"), [
        el("div", { class: "notice" }, [
          icon(icons.lock),
          el("span", {}, [t("wizard.deployNoVault")]),
        ]),
      ]),
    ];
  }
  if (!item) {
    return [
      panel(t("wizard.deployTitle"), [
        el("div", { class: "notice" }, [
          icon(icons.info),
          el("span", {}, [t("wizard.deployNeedsImage")]),
        ]),
      ]),
    ];
  }
  const check = el("input", { type: "checkbox", checked: understood });
  check.addEventListener("change", toggleUnderstood);
  const go = button(t("wizard.deployButton"), deploy, {
    variant: "primary",
    iconPaths: icons.reboot,
  });
  go.disabled = !understood;
  return [
    ...stepSummary(),
    panel(
      t("wizard.deployTitle"),
      [
        el("p", {}, [t("wizard.deployText", { name: item.name })]),
        item.state !== "verified"
          ? el("div", { class: "notice" }, [
              icon(icons.alert),
              el("span", {}, [t("wizard.uncheckedNote")]),
            ])
          : null,
        el("label", { class: "confirm-row" }, [check, el("span", {}, [t("wizard.deployCheck")])]),
        el("div", { class: "deploy-actions" }, [go]),
        el("p", { class: "muted small-note" }, [t("wizard.bootMenuNote")]),
      ],
      { iconPaths: icons.install },
    ),
  ];
}

/** Guarda lo que se escribe en un campo de las respuestas automáticas. */
function onAnswerInput(event: Event): void {
  const input = event.currentTarget as HTMLInputElement | HTMLSelectElement;
  const field = input.dataset.field as
    | "language"
    | "timeZone"
    | "userName"
    | "fullName"
    | "password";
  answers[field] = input.value;
}

/** Marca o desmarca responder automáticamente. */
function toggleAnswers(event: Event): void {
  answers.enabled = (event.currentTarget as HTMLInputElement).checked;
  lastSignature = "";
  update();
}

/** Marca o desmarca saltarse los requisitos de Windows 11. */
function toggleWinBypass(event: Event): void {
  winBypass = (event.currentTarget as HTMLInputElement).checked;
  lastSignature = "";
  update();
}

/** Marca o desmarca reducir la telemetría de Windows. */
function togglePrivacy(event: Event): void {
  privacy = (event.currentTarget as HTMLInputElement).checked;
  lastSignature = "";
  update();
}

/** Marca o desmarca arrancar el instalador una sola vez. */
function toggleBootOnce(event: Event): void {
  bootOnce = (event.currentTarget as HTMLInputElement).checked;
  lastSignature = "";
  update();
}

/** Campo de texto o de lista con su etiqueta. */
function field(label: string, control: HTMLElement, hint = ""): HTMLElement {
  return el("label", { class: "field" }, [
    el("span", {}, [label]),
    control,
    hint ? el("span", { class: "field-hint" }, [hint]) : null,
  ]);
}

/** Entrada de texto ligada a un campo de las respuestas. */
function textInput(name: string, type: string, value: string): HTMLInputElement {
  const input = el("input", {
    class: "text-input",
    type,
    value,
    "data-field": name,
    autocomplete: "off",
    maxlength: name === "userName" ? "32" : "64",
  });
  input.value = value;
  input.addEventListener("input", onAnswerInput);
  return input;
}

/** Lista desplegable ligada a un campo de las respuestas. */
function selectInput(name: string, options: [string, string][], value: string): HTMLSelectElement {
  const select = el("select", { class: "text-input", "data-field": name });
  for (const [optionValue, label] of options) {
    const option = el("option", { value: optionValue }, [label]);
    option.selected = optionValue === value;
    select.append(option);
  }
  select.addEventListener("change", onAnswerInput);
  return select;
}

/** Formulario de las respuestas automáticas. */
function answersForm(): HTMLElement {
  return el("div", { class: "answers-form" }, [
    field(
      t("wizard.answersLanguage"),
      selectInput("language", ANSWER_LANGUAGES.map(languageOption), answers.language),
    ),
    field(
      t("wizard.answersTimeZone"),
      selectInput(
        "timeZone",
        [["", t("wizard.answersTimeZoneAsk")], ...TIME_ZONES.map(zoneOption)],
        answers.timeZone,
      ),
    ),
    field(
      t("wizard.answersUser"),
      textInput("userName", "text", answers.userName),
      t("wizard.answersUserHint"),
    ),
    field(t("wizard.answersFullName"), textInput("fullName", "text", answers.fullName)),
    field(t("wizard.answersPassword"), textInput("password", "password", answers.password)),
  ]);
}

/** Opción de idioma para la lista. */
function languageOption(code: string): [string, string] {
  return [code, t("wizard.languages." + code)];
}

/** Opción de zona horaria para la lista. */
function zoneOption(zone: string): [string, string] {
  return [zone, zone.replaceAll("_", " ")];
}

/** Casilla con título y nota. */
function optionRow(
  title: string,
  note: string,
  checked: boolean,
  onChange: (event: Event) => void,
): HTMLElement {
  const check = el("input", { type: "checkbox", checked });
  check.addEventListener("change", onChange);
  return el("div", { class: "option-block" }, [
    el("label", { class: "confirm-row" }, [check, el("span", {}, [title])]),
    el("p", { class: "muted small-note" }, [note]),
  ]);
}

/** Paso 5: respuestas automáticas, requisitos de Windows 11 y arranque único. */
function stepInstallerSettings(): Node[] {
  const item = chosenItem();
  if (!item) {
    return [];
  }
  const system = systemOf(item);
  const blocks: (Node | null)[] = [];
  if (system === "other") {
    blocks.push(el("p", { class: "muted" }, [t("wizard.answersUnsupported")]));
  } else {
    blocks.push(
      optionRow(t("wizard.answersEnable"), t("wizard.answersText"), answers.enabled, toggleAnswers),
    );
    if (answers.enabled) {
      blocks.push(answersForm());
    }
  }
  if (system === "windows") {
    blocks.push(
      optionRow(t("wizard.winBypassTitle"), t("wizard.winBypassNote"), winBypass, toggleWinBypass),
      optionRow(t("wizard.privacyTitle"), t("wizard.privacyNote"), privacy, togglePrivacy),
    );
  }
  if (snapshot.system?.firmware === "uefi") {
    blocks.push(
      optionRow(t("wizard.bootOnceTitle"), t("wizard.bootOnceNote"), bootOnce, toggleBootOnce),
    );
  } else if (snapshot.system !== null) {
    blocks.push(el("p", { class: "muted small-note" }, [t("wizard.bootOnceNoUefi")]));
  }
  return [
    panel(t("wizard.settingsTitle"), blocks, {
      subtitle: t("wizard.settingsText"),
      iconPaths: icons.install,
    }),
  ];
}

/** Paso 5: resumen del plan. */
function stepSummary(): Node[] {
  const disk = (snapshot.disks ?? []).find(isSelected);
  const item = chosenItem();
  const rows: [string, string][] = [
    [t("wizard.stepOs"), item ? item.title || item.name : "—"],
    [
      t("wizard.stepTarget"),
      disk ? (disk.model || disk.name) + " · " + fmt.bytes(disk.sizeBytes) : "—",
    ],
    ["Firmware", snapshot.system ? snapshot.system.firmware.toUpperCase() : "—"],
  ];
  if (item && takeDrivers && systemOf(item) === "windows") {
    rows.push([t("wizard.stepDrivers"), t("wizard.driversTake")]);
  }
  if (item && answers.enabled && systemOf(item) !== "other") {
    rows.push([
      t("wizard.answersTitle"),
      t("wizard.languages." + answers.language) +
        (answers.timeZone ? " · " + answers.timeZone : "") +
        (answers.userName ? " · " + answers.userName : ""),
    ]);
  }
  if (item && winBypass && systemOf(item) === "windows") {
    rows.push(["Windows 11", t("wizard.winBypassTitle")]);
  }
  if (item && privacy && systemOf(item) === "windows") {
    rows.push([t("wizard.privacyShort"), "✓"]);
  }
  if (bootOnce && snapshot.system?.firmware === "uefi") {
    rows.push([t("wizard.bootOnceTitle"), "✓"]);
  }
  return [
    panel(
      t("wizard.blueprint"),
      [
        el(
          "dl",
          { class: "about-list" },
          rows.flatMap(function row([name, value]) {
            return [el("dt", {}, [name]), el("dd", {}, [value])];
          }),
        ),
      ],
      { iconPaths: icons.terminal },
    ),
  ];
}

/** Refresca el contenido del paso actual. */
function update(): void {
  if (content === null || footer === null || !content.isConnected) {
    return;
  }
  // * Solo se redibuja si cambia el paso o los datos que usa: así el foco del
  // * teclado no se pierde con cada sondeo periódico.
  const signature = JSON.stringify([
    step,
    selectedDisk,
    selectedImage,
    understood,
    snapshot.library,
    snapshot.vault?.available,
    snapshot.disks,
    snapshot.devices?.length,
    snapshot.system?.secureBoot,
    snapshot.system?.tpm,
    snapshot.system?.memoryBytes,
    snapshot.system?.firmware,
    driverPlan,
    takeDrivers,
    winBypass,
    privacy,
    bootOnce,
    answers.enabled,
  ]);
  if (signature === lastSignature) {
    return;
  }
  lastSignature = signature;
  renderStepper();
  const system = snapshot.system;
  const disks = snapshot.disks;
  // * La comprobación del equipo y de los drivers va plegada dentro de "Opciones": casi nunca hace falta.
  const views: Node[][] = [
    stepChooseOs(),
    stepTarget(disks),
    [
      ...stepInstallerSettings(),
      more(t("wizard.checkComputer"), [...stepInspect(system, disks), ...stepDrivers()]),
      ...stepSummary(),
    ],
    stepDeploy(),
  ];
  if (step === 2) {
    loadDriverPlan();
  }
  content.replaceChildren(...views[step]);

  const back = button(t("common.previous"), previousStep, { iconPaths: icons.chevronLeft });
  back.disabled = step === 0;
  const next = button(t("common.next"), nextStep, {
    variant: "primary",
    trailingIcon: icons.arrowRight,
  });
  next.disabled = step === STEPS.length - 1;
  footer.replaceChildren(
    back,
    el("span", { class: "muted" }, [step + 1 + " / " + STEPS.length]),
    next,
  );
}

/** Indica si una imagen se puede ofrecer para instalar. */
function isInstallable(item: LibraryItem): boolean {
  return (
    item.state !== "mismatch" && item.state !== "hashing" && item.bootStatus !== "incompatible"
  );
}

/** Indica si se sabe que una imagen no arranca desde el menú del USB. */
function isIncompatible(item: LibraryItem): boolean {
  return item.bootStatus === "incompatible";
}

/** Nombre de una imagen. */
function itemName(item: LibraryItem): string {
  return item.name;
}

/** Indica si un disco puede ser destino (interno o extraíble, no el de Rebootera). */
function isCandidate(disk: Disk): boolean {
  return disk.role === "internal" || disk.role === "removable";
}

/** Tamaño de un disco. */
function diskSize(disk: Disk): number {
  return disk.sizeBytes;
}

/** Indica si un disco es el elegido. */
function isSelected(disk: Disk): boolean {
  return disk.name === selectedDisk;
}

/** Indica si un dispositivo es una controladora de almacenamiento PCI. */
function isStorageController(device: { category: string; bus: string }): boolean {
  return device.category === "storage" && device.bus === "pci";
}

/** Indica si un dispositivo es de red. */
function isNetworkDevice(device: { category: string }): boolean {
  return device.category === "network" || device.category === "wireless";
}

/** Indica si un dispositivo es crítico. */
function isCritical(device: { critical: boolean }): boolean {
  return device.critical;
}

/** Indica si un dispositivo tiene driver. */
function hasDriver(device: { driver: string }): boolean {
  return device.driver !== "";
}

/** Indica si un dispositivo no tiene driver. */
function isWithoutDriver(device: { driver: string }): boolean {
  return device.driver === "";
}

/** Nombre legible de un dispositivo. */
function deviceName(device: { name: string; className: string; vendor: string }): string {
  return device.name || device.className || device.vendor;
}

export const wizardPage: Page = { render, update };

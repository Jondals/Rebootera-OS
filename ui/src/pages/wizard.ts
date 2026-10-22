// * Página "Instalar": tres pasos. Elegir sistema, elegir dónde (una partición o un hueco libre, con ayuda para
// * hacer sitio si no cabe) e instalar. Los drivers se llevan solos; lo técnico queda en "Más opciones".
// ! Nada de esta página escribe en los discos por sí sola. Elegir un destino solo lo apunta en el plan y en la
// ! ayuda del último paso; hacer sitio usa el gestor de particiones (con su vista previa y su confirmación). El
// ! último paso preselecciona la imagen en el menú de Ventoy (un archivo del USB) y reinicia, tras una confirmación
// ! explícita; el particionado final lo hace el instalador original, que pregunta.

import { getJSON, postJSON, type DriverPlan, type LibraryItem } from "../api.js";
import { badge, button, emptyState, errorBox, more, pageHeader, panel } from "../components.js";
import { el, icon } from "../dom.js";
import * as fmt from "../format.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";
import type { PartDisk, PartStatus, Segment } from "../partman.js";
import { prefs } from "../prefs.js";
import { navigate, type Page } from "../router.js";
import { snapshot } from "../state.js";
import {
  diskBar,
  diskHeader,
  findPiece,
  legend,
  runningPanel,
  sizeOf,
  startedStatus,
  titleOf,
} from "./diskmap.js";
import { openMakeRoom } from "./partdialogs.js";

/** Claves de los pasos, en orden. */
const STEPS = ["stepOs", "stepTarget", "stepDeploy"];

/** Espacio recomendado para cada tipo de sistema (Windows 11 pide 64 GB). */
const NEEDED_BYTES = { windows: 64 * 2 ** 30, debian: 20 * 2 ** 30, other: 20 * 2 ** 30 };
/**
 * Espacio por debajo del cual ni se intenta: aun así, el instalador de cada sistema tiene la última palabra. Entre este
 * mínimo y el recomendado se deja seguir con un aviso (antes el botón "Siguiente" se quedaba apagado sin más).
 */
const MIN_BYTES = { windows: 25 * 2 ** 30, debian: 10 * 2 ** 30, other: 10 * 2 ** 30 };
/** Memoria mínima para Windows 11 (4 GB, con margen por la memoria reservada). */
const MIN_RAM = 3.8e9;
/** Cada cuántos ms se mira si una operación de particiones sigue en marcha. */
const STATUS_MS = 2000;

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
let targetKey = "";
let askInstaller = false;
let partDisks: PartDisk[] | null = null;
let partStatus: PartStatus | null = null;
let statusTimer = 0;
let selectedImage = "";
let understood = false;
let driverPlan: DriverPlan | null = null;
let planRequested = false;
let takeDrivers = true;
let winBypass = false;
let privacy = false;
let noOnline = false;
let noBitLocker = false;
let qol = false;
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
    noOnline: noOnline && system === "windows",
    bitlocker: noBitLocker && system === "windows",
    qol: qol && system === "windows",
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

/** Anota la pieza (partición o hueco) elegida como destino. */
function selectPiece(event: MouseEvent): void {
  targetKey = (event.currentTarget as HTMLElement).dataset.key ?? "";
  askInstaller = false;
  lastSignature = "";
  update();
}

/** Prefiere elegir el destino en el instalador original. */
function chooseInInstaller(): void {
  askInstaller = true;
  targetKey = "";
  step = 2;
  update();
}

/** Lee la distribución de los discos (para elegir dónde instalar). */
async function loadParts(): Promise<void> {
  try {
    partDisks = await getJSON<PartDisk[]>("/api/partitions");
  } catch {
    partDisks = partDisks ?? [];
  }
  lastSignature = "";
  update();
}

/** Vigila la operación de particiones; al terminar vuelve a leer los discos. */
async function watchParts(): Promise<void> {
  if (content === null || !content.isConnected) {
    window.clearInterval(statusTimer);
    return;
  }
  let next: PartStatus | null = null;
  try {
    next = await getJSON<PartStatus>("/api/partitions/status");
  } catch {
    // * Sin respuesta se reintenta en el siguiente ciclo.
  }
  const wasRunning = partStatus?.running === true;
  partStatus = next;
  if (wasRunning && !next?.running) {
    await loadParts();
    return;
  }
  if (next?.running) {
    lastSignature = "";
    update();
  }
}

/** Una operación de particiones acaba de empezar. */
function partsStarted(): void {
  partStatus = startedStatus();
  lastSignature = "";
  update();
  void watchParts();
}

/** Dibuja la estructura fija de la página. */
function render(root: HTMLElement): void {
  // * Cada vez que se abre el asistente se vuelve a mirar el Vault (puede haber cambiado).
  planRequested = false;
  driverPlan = null;
  window.clearInterval(statusTimer);
  statusTimer = window.setInterval(watchParts, STATUS_MS);
  void loadParts();
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

/** Marca o desmarca llevar los drivers al instalador. */
function toggleTakeDrivers(event: Event): void {
  takeDrivers = (event.currentTarget as HTMLInputElement).checked;
  lastSignature = "";
  update();
}

/** Espacio que necesita el sistema elegido. */
function neededBytes(): number {
  const item = chosenItem();
  return item ? NEEDED_BYTES[systemOf(item)] : NEEDED_BYTES.other;
}

/** Espacio mínimo del sistema elegido. */
function minimumBytes(): number {
  const item = chosenItem();
  return item ? MIN_BYTES[systemOf(item)] : MIN_BYTES.other;
}

/** Discos que se pueden ofrecer como destino (internos y extraíbles; nunca el USB de Rebootera). */
function targetDisks(): PartDisk[] {
  return (partDisks ?? []).filter(function isTarget(disk) {
    return disk.role === "internal" || disk.role === "removable";
  });
}

/** La pieza elegida como destino, si sigue existiendo. */
function chosenPiece(): { disk: PartDisk; segment: Segment } | null {
  return findPiece(targetDisks(), targetKey);
}

/** Ruta de dispositivo de una pieza, para la ayuda del instalador. */
function pieceDevice(disk: PartDisk, segment: Segment): string {
  return segment.number > 0
    ? "/dev/" + (segment.name || disk.name + segment.number)
    : "/dev/" + disk.name;
}

/** Abre el gestor de discos. */
function openDisks(): void {
  navigate("storage");
}

/** Panel de la pieza elegida: si cabe, qué pasará con ella y cómo hacer sitio. */
function targetPanel(disk: PartDisk, segment: Segment): HTMLElement {
  const need = neededBytes();
  const size = sizeOf(segment);
  const fits = size >= need;
  const usable = size >= minimumBytes();
  const isFree = segment.number === 0;
  const good = isFree && fits;
  // * Entre el mínimo y lo recomendado se puede seguir (con aviso); por debajo del mínimo, no.
  const tight = usable && !fits;
  const text = isFree
    ? fits
      ? t("wizard.freeOk", { size: fmt.disk(size) })
      : tight
        ? t("wizard.freeTight", { have: fmt.disk(size), need: fmt.disk(need) })
        : t("wizard.freeSmall", { have: fmt.disk(size), need: fmt.disk(minimumBytes()) })
    : fits
      ? t("wizard.partOk", { used: fmt.disk(segment.usedBytes) })
      : tight
        ? t("wizard.partTight", {
            have: fmt.disk(size),
            need: fmt.disk(need),
            used: fmt.disk(segment.usedBytes),
          })
        : t("wizard.partSmall", { have: fmt.disk(size), need: fmt.disk(minimumBytes()) });
  const locked = disk.layout.locked ?? "";
  const room = button(
    t("wizard.makeRoom"),
    function makeRoom(): void {
      openMakeRoom(disk.layout, need, isFree ? segment : null, partsStarted);
    },
    { variant: usable ? "ghost" : "primary", iconPaths: icons.disk },
  );
  room.disabled = locked !== "";
  return el("div", { class: "target-panel" }, [
    el("strong", { class: "pm-name" }, [titleOf(segment) + " · " + fmt.disk(size)]),
    el("div", { class: "notice " + (good ? "notice-ok" : "notice-warn") }, [
      icon(good ? icons.check : icons.alert),
      el("span", {}, [text]),
    ]),
    el("div", { class: "target-actions" }, [
      room,
      button(t("wizard.openDisks"), openDisks, { iconPaths: icons.refresh }),
    ]),
    locked !== "" ? el("p", { class: "muted small-note" }, [t("pm.locked." + locked)]) : null,
  ]);
}

/** Tarjeta de un disco con su mapa; al pulsar una pieza sale lo que pasará con ella. */
function targetDiskCard(disk: PartDisk): HTMLElement {
  const need = minimumBytes();
  const picked = findPiece([disk], targetKey);
  return el("article", { class: "disk-card" }, [
    diskHeader(disk),
    disk.layout.segments.length > 0
      ? el("div", {}, [
          diskBar(disk, targetKey, selectPiece, function tooSmall(segment): boolean {
            return sizeOf(segment) < need;
          }),
          legend(disk),
        ])
      : el("p", { class: "muted" }, [t("storage.noTable")]),
    picked ? targetPanel(disk, picked.segment) : null,
  ]);
}

/** Paso 2: dónde instalar. */
function stepTarget(): Node[] {
  if (partStatus?.running) {
    return [runningPanel(partStatus)];
  }
  if (partDisks === null) {
    return [el("p", { class: "muted" }, [t("common.loading")])];
  }
  const disks = targetDisks();
  if (disks.length === 0) {
    return [
      panel(t("wizard.targetTitle"), [emptyState(icons.disk, t("wizard.noTargets"), "")], {
        iconPaths: icons.disk,
      }),
    ];
  }
  return [
    panel(
      t("wizard.targetTitle"),
      [
        el("p", { class: "muted" }, [
          t("wizard.targetNeed", {
            min: fmt.disk(minimumBytes()),
            size: fmt.disk(neededBytes()),
          }),
        ]),
        ...disks.map(targetDiskCard),
        el("div", { class: "target-skip" }, [
          button(t("wizard.askInstaller"), chooseInInstaller, { variant: "ghost" }),
        ]),
      ],
      { subtitle: t("wizard.targetText"), iconPaths: icons.disk },
    ),
  ];
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

/** Aviso del último paso cuando falta algo para poder instalar. */
function deployNotice(iconPaths: string, text: string): Node[] {
  return [
    panel(t("wizard.deployTitle"), [
      el("div", { class: "notice" }, [icon(iconPaths), el("span", {}, [text])]),
    ]),
  ];
}

/** Paso 3: resumen, ayuda para el instalador y confirmación. */
function stepDeploy(): Node[] {
  const item = chosenItem();
  if (!snapshot.vault?.available) {
    return deployNotice(icons.lock, t("wizard.deployNoVault"));
  }
  if (!item) {
    return deployNotice(icons.info, t("wizard.deployNeedsImage"));
  }
  const check = el("input", { type: "checkbox", checked: understood });
  check.addEventListener("change", toggleUnderstood);
  const go = button(t("wizard.deployButton"), deploy, {
    variant: "primary",
    iconPaths: icons.reboot,
  });
  go.disabled = !understood || (!askInstaller && chosenPiece() === null);
  return [
    ...stepSummary(item),
    installerHint(item),
    more(t("wizard.moreOptions"), stepInstallerSettings(), "wizard-more"),
    panel(
      t("wizard.deployTitle"),
      [
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

/** Marca o desmarca no exigir una cuenta de Microsoft en línea. */
function toggleNoOnline(event: Event): void {
  noOnline = (event.currentTarget as HTMLInputElement).checked;
  lastSignature = "";
  update();
}

/** Marca o desmarca desactivar el cifrado automático (BitLocker). */
function toggleNoBitLocker(event: Event): void {
  noBitLocker = (event.currentTarget as HTMLInputElement).checked;
  lastSignature = "";
  update();
}

/** Marca o desmarca las mejoras de calidad de vida (sin Copilot, OneDrive, etc.). */
function toggleQol(event: Event): void {
  qol = (event.currentTarget as HTMLInputElement).checked;
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

/** Opciones del instalador (dentro de "Más opciones"): drivers, respuestas automáticas, Windows 11 y arranque único. */
function stepInstallerSettings(): Node[] {
  const item = chosenItem();
  if (!item) {
    return [];
  }
  const system = systemOf(item);
  const blocks: Node[] = [];
  if (system === "windows") {
    blocks.push(
      optionRow(
        t("wizard.driversTake"),
        t("wizard.driversTakeHint"),
        takeDrivers,
        toggleTakeDrivers,
      ),
    );
  }
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
      optionRow(t("wizard.noOnlineTitle"), t("wizard.noOnlineNote"), noOnline, toggleNoOnline),
      optionRow(t("wizard.privacyTitle"), t("wizard.privacyNote"), privacy, togglePrivacy),
      optionRow(
        t("wizard.noBitLockerTitle"),
        t("wizard.noBitLockerNote"),
        noBitLocker,
        toggleNoBitLocker,
      ),
      optionRow(t("wizard.qolTitle"), t("wizard.qolNote"), qol, toggleQol),
    );
  }
  if (snapshot.system?.firmware === "uefi") {
    blocks.push(
      optionRow(t("wizard.bootOnceTitle"), t("wizard.bootOnceNote"), bootOnce, toggleBootOnce),
    );
  } else if (snapshot.system !== null) {
    blocks.push(el("p", { class: "muted small-note" }, [t("wizard.bootOnceNoUefi")]));
  }
  return blocks;
}

/** Cosas de Windows 11 que este equipo no cumple (nombres para el aviso). */
function windowsIssues(): string[] {
  const system = snapshot.system;
  if (system === null) {
    return [];
  }
  const issues: string[] = [];
  if (system.firmware !== "uefi") {
    issues.push(t("wizard.reqUefi"));
  }
  if (system.tpm !== "2.0") {
    issues.push(t("wizard.reqTpm"));
  }
  if (system.memoryBytes < MIN_RAM) {
    issues.push(t("wizard.reqRam"));
  }
  return issues;
}

/** Marca "saltar los requisitos" desde el aviso. */
function enableBypass(): void {
  winBypass = true;
  lastSignature = "";
  update();
}

/** Abre la página de drivers. */
function openDrivers(): void {
  navigate("drivers");
}

/** Texto y aviso sobre los drivers según el sistema elegido. */
function driversLine(item: LibraryItem): { text: string; warn: Node | null } {
  if (systemOf(item) !== "windows") {
    return { text: t("wizard.driversLinux"), warn: null };
  }
  if (!takeDrivers) {
    return { text: t("wizard.driversOff"), warn: null };
  }
  if (driverPlan === null) {
    return { text: t("common.loading"), warn: null };
  }
  const text =
    driverPlan.matches.length > 0
      ? t("wizard.driversReady", { n: driverPlan.matches.length })
      : t("wizard.driversNone");
  if (driverPlan.missing.length === 0) {
    return { text, warn: null };
  }
  const names = driverPlan.missing
    .map(function nameOfMatch(match) {
      return match.name || match.vendor || match.address;
    })
    .slice(0, 3)
    .join(", ");
  return {
    text,
    warn: el("div", { class: "notice notice-warn" }, [
      icon(icons.alert),
      el("span", {}, [t("wizard.driversMissing", { names })]),
      button(t("wizard.seeDrivers"), openDrivers, { iconPaths: icons.drivers }),
    ]),
  };
}

/** Nombre del disco para el resumen. */
function diskNameOf(disk: PartDisk): string {
  return [disk.vendor, disk.model].filter(Boolean).join(" ") || disk.name;
}

/** Paso 3: resumen del plan, con los avisos que importan. */
function stepSummary(item: LibraryItem): Node[] {
  const piece = chosenPiece();
  const where = askInstaller
    ? t("wizard.whereAsk")
    : piece
      ? [diskNameOf(piece.disk), titleOf(piece.segment), fmt.disk(sizeOf(piece.segment))].join(
          " · ",
        )
      : "—";
  const drivers = driversLine(item);
  const rows: [string, string, number | null][] = [
    [t("wizard.stepOs"), item.title || item.name, 0],
    [t("wizard.stepTarget"), where, 1],
    [t("wizard.stepDrivers"), drivers.text, null],
  ];
  if (answers.enabled && systemOf(item) !== "other") {
    rows.push([
      t("wizard.answersTitle"),
      t("wizard.languages." + answers.language) +
        (answers.timeZone ? " · " + answers.timeZone : "") +
        (answers.userName ? " · " + answers.userName : ""),
      null,
    ]);
  }
  const issues = systemOf(item) === "windows" && !winBypass ? windowsIssues() : [];
  return [
    panel(
      t("wizard.blueprint"),
      [
        el(
          "dl",
          { class: "about-list plan-list" },
          rows.flatMap(function row([name, value, goStep]) {
            const change =
              goStep === null
                ? null
                : button(t("wizard.change"), function change(): void {
                    step = goStep;
                    update();
                  });
            return [el("dt", {}, [name]), el("dd", {}, [value, change])];
          }),
        ),
        drivers.warn,
        issues.length > 0
          ? el("div", { class: "notice notice-warn" }, [
              icon(icons.alert),
              el("span", {}, [t("wizard.win11Issues", { list: issues.join(", ") })]),
              button(t("wizard.win11Skip"), enableBypass),
            ])
          : null,
      ],
      { iconPaths: icons.terminal },
    ),
  ];
}

/** Qué elegir cuando el instalador original pregunte dónde instalar. */
function installerHint(item: LibraryItem): HTMLElement {
  const piece = chosenPiece();
  let text = t("wizard.hintAsk");
  if (!askInstaller && piece) {
    const size = fmt.disk(sizeOf(piece.segment));
    const device = pieceDevice(piece.disk, piece.segment);
    const windows = systemOf(item) === "windows";
    text =
      piece.segment.number === 0
        ? t(windows ? "wizard.hintFreeWin" : "wizard.hintFreeLinux", { size })
        : t(windows ? "wizard.hintPartWin" : "wizard.hintPartLinux", { size, device });
  }
  return el("div", { class: "notice" }, [
    icon(icons.info),
    el("div", {}, [
      el("strong", {}, [t("wizard.hintTitle")]),
      el("span", {}, [text]),
      el("span", { class: "muted" }, [t("wizard.hintUsb")]),
    ]),
  ]);
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
    targetKey,
    askInstaller,
    partDisks,
    partStatus,
    selectedImage,
    understood,
    snapshot.library,
    snapshot.vault?.available,
    snapshot.system?.tpm,
    snapshot.system?.memoryBytes,
    snapshot.system?.firmware,
    driverPlan,
    takeDrivers,
    winBypass,
    privacy,
    noOnline,
    noBitLocker,
    qol,
    bootOnce,
    answers.enabled,
  ]);
  if (signature === lastSignature) {
    return;
  }
  lastSignature = signature;
  renderStepper();
  const views: Node[][] = [stepChooseOs(), stepTarget(), stepDeploy()];
  if (step === 2) {
    void loadDriverPlan();
  }
  content.replaceChildren(...views[step]);

  const back = button(t("common.previous"), previousStep, { iconPaths: icons.chevronLeft });
  back.disabled = step === 0;
  const next = button(t("common.next"), nextStep, {
    variant: "primary",
    trailingIcon: icons.arrowRight,
  });
  next.disabled = step === STEPS.length - 1 || !canContinue();
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

/** Indica si se puede pasar al paso siguiente. */
function canContinue(): boolean {
  if (step === 0) {
    return chosenItem() !== undefined;
  }
  if (step === 1) {
    const piece = chosenPiece();
    return askInstaller || (piece !== null && sizeOf(piece.segment) >= minimumBytes());
  }
  return true;
}

export const wizardPage: Page = { render, update };

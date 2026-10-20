// * Panel de Wi-Fi de la página de Ajustes: estado del adaptador, búsqueda de
// * redes, conexión con contraseña y desconexión. Solo existe en Rebootera OS:
// * Core pide cada operación al agente, que habla con NetworkManager.
// ! La contraseña solo vive en el campo del formulario y en la petición local
// ! a Core; no se guarda en la interfaz ni en el almacenamiento del navegador.

import {
  getJSON,
  postJSON,
  type Device,
  type DriverPlan,
  type WifiNetwork,
  type WifiStatus,
} from "../api.js";
import { badge, button, errorBox, panel } from "../components.js";
import { el, icon } from "../dom.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";
import { snapshot } from "../state.js";

/** Cada cuántos milisegundos se vuelve a pedir el estado como mucho. */
const STATUS_EVERY_MS = 6000;

let body: HTMLElement | null = null;
let status: WifiStatus | null = null;
let networks: WifiNetwork[] | null = null;
let scanning = false;
let connecting = false;
let selected: WifiNetwork | null = null;
let hidden = false;
let failure: unknown = null;
let lastStatusAt = 0;
let searching = false;
/** Resultado de buscar drivers en los discos del equipo para los adaptadores de red sin driver (null: sin buscar). */
let searchResult: string | null = null;
/** Si se recuerda la red en el USB al conectar (por defecto sí). */
let remember = true;
/** Aviso con el motivo por el que se llegó aquí (por ejemplo, una descarga sin conexión). */
let reason: string | null = null;
let scrollPending = false;

/** La interfaz lleva a Ajustes porque hace falta internet: el panel de Wi-Fi saldrá con ese aviso y a la vista. */
export function needWifi(message: string): void {
  reason = message;
  scrollPending = true;
}

/** Adaptadores de red o Wi-Fi que Linux ha detectado pero para los que no hay driver cargado. */
function adaptersWithoutDriver(): Device[] {
  return (snapshot.devices ?? []).filter(function noDriver(device) {
    return (
      (device.category === "network" || device.category === "wireless") && device.driver === ""
    );
  });
}

/** Busca drivers en los discos del equipo y cuenta si alguno sirve para los adaptadores sin driver. */
async function searchDisks(): Promise<void> {
  searching = true;
  searchResult = null;
  draw();
  try {
    await postJSON("/api/drivers/scan", {});
    const plan = await getJSON<DriverPlan>("/api/drivers/plan");
    const missing = adaptersWithoutDriver().map(function address(device) {
      return device.address;
    });
    const found = plan.matches.filter(function forMissing(match) {
      return missing.includes(match.address);
    });
    searchResult =
      found.length > 0
        ? t("wifi.searchFound", { name: found[0].name || found[0].address })
        : t("wifi.searchNone");
  } catch (error) {
    failure = error;
  }
  searching = false;
  draw();
}

/** Aviso de que falta el driver de un adaptador de red, con el botón para buscarlo en los discos. */
function missingDriverNotice(): HTMLElement | null {
  const missing = adaptersWithoutDriver();
  if (missing.length === 0) {
    return null;
  }
  return el("div", { class: "error-box wifi-driver" }, [
    icon(icons.alert),
    el("div", {}, [
      el("strong", {}, [t("wifi.noDriver", { name: missing[0].name })]),
      el("p", { class: "muted" }, [t("wifi.noDriverHint")]),
      searchResult !== null ? el("p", {}, [searchResult]) : null,
      button(searching ? t("wifi.searching") : t("wifi.searchDisks"), searchDisks, {
        iconPaths: icons.disk,
      }),
    ]),
  ]);
}

/** Nombre traducido del tipo de seguridad de una red. */
function securityLabel(security: string): string {
  return t("wifi." + security);
}

/** Indica si se puede intentar conectar a una red de este tipo. */
function isConnectable(network: WifiNetwork): boolean {
  return network.security === "open" || network.security === "wpa" || network.security === "wpa3";
}

/** Lee el estado del adaptador. */
async function loadStatus(): Promise<void> {
  lastStatusAt = Date.now();
  try {
    status = await getJSON<WifiStatus>("/api/wifi");
  } catch (error) {
    failure = error;
  }
  draw();
}

/** Busca redes. */
async function scan(): Promise<void> {
  scanning = true;
  failure = null;
  draw();
  try {
    networks = await postJSON<WifiNetwork[]>("/api/wifi/scan", {});
  } catch (error) {
    failure = error;
  }
  scanning = false;
  draw();
}

/** Conecta con los datos del formulario. */
async function connect(): Promise<void> {
  const ssidInput = body?.querySelector<HTMLInputElement>("input[name=ssid]");
  const passwordInput = body?.querySelector<HTMLInputElement>("input[name=password]");
  const ssid = hidden ? (ssidInput?.value ?? "") : (selected?.ssid ?? "");
  const password = passwordInput?.value ?? "";
  if (ssid === "") {
    return;
  }
  connecting = true;
  failure = null;
  draw();
  try {
    await postJSON("/api/wifi/connect", { ssid, password, hidden, remember });
    selected = null;
    hidden = false;
    reason = null;
  } catch (error) {
    failure = error;
  }
  connecting = false;
  await loadStatus();
}

/** Desconecta y olvida la red. */
async function disconnect(): Promise<void> {
  failure = null;
  try {
    await postJSON("/api/wifi/disconnect", {});
  } catch (error) {
    failure = error;
  }
  await loadStatus();
}

/** Abre el formulario de contraseña de una red. */
function chooseNetwork(event: MouseEvent): void {
  const ssid = (event.currentTarget as HTMLElement).dataset.ssid ?? "";
  selected =
    (networks ?? []).find(function same(network) {
      return network.ssid === ssid;
    }) ?? null;
  hidden = false;
  failure = null;
  draw();
}

/** Abre el formulario de una red oculta. */
function chooseHidden(): void {
  selected = null;
  hidden = true;
  failure = null;
  draw();
}

/** Cierra el formulario. */
function cancelForm(): void {
  selected = null;
  hidden = false;
  draw();
}

/** Cambia si se recuerda la red. */
function onRemember(event: Event): void {
  remember = (event.currentTarget as HTMLInputElement).checked;
}

/** Envía el formulario con la tecla Intro. */
function submitOnEnter(event: KeyboardEvent): void {
  if (event.key === "Enter") {
    connect();
  }
}

/** Fila de una red de la lista. */
function networkRow(network: WifiNetwork): HTMLElement {
  const row = el(
    "button",
    {
      class: "wifi-row" + (network.inUse ? " wifi-active" : ""),
      type: "button",
      "data-ssid": network.ssid,
      disabled: !isConnectable(network) || connecting,
    },
    [
      icon(icons.wifi),
      el("span", { class: "wifi-name" }, [network.ssid]),
      el("span", { class: "mono-small muted" }, [t("wifi.signal", { n: network.signal })]),
      status?.remembered?.includes(network.ssid) ? badge(t("wifi.remembered"), "ok") : null,
      badge(securityLabel(network.security), network.security === "open" ? "amber" : "muted"),
    ],
  );
  row.addEventListener("click", chooseNetwork);
  return row;
}

/** Formulario de contraseña (o de nombre y contraseña si la red es oculta). */
function connectForm(): HTMLElement {
  const needsPassword = hidden || (selected !== null && selected.security !== "open");
  const nameInput = hidden
    ? el("input", {
        class: "text-input",
        name: "ssid",
        type: "text",
        maxlength: "32",
        autocomplete: "off",
        placeholder: t("wifi.hiddenName"),
      })
    : null;
  const passwordInput = needsPassword
    ? el("input", {
        class: "text-input",
        name: "password",
        type: "password",
        maxlength: "64",
        autocomplete: "off",
        placeholder: t("wifi.password"),
      })
    : null;
  const rememberBox = el("input", { type: "checkbox", class: "switch-input", checked: remember });
  rememberBox.addEventListener("change", onRemember);
  nameInput?.addEventListener("keydown", submitOnEnter);
  passwordInput?.addEventListener("keydown", submitOnEnter);
  const go = button(connecting ? t("wifi.connecting") : t("wifi.connect"), connect, {
    variant: "primary",
    iconPaths: icons.wifi,
  });
  go.disabled = connecting;
  return el("div", { class: "wifi-form" }, [
    el("strong", {}, [hidden ? t("wifi.hiddenToggle") : (selected?.ssid ?? "")]),
    nameInput,
    passwordInput,
    snapshot.vault?.available
      ? el("label", { class: "setting-row" }, [el("span", {}, [t("wifi.remember")]), rememberBox])
      : null,
    el("div", { class: "wifi-actions" }, [go, button(t("wifi.cancel"), cancelForm)]),
  ]);
}

/** Dibuja el contenido del panel según el estado. */
function draw(): void {
  if (body === null || !body.isConnected) {
    return;
  }
  // * Con el formulario abierto no se redibuja por sondeo: se perdería lo escrito.
  if (body.querySelector(".wifi-form") && (selected !== null || hidden) && !connecting) {
    const typing = body.querySelector("input:focus");
    if (typing) {
      return;
    }
  }
  const children: (Node | null)[] = [
    reason !== null
      ? el("div", { class: "notice" }, [icon(icons.alert), el("span", {}, [reason])])
      : null,
    missingDriverNotice(),
  ];
  if (status === null) {
    children.push(el("p", { class: "muted" }, [t("common.loading")]));
  } else if (!status.adapter) {
    children.push(
      el("div", { class: "notice" }, [
        icon(icons.alert),
        el("span", {}, [t("wifi.none") + " " + t("wifi.noneHint")]),
      ]),
    );
  } else {
    children.push(statusLine(status));
    if (!status.enabled) {
      children.push(el("p", { class: "muted small-note" }, [t("wifi.off")]));
    }
    children.push(
      el("div", { class: "wifi-actions" }, [
        button(scanning ? t("wifi.scanning") : t("wifi.scan"), scan, {
          iconPaths: icons.refresh,
        }),
        button(t("wifi.hiddenToggle"), chooseHidden),
      ]),
    );
    if (selected !== null || hidden) {
      children.push(connectForm());
    } else if (networks !== null) {
      children.push(
        networks.length > 0
          ? el("div", { class: "wifi-list" }, networks.map(networkRow))
          : el("p", { class: "muted" }, [t("wifi.empty")]),
      );
    }
    children.push(el("p", { class: "muted small-note" }, [t("wifi.note")]));
  }
  if (failure !== null) {
    children.push(errorBox(failure));
  }
  body.replaceChildren(...children.filter(notNull));
  if (scrollPending) {
    scrollPending = false;
    body.scrollIntoView({ block: "center" });
  }
}

/** Filtra los nulos de una lista de nodos. */
function notNull(node: Node | null): node is Node {
  return node !== null;
}

/** Línea con el estado de la conexión y el botón de desconectar. */
function statusLine(current: WifiStatus): HTMLElement {
  const connected = current.state === "connected";
  return el("div", { class: "wifi-status" }, [
    badge(
      connected ? t("wifi.connected", { ssid: current.connection }) : t("wifi.disconnected"),
      connected ? "ok" : "muted",
    ),
    connected ? button(t("wifi.disconnect"), disconnect) : null,
  ]);
}

/** Crea el panel de Wi-Fi. */
export function wifiPanel(): HTMLElement {
  body = el("div", { class: "wifi-body" });
  status = null;
  networks = null;
  selected = null;
  hidden = false;
  failure = null;
  loadStatus();
  void autoScan();
  return panel(t("wifi.title"), [body], { subtitle: t("wifi.subtitle"), iconPaths: icons.wifi });
}

/** Refresca el estado de vez en cuando (se llama con cada sondeo de datos). */
export function wifiUpdate(): void {
  if (body === null || !body.isConnected) {
    return;
  }
  if (Date.now() - lastStatusAt > STATUS_EVERY_MS && !scanning && !connecting) {
    loadStatus();
  }
}

/** Si se llegó aquí por falta de internet, la lista de redes se busca sola: un toque menos. */
async function autoScan(): Promise<void> {
  if (reason !== null) {
    await scan();
  }
}

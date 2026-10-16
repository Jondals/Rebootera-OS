// * Diálogos modales de la interfaz: energía (reiniciar / apagar), registro
// * completo del kernel y confirmaciones. Usan <dialog>, que gestiona el foco
// * y la tecla Esc.

import { postJSON } from "./api.js";
import { errorBox } from "./components.js";
import { el, icon } from "./dom.js";
import { t } from "./i18n.js";
import { icons } from "./icons.js";
import { isProblemLine } from "./insights.js";
import { snapshot } from "./state.js";

/** Crea un diálogo con título y cuerpo, lo abre y lo elimina al cerrarse. */
export function openModal(title: string, body: Node[], className = ""): HTMLDialogElement {
  const closeButton = el(
    "button",
    { class: "icon-button dialog-close", type: "button", "aria-label": t("common.close") },
    [icon(icons.x)],
  );
  const dialog = el("dialog", { class: "modal " + className }, [
    el("header", { class: "modal-header" }, [el("h2", {}, [title]), closeButton]),
    el("div", { class: "modal-body" }, body),
  ]);
  closeButton.addEventListener("click", closeParentDialog);
  dialog.addEventListener("close", removeDialog);
  document.body.append(dialog);
  dialog.showModal();
  return dialog;
}

/** Cierra el diálogo que contiene el botón pulsado. */
function closeParentDialog(event: MouseEvent): void {
  (event.currentTarget as HTMLElement).closest("dialog")?.close();
}

/** Elimina del documento un diálogo cerrado. */
function removeDialog(event: Event): void {
  (event.currentTarget as HTMLElement).remove();
}

/** Abre el diálogo de energía. */
export function openPowerDialog(): void {
  const reboot = el("button", { class: "power-action", type: "button", "data-action": "reboot" }, [
    icon(icons.reboot),
    el("span", {}, [t("power.reboot")]),
  ]);
  const poweroff = el(
    "button",
    { class: "power-action power-danger", type: "button", "data-action": "poweroff" },
    [icon(icons.power), el("span", {}, [t("power.poweroff")])],
  );
  reboot.addEventListener("click", runPowerAction);
  poweroff.addEventListener("click", runPowerAction);
  openModal(
    t("power.title"),
    [
      el("p", { class: "muted" }, [t("power.text")]),
      el("div", { class: "power-actions" }, [reboot, poweroff]),
    ],
    "modal-small",
  );
}

/** Ejecuta la acción de energía del botón pulsado. */
async function runPowerAction(event: MouseEvent): Promise<void> {
  const button = event.currentTarget as HTMLElement;
  const action = button.dataset.action as "reboot" | "poweroff";
  const dialog = button.closest("dialog");
  const overlay = document.getElementById("overlay") as HTMLElement;
  const overlayText = document.getElementById("overlay-text") as HTMLElement;
  overlayText.textContent = action === "reboot" ? t("power.rebooting") : t("power.poweringOff");
  dialog?.close();
  overlay.hidden = false;
  try {
    await postJSON("/api/power", { action });
  } catch (error) {
    overlay.hidden = true;
    openModal(t("power.title"), [errorBox(error)], "modal-small");
  }
}

/** Abre el registro completo del kernel. */
export function openKernelLog(): void {
  const lines = snapshot.kernelLog ?? [];
  openModal(
    t("overview.kernelTitle"),
    [
      el(
        "pre",
        { class: "log-full" },
        lines.map(function logLine(line) {
          return el("span", { class: isProblemLine(line) ? "log-line log-problem" : "log-line" }, [
            line + "\n",
          ]);
        }),
      ),
    ],
    "modal-wide",
  );
}

/** Resolución pendiente del diálogo de confirmación abierto. */
let confirmResolve: ((value: boolean) => void) | null = null;

/**
 * Pide confirmación explícita antes de una acción. Devuelve true solo si se
 * pulsa el botón de confirmar; cerrar el diálogo o Esc cuenta como cancelar.
 */
export function confirmAction(title: string, text: string, confirmLabel: string): Promise<boolean> {
  const confirm = el("button", { class: "btn btn-danger", type: "button" }, [confirmLabel]);
  const cancel = el("button", { class: "btn btn-ghost", type: "button" }, [t("common.cancel")]);
  const dialog = openModal(
    title,
    [el("p", { class: "muted" }, [text]), el("div", { class: "modal-actions" }, [cancel, confirm])],
    "modal-small",
  );
  confirm.addEventListener("click", acceptConfirm);
  cancel.addEventListener("click", closeParentDialog);
  dialog.addEventListener("close", rejectConfirm);
  cancel.focus();
  return new Promise<boolean>(function confirmResolver(resolve) {
    confirmResolve = resolve;
  });
}

/** Confirma y cierra el diálogo. */
function acceptConfirm(event: MouseEvent): void {
  confirmResolve?.(true);
  confirmResolve = null;
  (event.currentTarget as HTMLElement).closest("dialog")?.close();
}

/** Cancela si el diálogo se cierra sin confirmar. */
function rejectConfirm(): void {
  confirmResolve?.(false);
  confirmResolve = null;
}

// * Diálogo "Personalizar" de una imagen de la biblioteca: le pone un nombre y una foto (por ejemplo, el logotipo de
// * un Windows que has preparado) para reconocerla de un vistazo. Solo cambia lo que se enseña: el archivo de la
// * imagen no se toca. La foto se elige con el selector del sistema, se recorta cuadrada y se reduce a 128 px aquí
// * mismo, así que lo que se guarda en el USB es pequeño.

import { postJSON, type LibraryItem } from "../api.js";
import { button, errorBox } from "../components.js";
import { openModal } from "../dialogs.js";
import { el, icon } from "../dom.js";
import { t } from "../i18n.js";
import { icons } from "../icons.js";

/** Lado (px) al que se reduce la foto. */
const SIZE = 128;

/** Calidad del JPEG de la foto (la imagen es pequeña: pesa unos pocos KB). */
const QUALITY = 0.86;

/** Lee un archivo de imagen y lo devuelve recortado cuadrado y reducido, como data URL JPEG. */
function shrink(file: File): Promise<string> {
  return new Promise(function shrinkPromise(resolve, reject) {
    const url = URL.createObjectURL(file);
    const image = new Image();
    image.onload = function drawSmall(): void {
      const side = Math.min(image.naturalWidth, image.naturalHeight);
      if (side === 0) {
        URL.revokeObjectURL(url);
        reject(new Error(t("look.badImage")));
        return;
      }
      const canvas = document.createElement("canvas");
      canvas.width = SIZE;
      canvas.height = SIZE;
      const context = canvas.getContext("2d");
      if (context === null) {
        URL.revokeObjectURL(url);
        reject(new Error(t("look.badImage")));
        return;
      }
      // * Fondo claro para las imágenes con transparencia (un logotipo PNG), que en JPEG saldrían negras.
      context.fillStyle = "#ffffff";
      context.fillRect(0, 0, SIZE, SIZE);
      const left = (image.naturalWidth - side) / 2;
      const top = (image.naturalHeight - side) / 2;
      context.drawImage(image, left, top, side, side, 0, 0, SIZE, SIZE);
      URL.revokeObjectURL(url);
      resolve(canvas.toDataURL("image/jpeg", QUALITY));
    };
    image.onerror = function failed(): void {
      URL.revokeObjectURL(url);
      reject(new Error(t("look.badImage")));
    };
    image.src = url;
  });
}

/** Foto de una imagen de la biblioteca (o el icono genérico si no tiene). */
export function itemPicture(item: LibraryItem): HTMLElement {
  if (item.image) {
    const picture = el("img", { class: "item-picture", alt: "", src: item.image });
    return el("span", { class: "device-icon device-photo" }, [picture]);
  }
  return el("span", { class: "device-icon" }, [
    icon(item.kind === "iso" ? icons.library : icons.disk),
  ]);
}

/** Abre el diálogo para cambiar el nombre y la foto de una imagen; `done` se llama al guardar. */
export function openLookDialog(item: LibraryItem, done: () => void): void {
  let image = item.image ?? "";
  const preview = el("span", { class: "look-preview" });
  const nameInput = el("input", {
    class: "text-input",
    type: "text",
    maxlength: "80",
    placeholder: item.name,
    "aria-label": t("look.name"),
  });
  nameInput.value = item.title ?? "";
  const fileInput = el("input", { type: "file", accept: "image/*", hidden: true });
  const message = el("div", { class: "look-message" });

  /** Dibuja la foto actual (o el icono) en la vista previa. */
  function drawPreview(): void {
    if (image) {
      preview.replaceChildren(el("img", { class: "item-picture", alt: "", src: image }));
    } else {
      preview.replaceChildren(icon(item.kind === "iso" ? icons.library : icons.disk));
    }
  }

  /** Elige la foto: se reduce y se enseña al momento. */
  async function onFile(): Promise<void> {
    const file = fileInput.files?.[0];
    if (!file) {
      return;
    }
    try {
      image = await shrink(file);
      message.replaceChildren();
      drawPreview();
    } catch (error) {
      message.replaceChildren(errorBox(error));
    }
  }

  /** Quita la foto. */
  function removePicture(): void {
    image = "";
    fileInput.value = "";
    drawPreview();
  }

  /** Guarda nombre y foto en el USB. */
  async function save(event: MouseEvent): Promise<void> {
    const dialog = (event.currentTarget as HTMLElement).closest("dialog");
    try {
      await postJSON("/api/library/look", {
        path: item.path,
        title: nameInput.value.trim(),
        image,
      });
      dialog?.close();
      done();
    } catch (error) {
      message.replaceChildren(errorBox(error));
    }
  }

  /** Abre el selector de archivos del sistema. */
  function choosePicture(): void {
    fileInput.click();
  }

  fileInput.addEventListener("change", onFile);
  drawPreview();
  openModal(
    t("look.title"),
    [
      el("p", { class: "muted" }, [t("look.hint")]),
      el("div", { class: "look-row" }, [
        preview,
        el("div", { class: "look-actions" }, [
          button(t("look.choose"), choosePicture, { iconPaths: icons.library }),
          button(t("look.remove"), removePicture, { iconPaths: icons.x }),
        ]),
      ]),
      el("label", { class: "confirm-field" }, [el("span", {}, [t("look.name")]), nameInput]),
      el("p", { class: "muted small-note" }, [item.name]),
      message,
      fileInput,
      el("div", { class: "deploy-actions" }, [
        button(t("look.save"), save, { variant: "primary", iconPaths: icons.check }),
      ]),
    ],
    "look-dialog",
  );
}

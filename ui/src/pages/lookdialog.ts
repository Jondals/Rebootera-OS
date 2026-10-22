// * Diálogo "Personalizar" de una imagen de la biblioteca: le pone un nombre, una foto (por ejemplo, el logotipo de
// * un Windows que has preparado) y, si se quiere, un GIF animado de vista previa para reconocerla de un vistazo. Solo
// * cambia lo que se enseña: el archivo de la imagen no se toca. La foto se elige con el selector del sistema, se
// * recorta cuadrada y se reduce a 128 px aquí mismo, así que lo que se guarda en el USB es pequeño. El GIF se guarda
// * tal cual (hasta 6 MB) para que siga animado.

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

/** Tamaño máximo del GIF de vista previa (el mismo límite que pone Core). */
const MAX_GIF = 6 << 20;

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

/** Lee un GIF como data URL, sin tocarlo (así conserva la animación). */
function readGif(file: File): Promise<string> {
  return new Promise(function readPromise(resolve, reject) {
    if (file.type !== "image/gif") {
      reject(new Error(t("look.badGif")));
      return;
    }
    if (file.size > MAX_GIF) {
      reject(new Error(t("look.gifTooBig")));
      return;
    }
    const reader = new FileReader();
    reader.onload = function done(): void {
      resolve(String(reader.result));
    };
    reader.onerror = function failed(): void {
      reject(new Error(t("look.badGif")));
    };
    reader.readAsDataURL(file);
  });
}

/** Primer fotograma de cada GIF ya cargado (para enseñarlo quieto hasta que se pasa por encima). */
const posters = new Map<string, string>();

/** Dibuja el primer fotograma de un GIF en un JPEG pequeño. Devuelve "" si no se puede. */
function posterOf(image: HTMLImageElement): string {
  try {
    const canvas = document.createElement("canvas");
    canvas.width = image.naturalWidth;
    canvas.height = image.naturalHeight;
    const context = canvas.getContext("2d");
    if (context === null || canvas.width === 0) {
      return "";
    }
    context.drawImage(image, 0, 0);
    return canvas.toDataURL("image/jpeg", QUALITY);
  } catch {
    return "";
  }
}

/**
 * Vista previa animada de una imagen. Quieta (su primer fotograma) en las listas, porque decenas de GIF moviéndose a la
 * vez gastan mucho en un equipo modesto; se anima al pasar el puntero o al enfocarla con el teclado.
 */
export function gifThumb(url: string, animated = false): HTMLElement {
  const picture = el("img", { class: "item-picture", alt: "" });
  const frame = el("span", { class: "device-icon device-photo item-gif" }, [picture]);
  if (animated) {
    picture.src = url;
    return frame;
  }
  /** Enseña el GIF animado. */
  function play(): void {
    picture.src = url;
  }
  /** Vuelve al primer fotograma (o deja el GIF si no se pudo sacar). */
  function rest(): void {
    picture.src = posters.get(url) || url;
  }
  const known = posters.get(url);
  if (known) {
    picture.src = known;
  } else {
    const probe = new Image();
    probe.onload = function captured(): void {
      const poster = posterOf(probe);
      if (poster !== "") {
        posters.set(url, poster);
      }
      picture.src = poster || url;
    };
    probe.onerror = function missing(): void {
      frame.replaceChildren(icon(icons.film));
    };
    probe.src = url;
  }
  frame.addEventListener("pointerenter", play);
  frame.addEventListener("pointerleave", rest);
  frame.tabIndex = 0;
  frame.addEventListener("focus", play);
  frame.addEventListener("blur", rest);
  return frame;
}

/** Foto de una imagen de la biblioteca: su GIF de vista previa, su foto o el icono genérico si no tiene. */
export function itemPicture(item: LibraryItem): HTMLElement {
  if (item.preview) {
    return gifThumb(item.preview);
  }
  if (item.image) {
    const picture = el("img", { class: "item-picture", alt: "", src: item.image });
    return el("span", { class: "device-icon device-photo" }, [picture]);
  }
  return el("span", { class: "device-icon" }, [
    icon(item.kind === "iso" ? icons.fileDisc : icons.disk),
  ]);
}

/** Abre el diálogo para cambiar el nombre, la foto y la vista previa de una imagen; `done` se llama al guardar. */
export function openLookDialog(item: LibraryItem, done: () => void): void {
  let image = item.image ?? "";
  // * undefined: sin cambios; "": quitar el GIF; otro texto: GIF nuevo (data URL).
  let gif: string | undefined;
  let gifShown = item.preview ?? "";
  const preview = el("span", { class: "look-preview" });
  const gifPreview = el("span", { class: "look-preview look-preview-wide" });
  const nameInput = el("input", {
    class: "text-input",
    type: "text",
    maxlength: "80",
    placeholder: item.name,
    "aria-label": t("look.name"),
  });
  nameInput.value = item.title ?? "";
  const fileInput = el("input", { type: "file", accept: "image/*", hidden: true });
  const gifInput = el("input", { type: "file", accept: "image/gif", hidden: true });
  const message = el("div", { class: "look-message" });

  /** Dibuja la foto actual (o el icono) en la vista previa. */
  function drawPreview(): void {
    if (image) {
      preview.replaceChildren(el("img", { class: "item-picture", alt: "", src: image }));
    } else {
      preview.replaceChildren(icon(item.kind === "iso" ? icons.fileDisc : icons.disk));
    }
  }

  /** Dibuja el GIF actual (o el icono de película) en su vista previa. */
  function drawGif(): void {
    if (gifShown) {
      gifPreview.replaceChildren(el("img", { class: "item-picture", alt: "", src: gifShown }));
    } else {
      gifPreview.replaceChildren(icon(icons.film));
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

  /** Elige el GIF: se enseña al momento y se guarda al pulsar Guardar. */
  async function onGif(): Promise<void> {
    const file = gifInput.files?.[0];
    if (!file) {
      return;
    }
    try {
      gif = await readGif(file);
      gifShown = gif;
      message.replaceChildren();
      drawGif();
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

  /** Quita el GIF. */
  function removeGif(): void {
    gif = "";
    gifShown = "";
    gifInput.value = "";
    drawGif();
  }

  /** Guarda nombre, foto y GIF en el USB. */
  async function save(event: MouseEvent): Promise<void> {
    const dialog = (event.currentTarget as HTMLElement).closest("dialog");
    try {
      await postJSON("/api/library/look", {
        path: item.path,
        title: nameInput.value.trim(),
        image,
      });
      if (gif !== undefined) {
        await postJSON("/api/library/preview", { path: item.path, gif });
      }
      dialog?.close();
      done();
    } catch (error) {
      message.replaceChildren(errorBox(error));
    }
  }

  /** Abre el selector de fotos del sistema. */
  function choosePicture(): void {
    fileInput.click();
  }

  /** Abre el selector de GIF del sistema. */
  function chooseGif(): void {
    gifInput.click();
  }

  fileInput.addEventListener("change", onFile);
  gifInput.addEventListener("change", onGif);
  drawPreview();
  drawGif();
  openModal(
    t("look.title"),
    [
      el("p", { class: "muted" }, [t("look.hint")]),
      el("div", { class: "look-row" }, [
        preview,
        el("div", { class: "look-actions" }, [
          button(t("look.choose"), choosePicture, { iconPaths: icons.image }),
          button(t("look.remove"), removePicture, { iconPaths: icons.x }),
        ]),
      ]),
      el("p", { class: "muted small-note" }, [t("look.gifHint")]),
      el("div", { class: "look-row" }, [
        gifPreview,
        el("div", { class: "look-actions" }, [
          button(t("look.gifChoose"), chooseGif, { iconPaths: icons.film }),
          button(t("look.gifRemove"), removeGif, { iconPaths: icons.x }),
        ]),
      ]),
      el("label", { class: "confirm-field" }, [el("span", {}, [t("look.name")]), nameInput]),
      el("p", { class: "muted small-note" }, [item.name]),
      message,
      fileInput,
      gifInput,
      el("div", { class: "deploy-actions" }, [
        button(t("look.save"), save, { variant: "primary", iconPaths: icons.check }),
      ]),
    ],
    "look-dialog",
  );
}

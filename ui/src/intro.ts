// * Intro de Rebootera OS al abrir la interfaz: la insignia se dibuja sola y el nombre entra letra a letra (la
// * animación está en intro.css). Dura lo que tarden los primeros datos del sistema, con un mínimo para que se
// * vea entera y un máximo para no hacer esperar. Se salta con cualquier tecla o clic y respeta "reducir
// * movimiento" y el modo ligero de los equipos modestos.

import { isLite, prefs } from "./prefs.js";

/** Duraciones en milisegundos. */
const MIN_MS = 2800;
const MAX_MS = 4600;
const REDUCED_MS = 350;
const LEAVE_MS = 520;

let finish: (() => void) | null = null;

/** Indica si hay que reducir el movimiento. */
function reducedMotion(): boolean {
  return (
    prefs.reduceMotion || isLite() || window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

/** Escribe el nombre letra a letra ("Rebootera" y "OS" con el color de acento). */
function buildWordmark(target: HTMLElement): void {
  target.replaceChildren();
  let index = 0;
  for (const letter of "Rebootera OS") {
    const span = document.createElement("span");
    if (letter === " ") {
      span.className = "intro-space";
    } else {
      span.textContent = letter;
    }
    if (index >= 10) {
      span.classList.add("intro-os");
    }
    span.style.setProperty("--i", String(index));
    target.append(span);
    index++;
  }
}

/** Espera unos milisegundos. */
function wait(ms: number): Promise<void> {
  return new Promise(function waitResolver(resolve) {
    window.setTimeout(resolve, ms);
  });
}

/** Termina la intro antes de tiempo (tecla o clic). */
function skip(): void {
  finish?.();
}

/**
 * Reproduce la intro hasta que `ready` termine (respetando el mínimo y el máximo) o hasta que el
 * usuario la salte. Si la intro está desactivada en los ajustes, la oculta al instante.
 */
export async function playIntro(ready: Promise<unknown>): Promise<void> {
  const intro = document.getElementById("intro");
  if (intro === null) {
    return;
  }
  if (!prefs.intro) {
    intro.remove();
    return;
  }
  const reduced = reducedMotion();
  intro.classList.toggle("intro-reduced", reduced);
  const wordmark = intro.querySelector<HTMLElement>(".intro-wordmark");
  if (wordmark) {
    buildWordmark(wordmark);
  }
  // * Un fotograma después, para que la animación de entrada tenga un punto de partida.
  window.requestAnimationFrame(function startPlay() {
    intro.classList.add("intro-play");
  });

  const skipped = new Promise<void>(function skipResolver(resolve) {
    finish = resolve;
  });
  window.addEventListener("keydown", skip, { once: true });
  intro.addEventListener("click", skip, { once: true });

  const dataAndMinimum = Promise.all([ready.catch(ignore), wait(reduced ? REDUCED_MS : MIN_MS)]);
  await Promise.race([dataAndMinimum, wait(MAX_MS), skipped]);

  intro.classList.add("intro-leave");
  await wait(reduced ? 120 : LEAVE_MS);
  window.removeEventListener("keydown", skip);
  intro.remove();
  finish = null;
}

/** No hace nada: un error al cargar no debe bloquear la intro. */
function ignore(): void {}

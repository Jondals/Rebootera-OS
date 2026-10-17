// * Intro de Rebootera OS al abrir la interfaz: auroras de fondo, logotipo que
// * se dibuja trazo a trazo, nombre letra a letra y una barra que avanza
// * mientras llegan los primeros datos del sistema. Dura lo que tarden esos
// * datos (con un mínimo para que se vea y un máximo para no hacer esperar).
// * Se salta con cualquier tecla o clic y respeta "reducir movimiento".

import { isLite, prefs } from "./prefs.js";

/** Duraciones en milisegundos. */
const MIN_MS = 3200;
const MAX_MS = 6500;
const REDUCED_MS = 700;
const LEAVE_MS = 900;
/** Retraso entre letras del nombre. */
const LETTER_STAGGER_MS = 55;
/** Momento en que empiezan a aparecer las letras. */
const LETTERS_START_MS = 1100;

let finish: (() => void) | null = null;

/** Indica si hay que reducir el movimiento. */
function reducedMotion(): boolean {
  return (
    prefs.reduceMotion || isLite() || window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

/** Escribe el nombre letra a letra con un retraso escalonado. */
function buildWordmark(target: HTMLElement): void {
  const word = "Rebootera";
  target.replaceChildren();
  for (let i = 0; i < word.length; i++) {
    const letter = document.createElement("span");
    letter.className = "intro-letter";
    letter.textContent = word[i];
    letter.style.animationDelay = LETTERS_START_MS + i * LETTER_STAGGER_MS + "ms";
    target.append(letter);
  }
  const os = document.createElement("span");
  os.className = "intro-letter intro-os";
  os.textContent = "OS";
  os.style.animationDelay = LETTERS_START_MS + (word.length + 2) * LETTER_STAGGER_MS + "ms";
  target.append(os);
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
 * Reproduce la intro hasta que `ready` termine (respetando el mínimo y el
 * máximo) o hasta que el usuario la salte. Si la intro está desactivada en
 * los ajustes, la oculta al instante.
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
  const bar = intro.querySelector<HTMLElement>(".intro-progress-fill");
  intro.classList.add("intro-play");

  const skipped = new Promise<void>(function skipResolver(resolve) {
    finish = resolve;
  });
  window.addEventListener("keydown", skip, { once: true });
  intro.addEventListener("click", skip, { once: true });

  // * La barra llega al 70 % con el tiempo mínimo y se completa con los datos.
  bar?.classList.add("intro-progress-run");
  const dataAndMinimum = Promise.all([ready.catch(ignore), wait(reduced ? REDUCED_MS : MIN_MS)]);
  await Promise.race([dataAndMinimum, wait(MAX_MS), skipped]);
  bar?.classList.add("intro-progress-done");

  intro.classList.add("intro-leave");
  await wait(reduced ? 200 : LEAVE_MS);
  window.removeEventListener("keydown", skip);
  intro.remove();
  finish = null;
}

/** No hace nada: un error al cargar no debe bloquear la intro. */
function ignore(): void {}

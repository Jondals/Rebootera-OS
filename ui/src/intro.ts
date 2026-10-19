// * Intro de Rebootera OS al abrir la interfaz: logotipo, nombre y una barra fina que avanza mientras
// * llegan los primeros datos del sistema. Es ligera a propósito (solo opacity y transform) para que no
// * vaya a tirones ni en equipos con renderizado por software. Dura lo que tarden esos datos (con un
// * mínimo para que se vea y un máximo para no hacer esperar). Se salta con cualquier tecla o clic y
// * respeta "reducir movimiento".

import { isLite, prefs } from "./prefs.js";

/** Duraciones en milisegundos. */
const MIN_MS = 1200;
const MAX_MS = 3500;
const REDUCED_MS = 350;
const LEAVE_MS = 450;

let finish: (() => void) | null = null;

/** Indica si hay que reducir el movimiento. */
function reducedMotion(): boolean {
  return (
    prefs.reduceMotion || isLite() || window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

/** Escribe el nombre en dos tramos: "Rebootera" y "OS" en cursiva con el color de acento. */
function buildWordmark(target: HTMLElement): void {
  target.replaceChildren();
  const name = document.createElement("span");
  name.textContent = "Rebootera";
  const os = document.createElement("span");
  os.className = "intro-os";
  os.textContent = "OS";
  target.append(name, os);
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
  const bar = intro.querySelector<HTMLElement>(".intro-progress-fill");
  // * Un fotograma después, para que la transición de entrada tenga un punto de partida.
  window.requestAnimationFrame(function startPlay() {
    intro.classList.add("intro-play");
    bar?.classList.add("intro-progress-run");
  });

  const skipped = new Promise<void>(function skipResolver(resolve) {
    finish = resolve;
  });
  window.addEventListener("keydown", skip, { once: true });
  intro.addEventListener("click", skip, { once: true });

  const dataAndMinimum = Promise.all([ready.catch(ignore), wait(reduced ? REDUCED_MS : MIN_MS)]);
  await Promise.race([dataAndMinimum, wait(MAX_MS), skipped]);
  bar?.classList.add("intro-progress-done");

  intro.classList.add("intro-leave");
  await wait(reduced ? 120 : LEAVE_MS);
  window.removeEventListener("keydown", skip);
  intro.remove();
  finish = null;
}

/** No hace nada: un error al cargar no debe bloquear la intro. */
function ignore(): void {}

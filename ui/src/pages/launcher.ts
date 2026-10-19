// * Lanzador de las portadas: un título, una barra de búsqueda y la lista de lo que se puede hacer. Al escribir
// * filtra las acciones y, además, los sistemas del catálogo ("ubuntu" ofrece descargar Ubuntu). Se maneja entero
// * con el teclado: flechas para moverse, Intro para elegir. Lo usan Inicio de Rebootera OS y de Rebootera Desktop.

import { getJSON, type Family } from "../api.js";
import { logoTile } from "../components.js";
import { el, icon } from "../dom.js";
import { getLanguage, t } from "../i18n.js";
import { icons } from "../icons.js";
import { navigate } from "../router.js";
import { presetSearch } from "./downloads.js";

/** Una cosa que se puede hacer desde la portada. */
export interface LauncherAction {
  icon: string;
  title: string;
  text: string;
  /** Palabras extra con las que también se encuentra al escribir. */
  keywords?: string;
  /** Si es la acción destacada, sale primero y más grande. */
  primary?: boolean;
  run: () => void;
}

/** Opciones del lanzador. */
export interface LauncherOptions {
  title: string;
  subtitle: string;
  actions: LauncherAction[];
  /** Fila de estado bajo el título (red, arranque...), si hay. */
  status?: HTMLElement | null;
}

/** Familias del catálogo, leídas una sola vez para buscar sistemas desde la portada. */
let families: Family[] | null = null;

/** Lee las familias del catálogo (si falla, la búsqueda solo ofrece acciones). */
async function loadFamilies(): Promise<void> {
  if (families !== null) {
    return;
  }
  try {
    families = await getJSON<Family[]>("/api/catalog/families");
  } catch {
    families = [];
  }
}

/** Quita acentos y mayúsculas para comparar lo escrito con los textos. */
function plain(text: string): string {
  return text.normalize("NFD").replace(/[̀-ͯ]/g, "").toLowerCase();
}

/** Indica si todas las palabras escritas están en el texto. */
function matches(haystack: string, query: string): boolean {
  const text = plain(haystack);
  return plain(query)
    .split(/\s+/)
    .filter(Boolean)
    .every(function hasWord(word) {
      return text.includes(word);
    });
}

/** Una fila de resultado: acción o sistema del catálogo. */
interface Row {
  node: HTMLElement;
  run: () => void;
}

/** Construye el lanzador de una portada. */
export function launcher(options: LauncherOptions): HTMLElement {
  let query = "";
  let selected = 0;
  let rows: Row[] = [];
  const list = el("div", { class: "cmd-list", role: "listbox" });
  const input = el("input", {
    class: "cmd-input",
    type: "text",
    placeholder: t("home.search"),
    "aria-label": t("home.search"),
    autocomplete: "off",
    spellcheck: "false",
  });

  /** Fila de una acción. */
  function actionRow(action: LauncherAction, index: number): Row {
    const node = el(
      "button",
      { class: "cmd-row" + (action.primary ? " cmd-primary" : ""), type: "button", role: "option" },
      [
        el("span", { class: "cmd-icon" }, [icon(action.icon)]),
        el("span", { class: "cmd-text" }, [
          el("strong", {}, [action.title]),
          el("span", {}, [action.text]),
        ]),
        el("kbd", { class: "cmd-key" }, [query === "" ? String(index + 1) : "↵"]),
      ],
    );
    return { node, run: action.run };
  }

  /** Fila de un sistema del catálogo (lleva a Descargar con la búsqueda ya escrita). */
  function systemRow(family: Family): Row {
    const lang = getLanguage();
    const node = el("button", { class: "cmd-row", type: "button", role: "option" }, [
      logoTile(family.id, family.name),
      el("span", { class: "cmd-text" }, [
        el("strong", {}, [t("home.downloadSystem", { name: family.name })]),
        el("span", {}, [family.summary[lang] ?? family.summary.en]),
      ]),
      el("kbd", { class: "cmd-key" }, ["↵"]),
    ]);
    return {
      node,
      run: function open(): void {
        presetSearch(family.name);
        navigate("downloads");
      },
    };
  }

  /** Recalcula la lista según lo escrito. */
  function refresh(): void {
    const found = options.actions.filter(function keep(action) {
      return (
        query === "" ||
        matches(action.title + " " + action.text + " " + (action.keywords ?? ""), query)
      );
    });
    rows = found.map(actionRow);
    if (query.trim().length >= 2 && families !== null) {
      const lang = getLanguage();
      const systems = families
        .filter(function keep(family) {
          return matches(
            family.name + " " + family.vendor + " " + (family.summary[lang] ?? ""),
            query,
          );
        })
        .slice(0, 6);
      rows.push(...systems.map(systemRow));
    }
    selected = Math.min(selected, Math.max(rows.length - 1, 0));
    list.replaceChildren(
      ...(rows.length > 0
        ? rows.map(function node(row, index) {
            row.node.setAttribute("aria-selected", String(index === selected));
            row.node.addEventListener("click", row.run);
            row.node.addEventListener("pointerenter", function hover(): void {
              select(index);
            });
            return row.node;
          })
        : [el("p", { class: "muted cmd-empty" }, [t("home.noMatch")])]),
    );
  }

  /** Marca la fila elegida sin redibujar todo. */
  function select(index: number): void {
    selected = index;
    rows.forEach(function mark(row, position) {
      row.node.setAttribute("aria-selected", String(position === selected));
    });
  }

  /** Atiende las teclas de la barra: flechas, Intro y Escape. */
  function onKey(event: KeyboardEvent): void {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      if (rows.length === 0) {
        return;
      }
      const step = event.key === "ArrowDown" ? 1 : -1;
      select((selected + step + rows.length) % rows.length);
      rows[selected].node.scrollIntoView({ block: "nearest" });
    } else if (event.key === "Enter") {
      event.preventDefault();
      rows[selected]?.run();
    } else if (event.key === "Escape" && query !== "") {
      input.value = "";
      onInput();
    }
  }

  /** Lo escrito cambió: se vuelve a filtrar. */
  function onInput(): void {
    query = input.value;
    selected = 0;
    refresh();
  }

  input.addEventListener("input", onInput);
  input.addEventListener("keydown", onKey);
  void loadFamilies().then(refresh);
  refresh();

  const root = el("section", { class: "launcher" }, [
    el("header", { class: "home-head" }, [
      el("h1", { class: "home-title" }, [options.title]),
      el("p", { class: "home-subtitle" }, [options.subtitle]),
      options.status ?? null,
    ]),
    el("div", { class: "cmd" }, [
      el("div", { class: "cmd-bar" }, [icon(icons.search), input]),
      list,
    ]),
  ]);
  // * La barra recibe el foco al abrir la portada, para poder escribir o usar las flechas enseguida.
  window.setTimeout(function focusBar(): void {
    if (root.isConnected) {
      input.focus({ preventScroll: true });
    }
  }, 60);
  return root;
}

// * Comportamiento de la landing de Rebootera OS (sin dependencias): idioma y tema al instante, menús de la cabecera,
// * descarga según el sistema del visitante (lee la release "latest" de GitHub), icono de la pestaña con los colores
// * del tema, capturas que siguen al tema, carrusel, aparición al desplazarse e inclinación de la captura de portada
// * con el ratón. Se carga al final del <body>; el tema guardado ya se aplicó antes de pintar (ver Layout.astro).
(function () {
  "use strict";

  var REPO = "Jondals/Rebootera-OS";
  var BASE = "https://github.com/" + REPO + "/releases/latest/download/";
  var body = document.body;
  var root = document.documentElement;
  var lastRelease = null;
  var reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  /** Lee un valor guardado (o null si el almacenamiento no está disponible). */
  function readStored(key) {
    try {
      return window.localStorage.getItem(key);
    } catch (error) {
      return null;
    }
  }

  /** Guarda un valor (si el almacenamiento no está disponible, dura lo que dure la página). */
  function writeStored(key, value) {
    try {
      window.localStorage.setItem(key, value);
    } catch (error) {
      // Sin almacenamiento no se recuerda.
    }
  }

  // ---------- Idioma ----------

  /** Idioma guardado o, si no hay, el del navegador. */
  function currentLanguage() {
    var saved = readStored("rebootera-lang");
    if (saved === "en" || saved === "es") {
      return saved;
    }
    return (navigator.language || "en").toLowerCase().indexOf("es") === 0 ? "es" : "en";
  }

  /** Aplica un idioma al documento. */
  function applyLanguage(language) {
    body.setAttribute("data-lang", language);
    root.lang = language;
    showVersion();
  }

  /** Pasa al otro idioma al instante y lo recuerda. */
  function toggleLanguage() {
    var next = body.getAttribute("data-lang") === "en" ? "es" : "en";
    writeStored("rebootera-lang", next);
    applyLanguage(next);
  }

  // ---------- Tema ----------

  /** Tema activo. */
  function currentTheme() {
    return root.getAttribute("data-theme") || "amoled";
  }

  /** Valor de una variable CSS del tema activo. */
  function cssVar(name) {
    return getComputedStyle(root).getPropertyValue(name).trim();
  }

  /** Mezcla dos colores "#rrggbb": la parte "amount" (0 a 1) es del primero. */
  function mixHex(first, second, amount) {
    var a = parseInt(first.slice(1), 16);
    var b = parseInt(second.slice(1), 16);
    var out = 0;
    for (var shift = 16; shift >= 0; shift -= 8) {
      var channel = Math.round(((a >> shift) & 255) * amount + ((b >> shift) & 255) * (1 - amount));
      out = out * 256 + channel;
    }
    return "#" + ("000000" + out.toString(16)).slice(-6);
  }

  /** Redibuja el icono de la pestaña (el logotipo) con los colores del tema activo. */
  function updateFavicon() {
    var link = document.getElementById("favicon");
    var accent = cssVar("--accent");
    if (!link || accent.charAt(0) !== "#") {
      return;
    }
    var tile = mixHex(cssVar("--panel"), accent, 0.82);
    var svg =
      '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256"><defs>' +
      '<linearGradient id="a" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="' + tile + '"/><stop offset="1" stop-color="' + cssVar("--bg") + '"/></linearGradient>' +
      '<radialGradient id="b" cx="128" cy="170" r="120" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="' + accent + '" stop-opacity=".4"/><stop offset="1" stop-color="' + accent + '" stop-opacity="0"/></radialGradient>' +
      '<linearGradient id="c" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="' + mixHex(accent, "#ffffff", 0.55) + '"/><stop offset="1" stop-color="' + cssVar("--accent-strong") + '"/></linearGradient>' +
      '<clipPath id="d"><rect x="8" y="8" width="240" height="240" rx="58"/></clipPath></defs>' +
      '<rect x="8" y="8" width="240" height="240" rx="58" fill="url(#a)"/>' +
      '<rect x="8" y="8" width="240" height="240" fill="url(#b)" clip-path="url(#d)"/>' +
      '<rect x="9.5" y="9.5" width="237" height="237" rx="56.5" fill="none" stroke="' + accent + '" stroke-opacity=".45" stroke-width="3"/>' +
      '<g fill="none" stroke="' + accent + '" stroke-linecap="round">' +
      '<path d="M60.8 121A82 82 0 0 1 195.2 121" stroke-opacity=".9" stroke-width="8"/>' +
      '<path d="M37.9 116A104 104 0 0 1 218.1 116" stroke-opacity=".45" stroke-width="6"/>' +
      '<path d="M56 182H200" stroke-opacity=".85" stroke-width="8"/>' +
      '<path d="M86 200H170" stroke-opacity=".4" stroke-width="8"/></g>' +
      '<path d="M72 168A56 56 0 0 1 184 168Z" fill="url(#c)"/></svg>';
    link.setAttribute("href", "data:image/svg+xml," + encodeURIComponent(svg));
  }

  /** Ruta de la captura de una pantalla en un tema. */
  function shotPath(name, theme) {
    return "images/" + theme + "/" + name + ".webp";
  }

  /** Pone en cada captura la imagen del tema. */
  function updateShots() {
    var images = document.querySelectorAll("img[data-shot]");
    for (var i = 0; i < images.length; i++) {
      var name = images[i].getAttribute("data-shot");
      images[i].setAttribute("src", shotPath(name, currentTheme()));
      images[i].setAttribute("srcset", shotPath(name + "-sm", currentTheme()) + " 800w, " + shotPath(name, currentTheme()) + " 1440w");
    }
  }

  /** Marca el tema activo en las listas, pone el color de la barra del navegador y redibuja el icono. */
  function markTheme() {
    var options = document.querySelectorAll("[data-theme-opt]");
    for (var i = 0; i < options.length; i++) {
      options[i].setAttribute("aria-checked", String(options[i].getAttribute("data-theme-opt") === currentTheme()));
    }
    var swatches = document.querySelectorAll("[data-current-swatch]");
    for (var j = 0; j < swatches.length; j++) {
      swatches[j].style.setProperty("--s-bg", cssVar("--bg"));
      swatches[j].style.setProperty("--s-accent", cssVar("--accent"));
    }
    var meta = document.getElementById("theme-color");
    if (meta) {
      meta.setAttribute("content", cssVar("--bg") || "#000000");
    }
    updateFavicon();
  }

  /** Aplica un tema a toda la página. */
  function applyTheme(name) {
    // * Sin transiciones durante el cambio: todo pasa al nuevo tema a la vez, sin arrastre.
    root.classList.add("switching");
    root.setAttribute("data-theme", name);
    markTheme();
    updateShots();
    auroraPalette();
    window.requestAnimationFrame(function done() {
      window.requestAnimationFrame(function release() {
        root.classList.remove("switching");
      });
    });
  }

  /** Aplica el tema de la opción pulsada al instante y lo recuerda. */
  function pickTheme(event) {
    var name = event.currentTarget.getAttribute("data-theme-opt");
    if (name === currentTheme()) {
      return;
    }
    writeStored("rebootera-theme", name);
    applyTheme(name);
    closeMenus();
  }

  // ---------- Menús de la cabecera ----------

  /** Cierra todos los menús desplegables. */
  function closeMenus() {
    var panels = document.querySelectorAll("[data-menu]");
    for (var i = 0; i < panels.length; i++) {
      panels[i].setAttribute("data-open", "false");
    }
    var buttons = document.querySelectorAll("[data-menu-btn]");
    for (var j = 0; j < buttons.length; j++) {
      buttons[j].setAttribute("aria-expanded", "false");
    }
    showMenuIcon(false);
  }

  /** Enseña el icono de menú o el de cerrar en el botón del móvil. */
  function showMenuIcon(open) {
    var opened = document.querySelector("[data-icon-open]");
    var closed = document.querySelector("[data-icon-close]");
    if (opened && closed) {
      opened.classList.toggle("hidden", open);
      closed.classList.toggle("hidden", !open);
    }
  }

  /** Abre el menú del botón pulsado (y cierra el otro); si ya estaba abierto, lo cierra. */
  function toggleMenu(event) {
    var button = event.currentTarget;
    var name = button.getAttribute("data-menu-btn");
    var panel = document.querySelector('[data-menu="' + name + '"]');
    var wasOpen = panel.getAttribute("data-open") === "true";
    closeMenus();
    if (!wasOpen) {
      panel.setAttribute("data-open", "true");
      button.setAttribute("aria-expanded", "true");
      showMenuIcon(name === "mobile");
    }
  }

  /** Cierra los menús al pulsar fuera de ellos. */
  function closeOnOutside(event) {
    if (!event.target.closest("[data-menu], [data-menu-btn]")) {
      closeMenus();
    }
  }

  /** Cierra los menús con Escape. */
  function closeOnEscape(event) {
    if (event.key === "Escape") {
      closeMenus();
    }
  }

  // ---------- Enlaces ----------

  /** Pone el destino de un enlace (un href normal: funciona con clic central, teclado y lectores de pantalla). */
  function setLink(element, url) {
    element.setAttribute("href", url);
  }

  // ---------- Preguntas ----------

  /** Abre o cierra una pregunta (las demás se quedan como estaban). */
  function toggleFaq(event) {
    var item = event.currentTarget.closest(".faq");
    var open = item.getAttribute("data-open") !== "true";
    item.setAttribute("data-open", String(open));
    event.currentTarget.setAttribute("aria-expanded", String(open));
  }

  /** Conecta los botones de las preguntas. */
  function setupFaq() {
    var buttons = document.querySelectorAll(".faq-q");
    for (var i = 0; i < buttons.length; i++) {
      buttons[i].addEventListener("click", toggleFaq);
    }
  }

  // ---------- Descarga ----------

  var PLATFORMS = {
    windows: { name: "Windows", variants: [{ label: "x64", asset: "Rebootera-Desktop-Windows.exe" }] },
    macos: {
      name: "macOS",
      variants: [
        { label: "Apple silicon", asset: "Rebootera-Desktop-macOS-AppleSilicon" },
        { label: "Intel", asset: "Rebootera-Desktop-macOS-Intel" },
      ],
    },
    linux: {
      name: "Linux",
      variants: [
        { label: "x64", asset: "Rebootera-Desktop-Linux-x64" },
        { label: "arm64", asset: "Rebootera-Desktop-Linux-arm64" },
      ],
    },
  };
  var chosen = { id: "windows", variant: 0 };

  /** Tamaño legible. */
  function megabytes(bytes) {
    return bytes >= 1e9 ? (bytes / 1e9).toFixed(1) + " GB" : Math.round(bytes / 1e6) + " MB";
  }

  /** Sistema del visitante (null si no es un ordenador de escritorio). */
  function detectPlatform() {
    var agent = (navigator.userAgent || "").toLowerCase();
    var data = navigator.userAgentData;
    var platform = ((data && data.platform) || navigator.platform || "").toLowerCase();
    if (/android|iphone|ipad|ipod/.test(agent)) {
      return null;
    }
    if (platform.indexOf("win") === 0 || agent.indexOf("windows") >= 0) {
      return "windows";
    }
    if (platform.indexOf("mac") === 0 || agent.indexOf("mac os") >= 0) {
      return "macos";
    }
    if (platform.indexOf("linux") >= 0 || agent.indexOf("linux") >= 0) {
      return "linux";
    }
    return null;
  }

  /** Pulsa un botón de variante (Apple silicon, Intel, x64, arm64). */
  function pickVariant(event) {
    chosen.variant = Number(event.currentTarget.getAttribute("data-variant"));
    renderDownload();
  }

  /** Pulsa el botón de un sistema. */
  function pickPlatform(event) {
    chosen.id = event.currentTarget.getAttribute("data-os");
    chosen.variant = 0;
    renderDownload();
  }

  /** Crea una sola vez los botones de variante de cada sistema (así pulsarlos no los vuelve a dibujar). */
  function buildVariants() {
    var box = document.getElementById("dl-variants");
    Object.keys(PLATFORMS).forEach(function eachPlatform(id) {
      var variants = PLATFORMS[id].variants;
      if (variants.length < 2) {
        return;
      }
      var group = document.createElement("div");
      group.className = "flex items-center justify-center gap-2 lg:justify-start";
      group.setAttribute("data-on", "false");
      group.setAttribute("data-variants", id);
      variants.forEach(function eachVariant(variant, index) {
        var chip = document.createElement("button");
        chip.type = "button";
        chip.className = "chip";
        chip.textContent = variant.label;
        chip.setAttribute("data-variant", String(index));
        chip.addEventListener("click", pickVariant);
        group.appendChild(chip);
      });
      box.appendChild(group);
    });
  }

  /** Actualiza el botón grande, el sistema marcado, la nota y las variantes; solo cambia lo que cambia de verdad. */
  function renderDownload() {
    var platform = PLATFORMS[chosen.id];
    var label = document.getElementById("dl-os");
    if (label.textContent !== platform.name) {
      label.textContent = platform.name;
      // * El nombre nuevo entra con un fundido corto (no salta de golpe).
      if (!reduced && label.animate) {
        label.animate([{ opacity: 0, transform: "translateY(4px)" }, { opacity: 1, transform: "none" }], { duration: 320, easing: "cubic-bezier(0.16, 1, 0.3, 1)" });
      }
    }
    setLink(document.getElementById("dl-main"), BASE + platform.variants[chosen.variant].asset);
    var tabs = document.querySelectorAll("[data-os]");
    for (var i = 0; i < tabs.length; i++) {
      tabs[i].setAttribute("aria-pressed", String(tabs[i].getAttribute("data-os") === chosen.id));
    }
    var seg = document.querySelector(".seg");
    if (seg) {
      seg.style.setProperty("--i", String(["windows", "macos", "linux"].indexOf(chosen.id)));
    }
    var notes = document.querySelectorAll("[data-note]");
    for (var j = 0; j < notes.length; j++) {
      notes[j].setAttribute("data-on", String(notes[j].getAttribute("data-note") === chosen.id));
    }
    document.getElementById("dl-variants-wrap").setAttribute("data-open", String(platform.variants.length > 1));
    // * Si el sistema nuevo no tiene variantes, los botones del anterior se quedan mientras el bloque se cierra.
    var groups = document.querySelectorAll("[data-variants]");
    for (var g = 0; g < groups.length; g++) {
      var shown = groups[g].getAttribute("data-variants") === chosen.id;
      if (platform.variants.length > 1) {
        groups[g].setAttribute("data-on", String(shown));
      }
      if (shown) {
        var chips = groups[g].children;
        for (var k = 0; k < chips.length; k++) {
          chips[k].setAttribute("aria-pressed", String(k === chosen.variant));
        }
      }
    }
  }

  /** Ajusta la variante a la arquitectura del visitante, si el navegador la dice. */
  function detectArchitecture() {
    var agent = (navigator.userAgent || "").toLowerCase();
    if (chosen.id === "linux" && /aarch64|arm/.test(agent)) {
      chosen.variant = 1;
      renderDownload();
      return;
    }
    var data = navigator.userAgentData;
    if (chosen.id === "macos" && data && data.getHighEntropyValues) {
      data.getHighEntropyValues(["architecture"]).then(function onArchitecture(values) {
        if (values.architecture === "x86" && chosen.id === "macos" && chosen.variant === 0) {
          chosen.variant = 1;
          renderDownload();
        }
      });
    }
  }

  /** Conecta los botones de sistema, elige el del visitante y apunta el enlace de la ISO. */
  function setupDownload() {
    var tabs = document.querySelectorAll("[data-os]");
    for (var i = 0; i < tabs.length; i++) {
      tabs[i].addEventListener("click", pickPlatform);
    }
    setLink(document.getElementById("dl-iso"), BASE + "rebootera-os-amd64.iso");
    buildVariants();
    chosen.id = detectPlatform() || "windows";
    chosen.variant = 0;
    renderDownload();
    detectArchitecture();
  }

  /** Muestra versión y fecha de la última compilación (nada si todavía no se ha podido leer). */
  function showVersion() {
    var box = document.getElementById("dl-version");
    if (!box || !lastRelease) {
      return;
    }
    var es = body.getAttribute("data-lang") === "es";
    var when = new Date(lastRelease.published_at).toLocaleDateString(es ? "es-ES" : "en-GB", {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
    box.textContent = (es ? "Versión " : "Version ") + lastRelease.version + " · " + (es ? "publicada el " : "published ") + when;
  }

  var releaseAsked = false;

  /** La versión publicada se pide al primer gesto del visitante o, como tarde, al segundo de cargar (no hace falta para pintar). */
  function loadReleaseOnce() {
    window.removeEventListener("pointerdown", loadReleaseOnce);
    window.removeEventListener("scroll", loadReleaseOnce);
    window.removeEventListener("keydown", loadReleaseOnce);
    if (!releaseAsked) {
      releaseAsked = true;
      loadRelease();
    }
  }

  /** Lee la release "latest" de la API pública de GitHub (sin clave). Si no hay, la página no dice nada. */
  function loadRelease() {
    var request = new XMLHttpRequest();
    request.open("GET", "https://api.github.com/repos/" + REPO + "/releases/latest");
    request.setRequestHeader("Accept", "application/vnd.github+json");
    request.onload = function onLoad() {
      if (request.status !== 200) {
        return;
      }
      var data = JSON.parse(request.responseText);
      var notes = data.body || "";
      var version = (/version:\s*([0-9][^\s]*)/i.exec(notes) || [])[1] || data.name || "latest";
      lastRelease = { version: version, published_at: data.published_at };
      data.assets.forEach(function eachAsset(asset) {
        if (/\.iso$/.test(asset.name)) {
          document.getElementById("iso-meta").textContent = " · " + megabytes(asset.size);
        }
      });
      showVersion();
    };
    request.send();
  }

  // ---------- Aparición al desplazarse ----------

  /** Enseña cada bloque .reveal cuando entra en pantalla. */
  function setupReveal() {
    var items = document.querySelectorAll(".reveal");
    if (reduced || !("IntersectionObserver" in window)) {
      for (var i = 0; i < items.length; i++) {
        items[i].classList.add("in");
      }
      return;
    }
    var observer = new IntersectionObserver(
      function onSeen(entries) {
        entries.forEach(function eachEntry(entry) {
          if (entry.isIntersecting) {
            entry.target.classList.add("in");
            observer.unobserve(entry.target);
          }
        });
      },
      { threshold: 0.12, rootMargin: "0px 0px -6% 0px" },
    );
    for (var j = 0; j < items.length; j++) {
      observer.observe(items[j]);
    }
  }

  // ---------- Movimiento con el ratón y el scroll ----------

  var pointer = { x: 0, y: 0 };
  var eased = { x: 0, y: 0 };
  var running = false;

  /** Giro de reposo de la captura: de lado en pantallas anchas (donde va junto al texto) y recta en las demás. */
  function baseAngle() {
    return window.innerWidth >= 1024 ? -9 : 0;
  }

  /** Un fotograma: acerca poco a poco la inclinación de la captura a donde está el ratón. */
  function frame() {
    var dx = pointer.x - eased.x;
    var dy = pointer.y - eased.y;
    eased.x += dx * 0.07;
    eased.y += dy * 0.07;
    var tilt = document.getElementById("tilt");
    if (tilt) {
      tilt.style.transform = "perspective(1800px) rotateY(" + (baseAngle() + eased.x * 4).toFixed(3) + "deg) rotateX(" + (baseAngle() * -0.3 - eased.y * 3).toFixed(3) + "deg)";
      document.getElementById("tilt-glow").style.transform = "translate(" + (-eased.x * 16).toFixed(2) + "px, " + (-eased.y * 12).toFixed(2) + "px)";
    }
    if (Math.abs(dx) + Math.abs(dy) > 0.002) {
      window.requestAnimationFrame(frame);
    } else {
      running = false;
    }
  }

  /** Pone en marcha el bucle de movimiento si estaba parado. */
  function kick() {
    if (!running) {
      running = true;
      window.requestAnimationFrame(frame);
    }
  }

  /** Recuerda dónde está el ratón (de -1 a 1 en cada eje) y mueve la luz de la tarjeta que tenga debajo. */
  function onPointerMove(event) {
    pointer.x = (event.clientX / window.innerWidth - 0.5) * 2;
    pointer.y = (event.clientY / window.innerHeight - 0.5) * 2;
    kick();
    var card = event.target.closest && event.target.closest(".spot");
    if (card) {
      var box = card.getBoundingClientRect();
      card.style.setProperty("--mx", (event.clientX - box.left).toFixed(0) + "px");
      card.style.setProperty("--my", (event.clientY - box.top).toFixed(0) + "px");
    }
  }

  /** Devuelve la captura a su sitio cuando el ratón sale de la ventana. */
  function onPointerLeave() {
    pointer.x = 0;
    pointer.y = 0;
    kick();
  }

  var scrollRoom = 1;
  var scrollQueued = false;

  /** Recalcula cuánto se puede bajar (solo al cargar y al cambiar el tamaño, no en cada fotograma). */
  function measureScroll() {
    scrollRoom = Math.max(1, document.documentElement.scrollHeight - window.innerHeight);
    onScrollFrame();
  }

  /** La cabecera se vuelve translúcida en cuanto se baja un poco y el fondo avanza según lo que se ha bajado (de 0 a 1). */
  function onScrollFrame() {
    scrollQueued = false;
    var header = document.getElementById("header");
    if (header) {
      header.setAttribute("data-scrolled", String(window.scrollY > 12));
    }
    aurora.target = Math.min(1, Math.max(0, window.scrollY / scrollRoom));
    if (reduced) {
      stillAurora();
    }
  }

  /** Pide un fotograma para el scroll (varios eventos seguidos se juntan en uno). */
  function updateHeader() {
    if (!scrollQueued) {
      scrollQueued = true;
      window.requestAnimationFrame(onScrollFrame);
    }
  }

  /** Conecta el movimiento (nada con "reducir movimiento") y el estado de la cabecera. */
  function setupMotion() {
    window.addEventListener("scroll", updateHeader, { passive: true });
    window.addEventListener("resize", measureScroll);
    window.addEventListener("load", measureScroll);
    measureScroll();
    if (reduced) {
      return;
    }
    window.addEventListener("mousemove", onPointerMove, { passive: true });
    document.addEventListener("mouseleave", onPointerLeave);
    kick();
  }

  // ---------- Cifras que cuentan ----------

  /** Cuenta un número desde cero hasta su valor en algo más de un segundo, frenando al final. */
  function countUp(element) {
    var goal = Number(element.getAttribute("data-count"));
    var start = performance.now();
    function step(now) {
      var t = Math.min(1, (now - start) / 1300);
      element.textContent = String(Math.round(goal * (1 - Math.pow(1 - t, 3))));
      if (t < 1) {
        window.requestAnimationFrame(step);
      }
    }
    element.textContent = "0";
    window.requestAnimationFrame(step);
  }

  /** Las cifras empiezan a contar cuando entran en pantalla (con "reducir movimiento" se quedan en su valor). */
  function setupCounters() {
    var items = document.querySelectorAll("[data-count]");
    if (reduced || !("IntersectionObserver" in window)) {
      return;
    }
    var observer = new IntersectionObserver(
      function onSeen(entries) {
        entries.forEach(function eachEntry(entry) {
          if (entry.isIntersecting) {
            countUp(entry.target);
            observer.unobserve(entry.target);
          }
        });
      },
      { threshold: 0.6 },
    );
    for (var i = 0; i < items.length; i++) {
      observer.observe(items[i]);
    }
  }

  // ---------- Aurora del fondo ----------

  // * Cada cinta de luz: posición arriba (x, y) y abajo de la página (ly), cuánto se mece (ax, ay) y a qué ritmo
  // * (fx, fy), en qué punto del vaivén empieza (ph), su tamaño (r), lo alargada que es (sx, sy), su giro (rot) y lo que
  // * cabecea (sway). Todo en proporción al lienzo.
  var BLOBS = [
    { x: 0.25, y: 0.02, ly: 0.7, ax: 0.16, ay: 0.06, fx: 0.05, fy: 0.04, ph: 0, r: 0.42, sx: 2.2, sy: 0.55, rot: -0.35, sway: 0.18 },
    { x: 0.78, y: 0.12, ly: 0.92, ax: 0.14, ay: 0.08, fx: 0.04, fy: 0.06, ph: 2.1, r: 0.4, sx: 2, sy: 0.5, rot: 0.4, sway: 0.22 },
    { x: 0.5, y: -0.08, ly: 0.55, ax: 0.22, ay: 0.05, fx: 0.03, fy: 0.05, ph: 4.2, r: 0.38, sx: 2.6, sy: 0.4, rot: 0.08, sway: 0.15 },
    { x: 0.92, y: 0.5, ly: 0.3, ax: 0.08, ay: 0.16, fx: 0.05, fy: 0.03, ph: 1.3, r: 0.34, sx: 0.6, sy: 1.9, rot: 0.2, sway: 0.2 },
    { x: 0.06, y: 0.6, ly: 0.2, ax: 0.08, ay: 0.14, fx: 0.04, fy: 0.05, ph: 3.4, r: 0.3, sx: 0.65, sy: 1.8, rot: -0.15, sway: 0.2 },
  ];
  var aurora = { canvas: null, ctx: null, top: [], low: [], bg: "#000000", p: 0, target: 0, last: 0, running: false, mx: 0, my: 0 };

  /** "#rrggbb" a [tono (0 a 360), saturación y luminosidad (0 a 1)]. */
  function hexToHsl(hex) {
    var n = parseInt(hex.slice(1), 16);
    var r = ((n >> 16) & 255) / 255;
    var g = ((n >> 8) & 255) / 255;
    var b = (n & 255) / 255;
    var max = Math.max(r, g, b);
    var min = Math.min(r, g, b);
    var l = (max + min) / 2;
    var d = max - min;
    if (d === 0) {
      return [0, 0, l];
    }
    var s = d / (1 - Math.abs(2 * l - 1));
    var h = max === r ? ((g - b) / d) % 6 : max === g ? (b - r) / d + 2 : (r - g) / d + 4;
    return [(h * 60 + 360) % 360, s, l];
  }

  /** Color "hsla(...)" a partir de tono, saturación, luminosidad y opacidad. */
  function hsla(h, s, l, a) {
    return "hsla(" + (((h % 360) + 360) % 360).toFixed(1) + ", " + (s * 100).toFixed(1) + "%, " + (l * 100).toFixed(1) + "%, " + a + ")";
  }

  /** Colores de la aurora según el tema: el acento y tonos vecinos arriba, y otra mezcla de tonos al fondo de la página. */
  function auroraPalette() {
    var accent = cssVar("--accent");
    var strong = cssVar("--accent-strong");
    if (accent.charAt(0) !== "#" || strong.charAt(0) !== "#") {
      return;
    }
    var a = hexToHsl(accent);
    var s = hexToHsl(strong);
    aurora.bg = cssVar("--bg") || "#000000";
    aurora.top = [
      [a[0], a[1], a[2] * 0.85],
      [a[0] + 18, a[1], a[2] * 0.7],
      [s[0] - 14, s[1], s[2] * 0.65],
      [a[0] + 32, a[1] * 0.85, a[2] * 0.55],
    ];
    aurora.low = [
      [a[0] - 22, a[1] * 0.9, a[2] * 0.6],
      [s[0], s[1], s[2] * 0.75],
      [a[0] + 40, a[1] * 0.7, a[2] * 0.5],
      [a[0] + 10, a[1], a[2] * 0.7],
    ];
    drawAurora(aurora.last);
  }

  /** Ajusta el lienzo a la forma de la ventana (siempre diminuto: 96 píxeles de ancho). */
  function sizeAurora() {
    if (!aurora.canvas) {
      return;
    }
    aurora.canvas.width = 96;
    aurora.canvas.height = Math.max(40, Math.min(220, Math.round((96 * window.innerHeight) / window.innerWidth)));
    drawAurora(aurora.last);
  }

  /** Pinta un fotograma de la aurora en el instante "time" (en segundos). */
  function drawAurora(time) {
    var ctx = aurora.ctx;
    if (!ctx || !aurora.top.length) {
      return;
    }
    var w = aurora.canvas.width;
    var h = aurora.canvas.height;
    var p = aurora.p;
    var size = Math.max(w, h);
    ctx.globalCompositeOperation = "source-over";
    ctx.fillStyle = aurora.bg;
    ctx.fillRect(0, 0, w, h);
    ctx.globalCompositeOperation = "lighter";
    for (var i = 0; i < BLOBS.length; i++) {
      var blob = BLOBS[i];
      var top = aurora.top[i % aurora.top.length];
      var low = aurora.low[i % aurora.low.length];
      // * Cada cinta se inclina un poco hacia el ratón; las del fondo (las más altas en la lista) menos.
      var lean = 0.05 - i * 0.006;
      var x = (blob.x + blob.ax * Math.sin(time * blob.fx * 6.283 + blob.ph) + aurora.mx * lean) * w;
      var y = (blob.y + (blob.ly - blob.y) * p + blob.ay * Math.cos(time * blob.fy * 6.283 + blob.ph) + aurora.my * lean) * h;
      var radius = blob.r * size * (1 + 0.1 * Math.sin(time * 0.04 * 6.283 + blob.ph));
      var hue = top[0] + (low[0] - top[0]) * p;
      var sat = top[1] + (low[1] - top[1]) * p;
      var light = Math.min(0.5, top[2] + (low[2] - top[2]) * p);
      ctx.save();
      ctx.translate(x, y);
      ctx.rotate(blob.rot + blob.sway * Math.sin(time * 0.03 * 6.283 + blob.ph));
      ctx.scale(blob.sx, blob.sy);
      var gradient = ctx.createRadialGradient(0, 0, 0, 0, 0, radius);
      gradient.addColorStop(0, hsla(hue, sat, light, 0.36));
      gradient.addColorStop(0.35, hsla(hue, sat, light, 0.13));
      gradient.addColorStop(1, hsla(hue, sat, light, 0));
      ctx.fillStyle = gradient;
      ctx.fillRect(-radius, -radius, radius * 2, radius * 2);
      ctx.restore();
    }
  }

  /** Bucle de la aurora: unos 30 fotogramas por segundo, y el paso al color de abajo sigue al scroll con suavidad. */
  function auroraFrame(now) {
    if (document.hidden) {
      aurora.running = false;
      return;
    }
    window.requestAnimationFrame(auroraFrame);
    if (now - aurora.lastFrame < 33) {
      return;
    }
    aurora.lastFrame = now;
    aurora.p += (aurora.target - aurora.p) * 0.08;
    aurora.mx += (pointer.x - aurora.mx) * 0.05;
    aurora.my += (pointer.y - aurora.my) * 0.05;
    aurora.last = now / 1000;
    drawAurora(aurora.last);
  }

  /** Arranca el bucle si estaba parado (al cargar y al volver a la pestaña). */
  function startAurora() {
    if (!aurora.running && aurora.ctx && !reduced && !document.hidden) {
      aurora.running = true;
      aurora.lastFrame = 0;
      window.requestAnimationFrame(auroraFrame);
    }
  }

  /** Con "reducir movimiento" la aurora no se mueve sola: solo se repinta al bajar, ya en su sitio. */
  function stillAurora() {
    aurora.p = aurora.target;
    drawAurora(0);
  }

  /** Prepara el lienzo, los colores y el bucle. */
  function setupAurora() {
    aurora.canvas = document.getElementById("aurora");
    aurora.ctx = aurora.canvas && aurora.canvas.getContext ? aurora.canvas.getContext("2d") : null;
    if (!aurora.ctx) {
      return;
    }
    sizeAurora();
    auroraPalette();
    aurora.canvas.setAttribute("data-ready", "true");
    window.addEventListener("resize", sizeAurora);
    document.addEventListener("visibilitychange", startAurora);
    startAurora();
  }

  // ---------- Carrusel de capturas ----------

  /** Pestañas con barra de progreso: pasa solo a la siguiente, se detiene al pasar el ratón y se puede deslizar. */
  function setupShowcase() {
    var section = document.getElementById("showcase");
    if (!section) {
      return;
    }
    var tabs = section.querySelectorAll("[data-slide-tab]");
    var slides = section.querySelectorAll("[data-slide]");
    var texts = section.querySelectorAll("[data-slide-text]");
    var index = 0;
    var touchStart = null;

    /** Enseña la captura número "next". */
    function show(next) {
      index = (next + tabs.length) % tabs.length;
      for (var i = 0; i < tabs.length; i++) {
        tabs[i].setAttribute("aria-selected", String(i === index));
        slides[i].setAttribute("data-on", String(i === index));
        texts[i].classList.toggle("hidden", i !== index);
      }
    }

    /** Pulsar una pestaña lleva a su captura. */
    function onTab(event) {
      show(Array.prototype.indexOf.call(tabs, event.currentTarget));
    }

    /** Cuando una barra se llena, sigue la siguiente captura. */
    function onFilled(event) {
      if (event.animationName === "fill") {
        show(index + 1);
      }
    }

    /** Las flechas mueven entre pestañas. */
    function onKey(event) {
      if (event.key === "ArrowRight") {
        show(index + 1);
        tabs[index].focus();
      } else if (event.key === "ArrowLeft") {
        show(index - 1);
        tabs[index].focus();
      }
    }

    function onTouchStart(event) {
      touchStart = event.changedTouches[0].clientX;
    }

    function onTouchEnd(event) {
      if (touchStart === null) {
        return;
      }
      var delta = event.changedTouches[0].clientX - touchStart;
      touchStart = null;
      if (Math.abs(delta) > 50) {
        show(delta < 0 ? index + 1 : index - 1);
      }
    }

    for (var i = 0; i < tabs.length; i++) {
      tabs[i].addEventListener("click", onTab);
    }
    section.addEventListener("animationend", onFilled);
    section.addEventListener("keydown", onKey);
    var frame = document.getElementById("show-frame");
    frame.addEventListener("touchstart", onTouchStart, { passive: true });
    frame.addEventListener("touchend", onTouchEnd, { passive: true });
    if (reduced || !("IntersectionObserver" in window)) {
      section.setAttribute("data-still", "true");
      return;
    }
    var observer = new IntersectionObserver(
      function onSeen(entries) {
        section.setAttribute("data-live", String(entries[0].isIntersecting));
      },
      { threshold: 0.4 },
    );
    observer.observe(section);
  }

  // ---------- Arranque ----------

  document.addEventListener("contextmenu", function noMenu(event) {
    event.preventDefault();
  });
  document.addEventListener("dragstart", function noDrag(event) {
    event.preventDefault();
  });
  applyLanguage(currentLanguage());
  var languageButtons = document.querySelectorAll("[data-toggle-lang]");
  for (var l = 0; l < languageButtons.length; l++) {
    languageButtons[l].addEventListener("click", toggleLanguage);
  }
  var themeOptions = document.querySelectorAll("[data-theme-opt]");
  for (var t = 0; t < themeOptions.length; t++) {
    themeOptions[t].addEventListener("click", pickTheme);
  }
  var menuButtons = document.querySelectorAll("[data-menu-btn]");
  for (var m = 0; m < menuButtons.length; m++) {
    menuButtons[m].addEventListener("click", toggleMenu);
  }
  var closers = document.querySelectorAll("[data-close-menu]");
  for (var c = 0; c < closers.length; c++) {
    closers[c].addEventListener("click", closeMenus);
  }
  document.addEventListener("click", closeOnOutside);
  document.addEventListener("keydown", closeOnEscape);
  window.addEventListener("resize", closeMenus);
  applyTheme(currentTheme());
  setupDownload();
  setupFaq();
  setupReveal();
  window.addEventListener("pointerdown", loadReleaseOnce, { passive: true });
  window.addEventListener("scroll", loadReleaseOnce, { passive: true });
  window.addEventListener("keydown", loadReleaseOnce);
  window.setTimeout(loadReleaseOnce, 1200);
  setupAurora();
  setupCounters();
  setupMotion();
  setupShowcase();
})();

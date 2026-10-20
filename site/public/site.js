// * Comportamiento de la landing de Rebootera OS (sin dependencias): idioma y tema con fundido, menús de la cabecera,
// * descarga según el sistema del visitante (lee la release "latest" de GitHub), icono de la pestaña con los colores
// * del tema, capturas que siguen al tema, carrusel, aparición al desplazarse y movimiento con el ratón (inclinación
// * de la captura de portada y desplazamiento del fondo). Se carga al final del <body>; el tema guardado ya se aplicó
// * antes de pintar (ver Layout.astro).
(function () {
  "use strict";

  var REPO = "Jondals/Rebootera-OS";
  var BASE = "https://github.com/" + REPO + "/releases/download/latest/";
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

  /** Aplica un cambio con un fundido: el del navegador (mezcla de capturas) para los temas y, si se pide "text" o no existe, uno propio solo de texto. */
  function transition(change, kind) {
    if (reduced) {
      change();
      return;
    }
    if (kind !== "text" && document.startViewTransition) {
      document.startViewTransition(change);
      return;
    }
    body.classList.add("swapping");
    window.setTimeout(function swap() {
      change();
      window.requestAnimationFrame(function reveal() {
        body.classList.remove("swapping");
      });
    }, 220);
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

  /** Pasa al otro idioma con un fundido y lo recuerda. */
  function toggleLanguage() {
    var next = body.getAttribute("data-lang") === "en" ? "es" : "en";
    writeStored("rebootera-lang", next);
    transition(function change() {
      applyLanguage(next);
    }, "text");
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

  /** Descarga las capturas de un tema antes de cambiarlo (o sigue pasado un momento, para no dejar la página parada). */
  function preloadShots(theme, done) {
    var images = document.querySelectorAll("img[data-shot]");
    var pending = images.length;
    var finished = false;
    function finish() {
      if (!finished) {
        finished = true;
        done();
      }
    }
    function oneLoaded() {
      pending -= 1;
      if (pending <= 0) {
        finish();
      }
    }
    if (pending === 0) {
      finish();
      return;
    }
    for (var i = 0; i < images.length; i++) {
      var probe = new Image();
      probe.onload = oneLoaded;
      probe.onerror = oneLoaded;
      probe.src = shotPath(images[i].getAttribute("data-shot"), theme);
    }
    window.setTimeout(finish, 900);
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
    var active = document.querySelector('[data-theme-opt="' + currentTheme() + '"]');
    var nameEn = document.querySelector("[data-theme-name-en]");
    var nameEs = document.querySelector("[data-theme-name-es]");
    if (active && nameEn && nameEs) {
      nameEn.textContent = active.getAttribute("data-name-en");
      nameEs.textContent = active.getAttribute("data-name-es");
    }
    var meta = document.getElementById("theme-color");
    if (meta) {
      meta.setAttribute("content", cssVar("--bg") || "#000000");
    }
    updateFavicon();
  }

  /** Aplica un tema a toda la página. */
  function applyTheme(name) {
    root.setAttribute("data-theme", name);
    markTheme();
    updateShots();
  }

  /** Aplica el tema de la opción pulsada (con fundido, cuando sus capturas ya están) y lo recuerda. */
  function pickTheme(event) {
    var name = event.currentTarget.getAttribute("data-theme-opt");
    if (name === currentTheme()) {
      return;
    }
    writeStored("rebootera-theme", name);
    preloadShots(name, function ready() {
      transition(function change() {
        applyTheme(name);
      });
    });
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

  // ---------- Enlaces sin dirección visible ----------

  /** Guarda el destino de un enlace sin ponerlo en href (así el navegador no enseña la dirección al señalarlo). */
  function setLink(element, url) {
    element.removeAttribute("href");
    element.setAttribute("data-href", url);
  }

  /** Va al destino guardado: baja suave dentro de la página o abre la dirección (en otra pestaña si lo era). */
  function followLink(event) {
    var element = event.currentTarget;
    var url = element.getAttribute("data-href");
    if (!url) {
      return;
    }
    if (event.type === "keydown" && event.key !== "Enter") {
      return;
    }
    event.preventDefault();
    if (url.charAt(0) === "#") {
      var target = document.querySelector(url);
      if (target) {
        target.scrollIntoView({ behavior: reduced ? "auto" : "smooth", block: "start" });
      }
    } else if (element.getAttribute("data-blank") === "true") {
      window.open(url, "_blank", "noopener,noreferrer");
    } else {
      window.location.href = url;
    }
  }

  /** Quita el href de todos los enlaces de la página y los hace navegar con followLink (siguen siendo enlaces para el teclado y los lectores). */
  function hideLinks() {
    var links = document.querySelectorAll("a[href], a[data-href]");
    for (var i = 0; i < links.length; i++) {
      var link = links[i];
      if (link.getAttribute("target") === "_blank") {
        link.setAttribute("data-blank", "true");
        link.removeAttribute("target");
        link.removeAttribute("rel");
      }
      if (link.hasAttribute("href")) {
        setLink(link, link.getAttribute("href"));
      }
      link.setAttribute("role", "link");
      if (!link.hasAttribute("tabindex")) {
        link.setAttribute("tabindex", "0");
      }
      link.addEventListener("click", followLink);
      link.addEventListener("keydown", followLink);
    }
  }

  // ---------- Descarga ----------

  var PLATFORMS = {
    windows: { name: "Windows", variants: [{ label: "x64", asset: "Rebootera-Desktop.exe" }] },
    macos: {
      name: "macOS",
      variants: [
        { label: "Apple silicon", asset: "rebootera-desktop-darwin-arm64.tar.gz" },
        { label: "Intel", asset: "rebootera-desktop-darwin-amd64.tar.gz" },
      ],
    },
    linux: {
      name: "Linux",
      variants: [
        { label: "x64", asset: "rebootera-desktop-linux-amd64.tar.gz" },
        { label: "arm64", asset: "rebootera-desktop-linux-arm64.tar.gz" },
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

  /** Dibuja el botón grande, los botones de sistema, las variantes y la nota del sistema elegido. */
  function renderDownload() {
    var platform = PLATFORMS[chosen.id];
    document.getElementById("dl-os").textContent = platform.name;
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
      notes[j].classList.toggle("hidden", notes[j].getAttribute("data-note") !== chosen.id);
    }
    var box = document.getElementById("dl-variants");
    document.getElementById("dl-variants-wrap").setAttribute("data-open", String(platform.variants.length > 1));
    if (platform.variants.length > 1) {
      box.replaceChildren();
      for (var k = 0; k < platform.variants.length; k++) {
        var chip = document.createElement("button");
        chip.type = "button";
        chip.className = "chip";
        chip.textContent = platform.variants[k].label;
        chip.setAttribute("data-variant", String(k));
        chip.setAttribute("aria-pressed", String(k === chosen.variant));
        chip.addEventListener("click", pickVariant);
        box.appendChild(chip);
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

  /** La versión publicada se pide cuando el visitante interactúa por primera vez (no hace falta para pintar la página). */
  function loadReleaseOnce() {
    window.removeEventListener("pointerdown", loadReleaseOnce);
    window.removeEventListener("scroll", loadReleaseOnce);
    window.removeEventListener("keydown", loadReleaseOnce);
    loadRelease();
  }

  /** Lee la release "latest" de la API pública de GitHub (sin clave). Si no hay, la página no dice nada. */
  function loadRelease() {
    var request = new XMLHttpRequest();
    request.open("GET", "https://api.github.com/repos/" + REPO + "/releases/tags/latest");
    request.setRequestHeader("Accept", "application/vnd.github+json");
    request.onload = function onLoad() {
      if (request.status !== 200) {
        return;
      }
      var data = JSON.parse(request.responseText);
      var notes = data.body || "";
      var version = (/version:\s*([0-9][^\s]*)/i.exec(notes) || [])[1] || data.name || "latest";
      var sha = (/sha256:\s*([0-9a-f]{64})/i.exec(notes) || [])[1] || "";
      lastRelease = { version: version, published_at: data.published_at };
      data.assets.forEach(function eachAsset(asset) {
        if (/\.iso$/.test(asset.name)) {
          document.getElementById("iso-meta").textContent = " · " + megabytes(asset.size);
        }
      });
      if (sha) {
        document.getElementById("hash").textContent = "SHA-256 (ISO): " + sha;
      }
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

  var pointer = { x: 0, y: 0, px: -999, py: -999 };
  var eased = { x: 0, y: 0, scroll: 0, px: -999, py: -999 };
  var running = false;

  /** Giro de reposo de la captura: de lado en pantallas anchas (donde va junto al texto) y recta en las demás. */
  function baseAngle() {
    return window.innerWidth >= 1024 ? -9 : 0;
  }

  /** Un fotograma: acerca poco a poco lo mostrado a donde está el ratón (inclinación de la captura y fondo). */
  function frame() {
    var dx = pointer.x - eased.x;
    var dy = pointer.y - eased.y;
    var ds = window.scrollY - eased.scroll;
    var dpx = pointer.px - eased.px;
    var dpy = pointer.py - eased.py;
    eased.px += dpx * 0.18;
    eased.py += dpy * 0.18;
    eased.x += dx * 0.07;
    eased.y += dy * 0.07;
    eased.scroll += ds * 0.12;
    var tilt = document.getElementById("tilt");
    if (tilt) {
      tilt.style.transform = "perspective(1800px) rotateY(" + (baseAngle() + eased.x * 4).toFixed(3) + "deg) rotateX(" + (baseAngle() * -0.3 - eased.y * 3).toFixed(3) + "deg)";
      document.getElementById("tilt-glow").style.transform = "translate(" + (-eased.x * 16).toFixed(2) + "px, " + (-eased.y * 12).toFixed(2) + "px)";
    }
    var dawn = document.querySelector(".dawn");
    if (dawn) {
      var room = Math.max(1, document.documentElement.scrollHeight - window.innerHeight);
      dawn.style.setProperty("--p", Math.min(1, Math.max(0, eased.scroll / room)).toFixed(4));
    }
    if (Math.abs(dx) + Math.abs(dy) + Math.abs(ds) + Math.abs(dpx) + Math.abs(dpy) > 0.05) {
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

  /** Recuerda dónde está el ratón (de -1 a 1 en cada eje). */
  function onPointerMove(event) {
    pointer.x = (event.clientX / window.innerWidth - 0.5) * 2;
    pointer.y = (event.clientY / window.innerHeight - 0.5) * 2;
    pointer.px = event.clientX;
    pointer.py = event.clientY;
    kick();
  }

  /** Devuelve la captura y el fondo a su sitio cuando el ratón sale de la ventana. */
  function onPointerLeave() {
    pointer.x = 0;
    pointer.y = 0;
    pointer.px = -999;
    pointer.py = -999;
    kick();
  }

  /** La cabecera se vuelve transparente (con cristal esmerilado) en cuanto se baja un poco. */
  function updateHeader() {
    var header = document.getElementById("header");
    if (header) {
      header.setAttribute("data-scrolled", String(window.scrollY > 12));
    }
    var top = document.getElementById("to-top");
    if (top) {
      top.setAttribute("data-show", String(window.scrollY > 700));
    }
  }

  /** Sube a la portada, con desplazamiento suave salvo que se pida reducir el movimiento. */
  function scrollToTop() {
    window.scrollTo({ top: 0, behavior: reduced ? "auto" : "smooth" });
  }

  /** Conecta el movimiento (nada con "reducir movimiento"), el estado de la cabecera y el botón de volver arriba. */
  function setupMotion() {
    var topButton = document.getElementById("to-top");
    if (topButton) {
      topButton.addEventListener("click", scrollToTop);
    }
    window.addEventListener("scroll", updateHeader, { passive: true });
    updateHeader();
    if (reduced) {
      return;
    }
    window.addEventListener("mousemove", onPointerMove, { passive: true });
    document.addEventListener("mouseleave", onPointerLeave);
    window.addEventListener("scroll", kick, { passive: true });
    eased.scroll = window.scrollY;
    kick();
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
  hideLinks();
  setupReveal();
  window.addEventListener("pointerdown", loadReleaseOnce, { passive: true });
  window.addEventListener("scroll", loadReleaseOnce, { passive: true });
  window.addEventListener("keydown", loadReleaseOnce);
  setupMotion();
  setupShowcase();
})();

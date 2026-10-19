#!/usr/bin/python3
# * cpu-probe.py: mide cuánta CPU gasta la interfaz en reposo, página a página,
# * con el mismo WebKitGTK que la imagen. Abre la URL en un WebView, deja pasar
# * la intro y, para cada ruta, mide el proceso web durante unos segundos.
# * Sirve para cazar animaciones o repintados continuos sin construir la ISO.
# *
# *   xvfb-run -s "-screen 0 1280x800x24" python3 cpu-probe.py URL [RUTA...]
# *
# * Imprime la CPU (% de un núcleo) del proceso web por ruta.

import os
import sys

import gi

gi.require_version("Gtk", "4.0")
gi.require_version("WebKit", "6.0")
from gi.repository import GLib, Gtk, WebKit  # noqa: E402

URL = sys.argv[1]
ROUTES = sys.argv[2:] or ["#/overview", "#/library", "#/drivers", "#/downloads", "#/storage", "#/settings"]
INTRO_MS = 12000
SETTLE_MS = 3000
SAMPLE_MS = 6000
TICKS = os.sysconf("SC_CLK_TCK")


def web_pid():
    """Devuelve el pid del proceso web de WebKit, o 0."""
    for entry in os.listdir("/proc"):
        if entry.isdigit():
            try:
                with open("/proc/%s/comm" % entry) as handle:
                    if handle.read().strip() == "WebKitWebProces":
                        return int(entry)
            except OSError:
                continue
    return 0


def cpu_ticks(pid):
    """Tiempo de CPU (usuario + sistema) del proceso, en ticks."""
    with open("/proc/%d/stat" % pid) as handle:
        fields = handle.read().rsplit(")", 1)[1].split()
    return int(fields[11]) + int(fields[12])


class Probe:
    """Ventana con un WebView que mide la CPU ruta a ruta."""

    def __init__(self, app):
        """Abre la ventana y espera a que pase la intro."""
        self.app = app
        self.window = Gtk.ApplicationWindow(application=app)
        self.window.set_default_size(1280, 800)
        self.window.set_decorated(False)
        self.view = WebKit.WebView()
        self.window.set_child(self.view)
        self.window.present()
        self.view.load_uri(URL)
        self.index = 0
        GLib.timeout_add(INTRO_MS, self.next_route)

    def next_route(self):
        """Pasa a la siguiente ruta o termina."""
        if self.index >= len(ROUTES):
            self.app.quit()
            return GLib.SOURCE_REMOVE
        route = ROUTES[self.index]
        self.view.evaluate_javascript("location.hash = %r" % route, -1, None, None, None, None, None)
        GLib.timeout_add(SETTLE_MS, self.start_sample)
        return GLib.SOURCE_REMOVE

    def start_sample(self):
        """Empieza a medir la CPU del proceso web."""
        self.pid = web_pid()
        self.start = cpu_ticks(self.pid) if self.pid else 0
        GLib.timeout_add(SAMPLE_MS, self.end_sample)
        return GLib.SOURCE_REMOVE

    def end_sample(self):
        """Termina la medida e imprime el resultado."""
        used = (cpu_ticks(self.pid) - self.start) / TICKS if self.pid else -1
        print("%-14s CPU del proceso web: %5.1f %%" % (ROUTES[self.index], 100 * used / (SAMPLE_MS / 1000)), flush=True)
        # * Animaciones CSS activas en ese momento: la causa más habitual de repintado continuo.
        script = (
            "JSON.stringify([document.documentElement.dataset.gfx + '/' + document.documentElement.dataset.motion, document.getAnimations().length, document.getAnimations().slice(0, 6).map(function (a) {"
            "return (a.animationName || a.transitionProperty || 'anim') + '@' + "
            "(a.effect && a.effect.target ? (a.effect.target.className || a.effect.target.tagName) : '?'); })])"
        )
        self.view.evaluate_javascript(script, -1, None, None, None, self.on_animations, ROUTES[self.index])
        return GLib.SOURCE_REMOVE

    def on_animations(self, view, result, route):
        """Imprime las animaciones activas y pasa a la siguiente ruta."""
        try:
            print("%-14s animaciones activas: %s" % (route, view.evaluate_javascript_finish(result).to_string()), flush=True)
        except GLib.Error as error:
            print("%-14s animaciones: error %s" % (route, error.message), flush=True)
        self.index += 1
        GLib.timeout_add(100, self.next_route)
        return GLib.SOURCE_REMOVE


def on_activate(app):
    """Crea la ventana al activar la aplicación."""
    app.probe = Probe(app)


def main():
    """Arranca la aplicación GTK."""
    app = Gtk.Application(application_id="os.rebootera.CpuProbe")
    app.connect("activate", on_activate)
    return app.run(None)


if __name__ == "__main__":
    sys.exit(main())

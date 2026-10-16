#!/usr/bin/python3
# * ui-preview.py: captura la interfaz de Rebootera con el mismo WebKitGTK que
# * usa la imagen, sin construir la ISO. Carga la URL indicada (normalmente la
# * de scripts/dev-run-core.sh con su token), espera, ejecuta opcionalmente
# * JavaScript (por ejemplo, navegar a otra página) y guarda capturas PNG.
# * Se ejecuta en WSL dentro de un servidor X virtual:
# *
# *   xvfb-run -s "-screen 0 1440x900x24" python3 ui-preview.py URL SALIDA [ANCHO ALTO] [ESPERA] [RUTA...]
# *
# * Cada RUTA (por ejemplo "#/drivers") genera una captura más.

import sys

import gi

gi.require_version("Gtk", "4.0")
gi.require_version("WebKit", "6.0")
from gi.repository import Gdk, GLib, Gtk, WebKit  # noqa: E402

URL = sys.argv[1]
OUTPUT = sys.argv[2]
WIDTH = int(sys.argv[3]) if len(sys.argv) > 3 else 1440
HEIGHT = int(sys.argv[4]) if len(sys.argv) > 4 else 900
WAIT_MS = int(sys.argv[5]) if len(sys.argv) > 5 else 6000
ROUTES = sys.argv[6:]


class Preview:
    """Ventana con un WebView que hace capturas en secuencia."""

    def __init__(self, app):
        """Crea la ventana y carga la URL."""
        self.app = app
        self.window = Gtk.ApplicationWindow(application=app)
        self.window.set_default_size(WIDTH, HEIGHT)
        self.window.set_decorated(False)
        self.view = WebKit.WebView()
        self.window.set_child(self.view)
        self.window.present()
        self.queue = [None] + ROUTES
        self.index = 0
        self.view.load_uri(URL)
        GLib.timeout_add(WAIT_MS, self.capture_next)

    def capture_next(self):
        """Navega a la siguiente ruta (si la hay) y programa la captura."""
        route = self.queue[self.index]
        if route is not None:
            self.view.evaluate_javascript("location.hash = %r" % route, -1, None, None, None, None, None)
            GLib.timeout_add(1500, self.snapshot)
        else:
            self.snapshot()
        return GLib.SOURCE_REMOVE

    def snapshot(self):
        """Pide a WebKit una captura de la parte visible."""
        self.view.get_snapshot(
            WebKit.SnapshotRegion.VISIBLE, WebKit.SnapshotOptions.NONE, None, self.on_snapshot, None
        )
        return GLib.SOURCE_REMOVE

    def on_snapshot(self, view, result, data):
        """Guarda la captura y pasa a la siguiente."""
        texture = view.get_snapshot_finish(result)
        suffix = "" if self.index == 0 else "-" + self.queue[self.index].strip("#/").replace("/", "-")
        path = OUTPUT.replace(".png", suffix + ".png")
        texture.save_to_png(path)
        print("captura: " + path, flush=True)
        self.index += 1
        if self.index < len(self.queue):
            GLib.timeout_add(200, self.capture_next)
        else:
            self.app.quit()


def on_activate(app):
    """Crea la ventana al activar la aplicación."""
    app.preview = Preview(app)


def main():
    """Arranca GTK."""
    app = Gtk.Application(application_id="os.rebootera.Preview")
    app.connect("activate", on_activate)
    return app.run(None)


if __name__ == "__main__":
    Gdk  # noqa: B018 (se importa para inicializar el backend)
    sys.exit(main())

#!/usr/bin/python3
"""Banco de pruebas de Rebootera OS en máquina virtual.

Arranca una ISO de depuración en QEMU con KVM, en uno de estos modos:
  uefi      UEFI sin Secure Boot (OVMF)
  uefi-sb   UEFI con Secure Boot activo y claves de Microsoft (OVMF .ms)
  bios      BIOS heredada (SeaBIOS)

Comprueba por la consola serie (root sin contraseña, solo en imágenes de
depuración) que los servicios están activos y bien aislados, hace capturas
de pantalla con QMP y maneja la interfaz con el teclado virtual. Con
--poweroff termina pulsando "Apagar" en la interfaz y comprueba que la VM se
apaga sola.

Con --release prueba una imagen definitiva (sin consola serie): captura el
menú, la interfaz y apaga desde ella. Con --exec ORDEN... ejecuta órdenes de
diagnóstico en una imagen de depuración y muestra su salida.

Uso: vmtest.py MODO ISO SALIDA [--poweroff | --release | --exec ORDEN...]
"""

import json
import os
import shutil
import socket
import subprocess
import sys
import tempfile
import time

OVMF = "/usr/share/OVMF"
BOOT_TIMEOUT = 240
UI_SETTLE = 12
RELEASE_BOOT_WAIT = 6


class VM:
    """Una máquina virtual QEMU con consola serie y QMP por sockets Unix."""

    def __init__(self, mode, iso, outdir):
        """Prepara rutas, firmware y la línea de órdenes de QEMU."""
        self.mode = mode
        self.outdir = outdir
        self.tmp = tempfile.mkdtemp(prefix="rbt-vm-")
        self.serial_path = os.path.join(self.tmp, "serial.sock")
        self.qmp_path = os.path.join(self.tmp, "qmp.sock")
        self.serial_log = os.path.join(outdir, mode + "-serial.log")
        self.buffer = b""
        self.marker = 0

        cmd = [
            "qemu-system-x86_64",
            "-enable-kvm",
            "-cpu", "host",
            "-smp", os.environ.get("VMTEST_SMP", "4"),
            "-m", os.environ.get("VMTEST_MEM", "4096"),
            "-vga", "std",
            "-display", "none",
            "-nic", "user,model=virtio-net-pci",
            "-chardev", "socket,id=s0,path=%s,server=on,wait=off" % self.serial_path,
            "-serial", "chardev:s0",
            "-qmp", "unix:%s,server=on,wait=off" % self.qmp_path,
            "-device", "qemu-xhci,id=xhci",
            "-device", "usb-tablet,bus=xhci.0",
        ]
        if iso.endswith(".img"):
            # * Imagen de disco: se conecta como un pendrive USB real y se
            # * escribe en una copia temporal para no alterar el original.
            stick = os.path.join(self.tmp, "stick.qcow2")
            subprocess.run(["qemu-img", "create", "-q", "-f", "qcow2", "-F", "raw", "-b", os.path.abspath(iso), stick], check=True)
            cmd += [
                "-drive", "if=none,id=stick,format=qcow2,file=%s" % stick,
                "-device", "usb-storage,bus=xhci.0,drive=stick,bootindex=0",
            ]
        else:
            if os.environ.get("VMTEST_CD", "ide") == "scsi":
                # * El CD virtual por virtio-scsi lee la ISO mucho más rápido que el IDE emulado.
                cmd += [
                    "-device", "virtio-scsi-pci,id=scsi0",
                    "-drive", "if=none,id=cd0,media=cdrom,readonly=on,file=%s" % iso,
                    "-device", "scsi-cd,drive=cd0,bootindex=0",
                ]
            else:
                cmd += ["-cdrom", iso, "-boot", "d"]
        if os.environ.get("VMTEST_DISK"):
            # * Disco interno opcional para probar la detección de discos.
            # ! snapshot=on: la VM nunca escribe en el disco de prueba original.
            cmd += ["-drive", "file=%s,format=qcow2,if=virtio,snapshot=on" % os.environ["VMTEST_DISK"]]
        if mode in ("uefi", "uefi-sb"):
            code = "OVMF_CODE_4M.secboot.fd" if mode == "uefi-sb" else "OVMF_CODE_4M.fd"
            vars_src = "OVMF_VARS_4M.ms.fd" if mode == "uefi-sb" else "OVMF_VARS_4M.fd"
            vars_copy = os.path.join(self.tmp, "vars.fd")
            shutil.copy(os.path.join(OVMF, vars_src), vars_copy)
            cmd += [
                "-machine", "q35,smm=%s" % ("on" if mode == "uefi-sb" else "off"),
                "-drive", "if=pflash,format=raw,unit=0,readonly=on,file=%s/%s" % (OVMF, code),
                "-drive", "if=pflash,format=raw,unit=1,file=%s" % vars_copy,
            ]
            if mode == "uefi-sb":
                cmd += ["-global", "driver=cfi.pflash01,property=secure,value=on"]
        else:
            cmd += ["-machine", "pc"]
        self.cmd = cmd

    def start(self):
        """Arranca QEMU y conecta la consola serie y QMP."""
        log("arrancando VM en modo %s" % self.mode)
        self.proc = subprocess.Popen(self.cmd)
        self.serial = connect_unix(self.serial_path)
        self.serial.setblocking(False)
        self.qmp = connect_unix(self.qmp_path)
        self.qmp_file = self.qmp.makefile("rw")
        self.qmp_file.readline()
        self.qmp_cmd("qmp_capabilities")

    def qmp_cmd(self, name, **arguments):
        """Ejecuta una orden QMP y devuelve su resultado."""
        request = {"execute": name}
        if arguments:
            request["arguments"] = arguments
        self.qmp_file.write(json.dumps(request) + "\n")
        self.qmp_file.flush()
        while True:
            reply = json.loads(self.qmp_file.readline())
            if "return" in reply or "error" in reply:
                return reply

    def pump(self):
        """Lee lo pendiente de la consola serie y lo guarda en el registro."""
        try:
            while True:
                chunk = self.serial.recv(65536)
                if not chunk:
                    break
                self.buffer += chunk
                with open(self.serial_log, "ab") as handle:
                    handle.write(chunk)
        except BlockingIOError:
            pass

    def wait_for(self, text, timeout):
        """Espera a que aparezca un texto en la consola serie."""
        needle = text.encode()
        deadline = time.time() + timeout
        while time.time() < deadline:
            self.pump()
            if needle in self.buffer:
                return True
            if self.proc.poll() is not None:
                return False
            time.sleep(0.5)
        return False

    def run(self, command, timeout=60):
        """Ejecuta una orden en la consola root de la VM y devuelve su salida."""
        self.marker += 1
        end = "__RBT_END_%d__" % self.marker
        self.buffer = b""
        self.serial.sendall(("%s; echo %s\n" % (command, end)).encode())
        if not self.wait_for(end + "\r\n", timeout) and not self.wait_for(end + "\n", 1):
            raise RuntimeError("la orden no terminó: %s" % command)
        text = self.buffer.decode(errors="replace")
        # Se descarta el eco de la orden y la línea del marcador.
        body = text.split(end, 1)[1] if text.count(end) >= 2 else text
        return body.rsplit(end, 1)[0].strip()

    def screenshot(self, name):
        """Guarda una captura de la pantalla de la VM en PNG."""
        ppm = os.path.join(self.tmp, name + ".ppm")
        png = os.path.join(self.outdir, "%s-%s.png" % (self.mode, name))
        self.qmp_cmd("screendump", filename=ppm)
        subprocess.run(["convert", ppm, png], check=True)
        log("captura: %s" % png)
        return png

    def key(self, name):
        """Pulsa una tecla en el teclado virtual."""
        self.qmp_cmd("send-key", keys=[{"type": "qcode", "data": name}])
        time.sleep(0.6)

    def screen_size(self):
        """Devuelve el tamaño de la pantalla de la VM leyendo una captura PPM."""
        ppm = os.path.join(self.tmp, "size.ppm")
        self.qmp_cmd("screendump", filename=ppm)
        with open(ppm, "rb") as handle:
            header = handle.read(64).split()
        return int(header[1]), int(header[2])

    def click(self, x, y):
        """Hace clic en un punto de la pantalla con la tableta USB (coordenadas absolutas)."""
        width, height = self.screen_size()
        ax = int(x * 32767 / max(1, width - 1))
        ay = int(y * 32767 / max(1, height - 1))
        self.qmp_cmd("input-send-event", events=[
            {"type": "abs", "data": {"axis": "x", "value": ax}},
            {"type": "abs", "data": {"axis": "y", "value": ay}},
        ])
        time.sleep(0.2)
        for down in (True, False):
            self.qmp_cmd("input-send-event", events=[{"type": "btn", "data": {"button": "left", "down": down}}])
            time.sleep(0.1)
        time.sleep(0.8)

    def stop(self):
        """Apaga QEMU si sigue en marcha y limpia los temporales."""
        if self.proc.poll() is None:
            try:
                self.qmp_cmd("quit")
            except OSError:
                pass
            try:
                self.proc.wait(10)
            except subprocess.TimeoutExpired:
                self.proc.kill()
        shutil.rmtree(self.tmp, ignore_errors=True)


def connect_unix(path):
    """Conecta a un socket Unix esperando a que QEMU lo cree."""
    for _ in range(100):
        try:
            sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            sock.connect(path)
            return sock
        except (FileNotFoundError, ConnectionRefusedError):
            time.sleep(0.1)
    raise RuntimeError("QEMU no creó el socket %s" % path)


def log(message):
    """Escribe un mensaje de progreso."""
    print("[vmtest] " + message, flush=True)


class Checks:
    """Acumula comprobaciones con su resultado."""

    def __init__(self):
        """Empieza sin comprobaciones."""
        self.results = []

    def check(self, name, ok, detail=""):
        """Registra una comprobación."""
        self.results.append((name, ok, detail))
        log("%s %s %s" % ("OK  " if ok else "FALLO", name, detail))

    def passed(self):
        """Indica si todas las comprobaciones pasaron."""
        return all(result[1] for result in self.results)


def test_system(vm, checks):
    """Comprobaciones del sistema por la consola serie."""
    for unit in ("rebootera-agent", "rebootera-core", "rebootera-ui"):
        state = vm.run("systemctl is-active %s.service" % unit)
        checks.check("servicio %s activo" % unit, state.endswith("active") and "inactive" not in state, state)

    users = vm.run("ps -o user:32=,comm= -C rebootera-core,rebootera-agent,labwc,rebootera-shell | sort -u")
    flat = " ".join(users.split())
    checks.check("core sin root", "rebootera-core rebootera-core" in flat, flat)
    checks.check("agente como root", "root rebootera-agent" in flat, flat)
    checks.check("kiosco como usuario rebootera", "rebootera labwc" in flat and "rebootera rebootera-shell" in flat, flat)

    caps = vm.run("grep CapPrm /proc/$(pgrep -n -x rebootera-shell)/status")
    checks.check("kiosco sin capacidades", caps.strip().endswith("0000000000000000"), caps)

    # Cuarta columna de ss: dirección local en escucha.
    listen = vm.run("ss -ltnH 'sport = :7080' | awk '{print $4}'")
    checks.check("core solo en 127.0.0.1", listen.strip() == "127.0.0.1:7080", listen)

    perms = vm.run("stat -c '%a %U %G' /run/rebootera-core/ui-token /run/rebootera-agent/agent.sock")
    checks.check("permisos de token y socket", perms.split("\n")[0].strip() == "640 rebootera-core rebootera-ui"
                 and "660 root rebootera-core" in perms, perms.replace("\n", " | "))

    root = vm.run("passwd -S root | cut -d' ' -f2")
    checks.check("root bloqueado para contraseña", root.strip().endswith("L"), root)

    # * Navegador: las dos extensiones están en la imagen y la política de Firefox es JSON válido que las fija.
    browser = vm.run(
        "test -s /usr/share/rebootera/browser/ublock-origin.xpi && test -s /usr/share/rebootera/browser/dark-reader.xpi "
        "&& python3 -c \"import json;p=json.load(open('/usr/lib/firefox-esr/distribution/policies.json'))['policies'];"
        "print(sorted(k for k in p['ExtensionSettings'] if k!='*'))\""
    )
    checks.check("navegador con uBlock Origin y Dark Reader", "uBlock0" in browser and "darkreader" in browser.lower(), browser)

    # * Panel táctil: el toque para clic viene activado en la configuración de labwc que generó start-kiosk.
    tap = vm.run("grep -h '<tap>' /run/user/*/rebootera-labwc/rc.xml | head -1")
    checks.check("toque para clic activado por defecto", "<tap>yes</tap>" in tap, tap)

    fw = vm.run("[ -d /sys/firmware/efi ] && echo uefi || echo bios")
    checks.check("modo de firmware", fw.strip().endswith(("bios" if vm.mode == "bios" else "uefi")), fw)
    if vm.mode == "uefi-sb":
        sb = vm.run("od -An -t u1 -j4 -N1 /sys/firmware/efi/efivars/SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c")
        checks.check("Secure Boot activo", sb.strip().endswith("1"), sb)

    api = vm.run(
        "python3 -c \"import http.client as h; c=h.HTTPConnection('127.0.0.1',7080); "
        "c.request('GET','/api/system'); print('code', c.getresponse().status)\""
    )
    checks.check("API rechaza sin sesión", api.strip().endswith("code 401"), api)

    # Con la cookie de sesión (root puede leer el token) la API debe informar
    # del modo de firmware y de Secure Boot que ve el propio sistema.
    expected = {"bios": "fw bios n/a", "uefi": "fw uefi off", "uefi-sb": "fw uefi on"}[vm.mode]
    report = vm.run(
        "python3 -c \"import http.client as h, json; t=open('/run/rebootera-core/ui-token').read(); "
        "c=h.HTTPConnection('127.0.0.1',7080); "
        "c.request('GET','/api/system',headers={'Cookie':'rebootera_session='+t}); "
        "d=json.load(c.getresponse()); print('fw', d['firmware'], d['secureBoot'])\""
    )
    checks.check("API informa firmware y Secure Boot", report.strip().endswith(expected), report)

    timing = vm.run("systemd-analyze | head -n1")
    checks.check("tiempo de arranque (informativo)", True, timing)

    vm.run("journalctl -b -u rebootera-ui -u rebootera-core -u rebootera-agent --no-pager -o cat > /tmp/rbt.log 2>&1")
    journal = vm.run("tail -n 40 /tmp/rbt.log")
    with open(os.path.join(vm.outdir, vm.mode + "-journal.txt"), "w", encoding="utf-8") as handle:
        handle.write(journal + "\n")


# * Posiciones de las pestañas de la cabecera en pantallas de 1280 px de ancho (coinciden con topnav.css):
# * (x, y) del centro de cada una. Ajustes es la última.
NAV = {
    "overview": (176, 33),
    "install": (277, 33),
    "library": (394, 33),
    "drivers": (516, 33),
    "downloads": (648, 33),
    "storage": (760, 33),
    "computer": (866, 33),
    "settings": (967, 33),
}


def stable_screen_size(vm):
    """Tamaño de pantalla una vez estable: el kiosco puede cambiar de modo de vídeo al arrancar (sobre todo con la
    máquina cargada) y un clic calculado con el tamaño viejo caería en otro botón."""
    previous = vm.screen_size()
    for _ in range(8):
        time.sleep(2)
        current = vm.screen_size()
        if current == previous:
            return current
        previous = current
    return previous


def test_ui(vm, checks, poweroff):
    """Captura la intro y las páginas, cambia de idioma y apaga desde la interfaz."""
    # * La interfaz arranca a la vez que la consola serie: la intro está en curso.
    time.sleep(1.5)
    vm.screenshot("1-intro")
    time.sleep(UI_SETTLE)
    width, height = stable_screen_size(vm)
    checks.check("resolución de pantalla (informativo)", True, "%dx%d" % (width, height))
    vm.screenshot("2-resumen")
    for index, route in enumerate(("drivers", "storage", "install"), start=3):
        vm.click(*NAV[route])
        time.sleep(1.5)
        vm.screenshot("%d-%s" % (index, route))

    # * En la cabecera solo queda el botón de energía (el idioma y el tema están en Ajustes, la última pestaña).
    power_x, top_y = width - 39, 33
    vm.click(*NAV["overview"])
    time.sleep(1)
    vm.click(*NAV["settings"])
    time.sleep(1.5)
    vm.screenshot("7-ajustes")
    if not poweroff:
        return
    # * El diálogo enfoca primero "Cerrar"; Tab pasa a "Reiniciar" y otro a "Apagar".
    vm.click(power_x, top_y)
    time.sleep(1)
    vm.screenshot("8-dialogo-energia")
    vm.key("tab")
    vm.key("tab")
    vm.key("ret")
    try:
        vm.proc.wait(90)
        checks.check("apagado desde la interfaz", True, "QEMU terminó (código %s)" % vm.proc.returncode)
    except subprocess.TimeoutExpired:
        vm.screenshot("4-tras-apagar")
        checks.check("apagado desde la interfaz", False, "la VM sigue encendida tras 90 s")


def main():
    """Ejecuta la batería de pruebas en el modo pedido."""
    if len(sys.argv) < 4:
        print(__doc__)
        return 2
    mode, iso, outdir = sys.argv[1], sys.argv[2], sys.argv[3]
    poweroff = "--poweroff" in sys.argv
    # Una subcarpeta por modo: así se pueden lanzar varios modos en paralelo
    # sin que la limpieza de uno borre los archivos de otro.
    outdir = os.path.join(outdir, mode)
    shutil.rmtree(outdir, ignore_errors=True)
    os.makedirs(outdir)

    vm = VM(mode, iso, outdir)
    checks = Checks()
    started = time.time()
    try:
        vm.start()
        # El menú de arranque se muestra 3 segundos tras el firmware.
        time.sleep(3.5)
        vm.screenshot("0-menu-arranque")
        # * Tras el menú (3 s) arranca el kernel: en la imagen definitiva se ve
        # * Plymouth; en la de depuración, el texto de la consola.
        time.sleep(4.5)
        vm.screenshot("0b-arranque")
        if "--release" in sys.argv:
            # Imagen definitiva: no hay consola serie. Solo se comprueba lo que
            # ve el usuario y que la interfaz apaga el equipo de verdad.
            # * No se sabe cuánto tarda el arranque (sobre todo en BIOS y con la máquina cargada): se espera a
            # * que la pantalla cambie al modo de la interfaz (más de 1024 px de ancho), hasta 90 s.
            time.sleep(RELEASE_BOOT_WAIT)
            deadline = time.time() + 90
            while time.time() < deadline and vm.screen_size()[0] < 1024:
                time.sleep(2)
            test_ui(vm, checks, True)
            return finish(outdir, mode, checks)
        booted = vm.wait_for("root@rebootera", BOOT_TIMEOUT)
        checks.check("arranque hasta consola", booted, "%.0f s" % (time.time() - started))
        if not booted:
            vm.screenshot("fallo-arranque")
            return 1
        # * La consola serie no tiene ancho: se fija uno enorme para que herramientas
        # * como findmnt no recorten columnas.
        vm.run("export TERM=dumb PAGER=cat SYSTEMD_COLORS=0; stty -echo cols 4000 rows 200")
        if "--exec" in sys.argv:
            # Modo de diagnóstico: ejecuta las órdenes indicadas y muestra su salida.
            for command in sys.argv[sys.argv.index("--exec") + 1:]:
                print("$ " + command)
                print(vm.run(command, timeout=int(os.environ.get("VMTEST_EXEC_TIMEOUT", "120"))))
            vm.screenshot("diagnostico")
            return 0
        test_system(vm, checks)
        test_ui(vm, checks, poweroff)
    finally:
        vm.stop()
    return finish(outdir, mode, checks)


def finish(outdir, mode, checks):
    """Guarda el resumen de comprobaciones y devuelve el código de salida."""
    with open(os.path.join(outdir, mode + "-resultado.txt"), "w", encoding="utf-8") as handle:
        for name, ok, detail in checks.results:
            handle.write("%s\t%s\t%s\n" % ("OK" if ok else "FALLO", name, detail))
    log("resultado: %s" % ("TODO OK" if checks.passed() else "HAY FALLOS"))
    return 0 if checks.passed() else 1


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/python3
"""Prueba de extremo a extremo de un USB de Rebootera en máquina virtual.

Arranca la imagen USB (creada con make-usb.sh) como un pendrive, con un segundo
disco "del usuario" que contiene una ISO, y comprueba dentro del sistema:

  1. El Vault se monta (exFAT, sin ejecutables ni setuid).
  2. Una descarga real desde la fuente oficial llega verificada a la biblioteca.
  3. Una ISO de otro disco se importa abriéndolo en solo lectura.
  4. Los ajustes se guardan en el USB.
  5. "Instalar" preselecciona la imagen en Ventoy y reinicia; Ventoy la arranca.
  6. Al volver a Rebootera, Ventoy vuelve a tener Rebootera por defecto y los
     ajustes y la biblioteca siguen ahí (persistencia real entre arranques).

Uso: vmtest-usb.py MODO IMAGEN_USB DISCO_USUARIO.qcow2 SALIDA
"""

import base64
import json
import os
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import importlib.util  # noqa: E402

spec = importlib.util.spec_from_file_location("vmtest", os.path.join(os.path.dirname(os.path.abspath(__file__)), "vmtest.py"))
vmtest = importlib.util.module_from_spec(spec)
spec.loader.exec_module(vmtest)

# * Ayudante que se copia a la VM: llama a la API de Core con la sesión del
# * kiosco (root puede leer el token) y la cabecera anti-CSRF.
API_HELPER = r'''
import http.client, json, sys
token = open("/run/rebootera-core/ui-token").read().strip()
method, path = sys.argv[1], sys.argv[2]
body = sys.argv[3] if len(sys.argv) > 3 else None
conn = http.client.HTTPConnection("127.0.0.1", 7080, timeout=120)
headers = {"Cookie": "rebootera_session=" + token, "Content-Type": "application/json", "X-Rebootera": "1"}
conn.request(method, path, body, headers)
resp = conn.getresponse()
print("RBT", resp.status, resp.read().decode())
'''

DOWNLOAD_ID = "alpine-virt"
DOWNLOAD_TIMEOUT = 900


def install_helper(vm):
    """Copia el ayudante de la API a la VM."""
    encoded = base64.b64encode(API_HELPER.encode()).decode()
    vm.run("echo %s | base64 -d > /tmp/api.py" % encoded)


def api(vm, method, path, body=None):
    """Llama a la API de Core dentro de la VM y devuelve (estado, json)."""
    command = "python3 /tmp/api.py %s '%s'" % (method, path)
    if body is not None:
        command += " '%s'" % json.dumps(body)
    out = vm.run(command, timeout=180)
    # * El prompt de la consola puede salir en la misma línea que la respuesta.
    line = [l[l.index("RBT "):] for l in out.splitlines() if "RBT " in l]
    if not line:
        raise RuntimeError("sin respuesta de la API: " + out)
    _, status, payload = line[-1].split(" ", 2)
    try:
        return int(status), json.loads(payload)
    except json.JSONDecodeError:
        return int(status), payload


def wait_transfer(vm, kind, file_name, timeout):
    """Espera a que una transferencia termine y devuelve su estado final."""
    deadline = time.time() + timeout
    job = None
    while time.time() < deadline:
        _, jobs = api(vm, "GET", "/api/transfers")
        job = next((j for j in jobs if j["fileName"] == file_name and j["kind"] == kind), None)
        if job and job["status"] in ("done", "failed", "cancelled"):
            return job
        time.sleep(5)
    return job


def wait_firmware(vm, timeout):
    """Espera al mensaje del firmware que indica un arranque nuevo y vacía la consola."""
    vm.pump()
    vm.buffer = b""
    found = vm.wait_for("BdsDxe: starting", timeout)
    vm.buffer = b""
    return found


def boot_rebootera(vm, checks, label):
    """Espera a la consola de Rebootera y prepara la sesión."""
    started = time.time()
    booted = vm.wait_for("root@rebootera", vmtest.BOOT_TIMEOUT * 2)
    checks.check("%s: arranque hasta consola" % label, booted, "%.0f s" % (time.time() - started))
    if not booted:
        vm.screenshot("fallo-%s" % label)
        return False
    vm.run("export TERM=dumb PAGER=cat SYSTEMD_COLORS=0; stty -echo cols 4000 rows 200")
    install_helper(vm)
    # * Core tarda unos segundos en estar listo tras la consola.
    for _ in range(30):
        if "RBT 200" in vm.run("python3 /tmp/api.py GET /api/vault 2>&1 | tail -1"):
            break
        time.sleep(2)
    return True


def scenario(vm, checks):
    """Ejecuta todos los pasos de la prueba."""
    if not boot_rebootera(vm, checks, "primer arranque"):
        return

    # 1. Vault montado con opciones seguras.
    mount = vm.run("findmnt -no FSTYPE,OPTIONS /run/rebootera/vault")
    checks.check("Vault montado (exFAT, noexec, nosuid, nodev)",
                 "exfat" in mount and "noexec" in mount and "nosuid" in mount and "nodev" in mount, mount)
    status, vault = api(vm, "GET", "/api/vault")
    checks.check("API: Vault disponible", status == 200 and vault.get("available") is True, str(vault))
    _, disks = api(vm, "GET", "/api/storage")
    live = [d for d in disks if d["role"] == "live-media"]
    checks.check("USB reconocido como medio de Rebootera", bool(live) and live[0]["transport"] == "usb",
                 json.dumps([(d["name"], d["role"], d["transport"]) for d in disks]))

    # 2. Descarga real verificada.
    status, job = api(vm, "POST", "/api/transfers/download", {"id": DOWNLOAD_ID})
    checks.check("descarga encolada", status == 202, str(job)[:200])
    done = wait_transfer(vm, "download", job.get("fileName", ""), DOWNLOAD_TIMEOUT) if status == 202 else None
    checks.check("descarga terminada y verificada", bool(done) and done["status"] == "done",
                 (done or {}).get("status", "") + " " + (done or {}).get("error", ""))

    # 3. Importación desde el disco del usuario, abierto en solo lectura.
    status, answer = api(vm, "POST", "/api/media/mount", {"device": "/dev/vda1"})
    options = vm.run("findmnt -no OPTIONS /run/rebootera/media/vda1")
    checks.check("disco del usuario abierto en solo lectura", status == 200 and options.startswith("ro"),
                 "%s %s | %s" % (status, answer, options))
    _, entries = api(vm, "GET", "/api/media/browse?device=vda1&dir=Descargas/ISOs")
    names = [e["name"] for e in entries] if isinstance(entries, list) else []
    checks.check("se ven las ISO del disco (y no otros archivos)", names == ["alpine-virt-3.24.2-x86_64.iso"], str(entries)[:300])
    status, job = api(vm, "POST", "/api/transfers/import",
                      {"device": "vda1", "path": "Descargas/ISOs/alpine-virt-3.24.2-x86_64.iso", "category": "linux"})
    done = wait_transfer(vm, "import", "alpine-virt-3.24.2-x86_64.iso", 600) if status == 202 else None
    checks.check("importación terminada", bool(done) and done["status"] == "done", "%s %s" % (status, str(done or job)[:300]))
    status, _ = api(vm, "POST", "/api/media/unmount", {"device": "vda1"})
    checks.check("disco del usuario cerrado", status == 200, str(status))

    _, library = api(vm, "GET", "/api/library")
    states = sorted((i["path"], i["state"]) for i in library)
    checks.check("biblioteca: descarga verificada e importación marcada", states == [
        ("isos/linux/alpine-virt-3.24.2-x86_64.iso", "imported"),
        ("isos/tools/alpine-virt-3.24.2-x86_64.iso", "verified"),
    ], str(states))

    # 4. Ajustes guardados en el USB.
    status, _ = api(vm, "POST", "/api/settings", {"language": "es", "accent": "nebula", "intro": True, "reduceMotion": False})
    checks.check("ajustes guardados en el USB", status == 200, str(status))

    # Capturas de la interfaz con datos reales.
    vm.click(*vmtest.NAV["library"])
    time.sleep(2)
    vm.screenshot("1-biblioteca")
    vm.click(*vmtest.NAV["downloads"])
    time.sleep(4)
    vm.screenshot("2-descargas")

    # 5. Instalar: preselección en Ventoy y reinicio.
    # * Alpine está marcada como incompatible con el menú: la descarga del
    # * catálogo se rechaza; la copia importada (sin origen de catálogo) no.
    status, body = api(vm, "POST", "/api/boot/install", {"path": "isos/tools/alpine-virt-3.24.2-x86_64.iso"})
    checks.check("instalar: una imagen incompatible del catálogo se rechaza", status == 409 and "incompatible-image" in str(body), "%s %s" % (status, body))
    status, _ = api(vm, "POST", "/api/boot/install", {"path": "isos/linux/alpine-virt-3.24.2-x86_64.iso"})
    checks.check("instalar: preselección y reinicio aceptados", status == 202, str(status))
    next_boot = vm.run("grep -o '/isos/linux/alpine[^\"]*' /run/rebootera/vault/ventoy/ventoy.json")
    checks.check("ventoy.json apunta a la imagen elegida", "alpine-virt" in next_boot, next_boot)
    # * El apagado del sistema live tarda: se espera a que el firmware vuelva a arrancar.
    rebooted = wait_firmware(vm, 400)
    checks.check("el equipo se reinicia", rebooted, "")
    time.sleep(5)
    vm.screenshot("3-ventoy-preseleccion")
    time.sleep(15)
    vm.screenshot("4-instalador-arrancado")

    # 6. Volver a Rebootera desde el menú de Ventoy (última entrada).
    vm.qmp_cmd("system_reset")
    wait_firmware(vm, 60)
    time.sleep(5)
    vm.screenshot("5-ventoy-menu")
    # * El menú tiene la imagen descargada, la importada y Rebootera, en ese orden.
    vm.key("down")
    vm.key("down")
    vm.key("ret")
    if not boot_rebootera(vm, checks, "segundo arranque"):
        return
    restored = vm.run("grep -o '\"VTOY_DEFAULT_IMAGE\": \"[^\"]*\"' /run/rebootera/vault/ventoy/ventoy.json")
    checks.check("Rebootera vuelve a ser la imagen por defecto", "/rebootera/rebootera-os.iso" in restored, restored)
    _, settings = api(vm, "GET", "/api/settings")
    checks.check("los ajustes persisten tras reiniciar",
                 settings.get("persistent") and settings["settings"]["language"] == "es", str(settings))
    _, library = api(vm, "GET", "/api/library")
    checks.check("la biblioteca persiste tras reiniciar", len(library) == 2, str(len(library)))
    time.sleep(12)
    vm.screenshot("6-tras-reinicio-espanol")


def main():
    """Prepara la VM y ejecuta el escenario."""
    if len(sys.argv) != 5:
        print(__doc__)
        return 2
    mode, image, user_disk, outdir = sys.argv[1:]
    outdir = os.path.join(outdir, mode + "-usb")
    os.makedirs(outdir, exist_ok=True)
    for stale in os.listdir(outdir):
        os.remove(os.path.join(outdir, stale))
    os.environ["VMTEST_DISK"] = os.path.abspath(user_disk)
    vm = vmtest.VM(mode, image, outdir)
    checks = vmtest.Checks()
    try:
        vm.start()
        scenario(vm, checks)
    finally:
        vm.stop()
    return vmtest.finish(outdir, mode, checks)


if __name__ == "__main__":
    sys.exit(main())

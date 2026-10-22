#!/usr/bin/python3
"""Pruebas de las funciones de la 0.3.0 de Rebootera OS en máquina virtual.

Arranca un USB de Rebootera (con la ISO de depuración y drivers de Windows en
drivers/) en UEFI y comprueba de extremo a extremo:

  1. Descarga de Debian desde el catálogo con la firma GPG del índice
     comprobada (Core rechaza la descarga si la firma no es válida).
  2. Casado de los drivers de Windows del USB con los dispositivos reales de la
     VM (INF reales de virtio-win contra los Hardware ID de la VM).
  3. Wi-Fi con radios simuladas (mac80211_hwsim) y un punto de acceso virtual
     (hostapd + dnsmasq): búsqueda, contraseña incorrecta, conexión y
     desconexión, con el perfil de NetworkManager en un archivo 0600.
  4. "Instalar" Debian con respuestas automáticas (preseed) y arranque de una
     sola vez: el orden de arranque UEFI se cambia y se restaura al volver.
  5. "Instalar" una imagen de Windows con drivers, respuestas y atajo de
     Windows 11: archivos generados, entradas de Ventoy y limpieza al volver.

Uso: vmtest-v03.py IMAGEN_USB.img SALIDA [wifi] [debian] [windows]

Sin escenarios los ejecuta todos, uno tras otro, cada uno en su VM; con uno
solo (o varios procesos a la vez) se pueden repartir entre VM en paralelo.
"""

import importlib.util
import json
import os
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))


def load(name, filename):
    """Carga otro script de pruebas como módulo."""
    spec = importlib.util.spec_from_file_location(name, os.path.join(HERE, filename))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


usbtest = load("usbtest", "vmtest-usb.py")
vmtest = usbtest.vmtest

WIFI_SSID = "RbtTest"
WIFI_PASSWORD = "rebootera123"

# * Prepara el punto de acceso virtual: dos radios simuladas; la segunda va a su
# * propio espacio de red (NetworkManager no la toca) con hostapd y dnsmasq (DHCP).
WIFI_SETUP = """
set -x
modprobe mac80211_hwsim radios=2
sleep 4
ip netns add ap
iw phy phy1 set netns name ap
ip netns exec ap ip link set wlan1 up
ip netns exec ap ip addr add 192.168.77.1/24 dev wlan1
cat > /tmp/hostapd.conf <<'EOF'
interface=wlan1
driver=nl80211
ssid=%(ssid)s
hw_mode=g
channel=6
wpa=2
wpa_key_mgmt=WPA-PSK
rsn_pairwise=CCMP
wpa_passphrase=%(password)s
EOF
ip netns exec ap hostapd -B /tmp/hostapd.conf
ip netns exec ap dnsmasq --interface=wlan1 --bind-interfaces --except-interface=lo --dhcp-range=192.168.77.10,192.168.77.50,255.255.255.0,12h
sleep 3
ip netns exec ap iw dev wlan1 info | grep -E 'ssid|type'
""" % {"ssid": WIFI_SSID, "password": WIFI_PASSWORD}


def vm_script(vm, name, text):
    """Copia un script a la VM y lo ejecuta, devolviendo su salida."""
    import base64

    encoded = base64.b64encode(text.encode()).decode()
    vm.run("echo %s | base64 -d > /tmp/%s.sh" % (encoded, name))
    return vm.run("bash /tmp/%s.sh 2>&1 | tail -40" % name, timeout=180)


def test_download_signed(vm, checks):
    """Descarga Debian del catálogo con el índice firmado y verificado."""
    status, job = usbtest.api(vm, "POST", "/api/transfers/download", {"id": "debian-13-netinst"})
    ok = status == 202 and "Debian CD signing key" in str(job.get("signedBy", ""))
    checks.check("descarga con índice firmado: firma GPG de Debian comprobada", ok, str(job)[:300])
    if status != 202:
        return None
    done = usbtest.wait_transfer(vm, "download", job["fileName"], 1800)
    checks.check("descarga de Debian terminada y verificada", bool(done) and done["status"] == "done",
                 str(done)[:200] if done else "sin trabajo")
    return job["fileName"] if done and done["status"] == "done" else None


def test_drivers_plan(vm, checks):
    """Casa los INF reales de virtio-win con los dispositivos de la VM."""
    status, plan = usbtest.api(vm, "GET", "/api/drivers/plan")
    matches = plan.get("matches", []) if isinstance(plan, dict) else []
    names = [m["package"]["inf"] for m in matches]
    checks.check("drivers: el USB tiene paquetes de Windows", status == 200 and plan.get("packages", 0) >= 3,
                 "paquetes: %s" % plan.get("packages"))
    checks.check("drivers: la tarjeta de red virtio casa con netkvm.inf",
                 any(n.endswith("netkvm.inf") for n in names), str(names))
    nic = [m for m in matches if m["package"]["inf"].endswith("netkvm.inf")]
    checks.check("drivers: casa por el Hardware ID más específico y el catálogo está firmado",
                 bool(nic) and nic[0]["specific"] and nic[0]["package"]["signature"] == "present",
                 str(nic[0]["hardwareId"] if nic else ""))


def test_wifi(vm, checks):
    """Wi-Fi con radios simuladas y un punto de acceso virtual."""
    out = vm_script(vm, "wifi-setup", WIFI_SETUP)
    checks.check("wifi: radios simuladas y punto de acceso listos", WIFI_SSID in out and "type AP" in out, out[-300:])
    time.sleep(3)
    status, state = usbtest.api(vm, "GET", "/api/wifi")
    checks.check("wifi: Core ve el adaptador", status == 200 and state.get("adapter") and state.get("device") == "wlan0", str(state))
    status, networks = usbtest.api(vm, "POST", "/api/wifi/scan", {})
    ours = [n for n in networks if n["ssid"] == WIFI_SSID] if isinstance(networks, list) else []
    checks.check("wifi: la búsqueda encuentra la red WPA2", bool(ours) and ours[0]["security"] == "wpa", str(networks)[:300])
    # * Contraseña incorrecta: falla y no deja perfil guardado.
    status, error = usbtest.api(vm, "POST", "/api/wifi/connect", {"ssid": WIFI_SSID, "password": "incorrecta1"})
    profile = "/etc/NetworkManager/system-connections/rebootera-wifi.nmconnection"
    left = vm.run("test -e %s && echo QUEDA || echo LIMPIO" % profile)
    checks.check("wifi: una contraseña incorrecta se rechaza y no deja el perfil",
                 status == 502 and error.get("code") == "wifi-failed" and "LIMPIO" in left, "%s %s %s" % (status, error, left))
    # * Contraseña correcta.
    status, ok = usbtest.api(vm, "POST", "/api/wifi/connect", {"ssid": WIFI_SSID, "password": WIFI_PASSWORD})
    checks.check("wifi: conecta con la contraseña correcta", status == 200, "%s %s" % (status, ok))
    info = vm.run("nmcli -t -f GENERAL.STATE,IP4.ADDRESS device show wlan0; stat -c '%%a %%U' %s; ping -c1 -W3 192.168.77.1 | tail -2" % profile)
    checks.check("wifi: obtiene IP por DHCP y llega al punto de acceso", "(connected)" in info and "192.168.77." in info and "1 received" in info, info)
    checks.check("wifi: el perfil es 0600 de root", "600 root" in info, info)
    pattern = WIFI_PASSWORD[:-1] + "[" + WIFI_PASSWORD[-1] + "]"  # no coincide con la propia línea del grep
    leak = vm.run("grep -l '%s' /proc/[0-9]*/cmdline 2>/dev/null | wc -l" % pattern)
    checks.check("wifi: la contraseña no aparece en ninguna línea de órdenes", leak.strip().endswith("0"), leak)
    status, st = usbtest.api(vm, "GET", "/api/wifi")
    checks.check("wifi: el estado dice conectado", st.get("state") == "connected" and st.get("connection") == "rebootera-wifi", str(st))
    status, _ = usbtest.api(vm, "POST", "/api/wifi/disconnect", {})
    left = vm.run("test -e %s && echo QUEDA || echo LIMPIO; nmcli -t -f GENERAL.STATE device show wlan0" % profile)
    checks.check("wifi: desconectar olvida la red", status == 200 and "LIMPIO" in left and "(connected)" not in left, left)


def test_keyboard(vm, checks):
    """Cambia la distribución del teclado: se aplica sin reiniciar el kiosco."""
    status, state = usbtest.api(vm, "GET", "/api/keyboard")
    checks.check("teclado: arranca en español y ofrece la lista cerrada",
                 status == 200 and state.get("layout") == "es" and "us" in state.get("layouts", []), str(state))
    status, error = usbtest.api(vm, "POST", "/api/keyboard", {"layout": "es;reboot"})
    checks.check("teclado: una distribución fuera de la lista se rechaza",
                 status == 400 and error.get("code") == "keyboard-invalid", "%s %s" % (status, error))
    before = vm.run("pgrep -x labwc | head -1").strip()
    status, ok = usbtest.api(vm, "POST", "/api/keyboard", {"layout": "us"})
    checks.check("teclado: cambiar a inglés responde bien", status == 200, "%s %s" % (status, ok))
    time.sleep(6)
    after = vm.run("pgrep -x labwc | head -1").strip()
    log_line = vm.run("journalctl -u rebootera-agent --no-pager -n 40 | grep -c 'sin reiniciar'")
    checks.check("teclado: cambia al momento, sin reiniciar el kiosco",
                 after == before and log_line.strip().endswith(("1", "2")), "antes %s, ahora %s, avisos %s" % (before, after, log_line.strip()))
    shell = vm.run("pgrep -f rebootera-shell | head -1")
    checks.check("teclado: la interfaz sigue abierta", shell.strip().isdigit(), shell)
    saved = vm.run("cat /run/rebootera/vault/rebootera/state/keyboard.json; cat /run/rebootera/keyboard")
    checks.check("teclado: la elección se guarda en el Vault y en el estado del arranque",
                 '"layout":"us"' in saved.replace(" ", "") and saved.strip().endswith("us"), saved)


def labwc_config(vm):
    """Devuelve el rc.xml que start-kiosk generó para la sesión actual."""
    return vm.run("cat /run/user/$(id -u rebootera)/rebootera-labwc/rc.xml")


def efi_state(vm):
    """Devuelve (BootOrder, BootNext, BootCurrent) de efibootmgr."""
    out = vm.run("efibootmgr")
    values = {"BootOrder": [], "BootNext": "", "BootCurrent": ""}
    for line in out.splitlines():
        for key in values:
            if line.startswith(key + ":"):
                value = line.split(":", 1)[1].strip()
                values[key] = value.split(",") if key == "BootOrder" else value
    return values["BootOrder"], values["BootNext"], values["BootCurrent"]


def go_back_to_rebootera(vm, checks, label, key):
    """Reinicia la VM, elige Rebootera en el menú de Ventoy y espera su consola.

    El menú está ordenado por nombre y el cursor empieza en la imagen
    preseleccionada; key es "down" o "up" según donde quede Rebootera."""
    vm.qmp_cmd("system_reset")
    usbtest.wait_firmware(vm, 90)
    time.sleep(6)
    vm.screenshot("%s-menu-ventoy" % label)
    vm.key(key)
    vm.key("ret")
    return usbtest.boot_rebootera(vm, checks, label)


def test_debian_install(vm, checks, image):
    """Instala Debian con preseed y arranque de una sola vez, y vuelve."""
    order, _, current = efi_state(vm)
    checks.check("uefi: hay un orden de arranque y el USB es la entrada actual", bool(order) and current in order, "%s %s" % (order, current))
    body = {
        "path": "isos/linux/" + image,
        "bootOnce": True,
        "unattended": {"language": "es", "timeZone": "Europe/Madrid", "userName": "ana", "fullName": "Ana Prueba", "password": "claveprueba1"},
    }
    # * Todo en una orden: la VM se reinicia 1,5 s después de aceptar la petición.
    command = ("python3 /tmp/api.py POST /api/boot/install '%s'; efibootmgr; "
               "cat /run/rebootera/vault/ventoy/rebootera/preseed.cfg") % json.dumps(body)
    out = vm.run(command, timeout=120)
    checks.check("instalar Debian: respuesta aceptada con el orden de arranque ajustado",
                 "RBT 202" in out and '"bootOrder":"adjusted"' in out, out[:200])
    after_order, after_next, _ = efi_state_from(out)
    checks.check("uefi: el USB pasa a ser el último y se fija BootNext",
                 bool(after_order) and after_order[-1] == current and after_next == current,
                 "orden %s, BootNext %s" % (after_order, after_next))
    checks.check("preseed: idioma, zona horaria y cuenta, con la contraseña cifrada y sin particionado",
                 "locale string es_ES.UTF-8" in out and "time/zone string Europe/Madrid" in out
                 and "passwd/user-password-crypted password $6$" in out and "claveprueba1" not in out and "partman" not in out,
                 out[-500:])
    # * La VM se reinicia sola y Ventoy arranca el instalador de Debian con el preseed.
    rebooted = usbtest.wait_firmware(vm, 400)
    checks.check("instalar Debian: el equipo se reinicia", rebooted, "")
    time.sleep(60)
    vm.screenshot("debian-1")
    return order, current


def efi_state_from(text):
    """Extrae BootOrder y BootNext de un texto que contiene la salida de efibootmgr."""
    order, nxt, cur = [], "", ""
    for line in text.splitlines():
        if line.startswith("BootOrder:"):
            order = line.split(":", 1)[1].strip().split(",")
        elif line.startswith("BootNext:"):
            nxt = line.split(":", 1)[1].strip()
        elif line.startswith("BootCurrent:"):
            cur = line.split(":", 1)[1].strip()
    return order, nxt, cur


def test_windows_prep(vm, checks):
    """Prepara una instalación de Windows con drivers, respuestas y atajo."""
    vm.run("mkdir -p /run/rebootera/vault/isos/windows && dd if=/dev/zero of=/run/rebootera/vault/isos/windows/Win11-prueba.iso bs=1M count=4 2>/dev/null")
    body = {
        "path": "isos/windows/Win11-prueba.iso",
        "drivers": True,
        "winBypass": True,
        "unattended": {"language": "es", "timeZone": "Europe/Madrid", "userName": "ana", "fullName": "Ana", "password": "claveprueba1"},
    }
    work = "/run/rebootera/vault/ventoy/rebootera"
    command = (
        "python3 /tmp/api.py POST /api/boot/install '%s'; "
        "ls -la %s; 7z l %s/drivers.7z | grep -E 'netkvm|viostor|vioscsi|files'; "
        "echo UNATTEND-DRIVERPATH=$(grep -c 'X:.rebootera-drivers' %s/autounattend.xml) DISKCONFIG=$(grep -c DiskConfiguration %s/autounattend.xml); "
        "grep -E 'injection|auto_install|archive|template|autosel|BYPASS' /run/rebootera/vault/ventoy/ventoy.json"
    ) % (json.dumps(body), work, work, work, work)
    out = vm.run(command, timeout=180)
    checks.check("Windows: la petición se acepta", "RBT 202" in out, out[:200])
    # * La VM solo tiene tarjeta de red virtio: el 7z lleva NetKVM y no los de disco.
    checks.check("Windows: el 7z lleva solo los drivers que casan (NetKVM) con su carpeta rebootera-drivers",
                 "rebootera-drivers/01-netkvm/netkvm.inf" in out and "vioscsi" not in out and "viostor" not in out, out[:700])
    checks.check("Windows: el autounattend.xml apunta a los drivers y no toca discos",
                 "UNATTEND-DRIVERPATH=2 DISKCONFIG=0" in out, out[-500:])
    checks.check("Windows: ventoy.json lleva injection, auto_install con autosel y el atajo de requisitos",
                 '"injection"' in out and '"auto_install"' in out and '"autosel": 1' in out and '"VTOY_WIN11_BYPASS_CHECK": "1"' in out, out[-600:])
    rebooted = usbtest.wait_firmware(vm, 400)
    checks.check("Windows: el equipo se reinicia", rebooted, "")


def scenario_wifi(vm, checks):
    """Casado de drivers con INF reales y Wi-Fi con radios simuladas."""
    if not usbtest.boot_rebootera(vm, checks, "primer arranque"):
        return
    test_drivers_plan(vm, checks)
    test_wifi(vm, checks)
    test_keyboard(vm, checks)


def scenario_debian(vm, checks):
    """Descarga con firma GPG, preseed de Debian y arranque de una sola vez."""
    if not usbtest.boot_rebootera(vm, checks, "primer arranque"):
        return
    image = test_download_signed(vm, checks)
    if image is None:
        return
    original_order, _ = test_debian_install(vm, checks, image)
    # * Menú: debian, Rebootera. El cursor empieza en Debian (preseleccionada).
    if not go_back_to_rebootera(vm, checks, "segundo arranque", "down"):
        return
    vm.run("export TERM=dumb PAGER=cat SYSTEMD_COLORS=0")
    order, nxt, _ = efi_state(vm)
    checks.check("uefi: al volver se restaura el orden de arranque original y no queda BootNext",
                 order == original_order and nxt == "", "antes %s, ahora %s, BootNext '%s'" % (original_order, order, nxt))
    gone = vm.run("ls /run/rebootera/vault/ventoy/rebootera 2>&1 | head -2; grep -c auto_install /run/rebootera/vault/ventoy/ventoy.json")
    checks.check("preseed: se borra la carpeta de trabajo (contraseñas) al volver", "No such file" in gone, gone)
    restored = vm.run("grep -o '\"VTOY_DEFAULT_IMAGE\": \"[^\"]*\"' /run/rebootera/vault/ventoy/ventoy.json")
    checks.check("Ventoy vuelve a tener Rebootera por defecto", "/rebootera/rebootera-os.iso" in restored, restored)


def scenario_adopt(vm, checks):
    """USB "adoptado": Rebootera añadido sin borrar a un USB tipo Rufus (NTFS con archivos del usuario)."""
    if not usbtest.boot_rebootera(vm, checks, "arranque del USB adoptado"):
        return
    mount = vm.run("findmnt -no FSTYPE,OPTIONS /run/rebootera/vault")
    checks.check("adopt: el Vault es el NTFS del USB (ntfs3, noexec, nosuid, nodev)",
                 "ntfs3" in mount and "noexec" in mount and "nosuid" in mount and "nodev" in mount, mount)
    status, vault = usbtest.api(vm, "GET", "/api/vault")
    checks.check("adopt: la API ve el Vault disponible", status == 200 and vault.get("available") is True, str(vault))
    out = vm.run("cd /run/rebootera/vault && sha256sum -c RUFUS-USER-FILES.sha256 2>&1 | tail -5")
    checks.check("adopt: los archivos del usuario (Windows de Rufus y 'Mis cosas') siguen intactos",
                 out.count(": OK") == 3 and "FAILED" not in out, out)
    folders = vm.run("ls /run/rebootera/vault /run/rebootera/vault/isos 2>&1 | tr '\\n' ' '")
    checks.check("adopt: Rebootera creó sus carpetas junto a los archivos del usuario",
                 "rebootera" in folders and "windows" in folders and "sources" in folders and "Mis cosas" in folders, folders)
    # * Se puede escribir en el Vault NTFS (lo que usan las descargas y las imágenes añadidas).
    wrote = vm.run("su -s /bin/sh rebootera-core -c 'echo prueba > /run/rebootera/vault/isos/linux/prueba.txt && cat /run/rebootera/vault/isos/linux/prueba.txt && rm /run/rebootera/vault/isos/linux/prueba.txt' 2>&1")
    checks.check("adopt: Core puede escribir en el Vault NTFS", "prueba" in wrote, wrote)


def wait_partitions(vm, timeout=420):
    """Espera a que termine la operación de particiones y devuelve su estado final."""
    deadline = time.time() + timeout
    state = {}
    while time.time() < deadline:
        time.sleep(3)
        status, state = usbtest.api(vm, "GET", "/api/partitions/status")
        if status == 200 and not state.get("running") and (state.get("done") or state.get("error")):
            return state
    return state


def verify_partitions(vm, label):
    """Monta las dos particiones de prueba en solo lectura y comprueba sus archivos y sus sistemas de archivos."""
    return vm.run(
        "for p in 1 2; do mkdir -p /mnt/p$p; "
        "mount -o ro /dev/vda$p /mnt/p$p 2>/dev/null || ntfs-3g -o ro /dev/vda$p /mnt/p$p; "
        "(cd /mnt/p$p && sha256sum -c SUMS 2>&1 | grep -c ': OK'); umount /mnt/p$p; done; "
        "e2fsck -fn /dev/vda1 >/dev/null 2>&1 && echo EXT4-SANA; ntfsresize --check /dev/vda2 >/dev/null 2>&1 && echo NTFS-SANA")


def apply_part(vm, checks, label, op, expect_move):
    """Pide el plan, lo aplica con la confirmación escrita y comprueba que termina bien."""
    status, plan = usbtest.api(vm, "POST", "/api/partitions/plan", {"op": op})
    kinds = [step["kind"] for step in plan.get("steps", [])] if status == 200 else []
    checks.check("particiones: %s: la vista previa da los pasos%s" % (label, " (con copia de datos)" if expect_move else ""),
                 status == 200 and ("move" in kinds) == expect_move, str(plan)[:300])
    if status != 200:
        return False
    status, error = usbtest.api(vm, "POST", "/api/partitions/apply", {"op": op, "confirm": "yes"})
    checks.check("particiones: %s: sin la confirmación escrita no se aplica" % label,
                 status == 400 and error.get("code") == "part-confirm", "%s %s" % (status, error))
    status, started = usbtest.api(vm, "POST", "/api/partitions/apply", {"op": op, "confirm": plan["confirm"]})
    if status != 202:
        checks.check("particiones: %s: se acepta con la confirmación" % label, False, "%s %s" % (status, started))
        return False
    final = wait_partitions(vm)
    checks.check("particiones: %s: termina bien" % label, final.get("done") is True and not final.get("error"), str(final))
    return final.get("done") is True


def scenario_partitions(vm, checks):
    """Gestor de particiones sobre un disco virtual real con ext4 y NTFS con datos."""
    if not usbtest.boot_rebootera(vm, checks, "primer arranque"):
        return
    status, disks = usbtest.api(vm, "GET", "/api/partitions")
    by_name = {d["name"]: d for d in disks} if status == 200 else {}
    stick = next((d for d in by_name.values() if d["role"] == "live-media"), None)
    checks.check("particiones: el USB de arranque sale bloqueado", bool(stick) and stick["layout"].get("locked") == "protected", str(stick)[:200])
    disk = by_name.get("vda")
    checks.check("particiones: el disco de pruebas (ext4 + NTFS) se ve con su distribución",
                 bool(disk) and [s.get("fsType") for s in disk["layout"]["segments"] if s["number"]] == ["ext4", "ntfs"], str(disk)[:300])
    if not disk:
        return
    if stick:
        status, error = usbtest.api(vm, "POST", "/api/partitions/plan", {"op": {"kind": "delete", "disk": stick["name"], "part": 1}})
        checks.check("particiones: no se puede planificar nada sobre el USB de Rebootera",
                     status == 400 and error.get("code") == "part-protected", "%s %s" % (status, error))
    # * Capturas de la página con una partición elegida (los clics son a ojo: solo sirven para mirarla).
    vm.click(*vmtest.NAV["storage"])
    time.sleep(3)
    vm.screenshot("particiones-lista")
    vm.click(490, 510)
    time.sleep(1)
    vm.screenshot("particiones-seleccion")
    seg = {s["number"]: s for s in disk["layout"]["segments"] if s["number"]}
    left_mib = seg[1]["sectors"] // 2048
    base = {"kind": "boundary", "disk": "vda", "part": 1, "other": 2, "fs": "", "label": "", "startMiB": 0}
    # 1. La ext4 (izquierda) crece 60 MiB a costa de la NTFS.
    if not apply_part(vm, checks, "ext4 gana 60 MiB", dict(base, sizeMiB=left_mib + 60), True):
        return
    out = verify_partitions(vm, "tras crecer")
    checks.check("particiones: los datos de las dos particiones siguen intactos (3 y 3) y están sanas tras mover el límite",
                 out.split().count("3") == 2 and "EXT4-SANA" in out and "NTFS-SANA" in out, out)
    # 2. La ext4 encoge 100 MiB y la NTFS la sigue.
    if not apply_part(vm, checks, "ext4 cede 100 MiB", dict(base, sizeMiB=left_mib - 40), True):
        return
    out = verify_partitions(vm, "tras encoger")
    checks.check("particiones: datos intactos y sistemas de archivos sanos en la otra dirección",
                 out.split().count("3") == 2 and "EXT4-SANA" in out and "NTFS-SANA" in out, out)
    # 3. La NTFS encoge por el final, se crea una exFAT en el hueco y se borra.
    status, disks = usbtest.api(vm, "GET", "/api/partitions")
    seg2 = {s["number"]: s for d in disks if d["name"] == "vda" for s in d["layout"]["segments"] if s["number"]}[2]
    ntfs_mib = seg2["sectors"] // 2048
    if not apply_part(vm, checks, "NTFS encoge 120 MiB por el final", {"kind": "resize", "disk": "vda", "part": 2, "other": 0, "sizeMiB": ntfs_mib - 120, "startMiB": 0, "fs": "", "label": ""}, False):
        return
    start_mib = (seg2["start"] + (ntfs_mib - 120) * 2048) // 2048
    if not apply_part(vm, checks, "crear una exFAT en el hueco", {"kind": "create", "disk": "vda", "part": 0, "other": 0, "sizeMiB": 100, "startMiB": start_mib, "fs": "exfat", "label": "NUEVA"}, False):
        return
    fs = vm.run("blkid -o value -s TYPE /dev/vda3")
    checks.check("particiones: la partición nueva está formateada como exFAT", "exfat" in fs, fs)
    if not apply_part(vm, checks, "borrar la partición nueva", {"kind": "delete", "disk": "vda", "part": 3, "other": 0, "sizeMiB": 0, "startMiB": 0, "fs": "", "label": ""}, False):
        return
    gone = vm.run("blkid /dev/vda3 >/dev/null 2>&1 && echo EXISTE || echo NO-EXISTE")
    out = verify_partitions(vm, "final")
    checks.check("particiones: la partición borrada desaparece y el resto sigue intacto",
                 "NO-EXISTE" in gone and out.split().count("3") == 2 and "EXT4-SANA" in out and "NTFS-SANA" in out, gone + " | " + out)


def scenario_windows(vm, checks):
    """Preparación de una instalación de Windows y limpieza al volver."""
    if not usbtest.boot_rebootera(vm, checks, "primer arranque"):
        return
    test_windows_prep(vm, checks)
    # * Menú: Rebootera, Win11-prueba. El cursor empieza en Win11-prueba (la última).
    if not go_back_to_rebootera(vm, checks, "segundo arranque", "up"):
        return
    gone = vm.run("ls /run/rebootera/vault/ventoy/rebootera 2>&1 | head -2; grep -cE 'injection|auto_install' /run/rebootera/vault/ventoy/ventoy.json; "
                  "grep -o '\"VTOY_WIN11_BYPASS_CHECK\": \"[^\"]*\"' /run/rebootera/vault/ventoy/ventoy.json")
    checks.check("Windows: al volver se borran drivers y respuestas y se quita el atajo de requisitos",
                 "No such file" in gone and '"VTOY_WIN11_BYPASS_CHECK": "0"' in gone, gone)


SCENARIOS = {"wifi": scenario_wifi, "debian": scenario_debian, "windows": scenario_windows, "adopt": scenario_adopt, "partitions": scenario_partitions}


def main():
    """Ejecuta los escenarios pedidos, cada uno en su propia VM."""
    if len(sys.argv) < 3:
        print(__doc__)
        return 2
    image, outdir = sys.argv[1:3]
    names = sys.argv[3:] or list(SCENARIOS)
    code = 0
    for name in names:
        folder = os.path.join(outdir, "uefi-v03-" + name)
        os.makedirs(folder, exist_ok=True)
        for stale in os.listdir(folder):
            os.remove(os.path.join(folder, stale))
        vm = vmtest.VM("uefi", image, folder)
        checks = vmtest.Checks()
        try:
            vm.start()
            SCENARIOS[name](vm, checks)
        finally:
            vm.stop()
        code |= vmtest.finish(folder, "uefi", checks)
    return code


if __name__ == "__main__":
    sys.exit(main())

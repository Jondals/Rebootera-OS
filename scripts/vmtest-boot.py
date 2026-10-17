#!/usr/bin/python3
"""Arranca una imagen en QEMU/KVM y hace capturas a intervalos regulares.

Sirve para comprobar visualmente qué arranca (por ejemplo, la imagen que
Ventoy tiene preseleccionada en un USB de Rebootera) aunque el sistema no
escriba nada en la consola serie. Guarda una captura cada INTERVALO segundos
durante DURACIÓN segundos y la consola serie completa.

Con --keys "12:down,13:ret" pulsa teclas en esos segundos (por ejemplo,
para elegir una entrada de un menú de arranque).

Uso: vmtest-boot.py MODO IMAGEN SALIDA [DURACIÓN] [INTERVALO] [--keys LISTA]
"""

import os
import shutil
import sys
import time
import importlib.util

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("vmtest", os.path.join(HERE, "vmtest.py"))
vmtest = importlib.util.module_from_spec(spec)
spec.loader.exec_module(vmtest)


def main():
    """Arranca la VM y guarda las capturas."""
    if len(sys.argv) < 4:
        print(__doc__)
        return 2
    keys = []
    args = sys.argv[1:]
    if "--keys" in args:
        index = args.index("--keys")
        for item in args[index + 1].split(","):
            second, key = item.split(":")
            keys.append((int(second), key))
        del args[index:index + 2]
    mode, image, outdir = args[0], args[1], args[2]
    duration = int(args[3]) if len(args) > 3 else 90
    interval = int(args[4]) if len(args) > 4 else 10
    outdir = os.path.join(outdir, mode + "-boot")
    shutil.rmtree(outdir, ignore_errors=True)
    os.makedirs(outdir)
    vm = vmtest.VM(mode, image, outdir)
    try:
        vm.start()
        started = time.time()
        shot = 0
        next_shot = interval
        while time.time() - started < duration:
            time.sleep(0.5)
            elapsed = time.time() - started
            # ! Vaciar la consola serie siempre: si nadie la lee, QEMU frena al
            # ! invitado cuando escribe en ella (una ISO de depuración parece
            # ! "lentísima" solo porque la consola está llena).
            vm.pump()
            # * Teclas programadas que ya toca pulsar.
            while keys and keys[0][0] <= elapsed:
                vm.key(keys.pop(0)[1])
            if elapsed >= next_shot:
                vm.pump()
                shot += 1
                vm.screenshot("%02d-%03ds" % (shot, int(elapsed)))
                next_shot += interval
    finally:
        vm.pump()
        vm.stop()
    return 0


if __name__ == "__main__":
    sys.exit(main())

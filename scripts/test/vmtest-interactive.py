#!/usr/bin/python3
"""Arranca una imagen en QEMU/KVM y la deja controlar desde fuera.

Sirve para recorrer a mano cosas que no se pueden automatizar de antemano
(por ejemplo, MokManager al inscribir la clave de Ventoy con Secure Boot) y
para sacar capturas reales de una pantalla concreta.

Las órdenes se escriben, una por línea, en el archivo SALIDA/cmd.txt:

  key NOMBRE        pulsa una tecla de QEMU (ret, down, up, esc, tab, spc...)
  type TEXTO        escribe un texto (solo letras minúsculas, números y espacios)
  click X Y         hace clic en esa posición de la pantalla
  shot NOMBRE      guarda una captura en SALIDA/NOMBRE.png
  wait SEGUNDOS     espera (la consola serie se sigue vaciando)
  quit              apaga la VM y termina

Cada orden ejecutada se anota en SALIDA/done.txt con su número de línea.
Uso: vmtest-interactive.py MODO IMAGEN SALIDA [DURACIÓN_MÁXIMA]
"""

import importlib.util
import os
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("vmtest", os.path.join(HERE, "vmtest.py"))
vmtest = importlib.util.module_from_spec(spec)
spec.loader.exec_module(vmtest)

# * Teclas de QEMU para los caracteres que "type" sabe escribir.
SPECIAL_KEYS = {" ": "spc", ".": "dot", "-": "minus", "/": "slash"}


def main():
    """Ejecuta las órdenes del archivo hasta "quit" o hasta agotar el tiempo."""
    if len(sys.argv) < 4:
        print(__doc__)
        return 2
    mode, image, outdir = sys.argv[1], sys.argv[2], sys.argv[3]
    limit = int(sys.argv[4]) if len(sys.argv) > 4 else 3600
    os.makedirs(outdir, exist_ok=True)
    cmd_path = os.path.join(outdir, "cmd.txt")
    done_path = os.path.join(outdir, "done.txt")
    open(cmd_path, "w").close()
    open(done_path, "w").close()
    vm = vmtest.VM(mode, image, outdir)
    handled = 0
    started = time.time()
    wait_until = 0.0
    try:
        vm.start()
        while time.time() - started < limit:
            time.sleep(0.2)
            vm.pump()
            if time.time() < wait_until:
                continue
            with open(cmd_path, encoding="utf-8") as handle:
                lines = handle.read().splitlines()
            if handled >= len(lines):
                continue
            line = lines[handled].strip()
            handled += 1
            verb, _, arg = line.partition(" ")
            if verb == "key":
                vm.key(arg)
            elif verb == "type":
                for char in arg:
                    vm.key(SPECIAL_KEYS.get(char, char))
                    time.sleep(0.1)
            elif verb == "click":
                x, y = arg.split()
                vm.click(int(x), int(y))
            elif verb == "shot":
                vm.screenshot(arg)
            elif verb == "wait":
                wait_until = time.time() + float(arg)
            elif verb == "quit":
                break
            with open(done_path, "a", encoding="utf-8") as handle:
                handle.write("%d %s\n" % (handled, line))
    finally:
        vm.pump()
        vm.stop()
    return 0


if __name__ == "__main__":
    sys.exit(main())

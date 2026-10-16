# Pruebas

Cómo se prueba y qué resultados se han obtenido. Las pruebas se repiten con `scripts/vmtest.py` (ver README).

## Entorno

- QEMU 10.0 con KVM dentro de WSL 2 (Debian 13), 4 vCPU, 4 GB de RAM, VGA estándar, red virtio con NAT.
- UEFI: OVMF 2025.02 de Debian. `uefi` usa el firmware sin Secure Boot; `uefi-sb` usa `OVMF_CODE_4M.secboot.fd` con las claves de Microsoft (`OVMF_VARS_4M.ms.fd`) y SMM, es decir, Secure Boot activo y aplicado como en un PC normal.
- BIOS: SeaBIOS (máquina `pc`).
- Ningún disco real se ha tocado.

## Hito 1: imagen arrancable con la interfaz (2026-10-16)

Imágenes probadas:

| Imagen | SHA-256 |
| --- | --- |
| `rebootera-os-0.1.0-dev-amd64-debug.iso` | `fec5465f1a7a26c33132f3713a26cb19af795c18cfd9b573df738cb998f3ca98` |
| `rebootera-os-0.1.0-dev-amd64.iso` (definitiva) | `19200d1047a1d36a774241f218f31a2f9d52f3ede1f99df77efb2760c4a9969c` |

### Imagen de depuración, con consola serie

| Comprobación | UEFI | UEFI + Secure Boot | BIOS |
| --- | --- | --- | --- |
| Arranca hasta la consola | OK | OK | OK |
| `rebootera-agent`, `rebootera-core` y `rebootera-ui` activos | OK | OK | OK |
| Core se ejecuta como `rebootera-core`, el agente como root | OK | OK | OK |
| `cage` y la ventana kiosco se ejecutan como `rebootera` | OK | OK | OK |
| El kiosco no tiene ninguna capacidad (`CapPrm` a cero) | OK | OK | OK |
| Core escucha solo en `127.0.0.1:7080` | OK | OK | OK |
| Token 0640 `rebootera-core:rebootera-ui`, socket del agente 0660 `root:rebootera-core` | OK | OK | OK |
| root bloqueado para contraseña | OK | OK | OK |
| El kernel ve el modo de firmware esperado | OK | OK | OK |
| Secure Boot activo según el firmware | | OK | |
| La API rechaza peticiones sin sesión (401) | OK | OK | OK |
| La API informa del firmware y de Secure Boot correctos | OK (`uefi off`) | OK (`uefi on`) | OK (`bios n/a`) |
| Apagar desde la interfaz (teclado: Tab, Intro, Tab, Intro) apaga la VM | OK | OK | OK |
| Tiempo de arranque según `systemd-analyze` | 5,8 s | 6,9 s | 5,2 s |

### Imagen definitiva, sin consola

| Comprobación | UEFI + Secure Boot | BIOS |
| --- | --- | --- |
| Menú de arranque con la identidad de Rebootera | OK | OK |
| La interfaz aparece sola, a pantalla completa | OK | OK |
| "Este equipo" muestra datos reales | OK | OK |
| Apagar desde la interfaz apaga la VM | OK | OK |

### Pruebas unitarias de Core

`go test ./...`: protecciones de la API (Host ajeno 421, sin sesión 401, token incorrecto 401, cookie `HttpOnly` y `SameSite=Strict`, CSP, POST sin cabecera anti-CSRF 403, acción desconocida 400, agente caído 503) y lectura de firmware (BIOS, Secure Boot activado, desactivado, variable ausente y variable truncada, modelo de relleno en la DMI). Todas pasan.

### Errores encontrados y corregidos durante el hito

| Error | Causa | Arreglo |
| --- | --- | --- |
| Pantalla negra: la ventana kiosco moría al arrancar | `pam_systemd` (systemd 257) da `CAP_WAKE_ALARM` ambiental a las sesiones y `bubblewrap`, el sandbox de WebKitGTK, aborta con "Unexpected capabilities but not setuid" | `setpriv --ambient-caps=-all` antes de `cage`; servicio PAM propio y mínimo `rebootera-kiosk` |
| Los cambios no entraban en la imagen | `lb clean --binary` conservaba el chroot y live-build saltaba includes y hooks | `lb clean` completo, conservando la caché de paquetes |
| Secure Boot siempre "desconocido" | GUID mal escrito en la variable EFI; las pruebas unitarias usaban la misma constante | GUID corregido; la prueba usa el nombre escrito a mano y la VM comprueba lo que devuelve la API |
| Secure Boot "desconocido" en firmware sin soporte | La variable no existe en ese firmware | Ausencia de la variable = desactivado, como dice la especificación UEFI |
| Apagar desde la interfaz tardaba más de 90 s | El `plymouthd` de apagado bloqueaba la fase final de `systemd-shutdown` | Pantallas de apagado y reinicio de Plymouth desactivadas |
| Entrada seleccionada invisible y artefactos en el menú de arranque | PNG de 16 bits y recuadro de selección incompleto | PNG24 de 8 bits y las nueve piezas del recuadro |
| `build.sh` podía ejecutar líneas desplazadas si se editaba durante una construcción | bash lee los scripts por posición | Todo el proceso dentro de `main()` |

### No probado todavía

- Hardware real (ver LIMITACIONES.md).
- La animación de Plymouth durante el arranque: no se ha capturado en ese instante. El tema se instala y se selecciona en la imagen, pero no se ha verificado visualmente.
- Reiniciar desde la interfaz (usa el mismo camino que apagar con `systemctl reboot`; solo se ha probado apagar de extremo a extremo).
- VirtualBox y Hyper-V.

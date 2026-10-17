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

## v0.2.0 (2026-10-16)

ISO definitiva `rebootera-os-0.2.0-amd64.iso` y la misma construcción con consola serie (`--debug`) para las pruebas automáticas. Rebootera Desktop se recompila con `scripts/build-desktop.sh`, que ejecuta `go test ./...` (todo pasa).

### ISO definitiva (sin consola): arranque hasta la interfaz

| Modo | Resultado |
| --- | --- |
| UEFI + Secure Boot (OVMF con claves de Microsoft) | OK: la interfaz aparece sola y muestra "Secure Boot: Active" |
| BIOS (SeaBIOS) | OK: la interfaz aparece sola y muestra "Legacy BIOS" |

### ISO de depuración: batería completa de `vmtest.py`

| Modo | Comprobaciones |
| --- | --- |
| UEFI + Secure Boot | 17 OK, 0 fallos |
| BIOS | 16 OK, 0 fallos |

### USB completo (`vmtest-usb.py`, UEFI): 20 comprobaciones OK

Vault exFAT con `noexec`, `nosuid` y `nodev`; descarga real verificada; disco del usuario abierto en solo lectura e importación; ajustes en el USB; "Instalar" rechaza con 409 una imagen incompatible del catálogo y acepta la importada; `ventoy.json` apunta a la elegida; el equipo reinicia; Rebootera vuelve a ser la imagen por defecto; ajustes y biblioteca persisten tras reiniciar.

### Decisión sobre Secure Boot en el USB

Con `VTOY_SECURE_BOOT_POLICY=1` Ventoy no deja arrancar Linux con Secure Boot. Se probó la `0` con UEFI + Secure Boot recorriendo MokManager a mano con `scripts/vmtest-interactive.py` (Enroll key from disk, VTOYEFI, `ENROLL_THIS_KEY_IN_MOKMANAGER.cer`, Yes, Reboot): tras inscribir la clave, Rebootera OS arranca hasta la interfaz en menos de 90 s con "Secure Boot: Active". **Decisión: política 0**, con aviso en la interfaz, el README y LIMITACIONES.md de que, con la clave inscrita, el equipo también arranca imágenes sin firma comprobada.

### Compatibilidad de arranque desde el menú del USB

| Imagen | Resultado |
| --- | --- |
| Debian 13.7 netinst, Ubuntu 26.04.1, Fedora 44-1.7 | Arrancan (UEFI) |
| Arch Linux 2026.10.01 | Arranca en BIOS hasta la consola (Arch 7.2.7); en UEFI se queda en pantalla negra |
| Alpine 3.24 | No arranca con Ventoy 1.1.18 (UEFI y BIOS) |
| Ubuntu 24.04, Windows 11 | Sin probar (Ubuntu 24.04 probada en la v0.4.0) |

### Errores encontrados

| Error | Causa | Arreglo |
| --- | --- | --- |
| La ISO de depuración parecía tardar minutos en arrancar desde el USB y acababa en pantalla negra | `vmtest-boot.py` solo leía la consola serie al hacer cada captura; con el socket lleno QEMU frenaba al invitado. No era Secure Boot ni la política de Ventoy | `vmtest-boot.py` vacía la consola en cada vuelta |
| `vmtest-usb.py` fallaba al instalar con un 409 | Alpine está marcada como incompatible y la API ya no la preselecciona | La prueba comprueba el 409 e instala la copia importada; y pulsa dos veces "abajo" porque el menú tiene tres entradas |

### Capturas

`docs/images/`: resumen (UEFI + Secure Boot tras inscribir la clave), intro y menú de arranque (UEFI), Centro de descargas y Rebootera Desktop (Windows 11, recortada para no mostrar las unidades del equipo).

### No probado

- "Crear USB" de Rebootera Desktop con un pendrive real (borra el disco).
- Hardware real, VirtualBox y Hyper-V.


## v0.3.0 (2026-10-17)

Todo se ejecuta con `scripts/test-all.sh` (compila las dos ISO y lanza las pruebas en paralelo, tres VM de 1,5 GB a la vez). Con las ISO ya compiladas tarda unos 10 minutos (antes, más de una hora una prueba detrás de otra); `--quick` (arranque de las dos ISO en UEFI + Secure Boot) tarda 1 minuto. Última ejecución completa: **93 comprobaciones, 0 fallos**.

| Prueba | Resultado |
| --- | --- |
| `go test ./...` (en cada compilación) | Todo pasa: drivers (INF), respuestas desatendidas (con `sha512-crypt` comprobado contra `openssl`), firmas OpenPGP reales de Debian, Ubuntu y Fedora, Wi-Fi, orden de arranque UEFI, descarga de Rebootera con servidor HTTPS local, API de instalación |
| ISO de depuración, UEFI + Secure Boot | 17 OK |
| ISO de depuración, BIOS | 16 OK |
| ISO definitiva, UEFI + Secure Boot y BIOS | 2 OK en cada una: la interfaz aparece sola y muestra el modo de firmware |
| USB de extremo a extremo (UEFI) | 20 OK: Vault, descarga real, importación, ajustes, instalar, persistencia |
| Escenario Wi-Fi | 14 OK: casado de drivers con INF reales de virtio-win y Wi-Fi con radios simuladas (`mac80211_hwsim`) y un punto de acceso virtual (`hostapd` + `dnsmasq`): búsqueda, contraseña incorrecta rechazada sin dejar perfil, conexión con IP por DHCP, perfil 0600 de root, contraseña ausente de todas las líneas de órdenes, desconexión |
| Escenario Debian | 12 OK: descarga con la firma GPG del índice comprobada, preseed con contraseña cifrada y sin particionado, arranque de una sola vez (orden UEFI cambiado y restaurado al volver), carpeta de trabajo borrada |
| Escenario Windows | 8 OK: 7z de drivers con solo lo que casa, `autounattend.xml` con las rutas de drivers y sin disco, `injection` y `auto_install` con `autosel` y atajo de Windows 11 en `ventoy.json`, y todo limpio al volver |
| Preseed de Debian, a ojo | Ventoy entrega la plantilla sin preguntar (`autosel`) y el instalador de Debian arranca en español, salta idioma, país, teclado, red, cuenta y zona horaria y se detiene en "Particionado de discos" |
| Rebootera Desktop en Windows | Sin USB crea la biblioteca `%USERPROFILE%\Rebootera`; descargar Alpine (catálogo) funciona y se verifica; "Crear USB" muestra primero la fuente de la imagen (descargar la última o elegir una ISO) |
| VirtualBox 7.2 con la ISO definitiva | La interfaz aparece con VMSVGA (EFI) y VBoxVGA (BIOS); antes se quedaba en negro |
| Instalador de Windows real | **No probado** (no hay ISO de Windows en el entorno) |
| Workflows de GitHub y landing | YAML válido y página renderizada en Firefox; **no se han podido ejecutar** desde aquí |
| Hardware real, "Crear USB" con un pendrive real | **No probado** |

### Rendimiento (VM de 1 GB, 2 CPU)

| Medida | Antes | Ahora |
| --- | --- | --- |
| CPU del proceso web en reposo | 90 % | 4 % |
| CPU de la ventana del kiosco en reposo | 47 % | 2 % |
| CPU de `cage` en reposo | 28 % | 1 % |
| Arranque de la ISO de depuración hasta la consola | 117 s | 20 s |
| `systemd-analyze` (kernel + espacio de usuario) | 49 s + 57 s | 2 s + 10 s |
| Test de arranque completo (`vmtest.py`) | unos 3 min | 59 s |
| Todas las pruebas | más de 60 min | unos 10 min |
| Memoria de WSL durante una compilación | 15 GB | tope de 6 GB, 1,3 GB en reposo |

### Errores encontrados y corregidos

| Error | Causa | Arreglo |
| --- | --- | --- |
| Pantalla negra en VirtualBox | Mesa elegía `vmwgfx` sin 3D, EGL fallaba y `cage` abortaba y se quedaba colgado | `start-kiosk`: tres niveles de renderizado |
| La interfaz gastaba un núcleo entero en reposo en una VM | Sin GPU, tres animaciones infinitas obligaban a componer 60 fotogramas por segundo por software; WebKit no avisa de que dibuja por software | `start-kiosk` avisa (`REBOOTERA_LITE`) y la interfaz entra en modo ligero |
| Arranques de VM de dos minutos | La consola serie emulada a 115200 baudios volcaba cientos de KB de mensajes de arranque de la imagen de depuración | La imagen de depuración arranca en silencio (`quiet`) |
| Registro del kernel vacío en la interfaz desde la v0.1 | Core llamaba a `journalctl --kernel` (no existe; es `--dmesg`) | Corregido en el agente |
| `ventoy_grub.cfg` no sirve para el "arranque único" | Solo se carga con F6 | Cambio de `BootOrder` y `BootNext` en UEFI, restaurado al volver |
| Ventoy preguntaba qué plantilla usar | Falta `autosel` en `auto_install` | `"autosel": 1` |
| Wi-Fi: el primer escaneo salía vacío | La radio recién encendida aún no ve redes | El agente reintenta hasta tres veces |
| Pruebas con 6 VM a la vez llenaban 15 GB | 4 GB por VM y caché de disco sin límite | 1,5 GB por VM, tres a la vez, tope de WSL y caché liberada al terminar |

## v0.4.0 (2026-10-17)

Batería completa (`scripts/test-all.sh`, 880 s en total con la compilación de las dos ISO): **0 fallos de código**.

| Prueba | Resultado |
| --- | --- |
| Go: `go vet` y `go test ./...` | OK, incluido `TestISOSignatures` con las ISO reales de Arch 2026.10.01 y Alpine 3.24.2 (aceptadas) y un archivo ajeno (rechazado) |
| ISO de depuración, UEFI + Secure Boot | 17 OK |
| ISO de depuración, BIOS | 16 OK |
| ISO definitiva, UEFI + Secure Boot y BIOS | 2 OK en cada una |
| USB de extremo a extremo (`vmtest-usb.py`) | 20 OK |
| Escenario Wi-Fi (incluye drivers y **teclado**) | 20 OK; las 6 del teclado: lista cerrada, rechazo de `es;reboot`, el kiosco se reinicia, cage arranca con `XKB_DEFAULT_LAYOUT=de`, interfaz de nuevo abierta, elección guardada en el Vault |
| Escenario Windows | 8 OK |
| Escenario Debian | 10 OK y 2 fallos en la primera pasada (la comprobación de `BootCurrent` del arranque único salió vacía); repetido solo, **todo OK**: intermitente, anotado |
| Arranque de la ISO definitiva final (UEFI + Secure Boot) tras el último cambio de catálogo | OK |
| Ubuntu 24.04.5.1 desde el menú del USB (UEFI, `compat-matrix.sh`) | Arranca el escritorio en vivo y abre el instalador; el catálogo pasa a `verified` |

### Incidencias

- Mientras se descargaba y preparaba la ISO de Ubuntu 24.04 (6,2 GB) WSL dejó de responder (`Wsl/Service/0x8007274c`); tras `wsl --shutdown` todo estaba intacto y la prueba se repitió con éxito.
- El modo `--quick` de `test-all.sh` solo recompila la ISO de depuración: tras un cambio del catálogo hay que ejecutar `scripts/build.sh` para la definitiva.

### No probado en la v0.4.0

Los instaladores reales de Windows (inyección de drivers), el arranque único en un firmware real, "Crear USB" con un pendrive real y la ejecución de los workflows de GitHub (`build.yml`, `pages.yml`, `test.yml`). Ver [LIMITACIONES.md](LIMITACIONES.md).

## v0.5.0 (2026-10-17)

Batería completa (`scripts/test-all.sh`, 862 s con la compilación de las dos ISO): **0 fallos** (dbg UEFI+SB 17, dbg BIOS 16, definitiva UEFI+SB 2 y BIOS 2, USB de extremo a extremo 20, Wi-Fi/drivers/teclado 20, Windows 8, Debian 12). El fallo intermitente de la 0.4.0 en el arranque único de Debian no se repitió.

Crear USB / actualizar (Rebootera Desktop, Windows 11), sin tocar ningún disco real:

| Prueba | Resultado |
| --- | --- |
| Tests de Go de `internal/desktop` en Linux y como binario nativo de Windows (validación, copia, actualización del sistema, drivers, diálogo de selección múltiple) | OK |
| `scripts/test-desktop-windows.ps1`: el `.exe` real contra un USB falso (carpeta temporal) | ISO de 40 MB copiada idéntica por hash, drivers copiados, sin `.part` sueltos, Rebootera OS intacto, "nada que hacer" y quitar con `..` rechazados |
| Capturas con `PrintWindow` (solo la ventana) del Resumen, Crear USB y Asistente de drivers con un USB falso montado con `subst` | Correctas tras arreglar una clave sin traducir (`nav.create`) |

### Errores encontrados y corregidos

- Al elegir una carpeta de drivers se copiaba todo lo que hubiera dentro (en la prueba, hasta una ISO): ahora se omiten `.exe`, `.msi`, comprimidos e imágenes de disco.
- El botón "Crear USB" del Resumen mostraba la clave `nav.create` sin traducir.

### No probado en la v0.5.0

Los diálogos nativos de elegir imágenes y carpetas (hay que pulsarlos con el ratón; solo se probó el análisis de su resultado), "Crear USB" borrando un pendrive real, instalar dentro de un USB ajeno (Rufus), un instalador real de Windows y la ejecución de los workflows de GitHub.

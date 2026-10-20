# Pruebas

Cómo se prueba y qué resultados se han obtenido. Las pruebas se repiten con `scripts/test/vmtest.py` (ver README).

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

ISO definitiva `rebootera-os-0.2.0-amd64.iso` y la misma construcción con consola serie (`--debug`) para las pruebas automáticas. Rebootera Desktop se recompila con `scripts/build/build-desktop.sh`, que ejecuta `go test ./...` (todo pasa).

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

Con `VTOY_SECURE_BOOT_POLICY=1` Ventoy no deja arrancar Linux con Secure Boot. Se probó la `0` con UEFI + Secure Boot recorriendo MokManager a mano con `scripts/test/vmtest-interactive.py` (Enroll key from disk, VTOYEFI, `ENROLL_THIS_KEY_IN_MOKMANAGER.cer`, Yes, Reboot): tras inscribir la clave, Rebootera OS arranca hasta la interfaz en menos de 90 s con "Secure Boot: Active". **Decisión: política 0**, con aviso en la interfaz, el README y LIMITACIONES.md de que, con la clave inscrita, el equipo también arranca imágenes sin firma comprobada.

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

Todo se ejecuta con `scripts/test/test-all.sh` (compila las dos ISO y lanza las pruebas en paralelo, tres VM de 1,5 GB a la vez). Con las ISO ya compiladas tarda unos 10 minutos (antes, más de una hora una prueba detrás de otra); sin opciones (modo rápido: arranque de las dos ISO en UEFI + Secure Boot y recompilación solo si cambió el código) tarda menos de 3 minutos; `--full` lanza todo (unos 17 minutos) y es lo que se ejecuta antes de publicar. Última ejecución completa: **93 comprobaciones, 0 fallos**.

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

Batería completa (`scripts/test/test-all.sh`, 880 s en total con la compilación de las dos ISO): **0 fallos de código**.

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
- El modo `--quick` de `test-all.sh` solo recompila la ISO de depuración: tras un cambio del catálogo hay que ejecutar `scripts/build/build.sh` para la definitiva.

### No probado en la v0.4.0

Los instaladores reales de Windows (inyección de drivers), el arranque único en un firmware real, "Crear USB" con un pendrive real y la ejecución de los workflows de GitHub (`build.yml`, `pages.yml`, `test.yml`). Ver [LIMITACIONES.md](LIMITACIONES.md).

## v0.5.0 (2026-10-17)

Batería completa (`scripts/test/test-all.sh`, 862 s con la compilación de las dos ISO): **0 fallos** (dbg UEFI+SB 17, dbg BIOS 16, definitiva UEFI+SB 2 y BIOS 2, USB de extremo a extremo 20, Wi-Fi/drivers/teclado 20, Windows 8, Debian 12). El fallo intermitente de la 0.4.0 en el arranque único de Debian no se repitió.

Crear USB / actualizar (Rebootera Desktop, Windows 11), sin tocar ningún disco real:

| Prueba | Resultado |
| --- | --- |
| Tests de Go de `internal/desktop` en Linux y como binario nativo de Windows (validación, copia, actualización del sistema, drivers, diálogo de selección múltiple) | OK |
| `scripts/test/test-desktop-windows.ps1`: el `.exe` real contra un USB falso (carpeta temporal) | ISO de 40 MB copiada idéntica por hash, drivers copiados, sin `.part` sueltos, Rebootera OS intacto, "nada que hacer" y quitar con `..` rechazados |
| Capturas con `PrintWindow` (solo la ventana) del Resumen, Crear USB y Asistente de drivers con un USB falso montado con `subst` | Correctas tras arreglar una clave sin traducir (`nav.create`) |

### Errores encontrados y corregidos

- Al elegir una carpeta de drivers se copiaba todo lo que hubiera dentro (en la prueba, hasta una ISO): ahora se omiten `.exe`, `.msi`, comprimidos e imágenes de disco.
- El botón "Crear USB" del Resumen mostraba la clave `nav.create` sin traducir.

### No probado en la v0.5.0

Los diálogos nativos de elegir imágenes y carpetas (hay que pulsarlos con el ratón; solo se probó el análisis de su resultado), "Crear USB" borrando un pendrive real, instalar dentro de un USB ajeno (Rufus), un instalador real de Windows y la ejecución de los workflows de GitHub.

## v0.15.0 (2026-10-20)

| Prueba | Resultado |
| --- | --- |
| `scripts/test/test-create-linux.sh`: Crear USB en Linux con la aplicación real sobre un disco virtual (loop) | 0 fallos: aparece en la lista (y el disco del sistema no), se rechaza sin confirmación y con un disco que no es USB, Ventoy instalado de verdad, GPT, exFAT «REBOOTERA», partición VTOYEFI, Rebootera OS idéntico byte a byte y estructura del USB |
| Compilación y `go vet` para Linux (x64 y arm64), Windows y macOS (Intel y Apple); `go test ./...` | sin avisos; pruebas nuevas de la lista de discos de Linux y macOS, del script de macOS y de los nombres de dispositivo aceptados |
| `test-all.sh` rápido con la ISO 0.15.0 (1263 MB, 61 MB menos) | depuración 19 OK, definitiva 2 OK, 0 fallos |
| `test-desktop-windows.ps1` y `test-installer-windows.ps1` con el exe 0.15.0 real | 0 fallos |
| Interfaz empaquetada con esbuild (un JS y un CSS) | compila, `tsc` limpio, arranca en WebKitGTK |

### No probado en la v0.15.0

- **Crear USB en macOS**: la lectura de `diskutil`, el reparto y el script están probados con datos de ejemplo, pero no se ha ejecutado en un Mac ni se ha arrancado un USB hecho así.
- **Crear USB en Linux sobre un pendrive físico** y con el diálogo real de polkit (se probó como root, sin pkexec, sobre un disco virtual).
- **El rendimiento** del arranque desde un pendrive real tras recortar la imagen y empaquetar la interfaz.

## v0.14.0 (2026-10-20)

| Prueba | Resultado |
| --- | --- |
| `test-all.sh` en modo rápido con la ISO 0.14.0 recién compilada (arranque en UEFI + Secure Boot) | depuración 19 OK, definitiva 2 OK (con el apagado desde la interfaz), 0 fallos, 393 s incluyendo la compilación de las dos ISO |
| `go vet` y `go test ./...` | sin avisos; pruebas nuevas de la caché de tamaños, la partición «DRIVERS», los colores del menú de Ventoy y la página principal con el tema guardado |
| `tsc`, `prettier`, auditoría de textos y de reglas CSS sin uso | limpio (se quitaron unas 70 reglas de CSS) |
| `scripts/test/test-desktop-windows.ps1` y `test-installer-windows.ps1` con el exe 0.14.0 real (sin ventana) | ISO y drivers copiados byte a byte, sistema intacto; ISO del instalador con El Torito BIOS + UEFI, 0 fallos |
| Interfaz (Inicio, Disks con datos de ejemplo, Este equipo, Ajustes con Wi-Fi primero, Descargar) | dibujada en WebKitGTK sin ventana y revisada a mano; capturas de `docs/images` rehechas |

### Errores encontrados y corregidos

| Error | Causa | Arreglo |
| --- | --- | --- |
| La prueba de la ISO definitiva no apagaba desde la interfaz | los clics de `vmtest.py` apuntaban a la cabecera antigua | coordenadas nuevas (incluida la pestaña «Este equipo») |

### No probado en la v0.14.0

- **El rendimiento en un equipo real con un pendrive lento**: los cambios (menos sondeos y redibujados, fondo quieto, lectura anticipada, servicios apagados) están hechos y las VM arrancan, pero no se midió en hardware.
- **El panel táctil I2C/HID real**, y **la búsqueda de drivers de la tarjeta de red en los discos de un equipo real**.
- **El borrado y las particiones con el botón «Continuar» en un disco real** (se probó la ruta de la API, no el clic).
- **El menú de Ventoy con los colores del tema** y su espera de 1 s en un USB real, y **el navegador integrado en modo oscuro** (se comprueba la política, no la ventana).

## v0.12.0 (2026-10-20)

| Prueba | Resultado |
| --- | --- |
| Batería completa de VM (`test-all.sh`: ISO de depuración y definitiva en UEFI + Secure Boot y BIOS, USB de extremo a extremo, Wi-Fi/teclado/puntero, Debian con preseed, adopt, particiones y Windows) | 0 fallos: depuración 18 y 19, definitiva 2 y 2 (con el apagado desde la interfaz), USB 20, adopt 6, Debian 12, particiones 23, Wi-Fi y puntero 25, Windows 8 |
| Arranque directo de la ISO (GRUB sin menú ni espera) | arranca hasta la interfaz en las cuatro pruebas de arranque |
| Reinicio de la sesión al cambiar el teclado o el puntero | el compositor antiguo muere y el nuevo arranca con el teclado elegido (antes sobrevivía) |
| `LoadInput` sin archivo guardado devuelve los valores por defecto | pruebas Go |
| Catálogo contra las fuentes reales (35 entradas nuevas: placas, Ubuntu 22.04, Debian 12, Fedora 43, LMDE 7, AlmaLinux 9, Windows 10 LTSC en inglés) | todas resuelven, con firma OpenPGP donde el proyecto la publica |
| Resolvedor "latest-sum" y nombre y foto de las imágenes | pruebas Go |
| `go vet`, `go test ./...`, `staticcheck`, `tsc`, `knip` | sin avisos |
| `scripts/test/test-desktop-windows.ps1` y `test-installer-windows.ps1` con el exe real (sin ventana) | ISO y drivers copiados byte a byte, sistema intacto; ISO del instalador con El Torito BIOS + UEFI |
| Navegación en cabecera con pestañas (sin barra lateral) y selector de tema: pulsar Amoled abre la lista y aplica el tema | comprobado pulsando en una ventana de prueba; el exe real sirve la cabecera nueva y no la barra lateral |
| Icono del exe | el grupo de iconos 2 del ejecutable es el amanecer nuevo (comprobado extrayéndolo) |

### Errores encontrados y corregidos

| Error | Causa | Arreglo |
| --- | --- | --- |
| Pantalla negra con el ratón y pantallazos en equipos reales | `LoadInput` devolvía ajustes vacíos si no había archivo guardado (se aplicaban en cada arranque y reiniciaban el kiosco); y labwc sobrevivía al reinicio de la sesión y se quedaba con la pantalla | valores por defecto al leer; `ExecStopPost` y el agente matan lo que quede del usuario del kiosco |
| Los clics de las pruebas apuntaban al menú lateral viejo | maquetación nueva | coordenadas nuevas en `vmtest.py` |

### No probado en la v0.12.0

- **El arreglo de la pantalla negra en tu equipo real**: se reprodujo y se comprobó en la VM, pero no en hardware real.
- **La intro, el icono y el menú en la ventana real del exe** (las pruebas son sin ventana) y **la ventana propia en macOS**.
- **El panel táctil real**, y **las imágenes de placas escritas en una tarjeta SD** (solo se comprueba la descarga y su suma).

## v0.11.0 (2026-10-19)

| Prueba | Resultado |
| --- | --- |
| Batería completa de VM (`test-all.sh`: ISO de depuración y definitiva en UEFI + Secure Boot y BIOS, USB de extremo a extremo, Wi-Fi, Debian con preseed, adopt, particiones y Windows) | USB 20 OK, adopt 6, Debian 12, particiones 23, Windows 8 y depuración 16 y 17, todo sin fallos; la definitiva falló dos veces (ver errores) |
| ISO definitiva tras corregir: UEFI + Secure Boot y BIOS | 2 de 2 en cada una, con el apagado desde la interfaz (clics reales en la barra superior) |
| Depuración con las comprobaciones nuevas: navegador (las dos extensiones en la imagen y política JSON válida) y toque para clic activado en la configuración de labwc | 20 OK, 0 fallos |
| `go vet`, `go test ./...`, `staticcheck` (U1000, SA, S1) y `deadcode` | sin avisos; se quitaron las funciones sin uso |
| Compilación cruzada del escritorio: Windows, Linux (x64 y arm64) y macOS (Intel y Apple) | las cinco compilan y pasan `go vet` |
| Escritorio de Linux en WSL (sin ventana): servidor local, unidades montadas y mensaje claro al pedir crear el USB | correcto |
| `scripts/test/test-desktop-windows.ps1` y `test-installer-windows.ps1` con el exe real (sin ventana) | ISO y drivers copiados byte a byte, sistema intacto, peticiones malas rechazadas; ISO del instalador con El Torito BIOS + UEFI, 7 de 7 |
| `tsc`, `knip` (sin archivos ni exportaciones sin uso), auditoría de textos y de reglas CSS sin uso | limpio |
| Interfaz (tres temas, Inicio, Descargar con el selector de versión, Crear USB, Drivers, Discos, Ajustes) | dibujada en WebKitGTK sin ventana y revisada a mano |

### Errores encontrados y corregidos

| Error | Causa | Arreglo |
| --- | --- | --- |
| La ventana del kiosco salía de 200 x 200 px en la ISO definitiva | una regla de labwc alternaba la pantalla completa y el propio shell ya la pedía, así que se anulaban | se quitó la regla |
| El apagado desde la interfaz fallaba en la prueba | los clics de la prueba (menú lateral y barra superior) apuntaban a la maquetación antigua | coordenadas nuevas en `vmtest.py` |
| Los scripts de prueba de Windows no encontraban el exe | los scripts se movieron de carpeta | rutas corregidas |

### No probado en la v0.11.0

- **Ocultar la dirección abajo a la izquierda en WebView2**: el código se ejecuta al abrir la ventana y las pruebas van sin ventana, así que no se vio en pantalla.
- **Rebootera Desktop en macOS** (solo se comprobó que compila y enlaza) y **la ventana propia de Linux con un navegador Chromium** (el binario se ejecutó sin ventana).
- **El panel táctil de un portátil real**: se comprobó que el toque para clic sale activado en la configuración, no el comportamiento del hardware.
- **Firefox con las extensiones abriéndose dentro de la sesión**, y la sensibilidad del puntero con un ratón real.
- **Que los drivers de un portátil real se detecten en su partición de fábrica** (probado con datos de ejemplo).
- Los enlaces de descarga a ISO propias (probado con servidor falso).

## v0.10.0 (2026-10-19)

| Prueba | Resultado |
| --- | --- |
| Batería completa de VM (`test-all.sh`: ISO de depuración y definitiva en UEFI + Secure Boot y BIOS, USB de extremo a extremo, Wi-Fi, Debian con preseed, adopt, particiones y Windows) | 0 fallos; el escenario de Windows falló una vez en "el equipo se reinicia" (intermitente) y pasó 8 de 8 al repetirlo solo |
| Resolución real del catálogo (65 + 4 entradas, `go test -tags online ./internal/catalog/`) | todas resuelven salvo fallos puntuales de espejos (Fedora/Tumbleweed) y un 500 puntual del Archive al hacer HEAD; el enlace de Fedora con ruta absoluta de otro espejo se arregló |
| `resolveArchive` (servidor falso), SHA-1 del Archive correcto y erróneo en el gestor de transferencias | pruebas Go |
| `autounattend.xml` con reducción de telemetría: válido como XML, órdenes en specialize, sin tocar idioma ni discos | pruebas Go |
| Interfaz (tres temas, Inicio, Descargar con logos y aviso del USB, Discos con datos de ejemplo, Ajustes, Instalar, Mis imágenes) | dibujada en WebKitGTK sin ventana (`scripts/dev/preview-ui.sh`, con `PREVIEW_THEME` y `PREVIEW_FIXTURE`) y revisada a mano |
| `scripts/test/test-installer-windows.ps1` con el exe real (IMAPI, sin ventana) | 7 de 7 |

### No probado en la v0.10.0

- **La reducción de telemetría en un Windows real instalado** (solo se comprobó el XML y que no toca idioma ni discos).
- **El arranque del USB MBR en un PC real** (solo se creó y arrancó en VM con la ruta de Linux; la de Windows con Ventoy `VTOYCLI` sin `/GPT` no se pudo ejecutar sin tocar un disco real).
- **El consumo de CPU de la interfaz nueva en Rebootera Desktop** (el fondo animado vuelve a estar activo en el escritorio).
- Que las imágenes nuevas (Archive, NixOS, Void, CentOS, sabores) arranquen desde el menú de Ventoy.

## v0.7.0 (2026-10-18)

| Prueba | Resultado |
| --- | --- |
| Resolución real de las 40 entradas del catálogo (`go test -tags online ./internal/catalog/`) | 37 resuelven y responden a HEAD, 2 son manuales (Windows 11 y 10 de Microsoft, solo desde su web); Tumbleweed falla a ratos porque el espejo elegido por `download.opensuse.org` aún no tiene la instantánea nueva (un reintento elige otro espejo) |
| Firmas OpenPGP de Mint, Kali y AlmaLinux (claves fijadas) y de Ubuntu/Debian/Fedora | verificadas con las listas de sumas reales |
| Descarga con varias conexiones contra un servidor local que limita cada conexión | 4 conexiones claramente más rápidas que 1; reanuda con el mapa de bloques sin repetir bloques; hash malo se rechaza; servidor sin Range cae a una conexión; con `-race` |
| Límite de velocidad y ajustes | cubeta de fichas con horas inventadas; los ajustes se guardan en el Vault y se validan |
| Instalador en carpetas a ISO (Linux, `genisoimage`) | firma `CD001`, catálogo El Torito con plataformas 0x00 (BIOS) y 0xEF (UEFI), etiqueta del volumen; cancelación |
| Instalador en carpetas a ISO (Windows 11, IMAPI, ejecutando el binario de pruebas compilado, sin ventana) | mismas comprobaciones: `CD001`, El Torito BIOS + EFI |
| Trabajo `iso` de la cola, registro en la biblioteca como "sin comprobar" y rutas de la API (buscar, inspeccionar, crear; rechaza carpetas que no son instaladores y rutas que salen del medio) | pruebas Go |
| `release.FetchTo` (descarga directa al USB, reanudar, parcial corrupto) | pruebas Go |
| Interfaz (Centro de descargas y Biblioteca) | dibujada en WebKitGTK sin ventana (`scripts/dev/preview-ui.sh`) y revisada a mano |

### No probado en la v0.7.0

- **Arrancar de verdad** cada imagen nueva desde el menú de Ventoy: todas figuran como "sin probar" (solo Rebootera OS, Debian y alguna más se probaron en versiones anteriores).
- **Que un instalador de Windows hecho por `Make ISO` arranque en un equipo real o en una VM con Windows de verdad**: solo se comprobó la estructura (ISO 9660 + El Torito con BIOS y EFI + UDF), no el arranque, porque en el entorno no hay un Windows extraído para probar.
- **Descargas de varios GB por varias conexiones** contra los servidores reales de cada proyecto: la prueba de velocidad es contra un servidor local.

## v0.6.0 (2026-10-18)

Batería completa (`scripts/test/test-all.sh`, unos 15 a 20 minutos con la compilación de las dos ISO): dbg UEFI+SB 17 OK, dbg BIOS 16, definitiva UEFI+SB 2 y BIOS 2, USB de extremo a extremo 20, Wi-Fi/drivers/teclado 20, Windows 8, Debian 12 y los dos escenarios nuevos:

| Prueba | Resultado |
| --- | --- |
| **Gestor de particiones, plan** (`internal/partman`, Go) | Tablas de números de cada operación y una simulación que ejecuta los pasos sobre un disco modelado y falla si algún paso pisa datos vivos, deja una partición más pequeña que su sistema de archivos o escribe en la tabla un inicio distinto de donde están los datos |
| **Gestor de particiones, ejecución real** (`REBOOTERA_LOOP_TESTS=1 go test ./cmd/rebootera-agent -run TestLoop`, como root en WSL) | Sobre discos loop reales con ext4 y NTFS con datos: mover el límite en las dos direcciones, encoger y agrandar un NTFS, fusionar conservando la derecha y la izquierda, crear exFAT y borrar. Todos los archivos intactos por hash, `e2fsck` y `ntfsresize --check` limpios y tablas exactas |
| **Gestor de particiones en la VM** (escenario `partitions`: Core, agente y API sobre un disco virtual) | 23 OK: el USB de arranque sale bloqueado y no se puede planificar nada sobre él, la vista previa da los pasos, sin la confirmación escrita no se aplica, mover el límite en las dos direcciones, encoger el NTFS por el final, crear una exFAT y borrarla, con los datos y los sistemas de archivos intactos tras cada paso |
| **Añadir Rebootera sin borrar** (escenario `adopt`: imagen tipo Rufus con NTFS lleno + partición UEFI:NTFS, Ventoy `-n`) | 6 OK: arranca en UEFI, el Vault es el NTFS (`ntfs3`, noexec, nosuid, nodev), la API lo ve, los archivos del usuario siguen intactos por hash, Rebootera crea sus carpetas al lado y Core puede escribir |
| Rebootera Desktop en Windows 11: **ventanas de PowerShell** (`scripts/test/test-no-popups.ps1`, sondeo cada 10 ms) | 0.5.0: **2 ventanas** nuevas; 0.6.0: **0** |
| Rebootera Desktop en Windows 11: **rendimiento en reposo** (app + WebView2 propios, 20 s) | 0.5.0: 39 % de un núcleo; 0.6.0: **1,5 %**; memoria unos 380 MB en 7 procesos |
| Copias en tubería (`internal/fastcopy`, con `-race`) | Integridad de varios tamaños, errores de lectura y escritura distinguidos y cancelación aunque el origen no responda |

### Errores encontrados y corregidos

| Error | Causa | Arreglo |
| --- | --- | --- |
| Escenario de Windows: "windows 11 bypass only applies to Windows images" | La caché del listado de la biblioteca (3 s) servía una lista vieja a la acción de instalar | `Library.ListFresh()` para las acciones; la caché solo para el listado de la interfaz |
| `ntfsresize` se negaba a tocar un NTFS que acababa de redimensionar | Deja el volumen marcado para que Windows lo revise | El agente fuerza solo en volúmenes que él mismo redimensionó en esta sesión; uno sucio por Windows se sigue rechazando |
| La imagen tipo Rufus no se creaba | `parted -s resizepart` se cancela solo al encoger | `sfdisk` |
| Un disco sin tabla (la ISO) se dibujaba como "espacio libre" | `partman.Build` creaba un hueco | Sin tabla conocida no hay segmentos |
| La prueba de la ISO definitiva en BIOS fallaba al apagar | Medía la pantalla (800x600) antes de que la interfaz cambiara de modo y el clic caía en otro botón | Espera a que la pantalla pase a más de 1024 px de ancho |

### No probado en la v0.6.0

El camino de Windows de "añadir Rebootera sin borrar" (`VTOYCLI /I /NonDest`) con una unidad real (se probó el de Linux con la misma herramienta sobre una imagen), "Crear USB" borrando un pendrive real, el gestor de particiones sobre un disco físico real (solo discos virtuales), los diálogos nativos de elegir imágenes y carpetas con el ratón, un instalador real de Windows y la ejecución de los workflows de GitHub.


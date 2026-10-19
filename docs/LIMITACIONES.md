# Limitaciones y errores conocidos

Estado a 2026-10-19, versión 0.10.0.

## Lo que todavía no existe

- **Internet Archive solo con software libre.** El Archive publica SHA-1 (no SHA-256): se comprueba que la copia llegó entera, pero no que sea auténtica. No se ofrecen ISO de Windows de terceros del Archive (copias sin licencia o modificadas, como Tiny11): para Windows 11 LTSC se usa la imagen de evaluación oficial de Microsoft (ya directa en el catálogo), y la telemetría se reduce con la opción de Instalar (o con Rufus).
- **Reducir la telemetría** se hace con órdenes en la fase specialize del autounattend.xml; se comprobó que el XML es válido, **no** en un Windows real instalado. En Home y Pro Windows no admite el nivel 0 y lo trata como básico.

- **Imágenes del catálogo sin probar el arranque:** la mayoría de las 65 imágenes figuran como "sin probar" desde el USB de Rebootera. Las evaluaciones de Microsoft no publican suma (se calcula al bajarlas, pero no hay con qué compararla). Tumbleweed puede dar un 404 si el espejo que le toca aún no tiene la instantánea: reintentar elige otro.
- **Instalador en carpetas a ISO (Rufus):** solo se reconocen instaladores de **Windows** (bootmgr + sources\boot.wim + install.wim/esd/swm). No se comprobó que la ISO resultante arranque en un equipo real; se comprobó su estructura. En Windows se usa IMAPI mediante PowerShell: si una directiva de grupo impide PowerShell, no funciona.

- **Drivers de Windows en un instalador real**: se leen los INF, se casan con el hardware, se empaquetan en un `.7z`, se generan `autounattend.xml` y las entradas `injection` y `auto_install` de Ventoy, y todo se comprueba en VM con INF reales de virtio-win (ver PRUEBAS.md). Ventoy extrae el archivo en `X:\` (se comprobó en el código de su ayudante de WinPE, `vtoyjump64.exe`: `7za.exe x -y -aoa -oX:\`), pero **no** se ha podido comprobar que el instalador de Windows cargue esos drivers: no hay una ISO de Windows en el entorno de pruebas (Microsoft no da una descarga directa sin sesión) y no se sabe si WinPE lee las rutas de drivers del `autounattend.xml` antes o después de que Ventoy extraiga el archivo. Si un dispositivo crítico sigue sin disco o red en el instalador, queda el método de siempre: "Cargar controlador" y examinar `X:\rebootera-drivers` (o la carpeta `drivers/` del USB).
- **Los drivers los pones tú**: Rebootera no descarga drivers de los fabricantes (Dell, HP, Lenovo, Intel...); casa los que copies en `drivers/`. Del catálogo `.cat` se comprueba que exista y sea un PKCS#7 firmado, pero **no** se verifica la cadena de certificados (eso lo hace Windows al cargar el driver). No se ejecuta nunca un `.exe`.
- **Respuestas automáticas solo en Windows y Debian**: idioma, teclado, zona horaria y cuenta. Nunca incluyen el disco: el instalador siempre pregunta dónde instalar (comprobado en VM con Debian: salta idioma, país, teclado, red, nombre del equipo, cuenta y zona horaria, y se detiene en "Particionado de discos"). Ubuntu, Fedora, Arch y Alpine no están soportados. La contraseña de la cuenta se guarda en el USB (codificada en Windows, con hash SHA-512 en Debian) hasta que Rebootera vuelve a arrancar y entonces se borra; si el equipo no vuelve a arrancar Rebootera, el archivo sigue en `ventoy/rebootera/`.
- **Arranque de una sola vez solo en UEFI y opcional**: cambia `BootOrder` y `BootNext` en la NVRAM del firmware (el USB pasa a ser el último) y los restaura al volver a Rebootera. Probado en OVMF; **no** se ha probado en firmware real, donde algunas placas ignoran o reescriben esas variables. En BIOS (heredado) no existe: si el equipo vuelve a arrancar desde el USB tras el primer reinicio del instalador, el menú de Ventoy lo volverá a lanzar (quita el USB o elige el disco en el menú de arranque del equipo). Si no vuelves a arrancar Rebootera, el orden de arranque queda con el USB el último.
- **Firmas GPG en todas las distribuciones del catálogo salvo Windows**: en Ubuntu, Debian y Fedora se verifica la firma OpenPGP de la lista de sumas antes de descargar; en Arch Linux y Alpine, que firman cada ISO, se verifica la firma de la imagen ya descargada (y se borra si no coincide). Las claves están fijadas en el programa (huellas comprobadas contra las páginas de los propios proyectos). Si un proyecto rota su clave, hay que actualizar `internal/catalog/keys/`. Windows no se descarga desde Rebootera.
- **Wi-Fi**: WPA2, WPA3 y redes abiertas (también ocultas, suponiendo WPA2 si se escribe contraseña). No se admiten WPA-Enterprise (802.1X), WEP ni OWE, ni portales cautivos. La contraseña no se guarda: se olvida al apagar. Probado con radios simuladas (`mac80211_hwsim`) y un punto de acceso virtual, no con adaptadores ni redes reales.
- **Descarga automática de Rebootera OS y compilación continua**: Rebootera Desktop descarga la última ISO de `github.com/Jondals/Rebootera-OS-downloads/releases/download/latest/`, que publica `.github/workflows/build.yml` en cada push a `master`. El flujo está probado con un servidor HTTPS local (descarga, SHA-256 correcto e incorrecto, sin release, solo HTTPS), pero **los workflows de GitHub no se han podido ejecutar desde aquí**: la primera ejecución real será el primer push, y hasta entonces "descargar la última versión" responde que todavía no hay una versión publicada (la alternativa es elegir una ISO del PC). La landing requiere activar una vez **Settings → Pages → Source: GitHub Actions**.
- **Rebootera Desktop en Linux**: abre la interfaz en el navegador y gestiona un USB ya montado, pero "Crear USB" solo existe en Windows (en Linux está `scripts/make-usb.sh`). Los diálogos de elegir imágenes y carpetas son de Windows, así que en Linux no se pueden añadir imágenes ni drivers desde la aplicación: se copian a mano a `isos/` y `drivers/`.
- **Añadir Rebootera a un USB con datos (por ejemplo hecho con Rufus)** usa la instalación no destructiva de Ventoy y solo se ofrece si su primera partición es NTFS o exFAT con datos (FAT32 no admite una ISO de Windows de más de 4 GB). Se ha probado de extremo a extremo con una imagen de disco tipo Rufus (NTFS con datos y partición UEFI:NTFS): los archivos siguen intactos y Rebootera arranca y usa ese NTFS como Vault. **El camino de Windows (`VTOYCLI /I /NonDest`) no se ha probado con una unidad real.** El arranque propio del USB (el instalador de Rufus) se sustituye por el menú de Rebootera: lo que se conserva son los archivos y las ISO que ya hubiera. Haz antes una copia de lo importante.
- **Al añadir una carpeta de drivers** solo se copian archivos de drivers (se omiten `.exe`, `.msi`, archivos comprimidos e imágenes de disco) y hay que tener cada paquete con su `.inf` x64; los drivers que vengan dentro de un instalador `.exe` hay que extraerlos antes. Las imágenes añadidas se copian sin releerlas del USB (se comprueba el tamaño); Rebootera OS sí se relee y se compara por hash.
- **Gestor de particiones** (solo en Rebootera OS): cambia el tamaño, mueve límites, fusiona, borra y crea. Solo ext4 y NTFS se pueden encoger y agrandar (exFAT y FAT32 no tienen herramienta para hacerlo sin perder datos). Mover el límite entre dos particiones copia los datos de la de la derecha: **si se corta la luz o se desconecta el disco a mitad, esa partición queda dañada** (la interfaz lo avisa y pide confirmación escrita). Un NTFS que Windows dejó sucio o hibernado se rechaza: hay que iniciar Windows y apagarlo del todo. Tras redimensionar un NTFS, Windows lo revisará solo en su siguiente arranque. No se editan tablas MBR con particiones extendidas ni se cambian particiones con LVM o RAID. En Rebootera Desktop (Windows) no hay gestor de particiones: Windows ya trae el suyo.
- La distribución del teclado de la interfaz se elige en Ajustes (español e inglés); por defecto es español y no cambia con el idioma de la interfaz. Cambiarla reinicia la pantalla unos segundos. Otras distribuciones no están en la lista cerrada del agente.
- Solo amd64. Equipos ARM (por ejemplo Snapdragon) no arrancan.

## Compatibilidad de imágenes con el menú de arranque

Comprobada en máquina virtual (UEFI) arrancando cada imagen desde un USB de Rebootera; el resultado se muestra en el Centro de descargas y el asistente no deja elegir las incompatibles:

| Imagen | Resultado |
| --- | --- |
| Debian 13.x netinst | Arranca: el instalador gráfico se abre |
| Ubuntu 26.04.1 Desktop | Arranca: escritorio en vivo e instalador |
| Fedora Workstation 44-1.7 | Arranca: escritorio en vivo e instalador |
| Arch Linux 2026.10.01 | Arranca solo en BIOS (heredado); en UEFI se queda en pantalla negra. En equipos solo UEFI, grábala sola en otro pendrive |
| Alpine 3.24 (standard y virt) | **No arranca** con Ventoy 1.1.18: en UEFI "cannot load image" y en BIOS "mount /dev/loop0 ... I/O error". Hay que grabarla sola en otro pendrive |
| Ubuntu 24.04.5.1 | Arranca (escritorio en vivo y arranque del instalador) en UEFI |
| Windows 11 | Sin probar todavía (no hay ISO en el entorno de pruebas) |

Las imágenes añadidas por el usuario no tienen compatibilidad conocida.

## Secure Boot

- Rebootera OS arranca con Secure Boot gracias al shim firmado de Debian cuando la ISO se graba directamente o se arranca desde DVD.
- Desde el USB de Rebootera, el menú lo pone Ventoy. La primera vez en cada equipo con Secure Boot hay que inscribir su clave en MokManager (pantalla azul): flujo probado de principio a fin en máquina virtual (UEFI con Secure Boot y claves de Microsoft).
- **Compromiso de seguridad:** Rebootera usa `VTOY_SECURE_BOOT_POLICY=0` porque con la política 1 Linux no arranca con Secure Boot. Una vez inscrita la clave de Ventoy, ese equipo también arranca imágenes sin firma comprobada. Inscríbela solo en equipos de confianza y bórrala con "Delete MOK" cuando no la necesites.

## Probado solo en máquina virtual

- Todas las pruebas de arranque son en QEMU/KVM con OVMF (UEFI, UEFI con Secure Boot y claves de Microsoft) y SeaBIOS. Falta probar en hardware real: GPUs NVIDIA, AMD e Intel recientes, equipos con Secure Boot que no confían en la CA de terceros de Microsoft y portátiles solo con Wi-Fi.
- El USB emulado de QEMU es muy lento: los tiempos de arranque desde el USB virtual (varios minutos) no representan un pendrive real.
- **"Crear USB" de Rebootera Desktop** no se ha probado con un pendrive real: se ha probado todo menos el paso que borra el disco (descarga y verificación de Ventoy, comprobaciones previas, rechazo de una confirmación incorrecta y de discos que no son USB). Las demás funciones de la aplicación se han probado en Windows 11 con una unidad simulada.
- **VirtualBox**: probado con VirtualBox 7.2 (en un equipo con Hyper-V activo, modo NEM) y VMSVGA (EFI y BIOS), VBoxSVGA (EFI) y VBoxVGA (BIOS): la interfaz arranca en los cuatro. VBoxSVGA con EFI se quedó una vez parado en el firmware de VirtualBox antes de leer la ISO (el registro de la VM termina en la carga de `PciHostBridgeDxe`), sin relación con Rebootera. Hyper-V y VMware no se han probado.

## Decisiones con contrapartida

- `WEBKIT_DISABLE_DMABUF_RENDERER=1`: la interfaz se dibuja con memoria compartida en lugar de DMA-BUF. Es más compatible con GPUs virtuales y drivers básicos, a cambio de algo de rendimiento.
- `WLR_RENDERER_ALLOW_SOFTWARE=1` y `start-kiosk`: sin aceleración gráfica, el compositor usa renderizado por software en lugar de negarse a arrancar. `start-kiosk` prueba tres niveles (hardware, Mesa por software, pixman con cairo) y puede tardar hasta 25 s por nivel en un equipo cuyo primer nivel falla; recuerda el que funciona mientras dure la sesión.
- Capacidades de la sesión kiosco: `pam_systemd` de systemd 257 concede `CAP_WAKE_ALARM` como capacidad ambiental a las sesiones de usuario, y el sandbox de WebKitGTK (`bubblewrap`) se niega a ejecutarse con cualquier capacidad. La unidad `rebootera-ui.service` la retira con `setpriv` antes de lanzar `cage`.
- `rebootera-agent.service` no usa `PrivateTmp` ni `ProtectHome`: crearían un espacio de montajes privado y los discos que monta el agente no serían visibles para Core.
- La imagen incluye todo el firmware no libre de Debian (`--firmware-chroot true`) para maximizar el hardware compatible. Eso explica buena parte de su tamaño.
- Las imágenes importadas desde otro disco se marcan como "importadas" y no se verifican solas: no hay una suma oficial con la que compararlas. El usuario puede calcular su SHA-256 desde la biblioteca.
- Si dos imágenes tienen el mismo nombre en carpetas distintas, el menú de Ventoy las muestra con el mismo nombre.

## Riesgos abiertos (detalle en PLAN.md)

- Ventoy se usa como binario publicado (verificado con su SHA-256), no compilado desde el código fuente.

# Registro de cambios

Todos los cambios importantes de Rebootera OS. Las fechas usan el formato AAAA-MM-DD.
Versión en inglés: [CHANGELOG.md](CHANGELOG.md).

## [0.8.0] - 2026-10-19

### Cambiado

- **Todos los menús rehechos para que sean simples.** Un aspecto tranquilo en todas partes: colores planos, letra normal en vez de las mayúsculas técnicas pequeñas, texto y botones más grandes y mucho menos en pantalla.
  - **Inicio** es una pregunta ("¿Qué quieres hacer?") y tarjetas grandes: Instalar un sistema, Descargar sistemas, Mis imágenes, Discos, Drivers (en Rebootera Desktop: tu USB de un vistazo y Crear USB, Descargar, Mis imágenes, Drivers). Procesador, discos y firmware pasaron a "Sobre este equipo", plegado; los widgets de temperaturas, red y registro del kernel ya no están en la portada.
  - **Barra superior**: solo avisa cuando algo falla (sin red, sin USB). Ya no están el reloj, el modo de arranque ni Secure Boot.
  - **Nombres del menú** claros: Inicio, Instalar, Mis imágenes, Drivers, Descargar, Discos.
  - **Descargar** enseña una tarjeta pequeña por sistema: nombre, una línea, un desplegable para elegir la versión (con su tamaño) y un botón Descargar. La verificación, la compatibilidad de arranque y el SHA-256 están en "Detalles". La barra inferior solo aparece mientras se descarga algo; la cola y el ancho de banda son botones de la cabecera.
  - **Instalar** tiene cuatro pasos (Sistema, Disco, Opciones, Instalar) en vez de seis; las comprobaciones del equipo y de los drivers van plegadas dentro de Opciones y el aviso largo de seguridad desaparece (el subtítulo dice que no se escribe nada hasta que confirmes).
  - **Drivers** lista los dispositivos con solo un nombre, un tipo y "Con driver" / "Sin driver"; los identificadores salen al pasar el ratón. Lo que casa con el USB va plegado.
  - **Crear USB**: el origen de Rebootera OS y las imágenes y drivers opcionales van plegados; solo quedan abiertos el USB y el botón final.
  - **Ajustes**: primero idioma, teclado y Wi-Fi; el color de acento, la intro, el movimiento reducido y los datos del sistema están en "Más opciones".
  - **Mis imágenes**: textos más cortos y filas más suaves.

## [0.7.0] - 2026-10-18

### Añadido

- **Muchos más sistemas, 40 imágenes en 21 familias.** Windows 11/10 y Server 2022/2025 (las imágenes de evaluación se descargan directamente de Microsoft), Ubuntu (Desktop, Server, Kubuntu, Xubuntu, Lubuntu, Ubuntu MATE), Debian (netinst y live GNOME/KDE/Xfce), Fedora (Workstation, Xfce, Cinnamon, Server), Linux Mint (Cinnamon, Xfce, MATE), Arch, Alpine, openSUSE Tumbleweed, Kali, Rocky, AlmaLinux, Proxmox VE, FreeBSD, OpenBSD, SystemRescue y Rescuezilla. Cuando el proyecto firma sus sumas (Ubuntu, Debian, Fedora, Mint, Kali, AlmaLinux) se verifica la firma OpenPGP con una clave fijada; el resto se comprueba con la suma que el proyecto publica por HTTPS. Resolutores nuevos para BSD, releases de GitHub, SourceForge y enlaces directos. Cada entrada descargable se resolvió contra los servidores reales (Windows 11 y 10 Home/Pro siguen siendo manuales, desde la página de Microsoft).
- **Los instaladores hechos con Rufus (o con cualquier herramienta que extraiga Windows en carpetas) se convierten en ISO.** Rebootera busca carpetas de instalador de Windows en tus discos (la raíz de cada unidad y sus carpetas de primer nivel; en Rebootera OS, en los medios abiertos) y "Crear ISO" reempaqueta una en una ISO arrancable con arranque BIOS y UEFI (y UDF para archivos de más de 4 GB como `install.wim`) directamente en la biblioteca, como un trabajo normal de la cola con su progreso. Rebootera Desktop también deja elegir una carpeta a mano. La carpeta de origen solo se lee. En Linux se usa `genisoimage` y en Windows IMAPI (viene con Windows); las dos se probaron de verdad (firma ISO 9660, catálogo El Torito con plataformas BIOS y EFI, etiqueta del volumen).
- **Descargas con varias conexiones.** Las imágenes grandes se descargan en bloques de 1 MiB por varias conexiones a la vez (hasta 8, 4 por defecto) con un mapa de bloques, así que un corte se reanuda donde quedó, y el hash se sigue calculando en orden. **Ajustar ancho de banda** (Centro de descargas) fija el número de conexiones y un límite de velocidad que se aplica al momento, también a una descarga en curso, y se recuerda en el USB.
- **Directo al USB.** Las descargas ya iban directas al USB; ahora "Actualizar Rebootera OS (la última)" en Rebootera Desktop también descarga la imagen del sistema directamente en el USB (como `.new`, así que la versión que ya tenía sigue intacta hasta que el SHA-256 coincide con el publicado), se reanuda si se corta y no hace nada si el USB ya tiene esa versión. Al actualizar, la imagen ya no se descarga antes al disco del PC.

### Cambiado

- **Centro de descargas nuevo**: una tarjeta por familia con versiones elegibles (etiquetas como Recomendado, LTS, Ligero, Live; tamaño en cada una), buscador, pestañas (Todos, Windows, Linux, Servidores y BSD, Rescate y herramientas), tarjeta con el estado del USB de destino (nombre y espacio libre), barra inferior con la descarga activa, **Ver cola** y **Ajustar ancho de banda**, copiar el SHA-256 de cada imagen y un estado claro de "ya está en tu USB".
- **Biblioteca** muestra las carpetas de instalador encontradas y su botón "Crear ISO".
- Rebootera Desktop abre en tu navegador los enlaces a las páginas de los fabricantes.

### Corregido

- La comprobación del catálogo ya no falla con imágenes sin suma publicada (las de evaluación de Microsoft): se importan como "sin comprobar" y la interfaz lo dice.
- Se quita `kali-linux-live`: Kali ya no aloja las imágenes live para descarga directa (solo torrents).

## [0.6.0] - 2026-10-18

### Añadido

- **Gestor de particiones** (Discos y particiones, en Rebootera OS): cada disco es una barra visual con una pieza por partición y por hueco libre. Eliges una pieza y puedes **cambiar su tamaño**, **repartir espacio con su vecina** (mover el límite entre dos particiones con un deslizador), **fusionar** dos vecinas, **borrar** o **crear** una partición en un hueco. Cada acción enseña cómo quedará el disco, los pasos exactos, lo que se borra y lo que se copia, y pide escribir `CHANGE <disco>`. Funciona con ext4 y NTFS (los sistemas de archivos que se pueden redimensionar sin perder archivos); exFAT y FAT32 se pueden absorber en una fusión, borrar y crear. Mover un límite copia los datos de la partición de la derecha, con progreso y el aviso de no desconectar ni apagar. Nunca se toca el USB desde el que arranca Rebootera, las unidades ópticas, los discos con particiones abiertas, los de solo lectura ni los MBR con particiones extendidas; el agente lo comprueba todo otra vez (no se fía de la interfaz) y deja cada operación en el registro.
- **Añadir Rebootera a un USB que ya tiene cosas, sin borrarlo** (por ejemplo uno hecho con Rufus): Crear USB lo ofrece junto a "USB nuevo". Usa la instalación no destructiva de Ventoy (Windows encoge un poco el volumen si el disco está lleno), deja los archivos donde están y muestra en el menú de arranque las imágenes (ISO) que ya tuviera. Rebootera OS acepta un Vault NTFS (montado con el controlador `ntfs3` del kernel) además de exFAT. El arranque del instalador de Rufus de esa unidad se sustituye por el menú de Rebootera.

### Cambiado

- **Descargar sistemas** es más simple: una lista corta de imágenes oficiales con un botón por fila y el tamaño a la derecha; las transferencias solo aparecen cuando las hay; los detalles (compatibilidad de arranque, firma, Secure Boot) quedan plegados tras un botón de información; filtro por Linux, Windows o herramientas. La última versión y su tamaño se consultan en segundo plano y Core las guarda 10 minutos, así que la página sale al instante.
- **Copias al USB más rápidas.** Ahora se lee y se escribe a la vez con búferes de 4 MiB (antes cada trozo se leía, se escribía y se calculaba su hash uno detrás de otro), la imagen de Rebootera OS ya no se relee tras copiarla (el sistema la servía desde su caché, así que no comprobaba nada y duplicaba el tiempo; se comprueba el tamaño tras vaciar el búfer) y el listado de la biblioteca se guarda 3 segundos.
- **Rebootera Desktop más ligero.** En reposo la interfaz gastaba un 40 % de un núcleo (WebView2 animando el fondo y los desenfoques): Desktop usa ahora siempre el modo ligero, sondea solo el USB cada 5 segundos (ni sensores, ni discos, ni registro del kernel, ni hardware, que no existen en Windows) y PowerShell ya no abre una ventana de golpe (todos los ayudantes se ejecutan ocultos).
- **Instalar es la segunda pestaña** de Rebootera OS (Resumen, Instalar, Biblioteca, Drivers, Descargar, Discos).
- Distribuciones de teclado: solo español e inglés.

## [0.5.0] - 2026-10-17

### Cambiado

- **Rebootera Desktop tiene una sola forma de escribir un USB: Crear USB.** "Mi USB" ha desaparecido. Crear USB hace ahora las dos cosas: un pendrive nuevo se borra y se convierte en un USB de Rebootera, y un USB de Rebootera que ya tienes se **actualiza sin borrar nada** (aparece en la misma lista, marcado como seguro). El sistema se puede conservar, volver a descargar (la última) o tomar de una ISO.
- **Resumen** y **Asistente de drivers** vuelven a Rebootera Desktop. El Resumen muestra tu USB de Rebootera de un vistazo (versión, espacio, imágenes, paquetes de drivers) con atajos; el Asistente de drivers lista los paquetes de drivers del USB agrupados por carpeta con el estado de la firma de su catálogo, y añade o quita carpetas de drivers.

### Añadido

- **Código privado, descargas públicas.** El código fuente sigue en el repositorio privado; las releases (`latest` y `beta`) y la web se publican en el repositorio público `Jondals/Rebootera-OS-downloads` con el secreto `RELEASES_TOKEN`. Rebootera Desktop descarga de ahí.
- **Canales estable y beta.** Cada push a `master` publica la release `latest` y, si la rama `beta` no existe, la crea desde ese commit (o la avanza si no tiene commits propios) y publica la prerelease `beta` con los mismos archivos. Los push a `beta` compilan y publican solo la prerelease `beta`, así que los cambios se prueban antes de llegar a `master`.
- **Mete tus cosas en el USB en el mismo paso.** Crear USB tiene un paso nuevo para añadir **imágenes** (varias ISO/IMG/WIM/VHD a la vez, por ejemplo un Windows 11 LTSC; cada una va a `isos/windows`, `isos/linux` o `isos/tools`, adivinado por el nombre y editable) y **carpetas de drivers**. Se copian justo después de Rebootera OS, con el avance de cada archivo, así que el USB sale listo para arrancar y usar sin mover nada a mano. Los archivos se escriben como `.part` y se renombran al terminar, de modo que un corte nunca deja una imagen a medias con su nombre definitivo.
- Todo se valida antes de tocar el USB: un archivo que no existe, una categoría que no existe, una imagen repetida o una carpeta de drivers sin ningún paquete `.inf` x64 se rechazan antes de borrar o cambiar nada. De una carpeta de drivers solo se copian archivos de drivers: se omiten instaladores (`.exe`, `.msi`), archivos comprimidos e imágenes de disco.
- Diálogos nativos de Windows para elegir varias imágenes y para elegir una carpeta.
- Pruebas: tests de Go de validación, copia, actualización del sistema y borrado de drivers (también ejecutados en Windows), un test en Windows del resultado del diálogo de selección múltiple y `scripts/test-desktop-windows.ps1`, que maneja el `.exe` real contra un USB **falso** (una carpeta temporal) y comprueba por hash los archivos copiados.

## [0.4.0] - 2026-10-17

### Añadido

- **Imágenes firmadas de Arch Linux y Alpine**: los dos proyectos firman cada ISO (no su lista de sumas), así que, cuando el SHA-256 coincide, Rebootera baja la firma OpenPGP separada (`.sig` en Arch, `.asc` en Alpine) y verifica la imagen entera, en streaming, con la clave fijada del proyecto (huellas comprobadas contra archlinux.org/download y alpinelinux.org/downloads). Una imagen cuya firma no coincide se borra, igual que una con la suma mala. El Centro de descargas muestra la insignia "Imagen firmada (GPG)". Comprobado con las imágenes reales de Arch 2026.10.01 y Alpine 3.24.2.
- **Distribución del teclado** en Ajustes (español e inglés). Core se lo pide al agente, que solo acepta esa lista cerrada, escribe la distribución donde la lee `start-kiosk` y reinicia la interfaz (cage ignora SIGTERM, así que se mata y systemd la reabre en 2 segundos). La elección se guarda en el Vault y se vuelve a aplicar en el siguiente arranque. Antes la distribución estaba fija en español.
- **Comprobaciones continuas** (`.github/workflows/test.yml`): `gofmt`, `go vet` y `go test` del núcleo, tipos y Prettier de la interfaz, en cada push y pull request.
- `scripts/vmtest-v03.py` incluye una prueba del teclado (el cambio reinicia el kiosco y cage arranca con el nuevo `XKB_DEFAULT_LAYOUT`).

### Cambiado

- El catálogo ahora indica `signature: gpg` en Arch Linux y Alpine: solo queda sin firma comprobada Windows (descarga manual).

## [0.3.0] - 2026-10-16

### Añadido

- **Drivers de Windows** (hitos 5 y 6): Rebootera lee los paquetes `.inf` de la carpeta `drivers/` del USB (UTF-16/UTF-8, sustitución de cadenas, solo secciones x64), casa sus Hardware ID con los dispositivos reales del equipo (primero el ID más específico, gana el paquete más reciente) y comprueba que cada paquete tenga catálogo firmado. El Asistente de drivers y el asistente de instalación muestran las coincidencias y los dispositivos críticos que siguen sin driver.
  - **Llevar al instalador de Windows los drivers que casan**: Rebootera empaqueta los paquetes que casan en un `.7z` que Ventoy extrae en el instalador (`X:\rebootera-drivers`) y genera un `autounattend.xml` cuyas rutas de drivers (`windowsPE` y `offlineServicing`) apuntan allí, de modo que Windows ve el disco y la tarjeta de red antes de pedir un disco y los drivers acaban también en el sistema instalado. Solo se usan paquetes `.inf`; no se ejecuta nada. La ISO original nunca se modifica.
- **Descargas con firma GPG verificada**: en Ubuntu 26.04 y 24.04, Debian 13 y Fedora 44, la lista de sumas se verifica con la clave OpenPGP del proyecto (fijada en el programa, con las huellas comprobadas contra las páginas de los propios proyectos) antes de descargar nada. Una descarga cuya firma falta o no coincide se rechaza. El Centro de descargas muestra la insignia "Índice firmado (GPG)". Arch Linux y Alpine no firman su lista de sumas y quedan marcados así.
- **Wi-Fi** en Ajustes: buscar, conectar (WPA2, WPA3 y redes abiertas, también ocultas) y desconectar. La contraseña nunca sale en una línea de órdenes: el agente escribe un perfil de NetworkManager solo de root (0600) y lo borra al desconectar o si falla la conexión.
- **Respuestas para el instalador** (Windows y Debian): idioma y teclado, zona horaria y una cuenta de usuario opcional, mediante `autounattend.xml` y `preseed` de Debian que entrega el `auto_install` de Ventoy. Nunca incluyen configuración de discos: el disco siempre lo eliges tú en el instalador. La contraseña se guarda codificada (Windows) o como hash SHA-512 (Debian) y, como todo archivo generado, se borra cuando Rebootera vuelve a arrancar.
- **Saltarse la comprobación de requisitos de Windows 11** (TPM, Secure Boot, RAM) como opción explícita del asistente, activa solo para esa instalación.
- **Arrancar el instalador una sola vez** (UEFI, opcional): antes de reiniciar al instalador, el agente pone el USB el último en el orden de arranque del firmware y lo fija como `BootNext`; cuando Rebootera vuelve a arrancar se restaura el orden original (conservando las entradas nuevas que añadió el sistema instalado). Así el instalador no vuelve a empezar tras su primer reinicio.
- **Rebootera Desktop**: **Crear USB** es ahora la primera (y única) forma de hacer un USB. Puede **descargar por ti la última versión de Rebootera OS** (comprobada con su SHA-256 publicado antes de tocar la unidad) o usar una ISO del PC; "Mi USB" ya no crea nada, solo gestiona un USB existente (añadir imágenes, **descargar e instalar la última** versión de Rebootera OS o elegir una ISO). Sin USB de Rebootera, **descargar sistemas ya funciona**: las imágenes se guardan en la carpeta `Rebootera` de tu usuario (antes el Centro de descargas estaba desactivado hasta conectar un USB). Las sesiones arrancadas desde un medio de solo lectura (un DVD o una máquina virtual) explican por qué no hay almacenamiento y cómo conseguirlo.
- **Web** (`site/`, en español e inglés) con los botones de descarga de `Rebootera-Desktop.exe` y de la ISO, y **GitHub Actions** que recompilan ambas en cada push a `master` y las publican en la release `latest` (`.github/workflows/`).
- `scripts/cpu-probe.py` mide la CPU en reposo de cada página (y lista las animaciones CSS activas) con el mismo WebKitGTK de la imagen.
- `scripts/vmtest-v03.py` (prueba de extremo a extremo de todo lo anterior, en tres escenarios independientes: Wi-Fi, Debian y Windows), `scripts/usb-add-drivers.sh` y un servicio de diagnóstico en las imágenes de depuración que vuelca por serie el estado gráfico.

### Cambiado

- **Las pruebas son unas 7 veces más rápidas y mucho más ligeras**: `scripts/test-all.sh` ejecuta toda la batería en unos 8 minutos (`--quick`: 1 minuto) con tres máquinas virtuales de 1,5 GB a la vez, en lugar de más de una hora una tras otra. Las imágenes de depuración arrancan en silencio (un arranque detallado por el puerto serie emulado a 115200 baudios tardaba dos minutos; ahora 20 segundos). WSL tiene un tope (`%USERPROFILE%\.wslconfig`: 6 GB y recuperación automática de memoria) y ya no se queda con 15 GB.
- Interfaz: el logotipo de Rebootera y la versión son ahora un pie sencillo del menú lateral.
- El asistente tiene un paso nuevo de "Ajustes del instalador" (respuestas, requisitos de Windows 11, arrancar una sola vez) y el paso de drivers muestra los drivers encontrados en el USB.
- El catálogo dice con honestidad qué entradas tienen índice firmado (`signature`: Arch Linux y Alpine son `none`).
- `p7zip-full` y `efibootmgr` forman parte de la imagen.

### Rendimiento

- **CPU de la interfaz en reposo: de casi un 90 % a un 4 %** en una máquina virtual (ventana del 47 % al 2 %, compositor del 28 % al 1 %). Sin GPU, la interfaz pintaba tres animaciones de pulso infinitas y cada fotograma se componía por software. `start-kiosk` ahora avisa a la página cuando se dibuja por software (no hay `/dev/dri/renderD*`, o se usan los niveles por software) y la interfaz pasa a un **modo ligero**: sin animaciones infinitas, sin desenfoques ni sombras de brillo, con una intro corta y sondeos más espaciados. En equipos con GPU real no cambia nada.
- **La interfaz arranca antes**: `rebootera-ui` ya no espera a `plymouth-quit-wait` (el final de todo el arranque) porque ya cierra Plymouth él mismo justo antes de abrir su ventana; además el halo de la pantalla de arranque se redibuja a un tercio de la frecuencia.
- Arranque hasta una interfaz funcional en una máquina virtual de 1 GB: unos 12 s (kernel 2 s + espacio de usuario 10 s). Memoria en uso tras arrancar: unos 560 MB.

### Corregido

- **El registro del kernel de la interfaz nunca funcionó**: Core pedía a `journalctl` la opción `--kernel`, que no existe (es `--dmesg`), así que el widget salía vacío y la petición fallaba cada 10 s.
- **Pantalla negra en VirtualBox** (gráficos VMSVGA sin 3D): Mesa elegía el driver `vmwgfx`, EGL fallaba ("No 3D enabled") y `cage` abortaba y luego se quedaba colgado, así que nada lo reiniciaba. Ahora la interfaz la arranca `start-kiosk`, que prueba renderizado por hardware, después renderizado por software de Mesa (llvmpipe) y por último wlroots con pixman y GTK con cairo, y recuerda el nivel que funciona. Comprobado en VirtualBox con VMSVGA (EFI y BIOS), VBoxSVGA (EFI) y VBoxVGA (BIOS).

## [0.2.0] - 2026-10-16

### Añadido

- **Interfaz nueva** basada en el diseño "Obsidian Aurora": menú lateral, barra superior con el estado en vivo (medio de arranque, red, Secure Boot, reloj) y un panel con datos reales del hardware.
  - Resumen: cabecera, tarjeta de instalación, accesos directos, perfil de hardware detectado (equipo, disco interno con su distribución de particiones, medio de arranque, seguridad del firmware con TPM) y tres widgets en vivo (temperaturas de los sensores, tráfico de red y registro del kernel).
  - Asistente de drivers: cada dispositivo PCI y USB con su driver de Linux, su Hardware ID de Windows (`PCI\VEN_…&DEV_…`) y los necesarios para instalar (disco y red) destacados.
  - USB y discos: discos y particiones con su función (EFI, Windows, recuperación, Vault de Rebootera…), sistema de archivos, uso y la protección Safe Disk Guard.
  - Asistente de instalación en seis pasos: elegir imagen, requisitos de Windows 11 y Linux, drivers críticos, disco de destino, plan de ejecución e inicio.
  - Ajustes: idioma, color de acento (Aurora turquesa o Nebula violeta), intro al arrancar y movimiento reducido.
- **Interfaz en inglés y español**, que se cambia desde la barra superior.
- **Intro animada**: fondo de auroras, el logotipo dibujado trazo a trazo, el nombre letra a letra y una barra que espera a los datos reales del sistema. Se puede saltar y respeta el movimiento reducido.
- **Pantalla de arranque** (Plymouth) y **menú de arranque** (GRUB y Ventoy) con la identidad de Rebootera: logotipo, halo, fuentes Inter y JetBrains Mono.
- **USB de Rebootera** (`scripts/make-usb.sh`): Ventoy 1.1.18 (descargado de su release oficial y comprobado con el SHA-256 publicado) y el Vault exFAT "REBOOTERA" con `isos/`, `drivers/`, `tools/`, `files/` y Rebootera OS como entrada por defecto.
  - Ventoy configurado con `VTOY_SECURE_BOOT_POLICY=0` y sin saltarse los requisitos de Windows 11 (`VTOY_WIN11_BYPASS_CHECK=0`, `VTOY_WIN11_BYPASS_NRO=0`). Con la política 1 Linux no arranca con Secure Boot, así que la 0 es la única que funciona; el precio es que, una vez inscrita la clave, el equipo también arranca imágenes sin firma comprobada. La interfaz, el README y el documento de limitaciones lo avisan.
- **Compatibilidad de arranque** de las imágenes oficiales desde el menú de Rebootera, comprobada en máquina virtual (`scripts/compat-matrix.sh`): se muestra en el Centro de descargas, la biblioteca y el asistente, que no deja elegir las incompatibles. Debian 13, Ubuntu 26.04 y Fedora 44 arrancan (UEFI), Arch Linux solo en BIOS (pantalla negra en UEFI) y Alpine 3.24 no arranca con Ventoy 1.1.18.
- **Nota de drivers** en el Asistente de drivers: el instalador de Windows necesita los drivers de disco y red en el USB (`drivers/`); inyectarlos en el instalador todavía no está hecho.
- `scripts/vmtest-interactive.py`: controla una VM desde fuera (teclas, clics, capturas); sirve para recorrer MokManager con Secure Boot.
- **Almacenamiento persistente**: el Vault se monta al arrancar (también cuando Ventoy tiene la partición ocupada, mediante `VentoyPart`) sin ejecutables, setuid ni dispositivos. Los ajustes sobreviven al reinicio.
- **Biblioteca**: imágenes del USB agrupadas por categoría, comprobación de integridad SHA-256, estado de verificación (verificada, importada, sin comprobar, dañada) y borrado seguro.
- **Centro de descargas**: catálogo oficial (Ubuntu 26.04 y 24.04 LTS, Debian 13, Fedora 44, Arch Linux, Alpine Linux y Windows 11 como descarga manual). Cada descarga obtiene el archivo más reciente y su SHA-256 del índice oficial de sumas por HTTPS, se puede pausar y reanudar (también tras reiniciar) y solo llega a la biblioteca si su suma coincide.
- **Añadir imágenes desde otro disco**: el disco se abre en solo lectura (sin reproducir diarios) y solo se muestran carpetas e imágenes.
- **Iniciar un instalador**: el asistente preselecciona la imagen elegida en el menú de Ventoy y reinicia; cuando Rebootera vuelve a arrancar, vuelve a ser la entrada por defecto.
- **Rebootera Desktop** (`out/Rebootera Desktop.exe`) para Windows (y para Linux a través del navegador): gestiona un USB de Rebootera sin arrancar desde él. Detecta el USB, muestra su versión y su espacio, añade imágenes con el diálogo nativo, descarga imágenes oficiales, actualiza Rebootera OS en el USB conservando la versión anterior como rescate y crea un USB de Rebootera nuevo en Windows con la herramienta oficial de Ventoy. Icono propio, barra de título oscura y soporte de pantallas de alta densidad.
- Pruebas nuevas en VM: `scripts/vmtest-usb.py` (prueba de extremo a extremo de un USB de Rebootera: Vault, descarga real, importación, ajustes, arranque de la imagen elegida y persistencia tras reiniciar) y `scripts/ui-preview.py` (capturas de la interfaz con el mismo WebKitGTK de la imagen).

### Cambiado

- La interfaz usa **pnpm** (versión fijada `pnpm@12.9.1` mediante corepack en la construcción).
- La construcción deja **solo la ISO definitiva** en `out/`; las imágenes de depuración se quedan dentro de la carpeta de construcción de WSL.
- Idioma por defecto y menú de arranque en inglés (español disponible).
- Los comentarios de cada archivo y función siguen el estilo de Better Comments (`*`, `!`, `?`, `TODO`).

### Corregido

- Fuentes del menú de arranque (GRUB): las entradas y el recuadro de selección se dibujan bien con las fuentes incluidas.
- `vmtest-boot.py` no vaciaba la consola serie, así que QEMU frenaba al invitado y una imagen de depuración parecía lentísima.
- Los montajes del agente quedaban encerrados en su espacio de montaje privado (`PrivateTmp`/`ProtectHome`) y Core no veía el disco importado.
- `TestSinVault` dependía de que el equipo de construcción tuviera un Vault.
- El paso "Crear USB" en Windows comprueba ahora la imagen de Rebootera OS antes de borrar el pendrive.
- Los discos USB grandes o con datos (como un SSD externo) muestran ahora sus volúmenes, los datos que contienen y un aviso claro antes de poder borrarlos.

## [0.1.0] - 2026-10-16

### Añadido

- Primera imagen arrancable (Debian 13, live-build): BIOS y UEFI, con Secure Boot mediante el shim firmado de Debian.
- Interfaz a pantalla completa en un kiosco (`cage` + GTK4/WebKitGTK) que se abre sola.
- Separación de privilegios: `rebootera-core` sin privilegios (API local en 127.0.0.1 con token por arranque, comprobación de Host, cabecera anti-CSRF y CSP) y `rebootera-agent` (root, lista cerrada de operaciones, comprueba quién llama con las credenciales del socket).
- Página "Este equipo" con datos reales y apagado o reinicio desde la interfaz.
- Banco de pruebas QEMU/KVM para UEFI, UEFI con Secure Boot y BIOS.

### Corregido

- Pantalla negra: `pam_systemd` (systemd 257) da a las sesiones una capacidad ambiental que hace abortar el sandbox de WebKitGTK (bubblewrap).
- El estado de Secure Boot siempre era "desconocido" (GUID de la variable EFI mal escrito).
- Apagar tardaba más de 90 segundos por Plymouth en un sistema live.

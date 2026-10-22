# Rebootera OS

**Instala, recupera y gestiona sistemas operativos desde un solo USB, sin conocimientos técnicos y sin necesitar otro sistema operativo.**

Rebootera OS es un entorno arrancable con identidad e interfaz propias. Conecta el USB de Rebootera a cualquier equipo, arranca desde él y una interfaz guiada a pantalla completa te permite descargar imágenes oficiales, instalar Windows o Linux, revisar el hardware y sus drivers y cuidar tus discos. Una aplicación complementaria, **Rebootera Desktop**, gestiona el mismo USB desde Windows sin reiniciar.

Versión en inglés: [README.md](README.md) · Cambios: [CHANGELOG.es.md](CHANGELOG.es.md)

![Resumen de Rebootera OS](docs/images/overview.png)

## Funciones

| Área | Qué hace |
| --- | --- |
| **Arranca en cualquier equipo** | BIOS y UEFI, con Secure Boot mediante el shim firmado de Debian. Menú de arranque, pantalla de arranque e intro animada propios. |
| **Inicio** | Una barra de búsqueda y una lista corta de acciones (instalar, descargar, mis imágenes, medios de instalación, discos, drivers, navegador, ajustes), todo manejable con el teclado. El perfil de hardware (equipo, discos internos, medio de arranque, Secure Boot y TPM, temperaturas, red, registro del kernel) está a un clic. |
| **Biblioteca** | Imágenes guardadas en el USB, agrupadas por categoría, con comprobación de integridad SHA-256. Una imagen dañada nunca se ofrece para instalar. |
| **Centro de descargas** | Imágenes oficiales (Ubuntu, Debian, Fedora, Arch Linux, Alpine). El archivo y su suma se obtienen del índice oficial de sumas por HTTPS; las descargas se reanudan tras un corte y solo llegan a la biblioteca si la suma coincide. En Ubuntu, Debian y Fedora, además, la lista de sumas se verifica con la clave GPG oficial del proyecto antes de descargar nada; en Arch Linux y Alpine, que firman cada ISO, se verifica la firma de la propia imagen una vez descargada. |
| **Medios de instalación** | Añade una instalación hecha con cualquier herramienta (un USB de Rufus o Ventoy, una carpeta, una `.iso`) desde otro USB o disco (abierto en solo lectura) o, desde Windows, con Rebootera Desktop. Cualquier `.iso` copiada a la carpeta `isos` también aparece. |
| **Asistente de instalación** | Comprobación de requisitos (UEFI, Secure Boot, TPM 2.0, memoria, disco), drivers críticos y disco de destino; después inicia el instalador original desde el menú de arranque. Opcional: responder automáticamente idioma, zona horaria y cuenta de usuario (Windows y Debian; el disco siempre lo eliges tú), llevar al instalador de Windows los drivers que casan, saltarse la comprobación de requisitos de Windows 11 y arrancar el instalador una sola vez (UEFI). |
| **Asistente de drivers** | Todos los dispositivos PCI y USB, el driver de Linux en uso y los Hardware ID de Windows, destacando lo que necesita el instalador (disco y red). Los paquetes de drivers de Windows (`.inf`) se encuentran **solos** en cada arranque (en el USB, en carpetas de drivers de las particiones de este equipo y en un Windows instalado) y se casan con esos Hardware ID; la página dice de dónde sale cada uno. |
| **Navegador** | Un navegador ligero dentro de Rebootera OS que abre al instante, solo para navegar: no guarda nada ni descarga archivos. |
| **Puntero** | Velocidad del puntero, toque para hacer clic y desplazamiento natural, guardados en el USB. |
| **Wi-Fi** | Conecta la sesión a una red inalámbrica desde Ajustes (WPA2/WPA3 y redes abiertas, también ocultas). |
| **USB y discos** | Discos y particiones con su función y su uso. **Safe Disk Guard**: los discos internos y el USB de Rebootera están protegidos y solo se leen. |
| **Discos y particiones** | Cada disco como una barra visual con tres acciones: crear o cambiar el tamaño de una partición, unirla con una vecina, borrarla (ext4 y NTFS se pueden redimensionar sin perder archivos). Cada cambio enseña antes los pasos exactos y pide escribir el nombre del disco; el USB desde el que arranca Rebootera nunca se toca. |
| **Rebootera Desktop** | Aplicación para Windows, Linux y macOS. Su opción **Crear USB** enseña cada USB una sola vez y ofrece lo que le corresponde: actualizar Rebootera o borrar y empezar de cero, con la última versión de Rebootera descargada por ti o tu propia ISO, y (solo en Windows) añadirlo conservando tus archivos. En Linux pide permisos de administrador con polkit; en macOS el USB arranca equipos con UEFI. También descarga imágenes oficiales y gestiona la biblioteca sin USB (se guardan en la carpeta `Rebootera` de tu usuario). |
| **Aspecto** | Ocho temas oscuros (Amoled, Obsidiana, Zafiro, Esmeralda, Amatista, Brasa, Naranja, Grafito), fondo animado con desenfoque configurable que se puede apagar, cabecera con pestañas, tamaño de la interfaz, inglés y español. |
| **Placas e IoT** | Raspberry Pi OS, Ubuntu para Raspberry Pi, DietPi, OpenWrt y Home Assistant OS, comprobados con su SHA-256 publicado (imágenes de disco para grabar en una tarjeta SD). Puedes renombrar cualquier imagen y ponerle una foto desde Mis imágenes. |

| | |
| --- | --- |
| ![Discos y particiones](docs/images/disks.png) | ![Temas](docs/images/themes.png) |
| ![Centro de descargas](docs/images/downloads.png) | ![Rebootera Desktop](docs/images/desktop.png) |

## Cómo conseguir un USB de Rebootera

**Descarga:** la web y las releases públicas (repositorio `Jondals/Rebootera-OS`; el código fuente es privado) ofrecen siempre la última compilación: `Rebootera-Desktop-Windows.exe` para Windows y `rebootera-os-amd64.iso`. Ambas se recompilan al llegar código a `master` y cada vez se publica una release nueva ([todas las releases](https://github.com/Jondals/Rebootera-OS/releases)).

**Ramas:** se trabaja en `beta` (ahí solo corren las pruebas) y se abre una pull request `beta` a `master`: al fusionarla se compila y se publica una release `v<versión>` nueva, y se conservan las anteriores.

**Desde Windows**: ejecuta `Rebootera Desktop.exe` y abre **Crear USB** (la primera opción). Cada pendrive sale una sola vez; elige el tuyo y decide qué hacer con él: **actualizar Rebootera** (si ya lo tiene, no se borra nada), **añadir Rebootera y conservar mis archivos** (para un pendrive hecho con Rufus, por ejemplo) o **borrarlo y empezar de cero**. Después elige de dónde sale Rebootera OS: la **última versión** (lo predeterminado; descargada y comprobada con su SHA-256 publicado antes de borrar nada) o **mi ISO de Rebootera**. Windows pedirá permiso de administrador para preparar el pendrive, y borrar o cambiar un disco exige escribir el texto de confirmación exacto. Las imágenes y los drivers se añaden después desde Descargar, Mis imágenes y Medios de instalación; los drivers no hace falta añadirlos, Rebootera los encuentra solo.

**Desde Linux**, como root:

```bash
scripts/usb/make-usb.sh --device /dev/sdX --confirm /dev/sdX --iso out/rebootera-os-0.20.0-amd64.iso
```

El script rechaza los discos no extraíbles y el disco del sistema.

**Sin persistencia**: la ISO es híbrida, así que también se puede grabar directamente en un pendrive (por ejemplo con Rufus o `dd`) o arrancar desde un DVD. En ese caso Rebootera OS funciona en solo lectura: sin biblioteca ni ajustes guardados.

### Arrancar el equipo desde el USB

1. Conecta el USB y reinicia el equipo.
2. Abre el menú de arranque nada más encender (normalmente F12, F11, F8, F9 o Esc) y elige el USB.
3. Rebootera OS arranca a los pocos segundos.

Con Secure Boot activado, el primer arranque en cada equipo muestra una pantalla azul de MokManager para registrar la clave del menú de arranque: elige **Enroll key from disk**, abre la unidad **VTOYEFI**, selecciona `ENROLL_THIS_KEY_IN_MOKMANAGER.cer`, confirma y reinicia. Solo ocurre una vez por equipo.

> **Compromiso de seguridad:** la política 1 de Ventoy (que rechaza imágenes sin firma comprobada) no deja arrancar Linux con Secure Boot, así que Rebootera usa la política 0. Una vez inscrita la clave, ese equipo también arrancará imágenes cuya firma no se ha comprobado. Inscríbela solo en equipos de confianza y bórrala en MokManager (**Delete MOK**) cuando ya no la necesites.

### Qué hay en el USB

```
REBOOTERA (exFAT)
├─ isos/windows   Imágenes de instalación de Windows
├─ isos/linux     Imágenes de instalación de Linux
├─ isos/tools     Imágenes de rescate y utilidades
├─ drivers/       Paquetes de drivers
├─ tools/         Herramientas portables
├─ files/         Tus archivos
├─ rebootera/     Rebootera OS (no modificar)
└─ ventoy/        Ajustes del menú de arranque (no modificar)
```

## Seguridad

- La interfaz no tiene privilegios. **Rebootera Core** se ejecuta con un usuario sin privilegios y sirve su API solo en `127.0.0.1`, protegida con un token por arranque, comprobación de Host, cabecera anti-CSRF y una política de seguridad de contenido estricta.
- Solo **Rebootera Agent** se ejecuta como root, y acepta una lista cerrada de operaciones únicamente del usuario de Core (comprobado con las credenciales del socket).
- Los demás discos se abren siempre **en solo lectura**, sin reproducir los diarios de los sistemas de archivos.
- Las descargas solo vienen de fuentes oficiales por HTTPS y se comprueban con el SHA-256 oficial antes de llegar a la biblioteca. Cuando el proyecto firma su lista de sumas (Ubuntu, Debian, Fedora), antes se verifica la firma OpenPGP con la clave fijada del proyecto y la descarga se rechaza si no coincide; cuando firma cada ISO (Arch Linux, Alpine), la imagen se verifica tras descargarla y se borra si la firma no coincide.
- Las contraseñas de Wi-Fi nunca salen en una línea de órdenes: van a un perfil de NetworkManager solo de root (0600) que desaparece al desconectar o apagar.
- Los archivos que responden al instalador (con una contraseña codificada) viven en el USB solo hasta que Rebootera vuelve a arrancar, y entonces se borran.
- El menú de arranque no esconde su compromiso de seguridad: con Secure Boot, inscribir la clave de Ventoy permite que ese equipo arranque imágenes sin firma comprobada (la interfaz y este README lo dicen), y los atajos que se saltan los requisitos de Windows 11 están desactivados.

## Web y compilaciones automáticas

- `site/` es la landing page (HTML sencillo, en español e inglés). `.github/workflows/pages.yml` la publica en GitHub Pages; en los ajustes del repositorio hay que poner una vez **Pages → Source** en *GitHub Actions*.
- `.github/workflows/build.yml` compila la ISO y `Rebootera-Desktop-Windows.exe` dentro de un contenedor Debian 13 al llegar código a `master` y publican una release `v<versión>` nueva. La web lee de esa release la versión, la fecha y el SHA-256, y Rebootera Desktop descarga de ella la ISO. La primera compilación solo existe tras el primer push.

## Construir

Todo se construye en Windows dentro de una distribución WSL dedicada con Debian 13 llamada `RebooteraBuild`. Las órdenes suponen que el proyecto está en `D:\dev\Rebootera OS`; si está en otro sitio, cambia `/mnt/d/dev/Rebootera OS`. Las dos construcciones necesitan conexión a internet (paquetes de Debian, módulos de Go y paquetes de pnpm).

### 1. Preparar el entorno de construcción (una sola vez)

Abre PowerShell y ejecuta:

```powershell
wsl --install Debian --name RebooteraBuild --location D:\WSL\RebooteraBuild --no-launch
wsl -d RebooteraBuild -u root --exec bash -c "apt-get update && apt-get install -y live-build debootstrap rsync curl golang nodejs node-corepack imagemagick librsvg2-bin grub-common fonts-inter fonts-jetbrains-mono xorriso mtools squashfs-tools dosfstools exfatprogs exfat-fuse gdisk parted qemu-system-x86 qemu-utils ovmf xvfb gir1.2-webkit-6.0 python3-gi"
```

Los seis últimos paquetes solo hacen falta para las pruebas en máquina virtual.

### 2. Construir la ISO

```powershell
wsl -d RebooteraBuild -u root --exec bash "/mnt/d/dev/Rebootera OS/scripts/build/build.sh"
```

Tarda entre 10 y 20 minutos. El script ejecuta las pruebas de Go, compila `rebootera-core` y `rebootera-agent`, compila la interfaz con pnpm, genera los gráficos del menú y de la pantalla de arranque y ejecuta live-build. El resultado es:

- `out/rebootera-os-<versión>-amd64.iso` y su `.sha256`. Las ISO anteriores de `out/` se borran, así que en `out/` solo hay una ISO.

Para una imagen de pruebas con consola root en el puerto serie, añade `--debug`. Esa imagen se queda dentro de WSL en `/root/rebootera-build/out/` y nunca se mezcla con la definitiva.

### 3. Construir la aplicación de escritorio

```powershell
wsl -d RebooteraBuild --exec bash "/mnt/d/dev/Rebootera OS/scripts/build/build-desktop.sh"
```

Tarda alrededor de un minuto. El script compila la interfaz, la incrusta junto con los archivos de un USB de Rebootera nuevo (ajustes y tema del menú de arranque), añade el icono y el manifiesto de Windows y compila para Windows. El resultado es:

- `out/Rebootera-Desktop-<version>.exe`: un único archivo, sin instalación. Necesita Microsoft Edge WebView2 Runtime, que ya viene con Windows 10 y 11. Cierra la aplicación antes de volver a construirla.
- Con `--all`, `dist/Rebootera-Desktop-Linux-x64`, `-Linux-arm64`, `-macOS-Intel` y `-macOS-AppleSilicon`: un binario cada uno, sin dependencias (`out/` solo lleva la ISO y el .exe). Para tener ventana propia usa un navegador basado en Chromium si hay uno instalado; si no, abre tu navegador predeterminado.

### 4. Crear un USB de Rebootera

- **Pendrive real**: abre `Rebootera Desktop.exe` → **Crear USB** y elige la ISO del paso 2, (también disponible en Rebootera Desktop para Linux y macOS), o usa `scripts/usb/make-usb.sh --device` en Linux (ver [Cómo conseguir un USB de Rebootera](#cómo-conseguir-un-usb-de-rebootera)).
- **Imagen de disco para máquinas virtuales**: `wsl -d RebooteraBuild -u root --exec bash "/mnt/d/dev/Rebootera OS/scripts/usb/make-usb.sh" --image /root/usb.img --iso "/mnt/d/dev/Rebootera OS/out/rebootera-os-<versión>-amd64.iso"`.

### Desarrollo de la interfaz (pnpm)

```powershell
cd ui
pnpm install
pnpm run format:check
pnpm run check-types
pnpm run build
```

## Pruebas

| Script | Qué comprueba |
| --- | --- |
| `go test ./...` (en `core/`) | Protecciones de la API, inventario de discos y hardware, resolución del catálogo, descargas (reanudación, suma incorrecta), biblioteca, elección del arranque y kit del USB |
| `go test -tags online ./internal/catalog/` | Cada entrada del catálogo contra las fuentes oficiales reales |
| `scripts/test/test-all.sh [--full] [--build] [--no-build]` | **Rápido por defecto** (menos de 3 minutos): arranca las dos ISO en UEFI + Secure Boot y solo recompila una ISO si el código cambió. `--full` ejecuta todas las pruebas de VM en paralelo (tres VM de 1,5 GB a la vez, unos 17 minutos): úsalo antes de publicar |
| `scripts/test/cpu-probe.py URL` | CPU en reposo de cada página de la interfaz con el mismo WebKitGTK de la imagen (caza animaciones infinitas) |
| `scripts/test/vmtest.py MODO ISO SALIDA` | Arranca la ISO en QEMU/KVM (`uefi`, `uefi-sb`, `bios`): servicios, aislamiento, permisos, Secure Boot, capturas de la interfaz y apagado desde ella |
| `scripts/test/vmtest-v03.py USB.img SALIDA` | Funciones de la 0.3.0 de extremo a extremo: descarga con firma GPG verificada, casado de drivers con INF reales de virtio-win, Wi-Fi con radios simuladas y un punto de acceso virtual, preseed de Debian con arranque único, preparación y limpieza de Windows |
| `scripts/test/test-create-linux.sh` | Linux (en la distro WSL, como root): ejecuta el Rebootera Desktop real y crea un USB en un **disco virtual** (un dispositivo loop) por su API: instalación real de Ventoy, etiqueta, GPT, partición de arranque, Rebootera OS byte a byte y la estructura del USB |
| `scripts/test/test-desktop-windows.ps1` | Solo Windows: ejecuta el `Rebootera Desktop.exe` real contra un USB **falso** (una carpeta temporal) y comprueba que la imagen y los drivers se copian byte a byte, que el sistema no se toca y que las peticiones malas se rechazan |
| `scripts/test/make-partman-disk.sh SALIDA.qcow2` | Disco de pruebas del gestor de particiones: ext4 + NTFS con datos y sumas (solo archivos de imagen y un dispositivo loop) |
| `REBOOTERA_LOOP_TESTS=1 go test ./cmd/rebootera-agent -run TestLoop` | Como root en WSL: ejecuta el gestor de particiones sobre discos loop reales (cambiar tamaño, mover un límite en las dos direcciones, fusionar, crear, borrar) y comprueba los archivos por hash y la salud de los sistemas de archivos |
| `scripts/test/vmtest-interactive.py MODO IMG SALIDA` | Controla una VM desde fuera (teclas, clics, capturas), por ejemplo para recorrer MokManager |
| `scripts/test/vmtest-usb.py MODO USB.img DISCO.qcow2 SALIDA` | USB de Rebootera de extremo a extremo: Vault, descarga real, importación desde otro disco, ajustes, arranque de la imagen elegida y persistencia tras reiniciar |
| `scripts/dev/site.ps1` | Windows: enseña la página web (`site/`) en el navegador con un servidor local mínimo (`-Port`, `-NoOpen`) |
| `scripts/dev/ui-preview.py URL SALIDA` | Capturas de la interfaz con el mismo WebKitGTK de la imagen |

Los resultados están en [docs/PRUEBAS.md](docs/PRUEBAS.md).

## Estructura del proyecto

| Carpeta | Contenido |
| --- | --- |
| `branding/` | Logotipo |
| `core/` | Go: `rebootera-core`, `rebootera-agent`, `rebootera-desktop` y sus paquetes |
| `ui/` | Rebootera UI (TypeScript, sin dependencias en tiempo de ejecución) |
| `os/` | Configuración de live-build, servicios, menú y pantalla de arranque, contenido del USB de Rebootera y `os/shell` (la ventana del kiosco: GTK4 + WebKitGTK dentro de `labwc`) |
| `scripts/` | `build/` (ISO, aplicación de escritorio, gráficos de arranque), `usb/` (creación del USB), `test/` (pruebas en máquinas virtuales y en Windows) y `dev/` (vista previa de la interfaz) |
| `docs/` | Resultados de las pruebas y las capturas que se usan aquí |
| `site/` | La página web |

## Componentes de terceros

Rebootera OS se construye sobre Debian GNU/Linux. El USB de Rebootera usa [Ventoy](https://www.ventoy.net) (GPLv3) como capa multiboot, descargado de su release oficial y verificado con el SHA-256 publicado. Rebootera Desktop usa [go-webview2](https://github.com/jchv/go-webview2) (MIT). Fuentes: Inter y JetBrains Mono (SIL Open Font License).

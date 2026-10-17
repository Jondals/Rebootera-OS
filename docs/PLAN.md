# Rebootera OS: plan técnico

Documento vivo. Recoge las decisiones de arquitectura, las comparativas que las justifican, los riesgos conocidos y el plan de hitos. Las pruebas y sus resultados están en [PRUEBAS.md](PRUEBAS.md).

Fecha de la última revisión: 2026-10-16.

## 0. Entorno de desarrollo inspeccionado

| Elemento | Estado real |
| --- | --- |
| Anfitrión | Windows 11 IoT Enterprise LTSC 2024, i7-14700KF (28 hilos), 32 GB RAM |
| Disco de trabajo | D: con 1,5 TB libres |
| Linux de construcción | WSL 2.6.3 con una distro dedicada `RebooteraBuild` (Debian 13 trixie), instalada en `D:\WSL\RebooteraBuild`. Se elimina con `wsl --unregister RebooteraBuild` |
| Virtualización para pruebas | `/dev/kvm` disponible dentro de WSL (virtualización anidada). QEMU 10.0 + OVMF (UEFI normal, UEFI con Secure Boot y claves de Microsoft) y SeaBIOS (BIOS heredada). VirtualBox 7.2.20 instalado en Windows como segunda opción |
| Herramientas | live-build 20250505, Go 1.24, Node 20, xorriso, squashfs-tools, exfatprogs, gdisk, parted, wimtools, librsvg |
| Restricciones | Sin Docker. Sin permisos de administrador en las sesiones automáticas de Windows. No se formatean dispositivos reales: todo se prueba con discos virtuales |

## 1. Arquitectura técnica

```
 ┌──────────────────────────── Rebootera OS (Linux en vivo, inmutable) ───────────────────────────┐
 │                                                                                                │
 │  cage (compositor kiosco Wayland, tty1)                                                        │
 │   └─ rebootera-shell (GTK4 + WebKitGTK, pantalla completa, sin barra de direcciones)           │
 │        └─ Rebootera UI (HTML/CSS/TypeScript, sin framework)                                    │
 │               │  HTTP sobre 127.0.0.1 con token por arranque y comprobación de Host           │
 │               ▼                                                                                │
 │  rebootera-core  (usuario sin privilegios `rebootera-core`)                                    │
 │    biblioteca, catálogo, descargas, verificación, inventario de hardware, drivers, planes      │
 │               │  socket Unix /run/rebootera/agent.sock, lista cerrada de operaciones           │
 │               ▼                                                                                │
 │  rebootera-agent (root, mínimo)                                                                │
 │    energía, montar en solo lectura, escribir en el Vault, particionar con confirmación firmada  │
 └────────────────────────────────────────────────────────────────────────────────────────────────┘
```

Módulos del producto y dónde viven:

| Módulo | Qué es | Tecnología |
| --- | --- | --- |
| Rebootera OS | Sistema en vivo arrancable, inmutable (squashfs), con red, firmware y arranque directo a la interfaz | Debian 13 + live-build, kernel 6.12 LTS |
| Rebootera UI | Interfaz a pantalla completa, punto central de todo | HTML/CSS/TypeScript compilado con `tsc`, sin dependencias en tiempo de ejecución |
| rebootera-shell | Ventana kiosco que carga la UI, bloquea la navegación externa y se recupera si la UI cae | Python 3 + GTK4 + WebKitGTK 6 |
| Rebootera Core | Servicio sin privilegios: biblioteca, descargas, verificación, detección, coordinación | Go, solo biblioteca estándar (sin dependencias externas que auditar) |
| Rebootera Agent | Única pieza con root. Expone operaciones concretas, nunca una shell | Go, mismo módulo que Core |
| Rebootera Boot | Capa multiboot que lanza instaladores originales | Ventoy (ver sección 3) más un menú propio para Rebootera OS |
| Rebootera Drivers | Inventario de hardware, análisis de paquetes INF y de firmware, planes de integración | Go dentro de Core; `wimlib` para tocar `boot.wim` |
| Rebootera Vault | Partición exFAT con ISOs, drivers, herramientas, archivos del usuario | exFAT para que Windows y Linux la lean sin instalar nada |
| Rebootera Desktop | Aplicación opcional para Windows y Linux que gestiona un USB existente | El mismo Core compilado para Windows y Linux más la misma UI en una ventana nativa |

Principios que guían la arquitectura:

- La interfaz nunca tiene privilegios. Todo lo que modifica discos pasa por el agente, que vuelve a identificar el dispositivo (número de serie, tamaño, tabla de particiones) justo antes de escribir y rechaza la orden si algo cambió.
- Las operaciones destructivas exigen un "plan" que Core calcula, la UI muestra y el usuario confirma escribiendo o pulsando explícitamente. El agente solo ejecuta planes confirmados y con el identificador del dispositivo que se mostró.
- El sistema base es inmutable. Lo que cambia (biblioteca, configuración, registros) vive en particiones separadas.
- Core y la UI se comparten con Rebootera Desktop. Por eso Core está en Go (compila para Linux y Windows sin dependencias) y la UI es web.

## 2. Comparativa de distribuciones base

| Criterio | Debian 13 | Ubuntu 24.04/26.04 LTS | Fedora 44 | Arch | Alpine |
| --- | --- | --- | --- | --- | --- |
| Estabilidad y soporte | 5 años con LTS | 5 años (más con Pro) | 13 meses por versión | Rolling | 2 años por rama |
| Secure Boot | shim firmado por Microsoft + GRUB y kernel firmados | Sí | Sí | No (hay que firmar a mano) | No |
| Hardware reciente | Kernel 6.12 LTS; kernels nuevos en backports | Kernels HWE más nuevos | El más reciente | El más reciente | Kernel LTS |
| Firmware no libre | Componente oficial `non-free-firmware` | Incluido | Incluido salvo excepciones | Incluido | Paquetes `linux-firmware-*` |
| Herramienta de imagen en vivo | live-build, madura y documentada | livecd-rootfs (más ligada a Canonical) | kiwi / lorax | archiso (sencilla) | mkimage |
| Licencias y marca | Política clara para derivadas | Hay que retirar la marca Ubuntu | Hay que retirar la marca Fedora | Clara | Clara |
| Tamaño mínimo práctico con GUI | Medio | Medio-alto (snaps) | Medio | Bajo | Muy bajo (musl, menos compatible) |

**Decisión: Debian 13 (trixie), amd64.** Pesa más el Secure Boot firmado de serie, el ciclo estable largo, el firmware oficial y live-build. El punto débil es el hardware muy nuevo; la mitigación es una entrada de arranque alternativa con el kernel de `trixie-backports` (se verificará que esté firmado antes de ofrecerla con Secure Boot). Arm64 queda fuera de la primera versión.

## 3. Comparativa de gestores multiboot

| Opción | Licencia | ISOs de Windows | ISOs de Linux | Secure Boot | Integración |
| --- | --- | --- | --- | --- | --- |
| **Ventoy 1.1.18** | GPLv3 (código propio) más componentes con sus licencias | Sí, arranca la ISO original sin extraerla | Sí, cientos probadas | Con inscripción MOK la primera vez en cada equipo | Plugins: `auto_install` (autounattend, preseed, kickstart), `injection` (archivos en WinPE o initramfs), `theme`, `menu_alias`, `ventoy_grub.cfg` |
| GRUB2 con `loopback` (estilo GLIM) | GPLv3 | No sin herramientas extra | Sí, pero con parámetros distintos por distro que hay que mantener | Con shim y GRUB firmados | Total, pero todo es trabajo propio |
| GRUB2 + wimboot (iPXE) | GPLv2 | Arranca WinPE, pero el instalador no encuentra `install.wim` sin un montaje propio | No aplica | wimboot no está firmado por Microsoft | Habría que reescribir lo que Ventoy ya hace |
| Easy2Boot / YUMI | Mixta (usan Ventoy por debajo) | Sí | Sí | Igual que Ventoy | Menos control y más capas |
| Cargador propio | Propia | Muy costoso | Muy costoso | Requiere firma de Microsoft | No razonable al principio |

**Decisión: Ventoy como motor multiboot**, con estas condiciones obligatorias:

1. Se descarga la versión publicada, se comprueba su SHA-256 contra el publicado por el proyecto y se guarda el código fuente correspondiente junto a la imagen (obligación GPLv3). Ha habido críticas públicas por los binarios precompilados del repositorio de Ventoy; queda como riesgo y se estudiará compilarlo desde el código fuente.
2. `VTOY_SECURE_BOOT_POLICY` a `0` (decisión revisada en la v0.2, 2026-10-16): la política `1` es la más estricta, pero con ella Ventoy no deja arrancar Linux con Secure Boot. Probada la `0` en VM con UEFI y Secure Boot: tras inscribir la clave arranca Rebootera OS entero. El coste es que, una vez inscrita la clave, ese equipo arranca también imágenes sin firma comprobada; por eso la interfaz y el README avisan y recomiendan inscribirla solo en equipos de confianza (y borrarla con "Delete MOK").
3. `VTOY_WIN11_BYPASS_CHECK` y `VTOY_WIN11_BYPASS_NRO` a `0`: Ventoy los activa por defecto. Rebootera los ofrecerá solo como opción explícita con su advertencia.
4. Tema visual propio y alias de menú para que el arranque tenga la identidad de Rebootera.

Lo que Ventoy no resuelve y queda por investigar en el hito 4:

- No tiene "arrancar una vez". Si se deja una ISO como predeterminada, el instalador de Windows reiniciaría en bucle. Opciones: menú con la ISO elegida preseleccionada pero sin arranque automático; o un `ventoy_grub.cfg` con estado de un solo uso.
- Montar la partición de Ventoy desde un Linux arrancado como ISO desde ella da "dispositivo ocupado" salvo en las distros probadas por Ventoy. Para evitarlo, Rebootera OS no se arrancará como ISO, sino desde su propia partición mediante el menú personalizado de Ventoy (por confirmar en el hito 4) o con su propio GRUB firmado antes de Ventoy.
- Algunas imágenes no son compatibles (instaladores que verifican el medio, ISOs sin El Torito, imágenes que esperan un CD real). El catálogo guardará la compatibilidad conocida y el modo individual cubrirá esos casos.

## 4. Almacenamiento persistente

Estructura del USB (GPT):

| Nº | Etiqueta | Sistema de archivos | Contenido | Se toca al actualizar |
| --- | --- | --- | --- | --- |
| 1 | `REBOOTERA` | exFAT | Vault: `isos/`, `drivers/`, `tools/`, `files/`, `ventoy/` (configuración de Ventoy) | Nunca, salvo `ventoy/ventoy.json` que se regenera |
| 2 | `VTOYEFI` | FAT16 | Cargadores de Ventoy | Sí, con Ventoy2Disk `-u` |
| 3 | `RBT_SYSTEM` | ext4 (montada en solo lectura) | `A/` y `B/`, cada una con `vmlinuz`, `initrd.img`, `filesystem.squashfs` y su manifiesto firmado | Sí, solo la ranura inactiva |
| 4 | `RBT_STATE` | ext4 | `/var/lib/rebootera`: configuración, estado de descargas, registros, claves Wi-Fi | No |

- Las particiones 3 y 4 van en el espacio que reserva Ventoy al final del disco (`Ventoy2Disk.sh -r`).
- El Vault es exFAT para que la aplicación de escritorio y cualquier Windows o Linux lo lean sin controladores extra, y porque admite archivos de más de 4 GB.
- El sistema no usa la persistencia "todo el root en overlay" de live-boot: mantener la raíz inmutable hace que las actualizaciones sean limpias y que un fallo no deje el sistema a medias.
- Actualización A/B: se descarga la nueva versión, se verifica su manifiesto (SHA-256 de cada archivo y firma), se escribe en la ranura inactiva, se marca como "en prueba" y se arranca. Si el arranque no confirma que la UI levantó, el siguiente arranque vuelve a la ranura anterior.
- Las descargas se escriben en `isos/.partial/` con un archivo de estado; solo pasan a `isos/` tras verificar el hash. Una ISO incompleta nunca aparece como lista.
- Mientras no exista la estructura USB completa (hito 2), la imagen ISO del hito 1 funciona sin persistencia.

## 5. Plan de detección e instalación de drivers

### Inventario (Linux en vivo, sin privilegios salvo lo indicado)

- Equipo: `/sys/class/dmi/id` (fabricante, modelo, versión de BIOS; el número de serie solo con el agente y nunca en registros).
- PCI y USB: `/sys/bus/pci/devices/*/{vendor,device,subsystem_vendor,subsystem_device,class,revision}` y `/sys/bus/usb/devices`. Se generan los Hardware ID con el formato de Windows (`PCI\VEN_8086&DEV_A0F0&SUBSYS_...&REV_..`) para poder casarlos con los INF.
- Almacenamiento: clase PCI 01xx (NVMe, AHCI, RAID, VMD de Intel). Se marca como crítico cualquier controlador sin el que Windows no ve el disco (caso típico: Intel VMD/RST).
- Red: clase 02xx (Ethernet y Wi-Fi). Crítico si no hay otra vía de red durante la instalación.
- Firmware que el kernel pidió y no encontró: registros del kernel (`firmware: failed to load`).

### Origen de los paquetes, en orden

1. Vault del USB (`drivers/`), indexado por Hardware ID.
2. Particiones OEM del equipo, montadas **en solo lectura** por el agente, buscando `*.inf` en rutas conocidas (`C:\Drivers`, `SWSetup`, `DriverStore\FileRepository` de un Windows existente). Nunca se presupone que la partición de recuperación contiene drivers.
3. Fabricante del equipo: solo a través de catálogos oficiales con formato estable (por ejemplo, catálogos de drivers publicados por Dell, HP y Lenovo). Cada uno se añade cuando se haya verificado su formato y su URL oficial; no se inventan URLs.
4. Fabricante del componente (Intel, AMD, Realtek...), con la misma regla.
5. Repositorios oficiales del sistema destino (Windows Update Catalog solo si se puede consumir de forma documentada; repositorios de la distribución para Linux).

### Verificación antes de usar un paquete

- El INF declara el Hardware ID exacto o uno compatible del dispositivo.
- La sección `[Manufacturer]` declara la arquitectura (`NTamd64`) y la versión de Windows admitida.
- El catálogo `.cat` existe y la firma Authenticode es válida y de un editor esperado. En Linux no hay verificador completo de Authenticode en la biblioteca estándar de Go; se evaluará `osslsigncode` (GPLv3) para comprobar la cadena.
- Hash del archivo descargado contra el publicado.
- Nunca se ejecutan instaladores `.exe` descargados. Solo se usan paquetes INF; si un fabricante solo ofrece un `.exe`, se guarda para que el usuario lo ejecute ya en Windows y se registra como "requiere intervención".

### Integración en Windows

- Drivers críticos (disco, red): se inyectan en el WinPE del instalador sin modificar la ISO original, con el plugin `injection` de Ventoy (archivo extraído en `X:`) y un `autounattend.xml` cuyo `Microsoft-Windows-PnpCustomizationsWinPE/DriverPaths` apunta allí. Alternativa para el modo individual: `wimlib-imagex update` sobre una copia de `boot.wim` (equivalente a `DISM /Add-Driver` desde Linux).
- Resto de drivers: se copian a `$OEM$` o a una carpeta del Vault y se añaden en la fase `offlineServicing` (`Microsoft-Windows-PnpCustomizationsNonWinPE/DriverPaths`) para que entren en el Driver Store del Windows instalado.
- Tras la instalación, el informe del Vault lista los drivers que entraron y los pendientes. No se supone que todas las versiones de Windows aceptan lo mismo: el `autounattend.xml` se genera por versión y arquitectura y se prueba en VM.

### Integración en Linux

- Kernel y módulos: se comprueba con `modules.alias` del kernel de la ISO destino si cada dispositivo tiene controlador.
- Firmware: se calcula qué paquetes de firmware de la distribución hacen falta (por ejemplo `firmware-iwlwifi`, `firmware-realtek` en Debian) y se añaden con el mecanismo de cada instalador (en Debian, la carpeta `firmware/` del medio; en Ubuntu y Fedora ya viene incluido casi todo).
- Drivers propietarios (NVIDIA) y módulos DKMS: se avisa de que con Secure Boot requieren inscribir una clave MOK y se deja la decisión al usuario. Nunca se reutilizan paquetes de Windows en Linux.

## 6. Matriz de compatibilidad y riesgos

| Escenario | Estado esperado | Riesgo | Mitigación |
| --- | --- | --- | --- |
| UEFI con Secure Boot (claves de Microsoft) | Rebootera OS arranca con shim de Debian | Equipos que solo aceptan la CA de Windows (sin "3rd party UEFI CA") | Explicar cómo activar la opción en la BIOS; no desactivar Secure Boot en silencio |
| UEFI sin Secure Boot | Sí | Ninguno destacable | |
| BIOS heredada | Sí (GRUB i386-pc) | Equipos muy antiguos sin arranque USB fiable | Documentar |
| ISOs de Windows 10/11 vía Ventoy | Sí | MOK de Ventoy con Secure Boot; cambio de certificados "UEFI CA 2023" | Guía en pantalla antes de reiniciar; probar con OVMF y claves de Microsoft |
| ISOs Linux comunes | Sí en la mayoría | Distros no probadas por Ventoy | Compatibilidad conocida en el catálogo; modo individual como alternativa |
| Intel VMD/RST | Windows no ve el disco sin driver | Alto, es el caso clásico | Inyección del driver de almacenamiento en WinPE |
| Wi-Fi sin driver en Windows | No hay red en la instalación | Medio | Inyección en WinPE o aviso de usar Ethernet |
| Hardware muy nuevo en Rebootera OS | GPU o Wi-Fi sin soporte en kernel 6.12 | Medio | Entrada con kernel de backports; la UI funciona con renderizado por software |
| Equipos ARM (Snapdragon) | No soportado | | Detectar y explicar |
| Discos internos | Nunca se escriben sin plan confirmado | Crítico | Agente con reidentificación, montaje de solo lectura por defecto |
| Cadena de suministro de Ventoy | Binarios de terceros | Medio | Hash verificado, fuente guardada, compilar desde fuente más adelante |

## 7. Estructura del proyecto

```
Rebootera OS/
  README.md
  docs/            PLAN.md, PRUEBAS.md, LIMITACIONES.md
  branding/        logotipo SVG y recursos derivados
  ui/              Rebootera UI (TypeScript, tsc, Prettier)
  core/            módulo Go: cmd/rebootera-core, cmd/rebootera-agent, internal/...
  shell/           rebootera-shell (Python, GTK4 + WebKitGTK)
  os/              configuración de live-build: listas de paquetes, hooks, archivos del sistema, tema de GRUB y Plymouth
  scripts/         construcción y pruebas (build.sh, test-vm.sh)
  desktop/         Rebootera Desktop (hito 7)
  out/             imágenes generadas (no se versiona)
```

## 8. Hitos y pruebas

| Hito | Entregable | Criterio para darlo por terminado |
| --- | --- | --- |
| 1 | ISO híbrida que arranca en UEFI y BIOS y muestra la UI a pantalla completa | Captura de la UI en QEMU con OVMF, con OVMF + Secure Boot y con SeaBIOS; reiniciar y apagar funcionan de verdad |
| 2 | Imagen de USB con la estructura de particiones; detección segura de dispositivos | En un disco virtual: el Vault se monta, el estado persiste tras reiniciar, los discos internos aparecen protegidos |
| 3 | Catálogo, descargas reanudables y verificación | Descarga real de una ISO oficial pequeña con hash verificado, cortada y reanudada |
| 4 | Ventoy integrado, lanzamiento de instaladores | Arranque de una ISO Linux y una de Windows en VM desde el USB virtual |
| 5 | Inventario y análisis de INF | Hardware ID reales de la VM casados con paquetes INF reales |
| 6 | Integración de drivers en Windows y Linux | Instalación en VM con controlador de almacenamiento que exige driver (por ejemplo virtio-scsi en Windows) |
| 7 | Rebootera Desktop | Gestiona el USB virtual desde Windows |
| 8 | Batería de pruebas de arranque, instalación y recuperación | Resultados en PRUEBAS.md |

### Estado de los hitos (v0.4.0)

- **Hitos 1 a 5 y 7: hechos y probados en VM.** El hito 5 se cerró en la v0.3.0: los INF reales de virtio-win (NetKVM, viostor, vioscsi) se casan con los Hardware ID de la VM y la tarjeta de red virtio resulta en `netkvm.inf` por el ID más específico.
- **Hito 6 (drivers en Windows y Linux): hecho en lo que se puede comprobar sin un instalador de Windows.** Se genera el `.7z` de drivers que Ventoy extrae en `X:\rebootera-drivers`, el `autounattend.xml` con las rutas de drivers (`windowsPE` y `offlineServicing`) y las entradas `injection` y `auto_install` de `ventoy.json`, y todo se limpia al volver a Rebootera. **Falta el criterio final** (instalar Windows en una VM con un controlador de disco que exige driver): no se ha probado con un instalador de Windows real porque no hay una ISO de Windows en el entorno de pruebas y Microsoft no ofrece una descarga directa sin sesión. Los drivers de Linux (firmware de Debian/Ubuntu/Fedora) siguen sin integrarse: cada instalador de Linux ya trae casi todo.
- **Hito 8:** la batería está en [PRUEBAS.md](PRUEBAS.md) y crece con cada versión.

### Decisiones de la v0.5.0

- **Una sola opción para escribir el USB:** "Mi USB" duplicaba "Crear USB". Crear USB elige entre borrar un disco (confirmación escrita) y actualizar un USB de Rebootera existente (sin borrar), y en los dos casos copia en el mismo paso las imágenes y carpetas de drivers que el usuario elija. El Resumen y el Asistente de drivers del escritorio solo leen y gestionan la carpeta de drivers; nunca escriben el sistema.
- **Validar antes de tocar:** imágenes, categorías y carpetas de drivers se comprueban antes de borrar o cambiar nada (`desktop.Extras.Validate`). Las copias van a `.part` y se renombran al terminar.
- **Instalación no destructiva en un USB ajeno (Rufus):** no se hace; queda como idea futura con la opción no destructiva de Ventoy (ver LIMITACIONES).

### Decisiones de la v0.4.0

- **Firma de la ISO (Arch y Alpine):** estos proyectos firman cada imagen con una firma separada (`.sig`/`.asc`). Tras comprobar el SHA-256, `transfers.finish` baja la firma por HTTPS, verifica el archivo en streaming con la clave fijada (Arch `3E80…9A5C`, Alpine `0482…495A`) y borra la descarga si falla. Comprobado con las imágenes reales.
- **Teclado:** el agente solo acepta una lista cerrada de distribuciones y las escribe en `/run/rebootera/keyboard`; `start-kiosk` lo lee antes de lanzar cage. El cambio mata el kiosco con SIGKILL (cage ignora SIGTERM) y `Restart=always` lo reabre. Core guarda la elección en el Vault y la reaplica al arrancar.

### Decisiones de la v0.3.0

- **Firmas:** el índice de sumas se verifica con OpenPGP (`github.com/ProtonMail/go-crypto`) contra claves incluidas en el programa y fijadas por huella (Debian `DF9B…BE9B`, Ubuntu `8439…1092`, las cuatro claves de Fedora 43 a 46). Las huellas se comprobaron contra las páginas de verificación de cada proyecto. Arch Linux y Alpine solo firman cada ISO, no el índice: en la v0.3.0 no se verificaban; desde la v0.4.0 se verifica la firma de la ISO.
- **Arranque de una sola vez:** `ventoy_grub.cfg` solo se carga con F6 (se comprobó en el `grub.cfg` de Ventoy 1.1.18), así que no hay forma de hacer "una vez" dentro de Ventoy. Se hace con el firmware: en UEFI el agente reordena `BootOrder` (USB el último) y fija `BootNext`; al volver a Rebootera restaura el orden. Es opcional porque cambia variables NVRAM del firmware.
- **Respuestas automáticas:** nunca incluyen disco (`DiskConfiguration`, `partman`): el usuario elige siempre el disco. Soportado en Windows (`autounattend.xml`) y Debian (preseed). Ubuntu, Fedora, Arch y Alpine quedan fuera: no se han implementado ni probado (autoinstall y kickstart existen, pero para las ISO de escritorio/live del catálogo no hay un camino probado).
- **Wi-Fi:** el perfil de NetworkManager se escribe como archivo 0600 (no por `nmcli ... password`), porque los argumentos de un proceso son visibles para el resto del sistema.

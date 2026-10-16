# Registro de cambios

Todos los cambios importantes de Rebootera OS. Las fechas usan el formato AAAA-MM-DD.
Versión en inglés: [CHANGELOG.md](CHANGELOG.md).

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
  - Ventoy configurado de forma segura: `VTOY_SECURE_BOOT_POLICY=1` y sin saltarse los requisitos de Windows 11 (`VTOY_WIN11_BYPASS_CHECK=0`, `VTOY_WIN11_BYPASS_NRO=0`).
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

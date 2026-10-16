# Limitaciones y errores conocidos

Estado a 2026-10-16, versión 0.2.0.

## Lo que todavía no existe

- **Integración de drivers en Windows (hito 6)**: el asistente de drivers muestra cada dispositivo con su Hardware ID de Windows y marca los críticos (disco y red), pero todavía no busca paquetes INF ni los inyecta en el instalador de Windows. El asistente de instalación lo indica en el paso de drivers.
- **Wi-Fi desde la interfaz**: NetworkManager está incluido y conecta solo por cable con DHCP. No hay pantalla para elegir una red Wi-Fi y escribir su contraseña.
- **Firmas GPG**: las descargas se comprueban con el SHA-256 del índice oficial, obtenido por HTTPS del propio proyecto. La firma GPG de ese índice todavía no se comprueba.
- **Instalación desatendida** (`auto_install` de Ventoy, `autounattend.xml`, preseed, kickstart): no implementada. Se lanza siempre el instalador original, que el usuario completa.
- **Arrancar una sola vez**: Ventoy no tiene arranque de un solo uso. "Instalar" deja la imagen elegida como predeterminada en `ventoy.json` y reinicia; Rebootera restablece su entrada en cuanto vuelve a arrancar. Si el instalador reinicia el equipo y este vuelve a arrancar desde el USB, el menú de Ventoy volverá a lanzar el instalador (en Windows, el instalador continúa desde el disco interno si se quita el USB o se elige el disco en el menú de arranque del equipo).
- **Rebootera Desktop en Linux**: abre la interfaz en el navegador y gestiona un USB ya montado, pero "Crear USB" solo existe en Windows (en Linux está `scripts/make-usb.sh`).
- Distribución de teclado fija en español (`XKB_DEFAULT_LAYOUT=es` en `rebootera-ui.service`); no cambia con el idioma de la interfaz.
- Solo amd64. Equipos ARM (por ejemplo Snapdragon) no arrancan.

## Compatibilidad de imágenes con el menú de arranque

Comprobada en máquina virtual (UEFI) arrancando cada imagen desde un USB de Rebootera; el resultado se muestra en el Centro de descargas y el asistente no deja elegir las incompatibles:

| Imagen | Resultado |
| --- | --- |
| Debian 13.x netinst | Arranca: el instalador gráfico se abre |
| Ubuntu 26.04.1 Desktop | Arranca: escritorio en vivo e instalador |
| Fedora Workstation 44-1.7 | Arranca: escritorio en vivo e instalador |
| Arch Linux 2026.10.01 | Ver PRUEBAS.md |
| Alpine 3.24 (standard y virt) | **No arranca** con Ventoy 1.1.18: en UEFI "cannot load image" y en BIOS "mount /dev/loop0 ... I/O error". Hay que grabarla sola en otro pendrive |
| Ubuntu 24.04, Windows 11 | Sin probar todavía |

Las imágenes añadidas por el usuario no tienen compatibilidad conocida.

## Secure Boot

- Rebootera OS arranca con Secure Boot gracias al shim firmado de Debian cuando la ISO se graba directamente o se arranca desde DVD.
- Desde el USB de Rebootera, el menú lo pone Ventoy. La primera vez en cada equipo con Secure Boot hay que inscribir su clave en MokManager (pantalla azul): flujo probado de principio a fin en máquina virtual.

## Probado solo en máquina virtual

- Todas las pruebas de arranque son en QEMU/KVM con OVMF (UEFI, UEFI con Secure Boot y claves de Microsoft) y SeaBIOS. Falta probar en hardware real: GPUs NVIDIA, AMD e Intel recientes, equipos con Secure Boot que no confían en la CA de terceros de Microsoft y portátiles solo con Wi-Fi.
- El USB emulado de QEMU es muy lento: los tiempos de arranque desde el USB virtual (varios minutos) no representan un pendrive real.
- **"Crear USB" de Rebootera Desktop** no se ha probado con un pendrive real: se ha probado todo menos el paso que borra el disco (descarga y verificación de Ventoy, comprobaciones previas, rechazo de una confirmación incorrecta y de discos que no son USB). Las demás funciones de la aplicación se han probado en Windows 11 con una unidad simulada.
- No se ha probado en VirtualBox ni Hyper-V.

## Decisiones con contrapartida

- `WEBKIT_DISABLE_DMABUF_RENDERER=1`: la interfaz se dibuja con memoria compartida en lugar de DMA-BUF. Es más compatible con GPUs virtuales y drivers básicos, a cambio de algo de rendimiento.
- `WLR_RENDERER_ALLOW_SOFTWARE=1`: sin aceleración gráfica, el compositor usa renderizado por software en lugar de negarse a arrancar.
- Capacidades de la sesión kiosco: `pam_systemd` de systemd 257 concede `CAP_WAKE_ALARM` como capacidad ambiental a las sesiones de usuario, y el sandbox de WebKitGTK (`bubblewrap`) se niega a ejecutarse con cualquier capacidad. La unidad `rebootera-ui.service` la retira con `setpriv` antes de lanzar `cage`.
- `rebootera-agent.service` no usa `PrivateTmp` ni `ProtectHome`: crearían un espacio de montajes privado y los discos que monta el agente no serían visibles para Core.
- La imagen incluye todo el firmware no libre de Debian (`--firmware-chroot true`) para maximizar el hardware compatible. Eso explica buena parte de su tamaño.
- Las imágenes importadas desde otro disco se marcan como "importadas" y no se verifican solas: no hay una suma oficial con la que compararlas. El usuario puede calcular su SHA-256 desde la biblioteca.
- Si dos imágenes tienen el mismo nombre en carpetas distintas, el menú de Ventoy las muestra con el mismo nombre.

## Riesgos abiertos (detalle en PLAN.md)

- Ventoy se usa como binario publicado (verificado con su SHA-256), no compilado desde el código fuente.

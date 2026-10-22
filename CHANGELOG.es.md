# Registro de cambios

Todos los cambios importantes de Rebootera OS. Las fechas usan el formato AAAA-MM-DD.
Versión en inglés: [CHANGELOG.md](CHANGELOG.md).

## [0.20.0] - 2026-10-20

### Añadido

- **La partición pequeña de arranque (VTOYEFI) ya no sale en Windows.** Al crear o actualizar un USB desde Rebootera Desktop se le quita la letra y se le pide a Windows que no se la vuelva a dar en ese equipo.
- **Cambiar el nombre de las particiones** (ext4, NTFS, exFAT y FAT): el nombre se ve en Windows y Linux y no se tocan los archivos.
- **Un navegador ligero que abre al instante.** El botón Navegador ya no arranca Firefox desde el USB (tardaba muchísimo): abre una ventana en el mismo proceso de la interfaz, con atrás, adelante, recargar, barra de direcciones que también busca, barra de carga y botón de cerrar. No se guarda nada (ni cookies ni historial) y no descarga archivos. Firefox, uBlock Origin y Dark Reader salen de la imagen, que además pesa menos.
- **Las redes Wi-Fi se recuerdan.** Al conectar hay un interruptor "Recordar esta red en el USB" (activado por defecto); en el siguiente arranque Rebootera se conecta sola en cuanto el adaptador está listo. Las redes recordadas se marcan en la lista y "Desconectar" las olvida. La contraseña se guarda en el Vault de tu propio USB.
- **Sin internet, la descarga no se queda colgada.** Al pulsar Descargar sin conexión te lleva al panel de Wi-Fi (ya buscando redes) con un aviso, en vez de fallar.
- **Las descargas se reanudan solas.** Una descarga cortada por la red se reintenta cada pocos segundos desde donde se quedó, y la que estaba en marcha al apagar el equipo sigue en el siguiente arranque. Los enlaces de Windows caducan al cabo de un día: Rebootera pide uno nuevo a Microsoft y conserva lo ya descargado.
- **La ventana sigue al tema en Rebootera Desktop:** el icono (barra de título, barra de tareas y Alt+Tab) y el color de la barra de título cambian con el tema, y también el icono de la pestaña en cualquier navegador. El logotipo y la intro ya lo hacían.
- **Los cambios de particiones son más seguros.** Antes de cada cambio se guarda una copia de la tabla de particiones (en memoria y en el Vault del USB); si un paso falla antes de mover datos se restaura la tabla anterior; y no se cambia nada mientras hay una descarga en marcha.

### Cambiado

- **Orden de la barra superior.** Rebootera Desktop: Inicio, Crear USB, Descargar, Drivers, Mis imágenes, Ajustes. Rebootera OS: Inicio, Discos, Instalar, Mis imágenes, Drivers, Descargar, Este PC, Ajustes.
- **El puntero y el teclado se aplican al momento, sin reiniciar la pantalla** (antes cada cambio cerraba y volvía a abrir la interfaz, y en algunos equipos no volvía). El puntero se aplica recargando la configuración del compositor; el teclado tiene las dos distribuciones cargadas y cambia con una tecla Bloq Despl virtual.
- **Panel táctil:** el doble toque ya no espera a un posible arrastre (fallaba a veces) y el sentido del desplazamiento se fija explícitamente para los paneles táctiles.
- **Más fluido en el USB.** Cada sección se lee del USB en segundo plano cuando la interfaz ya está, así que cambiar es instantáneo; mientras una aún se lee, un punto que late muestra que la pulsación se atendió; el botón Navegador dice "Abriendo…". Sin GPU el fondo es plano (sin ruido, viñeta ni sombras desenfocadas), que era lo que más costaba al desplazarse o cambiar de sección.
- **Unir particiones es más claro y mucho más rápido.** Cada opción dice en una línea cuál se queda, cuánto pasa a medir, cuál se borra y si es rápido (no se copia nada) o lento (y cuánto tarda). Si la que se queda está a la derecha, primero se encoge a lo que realmente ocupa y solo se copia eso, en vez de la partición entera (eso era lo que tardaba 20 minutos).
- **Drivers:** más espacio entre bloques y filas. Una partición etiquetada DRIVERS aparece siempre en "dónde se buscan los drivers", aunque solo traiga instaladores y ningún paquete .inf (lo dice).

### Corregido

- **El desenfoque del fondo no funcionaba en Rebootera Desktop:** el modo ligero (siempre activo allí) quitaba todos los filtros SVG, también el del fondo. Ahora además es más fuerte (4, 10 y 22 px).
- Windows 10 y 11 (Home y Pro) se descargan directamente de Microsoft en la página Descargar (las entradas ya estaban en el código pero no en la versión publicada).

## [0.18.0] - 2026-10-20

### Cambiado

- **Instalar ahora son tres pasos claros: Sistema, Dónde, Instalar.** El paso «Dónde» enseña tus discos dibujados a escala y eliges una partición o un hueco libre, no solo un disco entero. Te dice al momento si el sistema cabe y qué pasará («esta partición se formateará» o «no se borra nada»). Si no cabe, **Hacer sitio** lista las particiones que se pueden encoger y abre el cambio de tamaño con la cantidad ya puesta; el avance se ve en la misma pantalla. El último paso dice qué elegir en el instalador original («elige la partición de 200 GB», «elige el Espacio sin asignar de 120 GB») y enseña un plan corto; el paso viejo con la revisión del hardware y la casilla de drivers ya no estorba: los drivers de este equipo van solos al instalador de Windows, si falta alguno se avisa con un botón a la página de Drivers, y todo lo técnico (respuestas automáticas, requisitos de Windows 11, drivers, arrancar una sola vez) está dentro de «Más opciones». Si el equipo no cumple Windows 11, un aviso lo dice con un botón para saltar la comprobación.
- **Discos, en palabras sencillas.** «Cambiar el tamaño» pasa a «Más grande o más pequeña» y dice que lo que quites queda como espacio libre; «Fusionar» pasa a «Unir con la vecina» y cada opción enseña el resultado («una partición de 300 GB») y, en rojo, lo que se borra. El hueco libre tiene «Crear una partición aquí» y «Añadir el espacio a <la partición de antes>», y el mensaje final dice que toques el espacio libre para usarlo. El cambio de tamaño ya no habla de repartir la diferencia salvo que haya vecina, y lo dice como «Dejarla como espacio libre» o «Repartir con <nombre>».
- **Drivers, rehechos.** Una respuesta arriba («Todo listo» o «Drivers que faltan: 1, sin ellos Windows puede no ver el Wi-Fi») y, debajo, lo importante para instalar (red y discos), cada pieza con Listo (y dónde se encontró el driver) o Falta (con un botón para buscarlo en internet con el navegador de Rebootera y, plegados, tres pasos para añadirlo). Los otros dispositivos y dónde se buscan los drivers están plegados. En Rebootera Desktop la página explica en tres pasos cómo funciona, enseña un resumen con aviso si algún paquete no tiene firma válida, agrupa los drivers por carpeta con su tipo (red, discos...) y tiene una ayuda sobre qué drivers hacen falta.
- **El navegador ya no está escondido:** Rebootera OS tiene un botón **Navegador** en la barra superior de todas las páginas (no solo una tarjeta en Inicio).
- Textos más cortos en todas estas páginas, en inglés y en español.

### Corregido

- Una partición abierta (montada) se podía ofrecer para cambiar su tamaño y luego fallaba; el botón ahora dice por qué está desactivado.

## [0.17.0] - 2026-10-20

### Añadido

- **Unraid** y **Synology DSM** (entradas de página oficial: Unraid se hace con su propio creador de USB y DSM solo se instala en los equipos de Synology, y las fichas lo dicen) y **Proxmox Backup Server**.

### Corregido

- **Particiones que a veces no se podían cambiar.** La búsqueda de drivers dejaba abierta en solo lectura cualquier partición con Windows o carpetas de drivers, y cualquier partición abierta bloqueaba su disco entero («cierra las unidades abiertas»); además `ntfsresize` se niega con una partición en uso. Los montajes de solo lectura de Rebootera ya no bloquean el disco y el agente los cierra antes de medir o cambiar nada. Una partición con etiqueta DRIVERS también se queda abierta para la búsqueda (antes se cerraba enseguida si sus archivos estaban sueltos en la raíz).
- **El desenfoque del fondo vuelve a funcionar en Rebootera OS** (el filtro va en cada pieza que se mueve, así la GPU lo reutiliza en vez de filtrar toda la pantalla en cada fotograma).
- **«Este equipo»**: las tarjetas están alineadas en una cuadrícula de 2 por 2 y desaparece la lista larga de «otros dispositivos» (eso es cosa de la página de Drivers); los gráficos van en la tarjeta del procesador. Desaparecen el bloque «Sobre este equipo» de Inicio (duplicado) y el desplegable «Más opciones» de Ajustes: sus ajustes se ven directamente.

### Cambiado

- **Mucho más rápido de abrir, sobre todo desde un USB:** cada página es ahora un archivo que se lee solo al abrirla (la primera pantalla carga unos 140 KB de JavaScript en vez de 220 KB; el resto llega con cada página), el HTML precarga lo que necesita, el texto se envía comprimido y los archivos con huella en el nombre se guardan para siempre. El espacio libre de las particiones montadas se mide en segundo plano y se guarda (las unidades NTFS podían tardar segundos en cada lectura de la página de Discos, que ahora dice «Cargando…» al instante). Lighthouse (perfil móvil, servidor local): rendimiento de 88 a 97, accesibilidad de 96 a 100, buenas prácticas 100. La intro se omite cuando un programa maneja la ventana.

## [0.16.0] - 2026-10-20

### Añadido

- **Más sistemas en el Centro de descargas** (todos comprobados contra sus fuentes reales, con tamaño y SHA-256): **TrueNAS SCALE** (25.10 Goldeye y 25.04 Fangtooth), **XCP-ng** 8.3, **openmediavault**, **VyOS** (rolling), **imágenes de nube de Ubuntu Server** para máquinas virtuales y VPS (24.04 y 22.04), el **Arduino IDE** para Windows, Linux y macOS (Intel y Apple silicon; el programa para programar las placas Arduino, que se guarda en el USB para instalarlo en otro equipo, no es un sistema operativo), y **OPNsense** y **pfSense** como entradas de página oficial (piden elegir un tipo de imagen o rellenar un formulario).
- Un resolvedor nuevo `manifest` para los proyectos que publican un JSON con la última versión y guardan la imagen y su SHA-256 en la carpeta de esa versión (TrueNAS).
- La biblioteca también guarda programas (`.exe`, `.AppImage`, `.dmg`) e imágenes de disco `.qcow2`.

### Cambiado

- **Actualizar Rebootera ahora borra la versión anterior** del USB (y cualquier copia «previous» vieja). La copia nueva se comprueba antes de sustituir a la actual, así que un corte a mitad nunca deja el USB sin sistema. La entrada «versión anterior» ya no está en el menú de arranque.

## [0.15.1] - 2026-10-20

### Corregido

- **El tema elegido se guarda y llega a Rebootera OS.** Rebootera Desktop lo perdía en cada arranque (el almacenamiento del navegador pertenece a una dirección local cuyo puerto cambia cada vez) y, sin USB elegido, no se escribía en ningún sitio. Ahora los ajustes también se guardan en un archivo del PC; al elegir un USB de Rebootera mandan los más recientes y pasan al USB, así que el sistema que arranca de él ya se abre con ese tema, su intro y los colores del menú de arranque.
- **El logo y la animación de la intro siguen el tema en la aplicación de escritorio.** Chromium no repintaba los degradados SVG cuando solo cambiaba la variable del tema; ahora sus colores se fijan en los propios elementos en cada cambio.
- El tema y el idioma guardados van escritos en la primera página, así que nada parpadea con el tema por defecto antes de la intro.

## [0.15.0] - 2026-10-20

### Añadido

- **Crear USB funciona ahora también en Linux y macOS**, no solo en Windows. En Linux usa el paquete oficial de Ventoy (descargado y verificado) con permisos de administrador pedidos por polkit, y monta el USB nuevo con udisks (o con un montaje normal si no hay udisks). En macOS monta la disposición de Ventoy con `diskutil` (GPT, exFAT «REBOOTERA» y la partición de arranque de 32 MiB escrita desde la imagen del propio Ventoy) y pide la contraseña con el diálogo del sistema; un USB hecho en un Mac arranca equipos con UEFI. Solo se ofrecen discos USB externos que no contienen el sistema. Añadir Rebootera a un USB sin borrarlo sigue siendo solo de Windows.
- `scripts/test/test-create-linux.sh` crea un USB en un disco virtual con la aplicación real y lo comprueba (instalación real de Ventoy, etiqueta, GPT, partición de arranque, Rebootera OS byte a byte).

### Cambiado

- La interfaz es ahora un solo JavaScript minimizado y una sola hoja de estilos en vez de unos 30 archivos (cada uno, una lectura del USB): de unos 750 KB a unos 450 KB y muchos menos archivos que abrir al arrancar (esbuild).
- Imagen más pequeña: sin páginas de manual, sin documentación salvo el texto de la licencia de cada paquete y solo con los idiomas del sistema inglés y español.

## [0.14.0] - 2026-10-20

### Añadido

- Pestaña nueva "Este equipo" con todo lo que Rebootera encontró del ordenador (firmware, procesador, memoria, gráficos, discos, red y otros dispositivos con su driver). Sustituye al registro del kernel.
- Si falta el driver de la tarjeta de red, Ajustes lo dice y ofrece buscarlo en los discos del equipo.
- La partición llamada «DRIVERS» (o parecido) se lee entera y gana al almacén de drivers de un Windows instalado (que sale como «solo se usa si nada más encaja»); el instalador de Windows lleva por defecto los drivers que casan.
- «Página oficial» abre el navegador integrado (modo oscuro, sin telemetría); los colores del tema también llegan al menú de Ventoy del USB.
- Panel táctil de portátiles I2C/HID (los módulos se cargan al arrancar).

### Cambiado

- Borrar o modificar una unidad ya no pide escribir una frase: un botón «Continuar» con el aviso la sustituye (también en los cambios de particiones).
- El Wi-Fi es el primer bloque de Ajustes; «Fusionar con una vecina» es solo «Fusionar».
- Los errores de particiones dicen el motivo (Windows hibernado, chkdsk pendiente, unidad en uso, sin espacio, disco con fallos...) con las últimas líneas de la herramienta.
- Los tamaños de las descargas salen al instante desde una caché en disco y se actualizan en segundo plano sin redibujar las tarjetas.
- Rendimiento: el fondo animado se queda quieto por defecto y nunca se desenfoca en movimiento en Rebootera OS; más lectura anticipada del USB. El tema y el idioma guardados van escritos en la primera página, así la intro ya los usa.
- El menú de Ventoy espera 1 s (siempre enseña su menú al menos ese tiempo).
- Menos lag: la interfaz solo se redibuja cuando cambia algo visible (no por el tiempo encendido ni los contadores de tráfico), el USB se consulta cada 6 s (cada 1,5 s solo mientras corre una descarga), el sistema y los sensores cada 6 s, y WebKit no usa desplazamiento animado ni caché de páginas (ni GPU en modo ligero). Las actualizaciones automáticas, las revisiones periódicas de discos y la espera de red se desactivan al arrancar.
- Limpieza: unas 70 reglas de CSS sin uso, los historiales de temperatura y tráfico y el código del registro del kernel han desaparecido.

## [0.13.0] - 2026-10-20

### Cambiado

- La web (`site/`) se rehace con Astro y Tailwind: fondo animado de partículas que sigue al tema, ocho tarjetas de tema en vivo (sin desplegable ni iconos en los botones), cuadrícula de funciones, galería de capturas, descargas por plataforma y preguntas frecuentes. `scripts/dev/site.ps1` ahora la compila antes de servirla y el workflow de sincronización publica `site/dist`.
- `scripts/test/test-all.sh` es rápido por defecto (menos de 3 minutos: las dos ISO en UEFI + Secure Boot, recompilando solo si cambió el código); la batería completa es `--full`. La prueba de la ISO definitiva hacía clic en coordenadas antiguas de la cabecera y no conseguía apagar: arreglado.

## [0.12.0] - 2026-10-20

### Cambiado

- **Rebootera OS arranca del tirón.** La imagen live ya no enseña su menú de arranque ni espera: va directa a la interfaz (mantén Mayús al encender en equipos BIOS para ver el menú con "safe graphics").
- **La sesión gráfica ya no se queda en una pantalla negra.** La interfaz avisa cuando ha dibujado su primer fotograma y el arranque solo da por bueno un nivel de renderizado cuando eso ocurre. Si la ventana se cae o se queda en negro prueba el siguiente nivel (hardware, software, software simple) y, como última salida, vuelve al compositor más simple, `cage`, en lugar de repetir la ventana una y otra vez.
- **Un logotipo completamente nuevo:** un amanecer (un sol saliendo del horizonte bajo dos arcos de señal: el "re" de reiniciar y la "era" de un nuevo comienzo) sobre una ficha oscura redondeada, recoloreado por cada tema, e iconos nuevos para la aplicación, el menú de arranque y Rebootera Desktop.
- **Una intro nueva en lugar de una barra de carga:** amanece: las líneas del horizonte se abren, el sol sube desde detrás, los arcos se trazan y el nombre entra letra a letra; se puede saltar y en equipos modestos es un fundido simple.
- **Ya no hay menú lateral: la navegación es una cabecera con pestañas.** A la izquierda la marca, en el centro las secciones como pestañas (con su nombre o, en ventanas más estrechas, solo el icono) y a la derecha el USB con su espacio libre, el tema, el idioma y la energía. Todo el ancho es para el contenido y Ajustes es la última pestaña.
- **Temas: todos oscuros, Amoled el primero y el de por defecto.** El tema claro (Marfil) desaparece (quien lo tuviera pasa a Amoled) y entra un tema **Naranja** junto a Obsidiana, Zafiro, Esmeralda, Amatista, Brasa y Grafito.
- **Los desplegables se animan:** las secciones plegables y la lista de versiones se abren y se cierran con suavidad en lugar de saltar.
- **"Crear USB" se abre al instante:** la ventana se quedaba vacía varios segundos mientras Windows listaba los discos. Ahora la página sale enseguida ("Buscando pendrives…"), los discos se leen en segundo plano desde que arranca la aplicación y se enseñan en cuanto llegan.
- **La página web sigue los temas de la aplicación** (los mismos ocho temas oscuros, con selector de tema y el logotipo recoloreado) y se puede ver con `scripts/dev/site.ps1`.
- **El tema y el idioma viven solo en Ajustes** (tarjetas con vista previa de cada tema y la elección de idioma), no en un desplegable de la cabecera.
- **La cabecera está arriba del todo** (una hoja de iconos oculta de tamaño cero ocupaba una línea en blanco encima), las tarjetas de descarga miden todas lo mismo con los botones abajo y el botón "Página oficial" es corto.
- **La intro se ve entera en Rebootera Desktop** (antes se recortaba a un tercio de segundo porque la aplicación cuenta como "ligera" para el fondo).
- **Más versiones:** Ubuntu 25.10, Debian 11 y todos los sabores live de Debian 12, Fedora 42 y los spins de la 43, Linux Mint 21.3, Alpine 3.22 y 3.21, Rocky y AlmaLinux 8, NixOS 25.11 y 25.05, CentOS Stream 9, FreeBSD 14.4, OpenBSD 7.8 y 7.7, Proxmox VE 8 y Windows Server 2019 y 2016 (evaluación).
- **Cambiar de idioma ya no lo recarga todo:** la página conserva su sitio y no repite la animación de entrada.
- **Crear USB enseña su progreso al instante** al volver a esa pestaña, en lugar de esperar a que se vuelvan a leer los discos.
- **Los botones de Ajustes ya se quedan seleccionados** al pulsarlos (fondo, desenfoque, tamaño, menú).
- **`out/` solo lleva la ISO y el .exe.** Los binarios de Linux y macOS van a `dist/`, que usa la publicación automática.

### Corregido

- **Pantalla negra con solo el puntero del ratón y pantallazos.** Dos causas. Core aplicaba en cada arranque unos ajustes de puntero "guardados" que no existían (todo a cero: velocidad 0, sin toque para clic), lo que reiniciaba a la fuerza la interfaz un segundo después de abrirla y dejaba el panel táctil sin toque para clic. Y al reiniciar la sesión, el compositor antiguo sobrevivía (systemd lo saca del grupo del servicio) y se quedaba con la pantalla, así que el nuevo no podía dibujar. Las dos están corregidas, con pruebas.

### Añadido

- **Seis temas más, todos oscuros:** se mantiene Zafiro y se suman Esmeralda, Amatista, Brasa, Grafito y Amoled a Obsidiana. El menú de temas enseña una miniatura de cada uno.
- **Desenfoque del fondo configurable** (ninguno, suave, medio, fuerte) para calmar el fondo animado; se apaga solo en equipos modestos.
- **Pestaña Placas e IoT** con Raspberry Pi OS (seis ediciones), Ubuntu para Raspberry Pi (26.04 y 24.04, server y desktop), DietPi (Raspberry Pi 1 a 5, ODROID-N2 y máquinas virtuales), OpenWrt (PC y Raspberry Pi 4) y Home Assistant OS (Raspberry Pi 3/4/5, Green y genérica). Se comprueban con su SHA-256 publicado. Arduino no es un sistema operativo, así que no está en la lista.
- **Más versiones de sistemas que ya estaban:** Ubuntu 22.04 (desktop y server), Debian 12 (netinst y live GNOME, KDE y Xfce), Fedora 43, Linux Mint LMDE 7, AlmaLinux 9 y Windows 10 LTSC 2021 en inglés.
- **Nombre y foto para tus imágenes:** "Personalizar" en Mis imágenes permite renombrar una imagen y ponerle una foto (reducida a 128 px; el archivo no se toca). El asistente de instalación enseña el nombre que elegiste.
- Un tipo de fuente nuevo ("último enlace + suma") para imágenes cuyo enlace estable redirige al archivo más reciente y publica su SHA-256 al lado.

## [0.11.0] - 2026-10-19

### Cambiado

- **Logotipo e iconos nuevos.** La marca es ahora una "R" dentro de un anillo con flecha de reinicio en latón, dibujada una sola vez y recoloreada por cada tema. Se redibujaron todos los iconos de la interfaz (también los de los discos) y el menú de arranque, la pantalla de arranque y el icono de Rebootera Desktop usan la misma marca.
- **Menú de arranque nuevo (UEFI/GRUB y Ventoy).** Tema rehecho con el logotipo, barras de selección redondeadas de 9 trozos, un título claro y las teclas a la vista; la ISO live arranca directa al kiosco.
- **Inicio es un lanzador.** Una barra de búsqueda y una lista corta de acciones con atajos de teclado (flechas, Intro, números); al escribir el nombre de un sistema se ofrece descargarlo. Sustituye a la cuadrícula de tarjetas.
- **Botones cuadrados.** Botones, etiquetas e insignias usan un radio de 8–11 px en lugar de píldoras.
- **Selector de tema con vistas previas de verdad.** La barra superior enseña una miniatura del tema actual y abre una lista con una ventana en miniatura de cada uno; Ajustes usa las mismas miniaturas.
- **Descargar: selector de versión propio** en lugar de la lista desplegable del navegador (edición, tamaño y marca "Recomendada").
- **Crear USB (Rebootera Desktop) simplificado.** Cada USB sale una sola vez. Lo eliges y decides qué hacer: actualizar Rebootera (si ya lo tiene), añadir Rebootera conservando tus archivos o borrar y empezar de cero. Rebootera sale de la última versión (descargada y comprobada) o de tu propia ISO de Rebootera. Añadir imágenes y carpetas de drivers salió de esta pantalla.
- **Gestor de particiones simplificado** a tres acciones: crear/cambiar tamaño, unir con una vecina, borrar. Desaparecen las opciones duplicadas de repartir espacio; el diálogo de cambiar tamaño dice qué hace. Nombres como `basic\x20data\x20partition` se muestran como "Datos" y los límites de NTFS (hibernación, inicio rápido, BitLocker, volumen sucio) se explican en lugar de fallar sin decir nada.
- **Más fluido y ligero.** Transiciones entre páginas con entrada escalonada, fondo animado con más movimiento (órbitas y haces de luz que barren, solo transform, sin manchas), sondeos que se paran con la ventana oculta y se espacian en equipos modestos, y fondo quieto automático en equipos de 4 núcleos o menos. Se quitó cerca del 20 % de la hoja de estilos, código sin usar y textos sin usar.
- **"Windows de Rufus" pasa a ser "Medios de instalación".** Se puede añadir cualquier instalación hecha con cualquier herramienta (Rufus, Ventoy, una carpeta, una ISO).
- **Estructura del proyecto.** Los scripts se agrupan en `scripts/build`, `scripts/usb`, `scripts/test` y `scripts/dev`; la ventana del kiosco pasó a `os/shell`. Se eliminaron los documentos de plan y limitaciones; lo que describían está hecho o figura en el README.

### Añadido

- **Los drivers se encuentran solos.** En cada arranque Rebootera busca paquetes de drivers de Windows en el USB, en carpetas de drivers de las particiones de este equipo (las que dejan los fabricantes, como una partición DRIVERS o SWSetup) y en un Windows instalado, siempre en solo lectura. La página de Drivers enseña de dónde sale cada paquete y tiene "Buscar otra vez".
- **Ajustes del puntero en el sistema live:** velocidad del puntero, toque para hacer clic (activado por defecto; antes el panel táctil exigía una pulsación fuerte) y desplazamiento natural, guardados en el USB.
- **Un navegador dentro de Rebootera OS:** Firefox, solo para navegar, con uBlock Origin y Dark Reader instalados y bloqueados, sin tienda de extensiones, sin sincronización y sin telemetría. El kiosco ahora funciona sobre labwc (un compositor ligero).
- **Descargar desde un enlace:** pega un enlace https a una imagen, o a un elemento del Internet Archive, y elige el archivo; se guarda en el USB como sin comprobar (el SHA-1 se verifica cuando el Archive lo publica).
- **Más personalización:** fondo animado (automático, siempre, nunca), tamaño de la interfaz (90–130 %), ajustes del puntero, tema, idioma, teclado y Wi-Fi.
- **Rebootera Desktop para Linux y macOS** (x64 y arm64, un solo binario cada uno). Se abre en su propia ventana si hay un navegador basado en Chromium, usa zenity/kdialog (Linux) o AppleScript (macOS) para los diálogos de archivos y detecta las unidades montadas. Crear el USB desde la aplicación sigue siendo solo de Windows (en Linux usa `scripts/usb/make-usb.sh`); crear una ISO desde una carpeta no está disponible en macOS.
- Opción de privacidad de Windows: `ReduceTelemetry` es ahora opcional por instalación y los bloques de idioma son opcionales.

### Corregido

- Rebootera Desktop para Windows ya no enseña la dirección abajo a la izquierda al pasar el ratón por el menú.
- El gestor de particiones podía negarse a cambiar el tamaño del disco o partición base; ahora los motivos son explícitos y se puede cambiar el tamaño en los sistemas de archivos compatibles.

## [0.10.0] - 2026-10-19

### Cambiado

- **Aspecto nuevo, desde cero.** Oscuro, elegante y tranquilo: negro cálido con acentos de latón, títulos con serifa (Fraunces) e Inter para el texto, filetes finos en vez de sombras pesadas, una barra lateral compacta de iconos y un fondo de luces que se mueven muy despacio con algunas motas de polvo (solo mueve capas, así que cuesta casi nada). Las fuentes van incluidas, así que se ve igual sin conexión.
- **Tres temas, que se recuerdan** (también en el USB): Obsidiana (oscuro, latón), Marfil (claro, como papel) y Zafiro (azul profundo). Se cambia con los tres puntos de la barra superior o en Ajustes.
- **Cada sistema tiene su logo** en Descargar (Windows, Ubuntu, Debian, Fedora, Mint, Arch, openSUSE, Kali, Rocky, Alma, Proxmox, FreeBSD, OpenBSD, Alpine, NixOS, Void, CentOS y más; el resto lleva un monograma).
- **Las descargas solo van a tu USB de Rebootera, y lo dice.** Un aviso arriba en Descargar enseña en qué USB se guardan y cuánto espacio queda; sin USB pide conectarlo. Desaparece la "biblioteca en este PC": nunca se descarga nada al disco del ordenador. Rebootera Desktop elige solo tu USB de Rebootera en cuanto se conecta.
- **Inicio** tiene menos tarjetas y más grandes, ahora con "Windows de Rufus".
- **Discos y particiones** rehecho: cada parte enseña lo llena que está, con leyenda de colores; al elegir una parte salen su resumen y tarjetas de acción claras (Cambiar tamaño, Repartir espacio con una vecina, Unir, Borrar, Crear) con una línea cada una y el motivo cuando alguna no se puede.
- **La intro es ligera**: sin capas desenfocadas ni mezclas (eso era lo que iba a tirones), solo un fundido y una barra fina; unos 1,2 s como mínimo y casi instantánea en equipos sin aceleración gráfica.

### Añadido

- **Reducir la telemetría de Windows** (Instalar, Opciones, en imágenes de Windows): al instalar pone a cero las directivas de datos de diagnóstico, desactiva los servicios DiagTrack y dmwappushservice, quita anuncios y contenido sugerido y se salta la pantalla de configuración rápida, sin tocar el idioma ni los discos. Funciona del todo en Enterprise, Education y LTSC (o sea, Windows 11 LTSC); en Home y Pro Windows mantiene su nivel básico.
- **Windows hecho con Rufus se añade más fácil**: "Windows de Rufus" en Inicio y arriba en Mis imágenes. Conecta el USB que hizo Rufus (o elige su carpeta) y se convierte en una ISO arrancable en tu USB de Rebootera, conservando todo lo que Rufus añadió (archivos de respuestas, ajustes de privacidad).
- **Internet Archive** como fuente: FreeDOS 1.2, Haiku R1/beta4, Tiny Core Linux (CorePlus) y KolibriOS, comprobando tras descargar el SHA-1 que publica el Archive (detecta copias dañadas, no falsificadas; la página lo dice). Pestaña nueva "Otros" para ellos.

### Corregido

- Rebootera Desktop ya no llena el disco del PC con una biblioteca local escondida.

## [0.9.0] - 2026-10-19

### Cambiado

- **Aspecto nuevo: Frutiger Aero.** Paneles de cristal, botones con brillo, un cielo con una colina verde y burbujas que suben despacio (solo mueven capas, así que cuestan casi nada), una barra lateral compacta de iconos y páginas más pequeñas y apretadas. Toda la interfaz se rehízo.
- **Tres temas, que se recuerdan.** Cielo (claro), Noche (cristal oscuro) y Bosque (claro, verde). Se elige con los tres puntos de la barra superior o en Ajustes; la elección se guarda (también en el USB, así que Rebootera OS arranca con él la próxima vez).
- **Ajustes** empieza ahora por las tarjetas de tema; el color de acento desaparece (los temas lo sustituyen).
- **Rebootera Desktop** mantiene el fondo animado (antes allí estaba apagado para ahorrar CPU); el desenfoque y los brillos siguen apagados.

### Corregido

- **Un USB nuevo creado desde Windows usa ahora tabla de particiones MBR** (la de Ventoy por defecto), que arranca en PC antiguos con BIOS y en UEFI. Antes era siempre GPT y el menú de arranque de algunos equipos no lista un USB solo GPT. GPT solo se usa en discos de más de 2 TiB.
- Los enlaces del catálogo con ruta absoluta (algunos espejos de Fedora) ya no rompen la búsqueda de la suma.

### Añadido

- **25 sistemas más** (65 en 24 familias): Ubuntu Budgie, Cinnamon, Unity, Edubuntu y Studio; Debian Live Cinnamon, MATE, LXQt, LXDE y Standard; Fedora Budgie, LXQt, COSMIC, Sway, Silverblue y Kinoite; NixOS 26.05 (mínimo y gráfico); Void Linux (base y Xfce); CentOS Stream 10 (boot y DVD); Proxmox Backup Server y Mail Gateway; Alpine Extended. Cada entrada nueva se resolvió contra los servidores reales; donde el proyecto firma sus sumas se verifica la firma.

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

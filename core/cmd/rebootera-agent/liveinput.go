//go:build linux

// * Cambios de teclado sin reiniciar la interfaz. Reiniciar el kiosco para cada ajuste era lento y, en algunos
// * equipos, la sesión nueva no llegaba a dibujar. La distribución del teclado
// * cambia con una pulsación de Bloq Despl enviada por un teclado virtual (labwc arranca con las dos distribuciones y
// * grp:sclk_toggle, ver start-kiosk).
package main

import (
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// kioskUser es la cuenta de la sesión gráfica.
const kioskUser = "rebootera"

// Constantes de linux/uinput.h y linux/input-event-codes.h.
const (
	uiDevCreate  = 0x5501
	uiDevDestroy = 0x5502
	uiSetEvbit   = 0x40045564
	uiSetKeybit  = 0x40045565
	evSyn        = 0x00
	evKey        = 0x01
	keyScrollLck = 70
	busVirtual   = 0x06
)

// uinputUserDev es struct uinput_user_dev (la forma antigua, válida en todos los núcleos).
type uinputUserDev struct {
	Name      [80]byte
	Bustype   uint16
	Vendor    uint16
	Product   uint16
	Version   uint16
	FFEffects uint32
	Absmax    [64]int32
	Absmin    [64]int32
	Absfuzz   [64]int32
	Absflat   [64]int32
}

// inputEvent es struct input_event de un núcleo de 64 bits.
type inputEvent struct {
	Sec   int64
	Usec  int64
	Kind  uint16
	Code  uint16
	Value int32
}

// * ioctl llama a ioctl con un argumento numérico.
func ioctl(fd uintptr, request uintptr, value uintptr) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, value); errno != 0 {
		return errno
	}
	return nil
}

// * writeEvent escribe un evento de entrada en el teclado virtual.
func writeEvent(file *os.File, kind uint16, code uint16, value int32) error {
	event := inputEvent{Kind: kind, Code: code, Value: value}
	buffer := (*[unsafe.Sizeof(event)]byte)(unsafe.Pointer(&event))[:]
	_, err := file.Write(buffer)
	return err
}

// * pressScrollLock crea un teclado virtual, pulsa y suelta Bloq Despl y lo destruye. Devuelve false si no pudo.
func pressScrollLock() bool {
	file, err := os.OpenFile("/dev/uinput", os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		run("modprobe", "-q", "uinput")
		file, err = os.OpenFile("/dev/uinput", os.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			return false
		}
	}
	defer file.Close()
	fd := file.Fd()
	if ioctl(fd, uiSetEvbit, evKey) != nil || ioctl(fd, uiSetKeybit, keyScrollLck) != nil {
		return false
	}
	var device uinputUserDev
	copy(device.Name[:], "Rebootera teclado virtual")
	device.Bustype = busVirtual
	device.Vendor = 0x1d6b
	device.Product = 0x0104
	device.Version = 1
	buffer := (*[unsafe.Sizeof(device)]byte)(unsafe.Pointer(&device))[:]
	if _, err := file.Write(buffer); err != nil {
		return false
	}
	if ioctl(fd, uiDevCreate, 0) != nil {
		return false
	}
	defer ioctl(fd, uiDevDestroy, 0)
	// * El compositor tarda un instante en ver el dispositivo nuevo: sin esperar, la pulsación se perdería.
	time.Sleep(700 * time.Millisecond)
	for _, value := range []int32{1, 0} {
		if writeEvent(file, evKey, keyScrollLck, value) != nil || writeEvent(file, evSyn, 0, 0) != nil {
			return false
		}
		time.Sleep(40 * time.Millisecond)
	}
	time.Sleep(150 * time.Millisecond)
	return true
}

// * activeLayout dice qué distribución tiene activa ahora la sesión: la primera de XKB_DEFAULT_LAYOUT con la que arrancó
// * labwc (start-kiosk pone la elegida primero) y, si ya se cambió, la última que aplicó este agente a ese mismo labwc.
func activeLayout() (string, bool) {
	out, err := exec.Command("pgrep", "-u", kioskUser, "-x", "labwc").Output()
	if err != nil {
		return "", false
	}
	pid := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	if liveSession.pid == pid && liveSession.layout != "" {
		return liveSession.layout, true
	}
	environ, err := os.ReadFile("/proc/" + pid + "/environ")
	if err != nil {
		return "", false
	}
	for _, entry := range strings.Split(string(environ), "\x00") {
		if value, found := strings.CutPrefix(entry, "XKB_DEFAULT_LAYOUT="); found {
			first, _, _ := strings.Cut(value, ",")
			liveSession = liveState{pid: pid, layout: first}
			return first, true
		}
	}
	return "", false
}

// liveState recuerda la distribución activa del labwc en marcha.
type liveState struct {
	pid    string
	layout string
}

var liveSession liveState

// * switchKeyboardLive pasa la sesión a la otra distribución sin reiniciarla. Devuelve false si no se pudo
// * (entonces el llamador reinicia la interfaz como antes).
func switchKeyboardLive(layout string) bool {
	current, running := activeLayout()
	if !running {
		// * La sesión aún no arrancó: leerá la distribución guardada al empezar.
		return true
	}
	if current == layout {
		return true
	}
	if !pressScrollLock() {
		return false
	}
	log.Printf("teclado: distribución %s aplicada sin reiniciar la interfaz", layout)
	liveSession.layout = layout
	return true
}

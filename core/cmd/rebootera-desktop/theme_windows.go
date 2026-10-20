//go:build windows

// * Tema de la ventana de Rebootera Desktop: la interfaz avisa del tema que tiene puesto (window.rebooteraTheme) y la
// * ventana cambia a juego el icono (barra de título, barra de tareas y Alt+Tab) y el color de la barra de título.
package main

import (
	"log"
	"syscall"
	"unsafe"
)

// themeLook es el aspecto de la ventana en un tema: posición de su icono en el ejecutable y color de la barra de título.
type themeLook struct {
	name string
	// * Color de fondo del tema (--bg de ui/src/styles/lux.css), que también lleva la barra de título.
	red, green, blue uint8
}

// * windowThemes sigue el orden de THEMES (ui/src/prefs.ts) y de scripts/build/make-theme-icons.py: el icono del tema
// * i está en el grupo de iconos número iconResourceID+i del ejecutable.
var windowThemes = []themeLook{
	{"amoled", 0x00, 0x00, 0x00},
	{"obsidian", 0x0b, 0x0d, 0x11},
	{"sapphire", 0x07, 0x0b, 0x17},
	{"emerald", 0x06, 0x0d, 0x0b},
	{"amethyst", 0x0b, 0x08, 0x14},
	{"ember", 0x0f, 0x08, 0x07},
	{"orange", 0x0d, 0x09, 0x05},
	{"graphite", 0x0d, 0x0e, 0x10},
}

// Constantes de Windows para cambiar el icono de una ventana.
const (
	wmSetIcon    = 0x0080
	iconSmall    = 0
	iconBig      = 1
	imageIcon    = 1
	smCxIcon     = 11
	smCyIcon     = 12
	smCxSmIcon   = 49
	smCySmIcon   = 50
	loadDefColor = 0x0000
)

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	procGetModuleHandle = kernel32.NewProc("GetModuleHandleW")
	procLoadImage       = user32.NewProc("LoadImageW")
	procSendMessage     = user32.NewProc("SendMessageW")
	// windowHandle es la ventana principal (la fija openWindow al abrirla).
	windowHandle uintptr
)

// * loadIcon carga un icono del ejecutable con el tamaño que pide el sistema para esa clase de icono.
func loadIcon(id int, widthMetric, heightMetric uintptr) uintptr {
	instance, _, _ := procGetModuleHandle.Call(0)
	width, _, _ := procGetSystemMetrics.Call(widthMetric)
	height, _, _ := procGetSystemMetrics.Call(heightMetric)
	handle, _, _ := procLoadImage.Call(instance, uintptr(id), imageIcon, width, height, loadDefColor)
	return handle
}

// * setWindowTheme es la función que la interfaz llama al cambiar de tema (se enlaza como window.rebooteraTheme).
func setWindowTheme(name string) {
	if windowHandle == 0 {
		return
	}
	for index, look := range windowThemes {
		if look.name != name {
			continue
		}
		if big := loadIcon(iconResourceID+index, smCxIcon, smCyIcon); big != 0 {
			procSendMessage.Call(windowHandle, wmSetIcon, iconBig, big)
		}
		if small := loadIcon(iconResourceID+index, smCxSmIcon, smCySmIcon); small != 0 {
			procSendMessage.Call(windowHandle, wmSetIcon, iconSmall, small)
		}
		captionTo(windowHandle, look)
		return
	}
	log.Printf("tema de ventana desconocido: %q", name)
}

// * captionTo pinta la barra de título del color de fondo del tema.
func captionTo(hwnd uintptr, look themeLook) {
	if procDwmSetWindowAttribute.Find() != nil {
		return
	}
	color := uint32(look.blue)<<16 | uint32(look.green)<<8 | uint32(look.red)
	procDwmSetWindowAttribute.Call(hwnd, dwmCaptionColor, uintptr(unsafe.Pointer(&color)), 4)
}

//go:build windows

// * Ventana de Rebootera Desktop en Windows con WebView2 (el motor de Edge
// * que Windows 10 y 11 incluyen), con el icono de Rebootera y la barra de
// * título oscura a juego con la interfaz. Si WebView2 no está instalado, se
// * abre el navegador predeterminado.
package main

import (
	"log"
	"os/exec"
	"path/filepath"
	"reflect"
	"syscall"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"github.com/jchv/go-webview2/pkg/edge"
)

// * iconResourceID es el grupo de iconos del ejecutable: rsrc da el ID 1 al
// * manifiesto y el 2 al grupo de iconos (ver scripts/build/build-desktop.sh).
const iconResourceID = 2

// Atributos de ventana de DWM (Windows 10 2004+ y Windows 11).
const (
	dwmUseImmersiveDarkMode = 20
	dwmCaptionColor         = 35
	// * Color de la barra de título: el fondo de la interfaz (#070b10) en COLORREF.
	captionColor = 0x00100b07
)

var (
	procDwmSetWindowAttribute = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
	user32                    = syscall.NewLazyDLL("user32.dll")
	procGetDpiForSystem       = user32.NewProc("GetDpiForSystem")
	procGetSystemMetrics      = user32.NewProc("GetSystemMetrics")
)

// Tamaño de la ventana en píxeles lógicos (a 96 ppp) y métricas del sistema.
const (
	windowWidth  = 1320
	windowHeight = 860
	smCxScreen   = 0
	smCyScreen   = 1
)

// * windowSize escala el tamaño de la ventana a los ppp del sistema (la
// * aplicación declara PerMonitorV2) sin pasar del 92 % de la pantalla.
func windowSize() (uint, uint) {
	dpi := uintptr(96)
	if procGetDpiForSystem.Find() == nil {
		if value, _, _ := procGetDpiForSystem.Call(); value > 0 {
			dpi = value
		}
	}
	width := uint(windowWidth * dpi / 96)
	height := uint(windowHeight * dpi / 96)
	screenW, _, _ := procGetSystemMetrics.Call(smCxScreen)
	screenH, _, _ := procGetSystemMetrics.Call(smCyScreen)
	if limit := uint(screenW) * 92 / 100; screenW > 0 && width > limit {
		width = limit
	}
	if limit := uint(screenH) * 92 / 100; screenH > 0 && height > limit {
		height = limit
	}
	return width, height
}

// * openWindow muestra la interfaz y vuelve cuando se cierra la ventana.
func openWindow(url string) {
	width, height := windowSize()
	view := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		// ! Los datos de WebView2 van al equipo, nunca a la carpeta del USB.
		DataPath: filepath.Join(dataDir(), "webview2"),
		WindowOptions: webview2.WindowOptions{
			Title:  "Rebootera Desktop",
			Width:  width,
			Height: height,
			IconId: iconResourceID,
			Center: true,
		},
	})
	if view == nil {
		log.Printf("WebView2 no está disponible; se abre el navegador")
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
		select {}
	}
	defer view.Destroy()
	hideStatusBar(view)
	darkTitleBar(uintptr(view.Window()))
	view.Navigate(url)
	view.Run()
}

// * hideStatusBar quita la barra de estado de WebView2 (el rótulo con la dirección que sale abajo a la izquierda
// * al pasar el ratón por un enlace). La biblioteca no ofrece ese ajuste, así que se alcanza el objeto
// * del navegador que guarda en un campo privado; si algo cambia en una versión futura, no se hace nada.
func hideStatusBar(view webview2.WebView) {
	defer recoverStatusBar()
	field := reflect.ValueOf(view).Elem().FieldByName("chromium")
	if !field.IsValid() {
		return
	}
	chromium, ok := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface().(*edge.Chromium)
	if !ok || chromium == nil {
		return
	}
	settings, err := chromium.GetSettings()
	if err != nil || settings == nil {
		return
	}
	settings.PutIsStatusBarEnabled(false)
}

// * recoverStatusBar absorbe un fallo de hideStatusBar: ocultar el rótulo es un detalle estético y no debe
// * impedir que la aplicación abra.
func recoverStatusBar() {
	if recover() != nil {
		log.Printf("no se pudo ocultar la barra de estado de WebView2")
	}
}

// * darkTitleBar pide a Windows una barra de título oscura del color de la
// * interfaz. En versiones que no lo admiten no hace nada.
func darkTitleBar(hwnd uintptr) {
	if procDwmSetWindowAttribute.Find() != nil {
		return
	}
	enabled := uint32(1)
	procDwmSetWindowAttribute.Call(hwnd, dwmUseImmersiveDarkMode, uintptr(unsafe.Pointer(&enabled)), 4)
	color := uint32(captionColor)
	procDwmSetWindowAttribute.Call(hwnd, dwmCaptionColor, uintptr(unsafe.Pointer(&color)), 4)
}

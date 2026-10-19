//go:build windows

// * Creación de una ISO arrancable (BIOS y UEFI, con archivos de más de 4 GiB) con las interfaces IMAPI de Windows.
package installer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// isoScript crea la ISO con IMAPI v2, que viene con Windows (no hace falta instalar nada ni ser
// administrador): sistemas de archivos ISO 9660 + Joliet + UDF (solo UDF si hay un archivo de más de
// 4 GiB) y las imágenes de arranque BIOS y UEFI. La ISO se escribe directamente en su destino.
const isoScript = `param([string]$Source, [string]$Dest, [string]$Label, [string]$Bios, [string]$Efi, [int]$Udf)
$ErrorActionPreference = 'Stop'
Add-Type -TypeDefinition @'
using System;
using System.IO;
using System.Runtime.InteropServices;
using System.Runtime.InteropServices.ComTypes;
public static class RebooteraIso {
  [DllImport("shlwapi.dll", CharSet = CharSet.Unicode, PreserveSig = false)]
  static extern void SHCreateStreamOnFileEx(string file, uint mode, uint attrs, bool create, IntPtr template, out IStream stream);
  public static IStream Open(string path) {
    IStream stream;
    SHCreateStreamOnFileEx(path, 0, 0, false, IntPtr.Zero, out stream);
    return stream;
  }
  public static void SetBoot(object image, object bios, object efi) {
    System.Collections.Generic.List<object> list = new System.Collections.Generic.List<object>();
    if (bios != null) list.Add(bios);
    if (efi != null) list.Add(efi);
    object[] options = list.ToArray();
    image.GetType().InvokeMember("BootImageOptionsArray", System.Reflection.BindingFlags.SetProperty, null, image, new object[] { options });
  }
  public static void Write(object comStream, string path) {
    IStream source = (IStream)comStream;
    System.Runtime.InteropServices.ComTypes.STATSTG stat;
    source.Stat(out stat, 1);
    long total = stat.cbSize;
    int chunk = 1 << 20;
    byte[] buffer = new byte[chunk];
    IntPtr readPtr = Marshal.AllocHGlobal(sizeof(int));
    try {
      using (FileStream output = new FileStream(path, FileMode.Create, FileAccess.Write, FileShare.None, chunk)) {
        long written = 0;
        while (written < total) {
          int want = (int)Math.Min((long)chunk, total - written);
          source.Read(buffer, want, readPtr);
          int got = Marshal.ReadInt32(readPtr);
          if (got <= 0) break;
          output.Write(buffer, 0, got);
          written += got;
        }
        output.Flush(true);
      }
    } finally {
      Marshal.FreeHGlobal(readPtr);
      Marshal.ReleaseComObject(source);
    }
  }
}
'@
function New-BootOption([string]$Path, [int]$Platform) {
  $option = New-Object -ComObject IMAPI2FS.BootOptions
  $option.AssignBootImage([RebooteraIso]::Open($Path))
  $option.PlatformId = $Platform
  $option.Emulation = 0
  return $option
}
$image = New-Object -ComObject IMAPI2FS.MsftFileSystemImage
$image.FileSystemsToCreate = $(if ($Udf -eq 1) { 4 } else { 7 })
$image.FreeMediaBlocks = 0
$image.VolumeName = $Label
$biosOption = $null
$efiOption = $null
if ($Bios) { $biosOption = New-BootOption (Join-Path $Source $Bios) 0 }
if ($Efi) { $efiOption = New-BootOption (Join-Path $Source $Efi) 239 }
if ($biosOption -or $efiOption) { [RebooteraIso]::SetBoot($image, $biosOption, $efiOption) }
$image.Root.AddTree($Source, $false)
$result = $image.CreateResultImage()
[RebooteraIso]::Write($result.ImageStream, $Dest)
`

// * MakeISO crea una ISO arrancable a partir de la carpeta de un instalador de Windows con IMAPI.
func MakeISO(ctx context.Context, info Info, dest string, reporter Reporter) error {
	script, err := os.CreateTemp("", "rebootera-iso-*.ps1")
	if err != nil {
		return err
	}
	defer os.Remove(script.Name())
	// * Con BOM: Windows PowerShell 5 lee así el UTF-8.
	if _, err := script.WriteString("\xef\xbb\xbf" + isoScript); err != nil {
		script.Close()
		return err
	}
	script.Close()
	udf := 0
	if info.LargeFile {
		udf = 1
	}
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script.Name(),
		"-Source", filepath.Clean(info.Root), "-Dest", dest, "-Label", info.Label,
		"-Bios", filepath.FromSlash(info.BIOSBoot), "-Efi", filepath.FromSlash(info.UEFIBoot), "-Udf", strconv.Itoa(udf))
	// * Sin consola: Rebootera Desktop no tiene y PowerShell abriría una ventana negra.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	var output strings.Builder
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		return err
	}
	finished := make(chan struct{})
	go watchSize(dest, reporter, finished)
	err = cmd.Wait()
	close(finished)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("IMAPI: %v: %s", err, tailOf(output.String()))
	}
	if reporter != nil {
		reporter.Report(info.Bytes)
	}
	return nil
}

// * watchSize publica cada segundo el tamaño que lleva escrito el archivo de destino.
func watchSize(path string, reporter Reporter, finished chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-finished:
			return
		case <-ticker.C:
			if stat, err := os.Stat(path); err == nil && reporter != nil {
				reporter.Report(stat.Size())
			}
		}
	}
}

// * tailOf devuelve el final de la salida de PowerShell para el mensaje de error.
func tailOf(text string) string {
	text = strings.TrimSpace(text)
	if len(text) > 2500 {
		text = text[len(text)-2500:]
	}
	if text == "" {
		return "sin salida"
	}
	return text
}

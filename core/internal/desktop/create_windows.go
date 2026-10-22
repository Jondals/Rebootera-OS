//go:build windows

// * Creación de un USB de Rebootera desde Windows:
// *   1. Descarga Ventoy para Windows de su release oficial y comprueba su SHA-256.
// *   2. Ejecuta Ventoy2Disk en modo línea de órdenes (VTOYCLI) como
// *      administrador sobre el disco USB elegido: tabla MBR (arranca en BIOS y UEFI) y Secure Boot.
// *   3. Renombra la partición de datos a REBOOTERA (la CLI de Ventoy no tiene
// *      opción de etiqueta).
// *   4. Copia la estructura del USB, la configuración y Rebootera OS.
// ! El paso 2 BORRA el disco (salvo en el modo "conservar", que usa la instalación sin
// ! borrar de Ventoy: /NonDest). Solo se aceptan discos USB que no son de
// ! sistema ni de arranque, y la interfaz exige confirmación explícita.
package desktop

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"rebootera/core/internal/release"
	"strconv"
	"strings"
	"time"
)

// maxMBRBytes es lo que direcciona una tabla MBR (2 TiB).
const maxMBRBytes = 2 << 40

// ventoyZip es el archivo de Ventoy para Windows.
const ventoyZip = "ventoy-" + ventoyVersion + "-windows.zip"

// * Create empieza a crear un USB de Rebootera en segundo plano.
func Create(diskNumber int, isoPath string, kit fs.FS, desktopExe string, extras Extras, keepData bool) error {
	if err := begin(); err != nil {
		return err
	}
	// ! Las imágenes y carpetas de drivers también se validan antes de borrar nada.
	if err := extras.Validate(); err != nil {
		finish("", err)
		return err
	}

	// ! Todo se comprueba ANTES de borrar el pendrive: si la ISO no sirve, el
	// ! USB no debe quedar borrado y sin sistema.
	// * Sin ISO elegida, se descarga la última versión publicada (ver steps).
	if isoPath != "" {
		if info, err := os.Stat(isoPath); err != nil || !info.Mode().IsRegular() || !strings.EqualFold(filepath.Ext(isoPath), ".iso") {
			err := errors.New("elige una imagen .iso de Rebootera OS que exista")
			finish("", err)
			return err
		}
	}
	if kit == nil {
		err := errors.New("falta el contenido del USB dentro de la aplicación")
		finish("", err)
		return err
	}

	disks, err := USBDisks()
	if err != nil {
		finish("", err)
		return err
	}
	allowed := false
	var diskSize uint64
	for _, d := range disks {
		if d.Number == diskNumber {
			allowed = true
			diskSize = d.SizeBytes
		}
	}
	if !allowed {
		err := fmt.Errorf("el disco %d no es un disco USB disponible", diskNumber)
		finish("", err)
		return err
	}
	// ! Conservar los archivos solo se admite en un disco que lo permite (NTFS o exFAT con datos).
	if keepData {
		adopt := false
		for _, d := range disks {
			if d.Number == diskNumber && d.Adoptable {
				adopt = true
			}
		}
		if !adopt {
			err := fmt.Errorf("al disco %d no se puede añadir Rebootera sin borrarlo: su primera partición no es NTFS ni exFAT", diskNumber)
			finish("", err)
			return err
		}
	}
	job := &createJob{disk: diskNumber, size: diskSize, iso: isoPath, kit: kit, desktopExe: desktopExe, extras: extras, keepData: keepData}
	go job.run()
	return nil
}

// createJob guarda los datos de una creación.
type createJob struct {
	disk       int
	size       uint64
	iso        string
	kit        fs.FS
	desktopExe string
	extras     Extras
	keepData   bool
}

// * run ejecuta todos los pasos e informa del resultado.
func (j *createJob) run() {
	drive, err := j.steps()
	finish(drive, err)
}

// * steps realiza la creación paso a paso.
func (j *createJob) steps() (string, error) {
	// ! La descarga va antes de tocar el disco: si falla, el USB no se borra.
	if j.iso == "" {
		setPhase("download", 0)
		path, err := release.Fetch(release.CacheDir(), downloadProgress, control)
		if err != nil {
			return "", err
		}
		if err := downloadCancelled(); err != nil {
			return "", err
		}
		j.iso = path
	}
	setPhase("ventoy-download", 0)
	ventoyDir, err := ensureVentoy()
	if err != nil {
		return "", err
	}

	setPhase("ventoy-install", 0)
	done := filepath.Join(ventoyDir, "cli_done.txt")
	os.Remove(done)
	// * MBR (el estilo por defecto de Ventoy) arranca en equipos con BIOS clásica y en UEFI: es lo más
	// * compatible, y muchos menús de arranque no ofrecen un USB solo GPT en BIOS. GPT solo hace falta
	// * en discos de más de 2 TiB, donde MBR no llega.
	args := fmt.Sprintf("VTOYCLI /I /PhyDrive:%d", j.disk)
	if j.size > maxMBRBytes {
		args += " /GPT"
	}
	if j.keepData {
		// * Sin /GPT: se respeta el estilo de particiones que ya tenga el disco. Ventoy mismo
		// * encoge la primera partición si hace falta sitio para su partición de arranque.
		args = fmt.Sprintf("VTOYCLI /I /PhyDrive:%d /NonDest", j.disk)
	}
	if _, err := RunElevated(filepath.Join(ventoyDir, "Ventoy2Disk.exe"), args, ventoyDir); err != nil {
		return "", err
	}
	if err := waitVentoy(ventoyDir, done); err != nil {
		return "", err
	}

	setPhase("label", 0)
	letter, err := waitDriveLetter(j.disk)
	if err != nil {
		return "", err
	}
	if _, err := RunElevated("powershell.exe", "-NoProfile -Command Set-Volume -DriveLetter "+letter+" -NewFileSystemLabel REBOOTERA", ventoyDir); err != nil {
		return "", err
	}
	root := letter + `:\`
	// * La partición de arranque de Ventoy (VTOYEFI, 32 MB) no hace falta verla: se le quita la letra y se le pide a Windows
	// * que no se la vuelva a dar. Si falla no es grave: solo sería un disco de más en el Explorador.
	if _, err := RunElevated("powershell.exe", "-NoProfile -Command "+hideBootPartitionScript(j.disk), ventoyDir); err != nil {
		log.Printf("no se pudo ocultar la partición de arranque: %v", err)
	}

	setPhase("copy-kit", 0)
	if err := WriteKit(root, j.kit); err != nil {
		return "", err
	}
	if j.desktopExe != "" {
		copyFile(j.desktopExe, filepath.Join(root, "Rebootera Desktop.exe"))
	}

	setPhase("copy-system", 0)
	if err := InstallSystem(root, j.iso, systemProgress); err != nil {
		return "", err
	}

	// * Imágenes y drivers elegidos antes de crear el USB: salen ya dentro, sin mover nada a mano.
	if err := ApplyExtras(root, j.extras); err != nil {
		return root, err
	}
	return root, nil
}

// * hideBootPartitionScript es el comando de PowerShell que oculta las particiones pequeñas (la de arranque de Ventoy) de un
// * disco: diskpart marca el volumen para que Windows no le dé letra en este equipo y le quita la que tenga, y Set-Partition
// * lo marca también en la propia tabla (GPT).
func hideBootPartitionScript(disk int) string {
	return fmt.Sprintf("\"Get-Partition -DiskNumber %d | Where-Object { $_.Size -le 64MB } | ForEach-Object { $l = $_.DriveLetter; if ($l) { ('select volume ' + $l), 'attributes volume set nodefaultdriveletter', ('remove letter=' + $l) | Out-File -Encoding ascii $env:TEMP\vt.txt; diskpart /s $env:TEMP\vt.txt | Out-Null }; try { Set-Partition -InputObject $_ -NoDefaultDriveLetter $true } catch {} }\"", disk)
}

// * PrepareVentoy descarga, verifica y extrae Ventoy sin tocar ningún disco
// * (lo usa la opción -prepare-ventoy para comprobarlo por separado).
func PrepareVentoy() (string, error) {
	return ensureVentoy()
}

// * ensureVentoy descarga, verifica y extrae Ventoy para Windows en la
// * carpeta de datos del usuario. Devuelve la carpeta con Ventoy2Disk.exe.
func ensureVentoy() (string, error) {
	dir, err := cacheRoot()
	if err != nil {
		return "", err
	}
	target := filepath.Join(dir, "ventoy-"+ventoyVersion)
	if _, err := os.Stat(filepath.Join(target, "Ventoy2Disk.exe")); err == nil {
		return target, nil
	}
	zipPath, err := fetchVentoyArchive(ventoyZip)
	if err != nil {
		return "", err
	}
	if err := unzip(zipPath, dir); err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(target, "Ventoy2Disk.exe")); err != nil {
		return "", errors.New("el zip de Ventoy no tiene la estructura esperada")
	}
	return target, nil
}

// * unzip extrae un zip sin permitir rutas que salgan de la carpeta destino.
func unzip(zipPath, dest string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		target := filepath.Join(dest, filepath.FromSlash(file.Name))
		// ! Protección frente a rutas con ".." dentro del zip (zip slip).
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(filepath.Separator)) {
			return fmt.Errorf("ruta no válida en el zip: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			os.MkdirAll(target, 0o755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			in.Close()
			return err
		}
		_, err = io.Copy(out, in)
		in.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// * waitVentoy espera a cli_done.txt (0 = éxito, 1 = fallo) mostrando el
// * porcentaje de cli_percent.txt.
func waitVentoy(dir, done string) error {
	deadline := time.Now().Add(20 * time.Minute)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(done); err == nil {
			if strings.TrimSpace(string(data)) == "0" {
				return nil
			}
			log, _ := os.ReadFile(filepath.Join(dir, "cli_log.txt"))
			return fmt.Errorf("Ventoy no pudo preparar el USB: %s", lastLines(string(log), 3))
		}
		if data, err := os.ReadFile(filepath.Join(dir, "cli_percent.txt")); err == nil {
			if percent, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
				setPhase("ventoy-install", float64(percent)/100)
			}
		}
		time.Sleep(time.Second)
	}
	return errors.New("Ventoy no terminó en 20 minutos")
}

// * waitDriveLetter espera a que Windows asigne letra a la partición 1 del disco.
func waitDriveLetter(disk int) (string, error) {
	script := fmt.Sprintf("(Get-Partition -DiskNumber %d -PartitionNumber 1).DriveLetter", disk)
	for i := 0; i < 30; i++ {
		out, err := quiet(exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)).Output()
		letter := strings.TrimSpace(string(out))
		if err == nil && len(letter) == 1 {
			return letter, nil
		}
		time.Sleep(time.Second)
	}
	return "", errors.New("Windows no asignó una letra al nuevo USB")
}

// * Package boot: elige qué imagen arranca Ventoy la próxima vez. Edita
// * ventoy/ventoy.json del Vault cambiando solo VTOY_DEFAULT_IMAGE, VTOY_MENU_TIMEOUT y la lista de imágenes del
// * menú, y respeta el resto de la configuración.
// * El menú del USB solo enseña Rebootera OS (y otra versión de Rebootera si hay una instalada al lado): las
// * imágenes de la biblioteca (Windows, Linux...) no salen en él, se arrancan desde el asistente de instalación, que
// * añade al menú solo la elegida mientras dura esa instalación.
// * Cuando Rebootera OS vuelve a arrancar, Restore deja otra vez Rebootera OS
// * como imagen por defecto y el menú solo con Rebootera.
// ! Ventoy no tiene "arrancar una sola vez": mientras Rebootera no vuelva a
// ! arrancar, el menú de Ventoy seguirá preseleccionando la imagen elegida
// ! (con cuenta atrás, así que siempre se puede elegir otra entrada).
package boot

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"rebootera/core/internal/vault"
)

// Valores de Ventoy que usa Rebootera.
const (
	RebooteraImage  = "/rebootera/rebootera-os.iso"
	defaultTimeout  = "1"
	installTimeout  = "10"
	keyDefaultImage = "VTOY_DEFAULT_IMAGE"
	keyMenuTimeout  = "VTOY_MENU_TIMEOUT"
	// keyImageList es el plugin de Ventoy que limita las imágenes que salen en el menú.
	keyImageList = "image_list"
	keyAlias     = "menu_alias"
)

// rebooteraDir es la carpeta del USB (dentro de /rebootera) donde viven las versiones de Rebootera OS.
const rebooteraDir = "rebootera"

// * configPath es ventoy/ventoy.json dentro del Vault.
func configPath() string {
	return vault.Path("ventoy", "ventoy.json")
}

// * readConfig lee ventoy.json conservando todas sus claves.
func readConfig() (map[string]any, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer la configuración de arranque: %w", err)
	}
	config := map[string]any{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("la configuración de arranque está dañada: %w", err)
	}
	return config, nil
}

// * setControl cambia (o añade) una clave de la lista "control" de Ventoy.
func setControl(config map[string]any, key, value string) {
	list, _ := config["control"].([]any)
	for _, item := range list {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if _, exists := entry[key]; exists {
			entry[key] = value
			return
		}
	}
	config["control"] = append(list, map[string]any{key: value})
}

// * getControl devuelve el valor de una clave de "control".
func getControl(config map[string]any, key string) string {
	list, _ := config["control"].([]any)
	for _, item := range list {
		if entry, ok := item.(map[string]any); ok {
			if value, ok := entry[key].(string); ok {
				return value
			}
		}
	}
	return ""
}

// * writeConfig guarda ventoy.json con una copia del anterior.
func writeConfig(config map[string]any) error {
	if data, err := os.ReadFile(configPath()); err == nil {
		os.WriteFile(configPath()+".bak", data, 0o640)
	}
	return vault.WriteJSON(configPath(), config)
}

// * ownImages lista las imágenes de Rebootera OS que hay en el USB: la actual y, si hay alguna más a su lado (otra versión
// * que se dejó al instalar encima), esas también. Son las únicas que salen en el menú de Ventoy en reposo.
func ownImages() []string {
	return ownImagesIn(vault.Path(rebooteraDir))
}

// * ownImagesIn es ownImages para la carpeta rebootera/ de un USB cualquiera (Rebootera Desktop no tiene Vault montado).
func ownImagesIn(dir string) []string {
	list := []string{RebooteraImage}
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		name := entry.Name()
		if entry.Type().IsRegular() && strings.EqualFold(filepath.Ext(name), ".iso") && name != filepath.Base(RebooteraImage) {
			list = append(list, "/"+rebooteraDir+"/"+name)
		}
	}
	return list
}

// * setMenu deja en el menú de Ventoy solo las imágenes de Rebootera OS y, si se indica, la imagen extra (la que se va a
// * instalar). A las otras versiones de Rebootera les pone un nombre que se entienda.
func setMenu(config map[string]any, own []string, extra string) {
	images := own
	if extra != "" {
		images = append(images, extra)
	}
	list := make([]any, 0, len(images))
	for _, image := range images {
		list = append(list, image)
	}
	config[keyImageList] = list
	aliases, _ := config[keyAlias].([]any)
	known := map[string]bool{}
	for _, item := range aliases {
		if entry, ok := item.(map[string]any); ok {
			if image, ok := entry["image"].(string); ok {
				known[image] = true
			}
		}
	}
	for _, image := range images[1:] {
		if known[image] || image == extra {
			continue
		}
		name := strings.TrimSuffix(filepath.Base(image), filepath.Ext(image))
		alias := "Rebootera OS (" + name + ")"
		if strings.Contains(name, "previous") {
			alias = "Rebootera OS (previous version)"
		}
		aliases = append(aliases, map[string]any{"image": image, "alias": alias})
	}
	if len(aliases) > 0 {
		config[keyAlias] = aliases
	}
}

// * menuIs indica si el menú de Ventoy ya enseña exactamente las imágenes de Rebootera OS (sin extra).
func menuIs(config map[string]any, want []string) bool {
	list, ok := config[keyImageList].([]any)
	if !ok || len(list) != len(want) {
		return false
	}
	for index, item := range list {
		if value, _ := item.(string); value != want[index] {
			return false
		}
	}
	return true
}

// * RestrictMenu hace que el menú del USB solo enseñe Rebootera OS. Es para USB hechos con versiones anteriores, donde el
// * menú enseñaba todas las imágenes de la biblioteca: lo llama Rebootera Desktop al actualizar un USB.
// ! Respeta lo demás de ventoy.json y, si el menú ya estaba así, no escribe nada.
func RestrictMenu(configFile string) error {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}
	config := map[string]any{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("la configuración de arranque está dañada: %w", err)
	}
	own := ownImagesIn(filepath.Join(filepath.Dir(filepath.Dir(configFile)), rebooteraDir))
	if menuIs(config, own) {
		return nil
	}
	setMenu(config, own, "")
	os.WriteFile(configFile+".bak", data, 0o640)
	return vault.WriteJSON(configFile, config)
}

// Prep son los extras de una instalación: archivo de respuestas, drivers
// inyectados y el atajo de requisitos de Windows 11.
type Prep struct {
	// Answers es el contenido del archivo de respuestas ("" = ninguno) y
	// AnswersName su nombre ("autounattend.xml" o "preseed.cfg").
	Answers     string
	AnswersName string
	// DriversArchive es un 7z ya creado dentro de ventoy/rebootera/ ("" = ninguno).
	DriversArchive string
	// WinBypass quita las comprobaciones de TPM, Secure Boot y RAM de Windows 11.
	WinBypass bool
	// WinNoOnline quita solo la exigencia de cuenta de Microsoft en línea (OOBE).
	WinNoOnline bool
}

// Rutas y claves de Ventoy que Rebootera gestiona. Todo lo que Rebootera crea
// vive en ventoy/rebootera/ y se borra entero al volver a arrancar.
const (
	workDirName         = "rebootera"
	workPathPrefix      = "/ventoy/rebootera/"
	keyAutoInstall      = "auto_install"
	keyInjection        = "injection"
	keyBypassCheck      = "VTOY_WIN11_BYPASS_CHECK"
	keyBypassNRO        = "VTOY_WIN11_BYPASS_NRO"
	driversArchive      = "drivers.7z"
	bypassOn, bypassOff = "1", "0"
)

// * WorkDir es la carpeta de trabajo de Rebootera dentro de ventoy/ del Vault.
func WorkDir() string {
	return vault.Path("ventoy", workDirName)
}

// * ArchivePath devuelve la ruta del 7z de drivers dentro del Vault y la
// * misma ruta tal como la ve Ventoy.
func ArchivePath() (string, string) {
	return filepath.Join(WorkDir(), driversArchive), workPathPrefix + driversArchive
}

// * Prepare hace que Ventoy preseleccione la imagen rel y, si se pide, le
// * entregue el archivo de respuestas y los drivers.
func Prepare(rel string, prep Prep) error {
	if !vault.Available() {
		return vault.ErrUnavailable
	}
	full, err := vault.Resolve(vault.IsosDir(), strings.TrimPrefix(rel, "isos/"))
	if err != nil {
		return err
	}
	if info, err := os.Stat(full); err != nil || !info.Mode().IsRegular() {
		return errors.New("la imagen no existe en la biblioteca")
	}
	config, err := readConfig()
	if err != nil {
		return err
	}
	ventoyPath := "/" + filepath.ToSlash(strings.TrimPrefix(full, vault.Root+string(filepath.Separator)))
	dropOwnEntries(config)
	if prep.Answers != "" {
		if err := writeAnswers(config, ventoyPath, prep); err != nil {
			return err
		}
	}
	if prep.DriversArchive != "" {
		addPlugin(config, keyInjection, map[string]any{"image": ventoyPath, "archive": prep.DriversArchive})
	}
	bypass := bypassOff
	if prep.WinBypass {
		bypass = bypassOn
	}
	setControl(config, keyBypassCheck, bypass)
	nro := bypass
	if prep.WinNoOnline {
		nro = bypassOn
	}
	setControl(config, keyBypassNRO, nro)
	setControl(config, keyDefaultImage, ventoyPath)
	setControl(config, keyMenuTimeout, installTimeout)
	// * Durante la instalación el menú enseña Rebootera y la imagen elegida, y nada más.
	setMenu(config, ownImages(), ventoyPath)
	return writeConfig(config)
}

// * writeAnswers guarda el archivo de respuestas y lo asocia a la imagen.
func writeAnswers(config map[string]any, image string, prep Prep) error {
	name := filepath.Base(prep.AnswersName)
	if name != "autounattend.xml" && name != "preseed.cfg" {
		return errors.New("nombre de archivo de respuestas no admitido")
	}
	if err := os.MkdirAll(WorkDir(), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(WorkDir(), name), []byte(prep.Answers), 0o640); err != nil {
		return err
	}
	// * autosel 1: Ventoy usa la plantilla sin preguntar (si no, muestra un menú).
	addPlugin(config, keyAutoInstall, map[string]any{"image": image, "template": workPathPrefix + name, "autosel": 1})
	return nil
}

// * addPlugin añade una entrada a una lista de plugins de Ventoy.
func addPlugin(config map[string]any, key string, entry map[string]any) {
	list, _ := config[key].([]any)
	config[key] = append(list, entry)
}

// * isOwnEntry indica si una entrada de plugin es de Rebootera (apunta a su carpeta).
func isOwnEntry(item any) bool {
	entry, ok := item.(map[string]any)
	if !ok {
		return false
	}
	for _, field := range []string{"template", "archive"} {
		if value, ok := entry[field].(string); ok && strings.HasPrefix(value, workPathPrefix) {
			return true
		}
	}
	return false
}

// * dropOwnEntries quita de ventoy.json lo que Rebootera añadió y respeta
// * cualquier entrada del usuario. Devuelve true si quitó algo.
func dropOwnEntries(config map[string]any) bool {
	removed := false
	for _, key := range []string{keyAutoInstall, keyInjection} {
		list, ok := config[key].([]any)
		if !ok {
			continue
		}
		kept := make([]any, 0, len(list))
		for _, item := range list {
			if isOwnEntry(item) {
				removed = true
				continue
			}
			kept = append(kept, item)
		}
		if len(kept) == 0 {
			delete(config, key)
		} else {
			config[key] = kept
		}
	}
	return removed
}

// * Restore vuelve a dejar Rebootera OS como imagen por defecto y borra el
// * archivo de respuestas, los drivers inyectados y el atajo de Windows 11.
// * Devuelve true si había algo que deshacer.
func Restore() (bool, error) {
	if !vault.Available() {
		return false, nil
	}
	config, err := readConfig()
	if err != nil {
		return false, err
	}
	_, workErr := os.Stat(WorkDir())
	hadWork := workErr == nil
	dropped := dropOwnEntries(config)
	bypassed := getControl(config, keyBypassCheck) == bypassOn || getControl(config, keyBypassNRO) == bypassOn
	if !dropped && !hadWork && !bypassed && menuIs(config, ownImages()) &&
		getControl(config, keyDefaultImage) == RebooteraImage && getControl(config, keyMenuTimeout) == defaultTimeout {
		return false, nil
	}
	setControl(config, keyBypassCheck, bypassOff)
	setControl(config, keyBypassNRO, bypassOff)
	setControl(config, keyDefaultImage, RebooteraImage)
	setControl(config, keyMenuTimeout, defaultTimeout)
	setMenu(config, ownImages(), "")
	if err := writeConfig(config); err != nil {
		return true, err
	}
	// ! Se borra al final y solo esta carpeta: contiene contraseñas codificadas.
	if hadWork {
		if err := os.RemoveAll(WorkDir()); err != nil {
			return true, err
		}
	}
	return true, nil
}

// * Next devuelve la imagen que Ventoy preselecciona ahora mismo.
func Next() string {
	config, err := readConfig()
	if err != nil {
		return ""
	}
	return getControl(config, keyDefaultImage)
}

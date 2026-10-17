//go:build linux

// * Arranque de una sola vez del instalador (solo UEFI): reordena el orden de
// * arranque del firmware con efibootmgr para que el USB sea la última opción y
// * lo fija como BootNext, y lo restaura cuando Rebootera vuelve a arrancar.
// ! Solo reordena entradas existentes y valida cada identificador antes de
// ! pasarlo a efibootmgr. Guarda el orden original en el Vault para poder
// ! restaurarlo aunque el sistema instalado haya añadido o borrado entradas.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"rebootera/core/internal/efiboot"
)

// savedOrderPath guarda el orden de arranque original del firmware.
var savedOrderPath = filepath.Join(vaultMount, "rebootera", "state", "bootorder.json")

// savedOrder es lo que se guarda antes de reordenar.
type savedOrder struct {
	Order   []string `json:"order"`
	Current string   `json:"current"`
}

// * efibootmgr ejecuta efibootmgr con argumentos fijos y un límite de tiempo.
func efibootmgr(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "efibootmgr", args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("efibootmgr: %v: %s", err, out)
	}
	return string(out), nil
}

// * bootOnce pone el USB actual el último del orden de arranque y lo fija como
// * BootNext. Devuelve "adjusted", "unchanged" o "skipped" (sin UEFI).
func bootOnce() ([]string, error) {
	out, err := efibootmgr()
	if err != nil {
		return []string{"skipped"}, nil
	}
	state, err := efiboot.Parse(out)
	if err != nil {
		return []string{"skipped"}, nil
	}
	plan, changed := efiboot.PlanOnce(state)
	if !changed || !efiboot.Valid(plan) || !efiboot.Valid([]string{state.Current}) {
		return []string{"unchanged"}, nil
	}
	// * El primer orden guardado es el original: no se sobrescribe si ya hay uno.
	if _, err := os.Stat(savedOrderPath); errors.Is(err, os.ErrNotExist) {
		data, _ := json.Marshal(savedOrder{Order: state.Order, Current: state.Current})
		if err := os.MkdirAll(filepath.Dir(savedOrderPath), 0o750); err != nil {
			return nil, err
		}
		if err := os.WriteFile(savedOrderPath, data, 0o600); err != nil {
			return nil, err
		}
	}
	if _, err := efibootmgr("-o", efiboot.Join(plan)); err != nil {
		return nil, err
	}
	if _, err := efibootmgr("-n", state.Current); err != nil {
		return nil, err
	}
	return []string{"adjusted"}, nil
}

// * bootRestore devuelve el orden de arranque original, conservando las
// * entradas nuevas delante. Devuelve "restored" o "none".
func bootRestore() ([]string, error) {
	data, err := os.ReadFile(savedOrderPath)
	if errors.Is(err, os.ErrNotExist) {
		return []string{"none"}, nil
	}
	if err != nil {
		return nil, err
	}
	var saved savedOrder
	if err := json.Unmarshal(data, &saved); err != nil || !efiboot.Valid(saved.Order) {
		os.Remove(savedOrderPath)
		return []string{"none"}, nil
	}
	out, err := efibootmgr()
	if err != nil {
		return nil, err
	}
	state, err := efiboot.Parse(out)
	if err != nil {
		return nil, err
	}
	order := efiboot.Restorable(saved.Order, state)
	if efiboot.Valid(order) {
		if _, err := efibootmgr("-o", efiboot.Join(order)); err != nil {
			return nil, err
		}
	}
	// * BootNext ya se consumió al arrancar; se borra por si el firmware lo conserva.
	efibootmgr("-N")
	os.Remove(savedOrderPath)
	return []string{"restored"}, nil
}

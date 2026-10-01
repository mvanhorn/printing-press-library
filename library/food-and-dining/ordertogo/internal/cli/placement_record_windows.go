// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

//go:build windows

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const windowsPlacementValue = "PendingPlace"
const windowsConfirmedValue = "ConfirmedPlace"

// Tests replace this with an isolated key. Production keeps one reservation
// per Windows user regardless of the CLI config file in use.
var windowsPlacementRegistryPath = `Software\PrintingPress\OrderToGo\Checkout`

var regFlushKeyProc = windows.NewLazySystemDLL("advapi32.dll").NewProc("RegFlushKey")

func placementRegistryValue(path string) string {
	if filepath.Base(path) == "confirmed-place.json" {
		return windowsConfirmedValue
	}
	return windowsPlacementValue
}

func pendingPlacementLocation(path string) string {
	return `HKEY_CURRENT_USER\` + windowsPlacementRegistryPath + `\` + placementRegistryValue(path)
}

func readPendingPlacement(path string) ([]byte, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, windowsPlacementRegistryPath, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, err
	}
	defer key.Close()
	data, _, err := key.GetBinaryValue(placementRegistryValue(path))
	if errors.Is(err, registry.ErrNotExist) {
		return nil, os.ErrNotExist
	}
	return data, err
}

// RegFlushKey does not return until the containing registry hive has been
// written to disk. A failed flush prevents checkout from reaching the POST.
var flushPlacementKey = func(key registry.Key) error {
	status, _, _ := regFlushKeyProc.Call(uintptr(key))
	if status != 0 {
		return syscall.Errno(status)
	}
	return nil
}

func writePendingPlacement(path string, record pendingPlace) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, windowsPlacementRegistryPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if err := key.SetBinaryValue(placementRegistryValue(path), data); err != nil {
		return err
	}
	if err := flushPlacementKey(key); err != nil {
		return fmt.Errorf("flushing checkout reservation registry key: %w", err)
	}
	readBack, _, err := key.GetBinaryValue(placementRegistryValue(path))
	if err != nil {
		return fmt.Errorf("verifying checkout reservation registry value: %w", err)
	}
	var verify pendingPlace
	if json.Unmarshal(readBack, &verify) != nil || verify.RequestID != record.RequestID || verify.CartFingerprint != record.CartFingerprint || verify.ConfirmedOrderID != record.ConfirmedOrderID {
		return fmt.Errorf("written checkout reservation did not read back intact")
	}
	return nil
}

func clearPendingPlacement(path string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, windowsPlacementRegistryPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer key.Close()
	if err := key.DeleteValue(placementRegistryValue(path)); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	if err := flushPlacementKey(key); err != nil {
		return fmt.Errorf("flushing cleared checkout reservation registry key: %w", err)
	}
	return nil
}

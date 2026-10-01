//go:build windows

package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
)

func isolateWindowsCheckout(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	previous := windowsPlacementRegistryPath
	windowsPlacementRegistryPath = fmt.Sprintf(`Software\PrintingPress\OrderToGo\Tests\%d`, time.Now().UnixNano())
	t.Cleanup(func() {
		_ = registry.DeleteKey(registry.CURRENT_USER, windowsPlacementRegistryPath)
		windowsPlacementRegistryPath = previous
	})
}

func TestWindowsCheckoutRegistryReservation(t *testing.T) {
	isolateWindowsCheckout(t)
	first, err := reservePlacement("same-fingerprint")
	if err != nil {
		t.Fatalf("reserve checkout: %v", err)
	}
	data, err := readPendingPlacement(pendingPlaceRecordPath())
	if err != nil || !strings.Contains(string(data), first.RequestID) {
		t.Fatalf("durable registry reservation missing: %v", err)
	}
	if _, err := os.Stat(pendingPlaceRecordPath()); !os.IsNotExist(err) {
		t.Fatalf("Windows reservation unexpectedly used a file: %v", err)
	}
	if _, err := reservePlacement("same-fingerprint"); err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("parallel checkout was not locked: %v", err)
	}
	first.Release()
	if _, err := reservePlacement("changed-fingerprint"); err == nil || !strings.Contains(err.Error(), "unknown outcome") {
		t.Fatalf("changed checkout bypassed pending reservation: %v", err)
	}
	if err := clearPendingPlacement(pendingPlaceRecordPath()); err != nil {
		t.Fatalf("clear confirmed outcome: %v", err)
	}
	second, err := reservePlacement("same-fingerprint")
	if err != nil {
		t.Fatalf("reserve after explicit clear: %v", err)
	}
	if second.RequestID == first.RequestID {
		t.Fatal("new checkout reused the old request ID")
	}
	if warn := second.ConfirmSuccess(); warn != "" {
		t.Fatalf("confirmed checkout warning: %s", warn)
	}
	second.Release()
	if _, err := readPendingPlacement(pendingPlaceRecordPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("confirmed checkout retained registry reservation: %v", err)
	}
}

func TestWindowsCheckoutRegistryFlushFailureBlocksPOST(t *testing.T) {
	isolateWindowsCheckout(t)
	previous := flushPlacementKey
	flushPlacementKey = func(registry.Key) error { return errors.New("synthetic registry flush failure") }
	t.Cleanup(func() { flushPlacementKey = previous })
	if _, err := reservePlacement("fingerprint"); err == nil || !strings.Contains(err.Error(), "refusing to place the order") {
		t.Fatalf("flush failure did not stop checkout: %v", err)
	}
}

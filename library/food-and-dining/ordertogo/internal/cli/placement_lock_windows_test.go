//go:build windows

package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
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
	defer first.Release()
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
	// A separate process with another USERPROFILE sees the same HKCU record.
	// Its named mutex must block before either process can rewrite that record.
	probe := exec.Command(os.Args[0], "-test.run=^TestWindowsCheckoutSubprocessProbe$")
	probe.Env = append(os.Environ(),
		"ORDERTOGO_TEST_CHILD=1",
		"ORDERTOGO_TEST_REGISTRY_PATH="+windowsPlacementRegistryPath,
		"USERPROFILE="+t.TempDir(),
	)
	if output, err := probe.CombinedOutput(); err != nil {
		t.Fatalf("different-profile process bypassed checkout lock: %v, output=%s", err, output)
	}
	first.Release()
	if _, err := reservePlacement("changed-fingerprint"); err == nil || !strings.Contains(err.Error(), "unknown outcome") {
		t.Fatalf("changed checkout bypassed pending reservation: %v", err)
	}
	if err := clearPendingPlacement(pendingPlaceRecordPath()); err != nil {
		t.Fatalf("clear inspected unknown outcome: %v", err)
	}
	second, err := reservePlacement("same-fingerprint")
	if err != nil {
		t.Fatalf("reserve after explicit clear: %v", err)
	}
	defer second.Release()
	if second.RequestID == first.RequestID {
		t.Fatal("new checkout reused the old request ID")
	}
	if warn := second.MarkConfirmed(456); warn != "" {
		t.Fatalf("confirmed checkout warning: %s", warn)
	}
	second.Release()
	if _, err := reservePlacement("same-fingerprint"); err == nil || !strings.Contains(err.Error(), "order 456") {
		t.Fatalf("confirmed receipt did not block a new checkout: %v", err)
	}
	third, err := reservePlacementAcknowledging("same-fingerprint", 456)
	if err != nil {
		t.Fatalf("acknowledge confirmed order: %v", err)
	}
	defer third.Release()
	third.Release()
	if _, err := readPendingPlacement(confirmedPlaceRecordPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("acknowledged checkout retained registry receipt: %v", err)
	}
}

func TestWindowsCheckoutSubprocessProbe(t *testing.T) {
	if os.Getenv("ORDERTOGO_TEST_CHILD") != "1" {
		return
	}
	windowsPlacementRegistryPath = os.Getenv("ORDERTOGO_TEST_REGISTRY_PATH")
	if windowsPlacementRegistryPath == "" {
		t.Fatal("missing isolated registry path")
	}
	if _, err := reservePlacement("same-fingerprint"); err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("second process acquired checkout mutex: %v", err)
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
	if _, err := readPendingPlacement(pendingPlaceRecordPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("flush failure left a reservation for an order that never posted: %v", err)
	}
}

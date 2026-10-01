//go:build windows

package cli

import (
	"os"
	"strings"
	"testing"
)

func TestCheckoutRefusesWithoutDurableWindowsMetadata(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	_, err := reservePlacement("test-fingerprint")
	if err == nil || !strings.Contains(err.Error(), "disabled on Windows") {
		t.Fatalf("checkout reservation error = %v, want Windows fail-closed refusal", err)
	}
	if _, err := os.Stat(pendingPlaceRecordPath()); !os.IsNotExist(err) {
		t.Fatalf("checkout touched a reservation file before refusing: %v", err)
	}
}

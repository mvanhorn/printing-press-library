// Copyright 2026 "Chris Carson" and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestStaleDevicesDryRun(t *testing.T) {
	out, err := runNovelDryRun(t, newNovelStaleDevicesCmd, "--offline-days", "14")
	if err != nil {
		t.Fatalf("dry-run err: %v", err)
	}
	if !strings.Contains(out, "would") {
		t.Fatalf("dry-run output missing 'would': %q", out)
	}
}

func TestStaleDevicesRebootDryRun(t *testing.T) {
	// --reboot should still take the dry-run path and not mutate.
	out, err := runNovelDryRun(t, newNovelStaleDevicesCmd, "--reboot", "--apply")
	if err != nil {
		t.Fatalf("dry-run err: %v", err)
	}
	if !strings.Contains(out, "would") {
		t.Fatalf("reboot dry-run output missing 'would': %q", out)
	}
}

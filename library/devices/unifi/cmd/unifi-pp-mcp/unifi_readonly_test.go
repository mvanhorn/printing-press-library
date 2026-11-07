// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.

package main

import (
	"os"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/cliutil"
)

// cliutil honors exactly "1", so UNIFI_MCP_READ_ONLY=true left every write tool enabled.
func TestReadOnlySwitchFailsClosed(t *testing.T) {
	for value, want := range map[string]bool{"1": true, "true": true, "TRUE": true, "yes": true, "on": true, "enabled": true, "": false, "0": false, "false": false, "off": false, "no": false, "${user_config.read_only}": false} {
		t.Setenv(cliutil.MCPReadOnlyEnvVar, value)
		normalizeMCPReadOnlySwitch()
		if got := cliutil.IsMCPReadOnlyEnv(); got != want {
			t.Fatalf("%s=%q: read-only=%v want %v (now %q)", cliutil.MCPReadOnlyEnvVar, value, got, want, os.Getenv(cliutil.MCPReadOnlyEnvVar))
		}
	}
}

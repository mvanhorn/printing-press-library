// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored, not generator output. An init rather than a line in main so
// regeneration of main.go cannot drop it.

package main

import (
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
)

func init() {
	normalizeMCPReadOnlySwitch()
	// The server runs some tools in-process, and those have no child marker.
	store.SetLearnEventSurface(store.LearnSurfaceMCP)
}

// normalizeMCPReadOnlySwitch exists because the reserved cliutil check honors exactly
// "1", so UNIFI_MCP_READ_ONLY=true left every write tool enabled while looking protected.
// A safety switch has to fail closed. Rewriting the variable, rather than checking
// elsewhere, also covers the registration code and the mirrored child processes, which
// all read it through that one check. EnvOverride keeps an unresolved MCPB placeholder
// equivalent to unset.
func normalizeMCPReadOnlySwitch() {
	switch strings.ToLower(strings.TrimSpace(cliutil.EnvOverride(cliutil.MCPReadOnlyEnvVar))) {
	case "", "0", "false", "no", "off":
		return
	}
	_ = os.Setenv(cliutil.MCPReadOnlyEnvVar, "1")
}

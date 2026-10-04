// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"os"
	"strings"
)

// The trusted mirror sets this child context independently of caller flags.
// Read-only native mirrors suppress implicit state writes. Optional learning
// read helpers keep truthful local-write hints and their current RW semantics.
func mcpReadOnlyChildActive() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("CARSTAY_MCP_READ_ONLY")))
	return value == "true" || value == "1" || value == "yes"
}

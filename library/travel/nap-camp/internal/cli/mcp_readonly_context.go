// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"os"
	"strings"
)

// Trusted mirrored read-only tools suppress automatic state writes without
// disabling any explicit writable learning helper.
func mcpReadOnlyChildActive() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("NAP_CAMP_MCP_READ_ONLY")))
	return value == "true" || value == "1" || value == "yes"
}

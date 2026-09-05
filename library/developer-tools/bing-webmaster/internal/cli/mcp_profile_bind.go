// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.

package cli

// BindMCPServerProfile validates the non-secret profile and registration at
// MCP server startup.
//
// Merge-reconciliation stub (reprint 2026-09-05, press 4.31.1): the current
// generator template's MCP entry point calls this. This CLI authenticates
// with a user API key (BING_WEBMASTER_API_KEY) and never registers a
// platform tenant source, so there is nothing to bind — matching the
// template behavior when no platform source is registered (nil in, nil out).
// If a platform source is ever registered for this CLI, replace this stub
// with the template's live identity-gate implementation.
func BindMCPServerProfile() error {
	return nil
}

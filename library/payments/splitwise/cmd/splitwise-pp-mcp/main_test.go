// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package main

import (
	"strings"
	"testing"
)

// TestStartupBannerIncludesVersionAndTransport pins the startup line the MCP
// server writes to stderr on load: it must carry the binary name, the build
// version, and the selected transport so the line in a host's MCP log
// (e.g. Claude Desktop) identifies exactly which build is running.
func TestStartupBannerIncludesVersionAndTransport(t *testing.T) {
	got := startupBanner("4.20.0-next.4", "stdio")
	for _, want := range []string{"splitwise-pp-mcp", "4.20.0-next.4", "stdio"} {
		if !strings.Contains(got, want) {
			t.Errorf("startupBanner missing %q; got %q", want, got)
		}
	}
}

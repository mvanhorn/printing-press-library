// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package mcp

import (
	"strings"
	"testing"
)

// TestEntryPointToolsAreDiscoverableByAppName pins the app name into the
// descriptions of the three app-level entry-point tools (context/search/sql).
// A deferred-tool-loading host (e.g. Claude Cowork) searches the tool surface by
// keyword; without the app name on at least the "call this first" tool, a search
// for "splitwise" matches nothing and the host wrongly concludes no connector is
// installed. Leading these descriptions with the app name gives that search a hit.
func TestEntryPointToolsAreDiscoverableByAppName(t *testing.T) {
	for name, desc := range map[string]string{
		"context": contextToolDesc,
		"search":  searchToolDesc,
		"sql":     sqlToolDesc,
	} {
		if !strings.Contains(desc, "Splitwise") {
			t.Errorf("%s tool description must mention the app name \"Splitwise\" so a deferred-tool search by app name surfaces it; got: %q", name, desc)
		}
	}
}

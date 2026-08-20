// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Test for review finding #10: the generated REST `import` command POSTs to
// /<resource> but DICE is GraphQL-only, so it's broken — hide it from both the
// CLI help and the MCP mirror.
package cli

import "testing"

func TestImportCommandHidden(t *testing.T) {
	cmd := newImportCmd(&rootFlags{})
	if !cmd.Hidden {
		t.Errorf("import command must be Hidden:true (broken for a GraphQL-only API)")
	}
	if cmd.Annotations["mcp:hidden"] != "true" {
		t.Errorf("import command must carry the mcp:hidden annotation, got %v", cmd.Annotations)
	}
}

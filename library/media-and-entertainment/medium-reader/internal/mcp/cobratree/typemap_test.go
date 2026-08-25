// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package cobratree

import (
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/cobra"
)

// TestToolOptionsForFlags_OmitsBlockedRootFlags is the schema-honesty half of the
// cookie-file MCP guard. A hand-added credential flag (cookie-file) that
// blockedRootFlags drops before argv must also NOT be advertised as a settable
// tool parameter — otherwise an agent sees it in the schema, sets it, and has it
// silently ignored. Normal per-command flags must still be advertised, so a
// tightening here cannot quietly drop the real surface.
func TestToolOptionsForFlags_OmitsBlockedRootFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "demo"}
	cmd.Flags().String("cookie-file", "", "path to a flat-JSON Medium session cookie file")
	cmd.Flags().Int("limit", 0, "max results")

	tool := mcplib.NewTool("demo", toolOptionsForFlags(cmd)...)

	if _, ok := tool.InputSchema.Properties["cookie-file"]; ok {
		t.Errorf("cookie-file must not be advertised as a settable MCP parameter (it is dropped by blockedRootFlags)")
	}
	if _, ok := tool.InputSchema.Properties["limit"]; !ok {
		t.Errorf("normal per-command flag 'limit' should still be advertised in the tool schema")
	}
}

// TestToolOptionForPositional_ExposesRequiredQuery pins the schema half of the
// N1 port: a named positional is advertised as a required string property, so an
// MCP client sees that search/corpus need a "query" instead of guessing it must
// smuggle the term through "args".
func TestToolOptionForPositional_ExposesRequiredQuery(t *testing.T) {
	positionals := namedPositionalsFor([]string{"search"})
	if len(positionals) != 1 {
		t.Fatalf("expected one positional for search, got %d", len(positionals))
	}
	tool := mcplib.NewTool("search", toolOptionForPositional(positionals[0]))
	if _, ok := tool.InputSchema.Properties["query"]; !ok {
		t.Error("search must advertise a named 'query' property (finding N1)")
	}
	required := false
	for _, r := range tool.InputSchema.Required {
		if r == "query" {
			required = true
		}
	}
	if !required {
		t.Error("search 'query' positional should be marked required")
	}
}

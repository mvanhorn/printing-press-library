// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package cobratree

import (
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

// TestRegisterAll_NarrowNamedPositionals is the end-to-end guard for the N1
// port: driving the real RegisterAll walker over a command tree and inspecting
// the registered MCP tool schemas via the in-process ListTools accessor. It
// proves the fork's narrow allowlist wiring: search/corpus advertise a required
// named "query" property and drop the generic "args" bag, while a positional
// command outside the allowlist (read) keeps "args" and gains no "query".
func TestRegisterAll_NarrowNamedPositionals(t *testing.T) {
	mk := func(use string) *cobra.Command {
		return &cobra.Command{Use: use, Short: use, Run: func(*cobra.Command, []string) {}}
	}
	root := &cobra.Command{Use: "root"}
	root.AddCommand(mk("search <query>"), mk("corpus <query>"), mk("read <url|id>"), mk("digest"))

	s := server.NewMCPServer("test", "0.0.0")
	RegisterAll(s, root, func() (string, error) { return "/nonexistent", nil })
	tools := s.ListTools()

	for _, name := range []string{"search", "corpus"} {
		st, ok := tools[name]
		if !ok {
			t.Fatalf("%s tool not registered", name)
		}
		if _, ok := st.Tool.InputSchema.Properties["query"]; !ok {
			t.Errorf("%s must expose a named 'query' property (finding N1)", name)
		}
		if _, ok := st.Tool.InputSchema.Properties["args"]; ok {
			t.Errorf("%s must NOT expose the generic 'args' once it has a named positional", name)
		}
		required := false
		for _, r := range st.Tool.InputSchema.Required {
			if r == "query" {
				required = true
			}
		}
		if !required {
			t.Errorf("%s 'query' should be marked required", name)
		}
	}

	// read is a positional command deliberately left OFF the allowlist: it keeps
	// the generic "args" fallback and gains no "query".
	if st, ok := tools["read"]; ok {
		if _, ok := st.Tool.InputSchema.Properties["args"]; !ok {
			t.Error("read should keep the generic 'args' fallback (not in the narrow allowlist)")
		}
		if _, ok := st.Tool.InputSchema.Properties["query"]; ok {
			t.Error("read must not gain a 'query' property")
		}
	} else {
		t.Error("read tool not registered")
	}

	// digest takes no positional at all: no "query" property.
	if st, ok := tools["digest"]; ok {
		if _, ok := st.Tool.InputSchema.Properties["query"]; ok {
			t.Error("digest must not expose a 'query' property")
		}
	}
}

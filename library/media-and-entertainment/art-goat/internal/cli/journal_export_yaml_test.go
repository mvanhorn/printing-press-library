// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestYamlTagsList locks the YAML-safe output shape so a future
// "simplification" can't reintroduce the unquoted-scalar bug Greptile
// flagged as P0 (any colon-containing tag corrupts frontmatter).
func TestYamlTagsList(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "[]"},
		{"   ", "[]"},
		{"oil", `["oil"]`},
		{"oil, repetition", `["oil", "repetition"]`},
		{"  oil ,  repetition  ", `["oil", "repetition"]`},
		// The exact P0 input — a tag containing colon-space — must be
		// emitted as a quoted scalar inside the flow list.
		{"morning: practice", `["morning: practice"]`},
		{"oil, morning: practice, water", `["oil", "morning: practice", "water"]`},
		// Empty entries (trailing commas, double commas) drop out.
		{"oil,,repetition,", `["oil", "repetition"]`},
		// Embedded double quotes are escaped by %q so the output
		// remains a valid YAML / JSON-style double-quoted scalar.
		{`says "hi"`, `["says \"hi\""]`},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, yamlTagsList(tc.in))
		})
	}
}

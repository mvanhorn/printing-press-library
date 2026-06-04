// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVibes_SaveThenGet(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	save := newVibesCmd(&rootFlags{asJSON: true})
	save.SetOut(&strings.Builder{})
	save.SetArgs([]string{"save", "synthwave", "--tags", "synth,retro", "--prompt-template", "a {topic} anthem"})
	if err := save.Execute(); err != nil {
		t.Fatalf("vibes save: %v", err)
	}

	get := newVibesCmd(&rootFlags{asJSON: true})
	var out strings.Builder
	get.SetOut(&out)
	get.SetArgs([]string{"get", "synthwave"})
	if err := get.Execute(); err != nil {
		t.Fatalf("vibes get: %v", err)
	}
	var r vibeRecipe
	if err := json.Unmarshal([]byte(out.String()), &r); err != nil {
		t.Fatalf("parse: %v\n%s", err, out.String())
	}
	if r.Name != "synthwave" || r.PromptTemplate != "a {topic} anthem" {
		t.Fatalf("round-trip mismatch: %+v", r)
	}
}

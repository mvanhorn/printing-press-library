// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestFilterSandboxRules(t *testing.T) {
	tests := []struct {
		name      string
		scenario  string
		method    string
		wantMin   int
		wantMagic string // substring that must appear in at least one matched rule
		wantEmpty bool
	}{
		{name: "invalid bank account", scenario: "invalid-account", method: "bank-transfer", wantMin: 1, wantMagic: "odd"},
		{name: "valid ewallet", scenario: "valid-account", method: "ewallet", wantMin: 1, wantMagic: "even"},
		{name: "all success", scenario: "success", wantMin: 2},
		{name: "all payment", method: "payment", wantMin: 1, wantMagic: "Simulator"},
		{name: "no filter lists all", wantMin: len(sandboxRules)},
		{name: "no match", scenario: "nope", method: "nope", wantEmpty: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterSandboxRules(tt.scenario, tt.method)
			if tt.wantEmpty {
				if len(got) != 0 {
					t.Fatalf("expected no matches, got %d", len(got))
				}
				return
			}
			if len(got) < tt.wantMin {
				t.Fatalf("got %d rules, want at least %d", len(got), tt.wantMin)
			}
			if tt.wantMagic != "" {
				found := false
				for _, r := range got {
					if strings.Contains(r.Magic, tt.wantMagic) || strings.Contains(r.HowTo, tt.wantMagic) {
						found = true
					}
				}
				if !found {
					t.Fatalf("no matched rule contained %q", tt.wantMagic)
				}
			}
		})
	}
}

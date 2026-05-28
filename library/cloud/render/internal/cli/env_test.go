// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
)

// envContainerFor builds an in-memory envContainer from a (key, value) map
// for deterministic diff testing.
func envContainerFor(id, kind string, values map[string]string) *envContainer {
	c := &envContainer{ID: id, Kind: kind, Entries: map[string]envEntry{}}
	for k, v := range values {
		c.Entries[k] = envEntry{Key: k, ValueHash: hashEnvValue(v), ValueLen: len(v)}
	}
	return c
}

func TestComputeEnvDiff(t *testing.T) {
	tests := []struct {
		name        string
		a, b        map[string]string
		wantOnlyA   []string
		wantOnlyB   []string
		wantChanged []string
	}{
		{
			name:        "all_only_in_a",
			a:           map[string]string{"FOO": "1", "BAR": "2"},
			b:           map[string]string{},
			wantOnlyA:   []string{"BAR", "FOO"},
			wantOnlyB:   []string{},
			wantChanged: []string{},
		},
		{
			name:        "identical_no_changes",
			a:           map[string]string{"FOO": "1"},
			b:           map[string]string{"FOO": "1"},
			wantOnlyA:   []string{},
			wantOnlyB:   []string{},
			wantChanged: []string{},
		},
		{
			name:        "value_changed",
			a:           map[string]string{"STRIPE_KEY": "sk_live_a"},
			b:           map[string]string{"STRIPE_KEY": "sk_live_b"},
			wantOnlyA:   []string{},
			wantOnlyB:   []string{},
			wantChanged: []string{"STRIPE_KEY"},
		},
		{
			name:        "mixed",
			a:           map[string]string{"FOO": "1", "STRIPE_KEY": "ka"},
			b:           map[string]string{"BAR": "2", "STRIPE_KEY": "kb"},
			wantOnlyA:   []string{"FOO"},
			wantOnlyB:   []string{"BAR"},
			wantChanged: []string{"STRIPE_KEY"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := envContainerFor("srv-a", "service", tc.a)
			b := envContainerFor("srv-b", "service", tc.b)
			got := computeEnvDiff(a, b)
			if len(got.OnlyInA) != len(tc.wantOnlyA) {
				t.Fatalf("only_in_a: got %d, want %d (%v vs %v)", len(got.OnlyInA), len(tc.wantOnlyA), got.OnlyInA, tc.wantOnlyA)
			}
			for i, k := range tc.wantOnlyA {
				if got.OnlyInA[i].Key != k {
					t.Errorf("only_in_a[%d] got %s want %s", i, got.OnlyInA[i].Key, k)
				}
			}
			if len(got.OnlyInB) != len(tc.wantOnlyB) {
				t.Fatalf("only_in_b: got %d, want %d", len(got.OnlyInB), len(tc.wantOnlyB))
			}
			for i, k := range tc.wantOnlyB {
				if got.OnlyInB[i].Key != k {
					t.Errorf("only_in_b[%d] got %s want %s", i, got.OnlyInB[i].Key, k)
				}
			}
			if len(got.Changed) != len(tc.wantChanged) {
				t.Fatalf("changed: got %d, want %d", len(got.Changed), len(tc.wantChanged))
			}
			for i, k := range tc.wantChanged {
				if got.Changed[i].Key != k {
					t.Errorf("changed[%d] got %s want %s", i, got.Changed[i].Key, k)
				}
				if got.Changed[i].ValueHashA == got.Changed[i].ValueHashB {
					t.Errorf("changed[%d] expected differing hashes", i)
				}
			}
		})
	}
}

func TestClassifyEnvTarget(t *testing.T) {
	tests := []struct {
		in       string
		wantKind string
		wantErr  bool
	}{
		{"srv-d12abc", "service", false},
		{"evg-shared", "env-group", false},
		{"plain-name", "", true},
		{"", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := classifyEnvTarget(tc.in)
			if tc.wantErr && err == nil {
				t.Errorf("expected error for %q", tc.in)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tc.wantKind {
				t.Errorf("got %q, want %q", got, tc.wantKind)
			}
		})
	}
}

func TestHashEnvValueDeterministic(t *testing.T) {
	// Hashes are stable so test outputs and golden fixtures don't drift.
	got1 := hashEnvValue("sk_live_xyz")
	got2 := hashEnvValue("sk_live_xyz")
	if got1 != got2 {
		t.Errorf("hash not deterministic: %s vs %s", got1, got2)
	}
	if len(got1) != 12 {
		t.Errorf("hash should be 12 chars, got %d", len(got1))
	}
}

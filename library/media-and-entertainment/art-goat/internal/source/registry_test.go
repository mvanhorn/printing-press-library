// Copyright 2026 justinwfu. Licensed under Apache-2.0.

package source

import (
	"context"
	"strings"
	"testing"
)

// stubSource implements Source with controllable name/description and a
// no-op Sync. Used by Lookup/All tests so the registry can be exercised
// without dialling real APIs.
type stubSource struct {
	name string
	desc string
	auth bool
}

func (s *stubSource) Name() string                                       { return s.name }
func (s *stubSource) Description() string                                { return s.desc }
func (s *stubSource) AuthRequired() bool                                 { return s.auth }
func (s *stubSource) Sync(_ context.Context, _ SyncOpts) ([]Work, error) { return nil, nil }

// resetRegistry restores the package-level registry to a fresh state.
// Returns a cleanup the caller defers to restore whatever was registered
// before the test ran (so the test doesn't leak into other tests sharing
// the package init() registrations).
func resetRegistry(t *testing.T) func() {
	t.Helper()
	prior := defaultRegistry
	defaultRegistry = newRegistry()
	return func() { defaultRegistry = prior }
}

func TestRegister_DeduplicatesByName(t *testing.T) {
	defer resetRegistry(t)()

	Register(&stubSource{name: "alpha", desc: "first"})
	Register(&stubSource{name: "alpha", desc: "second"})

	got, err := Lookup("alpha")
	if err != nil {
		t.Fatalf("Lookup(alpha): %v", err)
	}
	if got.Description() != "second" {
		t.Errorf("a second Register call with the same name should replace the prior entry; got description %q, want %q",
			got.Description(), "second")
	}
	if len(All()) != 1 {
		t.Errorf("All() should reflect the deduped registry; got %d sources, want 1", len(All()))
	}
}

func TestAll_ReturnsSortedByName(t *testing.T) {
	defer resetRegistry(t)()

	// Register out of order; All() must return alphabetical so doctor and
	// sources-list output is deterministic across runs.
	Register(&stubSource{name: "zeta"})
	Register(&stubSource{name: "alpha"})
	Register(&stubSource{name: "mu"})

	out := All()
	if len(out) != 3 {
		t.Fatalf("All(): got %d sources, want 3", len(out))
	}
	want := []string{"alpha", "mu", "zeta"}
	for i, w := range want {
		if out[i].Name() != w {
			t.Errorf("All()[%d]: got %q, want %q", i, out[i].Name(), w)
		}
	}
}

func TestLookup_UnknownNameSurfacesAvailable(t *testing.T) {
	defer resetRegistry(t)()

	Register(&stubSource{name: "aic"})
	Register(&stubSource{name: "apod"})

	_, err := Lookup("nope")
	if err == nil {
		t.Fatal("Lookup of unknown source returned nil error")
	}
	msg := err.Error()
	for _, sub := range []string{"nope", "aic", "apod"} {
		if !strings.Contains(msg, sub) {
			t.Errorf("error message should contain %q to help the user fix typos; got %q", sub, msg)
		}
	}
}

func TestLookup_EmptyRegistryReturnsError(t *testing.T) {
	defer resetRegistry(t)()

	if _, err := Lookup("anything"); err == nil {
		t.Error("lookup against an empty registry should not return a zero-value source")
	}
}

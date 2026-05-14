// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/openart/internal/openartmodels"
)

// modelRow must surface the Experimental flag so callers can tell which
// models need --accept-experimental before submitting. Skipping this key
// caused real friction in the 2026-05-14 donkey-photo session: the
// agent picked nano-banana-pro from JSON output that had no experimental
// signal, then learned about the opt-in flag only after the submit
// rejected it. Keeping this test red-lines the omission.
func TestModelRow_IncludesExperimentalForVerifiedModel(t *testing.T) {
	m := openartmodels.Resolve("nano-banana")
	if m == nil {
		t.Fatal("nano-banana missing from catalog")
	}
	row := modelRow(*m)
	got, ok := row["experimental"]
	if !ok {
		t.Fatal("modelRow output missing experimental key")
	}
	if got != false {
		t.Errorf("nano-banana experimental: want false, got %v", got)
	}
}

func TestModelRow_IncludesExperimentalForExperimentalModel(t *testing.T) {
	m := openartmodels.Resolve("nano-banana-pro")
	if m == nil {
		t.Fatal("nano-banana-pro missing from catalog")
	}
	row := modelRow(*m)
	got, ok := row["experimental"]
	if !ok {
		t.Fatal("modelRow output missing experimental key")
	}
	if got != true {
		t.Errorf("nano-banana-pro experimental: want true, got %v", got)
	}
}

// Every catalog entry must surface the experimental key, even when the
// value is false. Omitting the key for verified models would force
// callers to treat "missing key" as "not experimental" — the exact
// ambiguity this change removes.
func TestModelRow_EveryCatalogEntryExposesExperimental(t *testing.T) {
	for _, m := range openartmodels.Catalog {
		row := modelRow(m)
		if _, ok := row["experimental"]; !ok {
			t.Errorf("catalog entry %q (family=%s) missing experimental key in modelRow output", m.Slug, m.Family)
		}
	}
}

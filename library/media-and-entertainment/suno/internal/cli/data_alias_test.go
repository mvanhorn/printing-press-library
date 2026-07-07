// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"testing"
)

// TestWrapWithProvenanceAddsDataAlias locks the non-breaking convergence:
// every provenance-wrapped read command exposes a top-level "data" alias that
// mirrors "results", so JSON consumers can read .data uniformly across all
// commands without dropping the existing "results"/"meta" keys.
func TestWrapWithProvenanceAddsDataAlias(t *testing.T) {
	raw := json.RawMessage(`[{"id":"a"},{"id":"b"}]`)
	out, err := wrapWithProvenance(raw, DataProvenance{Source: "live"})
	if err != nil {
		t.Fatalf("wrapWithProvenance: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	for _, k := range []string{"results", "meta", "data"} {
		if _, ok := m[k]; !ok {
			t.Errorf("envelope missing %q key: %s", k, out)
		}
	}
	if string(m["data"]) != string(m["results"]) {
		t.Errorf("data (%s) must mirror results (%s)", m["data"], m["results"])
	}
}

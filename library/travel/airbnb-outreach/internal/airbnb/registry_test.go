// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package airbnb

import (
	"strings"
	"testing"
)

func TestRegistryBundledHashes(t *testing.T) {
	r := &Registry{overrides: map[string]string{}}
	// A core operation must resolve from the bundled snapshot.
	if h := r.Hash("StaysSearch"); len(h) != 64 {
		t.Errorf("StaysSearch bundled hash = %q (len %d), want 64-hex", h, len(h))
	}
	if r.Source("StaysSearch") != "bundled" {
		t.Errorf("StaysSearch source = %q, want bundled", r.Source("StaysSearch"))
	}
	if h := r.Hash("NoSuchOperation"); h != "" {
		t.Errorf("unknown op hash = %q, want empty", h)
	}
}

func TestRegistryOverridePreferred(t *testing.T) {
	r := &Registry{overrides: map[string]string{"StaysSearch": strings.Repeat("a", 64)}}
	if r.Hash("StaysSearch") != strings.Repeat("a", 64) {
		t.Error("override hash should win over bundled")
	}
	if r.Source("StaysSearch") != "refreshed" {
		t.Errorf("overridden op source = %q, want refreshed", r.Source("StaysSearch"))
	}
}

func TestMergeHashes(t *testing.T) {
	hash := "d39f949ec846c484f09df7d2ba282874c27aa4b4adc9a71399a8fe0ae3a9cf67"
	// Non-suffixed known op (StaysSearch) must be paired via the exact-name pass.
	js := `foo="StaysSearch"bar,` + strings.Repeat("x", 40) + `"` + hash + `"`
	dst := map[string]string{}
	mergeHashes(dst, js, map[string]struct{}{"StaysSearch": {}})
	if dst["StaysSearch"] != hash {
		t.Errorf("mergeHashes did not pair StaysSearch with its hash: %v", dst)
	}

	// Suffixed op is discovered even when not in the known set.
	js2 := `x="ViaductGetThreadAndDataQuery"y,` + strings.Repeat("z", 30) + `"` + hash + `"`
	dst2 := map[string]string{}
	mergeHashes(dst2, js2, map[string]struct{}{})
	if dst2["ViaductGetThreadAndDataQuery"] != hash {
		t.Errorf("mergeHashes discovery pass missed ViaductGetThreadAndDataQuery: %v", dst2)
	}
}

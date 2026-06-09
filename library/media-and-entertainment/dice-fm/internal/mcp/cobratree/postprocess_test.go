// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the cobratree output post-processor registry (the generic hook the
// mcp package uses to pseudonymize mirrored fans/door command output).
package cobratree

import "testing"

func TestPathKey(t *testing.T) {
	if got := pathKey([]string{"fans", "top"}); got != "fans top" {
		t.Errorf("pathKey = %q, want %q", got, "fans top")
	}
}

func TestRegisterAndLookupPostProcessor(t *testing.T) {
	called := false
	RegisterOutputPostProcessor("fans top", func(out string, args map[string]any) (string, error) {
		called = true
		return out + "-processed", nil
	})
	t.Cleanup(func() {
		postProcMu.Lock()
		delete(outputPostProcs, "fans top")
		postProcMu.Unlock()
	})

	pp, ok := lookupPostProcessor("fans top")
	if !ok {
		t.Fatalf("post-processor not found after registration")
	}
	out, err := pp("raw", nil)
	if err != nil {
		t.Fatalf("pp error: %v", err)
	}
	if !called || out != "raw-processed" {
		t.Errorf("post-processor not invoked correctly: called=%v out=%q", called, out)
	}

	if _, ok := lookupPostProcessor("unregistered cmd"); ok {
		t.Errorf("lookupPostProcessor returned ok for an unregistered path")
	}
}

func TestWithoutPostProcessorArgs(t *testing.T) {
	in := map[string]any{"event": "e1", "include_pii": true, "csv": true, "plain": true, "quiet": true, "limit": float64(10)}
	out := withoutPostProcessorArgs(in)
	for _, stripped := range []string{"include_pii", "csv", "plain", "quiet"} {
		if _, ok := out[stripped]; ok {
			t.Errorf("withoutPostProcessorArgs left %s: %v", stripped, out)
		}
	}
	if out["event"] != "e1" || out["limit"] != float64(10) {
		t.Errorf("withoutPostProcessorArgs dropped real args: %v", out)
	}
	// Input not mutated.
	for _, stripped := range []string{"include_pii", "csv", "plain", "quiet"} {
		if _, ok := in[stripped]; !ok {
			t.Errorf("withoutPostProcessorArgs mutated its input")
		}
	}
}

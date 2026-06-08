// Copyright 2026 "Chris Carson" and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestPatchStuckDryRun(t *testing.T) {
	out, err := runNovelDryRun(t, newNovelPatchStuckCmd, "--cycles", "5")
	if err != nil {
		t.Fatalf("dry-run err: %v", err)
	}
	if !strings.Contains(out, "would") {
		t.Fatalf("dry-run output missing 'would': %q", out)
	}
}

func TestSplitEntityKey(t *testing.T) {
	tests := []struct {
		in      string
		wantID  int64
		wantRem string
	}{
		{"5:KB001", 5, "KB001"},
		{"42:cpu load", 42, "cpu load"},
		{"nokey", 0, "nokey"},
		{"7:a:b", 7, "a:b"},
	}
	for _, tt := range tests {
		id, rem := splitEntityKey(tt.in)
		if id != tt.wantID || rem != tt.wantRem {
			t.Fatalf("splitEntityKey(%q) = (%d,%q), want (%d,%q)", tt.in, id, rem, tt.wantID, tt.wantRem)
		}
	}
}

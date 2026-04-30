// Copyright 2026 adrian-horning. Licensed under Apache-2.0. See LICENSE.
// Regression coverage for the cursor/page/min_time pagination encoding fix.

package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestCursorFlagsAreStringTyped ensures pagination flag declarations across the
// generated cli package use StringVar rather than Float64Var. Float64Var causes
// values >= ~10^6 to render in scientific notation (e.g. 1.740168616e+09), which
// upstream APIs reject as invalid integer cursors.
func TestCursorFlagsAreStringTyped(t *testing.T) {
	cursorFlagPattern := regexp.MustCompile(`Flags\(\)\.Float64Var\([^,]+,\s*"(cursor|min-time|max-time|page|offset)"`)

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read internal/cli: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if m := cursorFlagPattern.FindString(string(body)); m != "" {
			t.Errorf("%s: cursor pagination flag still typed as Float64Var: %s", name, m)
		}
	}
}

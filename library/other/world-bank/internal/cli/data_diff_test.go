// Copyright 2026 Luke J and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWbSnapshotPathSanitizesBothSeparators(t *testing.T) {
	dir := filepath.Join(filepath.Dir(defaultDBPath("world-bank-pp-cli")), "snapshots")

	tests := []struct {
		name      string
		country   string
		indicator string
	}{
		{name: "forward slash", country: "USA", indicator: "../etc/passwd"},
		{name: "backslash", country: "USA", indicator: `..\..\escape`},
		{name: "mixed separators", country: `..\usa`, indicator: `foo/bar\baz`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wbSnapshotPath(tt.country, tt.indicator)
			if filepath.Dir(got) != dir {
				t.Fatalf("escaped snapshots dir: got %q, want dir %q", got, dir)
			}
			base := filepath.Base(got)
			if strings.ContainsAny(base, `/\`) {
				t.Fatalf("basename still contains a separator: %q", base)
			}
			rel, err := filepath.Rel(dir, got)
			if err != nil {
				t.Fatalf("Rel(%q, %q): %v", dir, got, err)
			}
			if rel != base {
				t.Fatalf("rel is not a single filename: got %q", rel)
			}
		})
	}
}

func TestWbSnapshotPathPlainNames(t *testing.T) {
	got := wbSnapshotPath("usa", "NY.GDP.MKTP.CD")
	if !strings.HasSuffix(got, "USA_NY.GDP.MKTP.CD.json") {
		t.Fatalf("unexpected snapshot path: %q", got)
	}
	if strings.ContainsAny(filepath.Base(got), `/\`) {
		t.Fatalf("plain name introduced a separator: %q", got)
	}
}

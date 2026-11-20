// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"time"
)

// TestNovelDigestHelpWires smoke-tests that the digest command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelDigestHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"digest", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("digest --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "digest"} {
		if !strings.Contains(help, want) {
			t.Fatalf("digest --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelParseSince(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		in   string
		want time.Time
	}{
		{in: "7d", want: now.Add(-7 * 24 * time.Hour)},
		{in: "2026-06-01", want: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		got, err := novelParseSince(tt.in, now)
		if err != nil {
			t.Fatalf("novelParseSince(%q): %v", tt.in, err)
		}
		if !got.Equal(tt.want) {
			t.Fatalf("novelParseSince(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestE3DigestTimestampOrder(t *testing.T) {
	sliceDStore(t, "UPDATE highlights SET highlighted_at='2026-06-15 01:00:00'")
	view, err := novelDigestReport(newNovelDigestCmd(&rootFlags{}), time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), nil)
	if err != nil || view.Count != 1 {
		t.Fatalf("%+v err=%v", view, err)
	}
}

// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelQdsHelpWires smoke-tests that the qds command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelQdsHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"qds", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("qds --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "qds"} {
		if !strings.Contains(help, want) {
			t.Fatalf("qds --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestDetectLTSForVersion guards the release attribution of LTS statements: an
// article naming several Designer releases must not let one release's LTS
// designation bleed onto another release the article merely mentions.
func TestDetectLTSForVersion(t *testing.T) {
	multi := "Q-SYS Designer 9.8 is the long-term support (LTS) release; LTS support ends March 2027. Designer 9.10 introduces new hardware and is not covered by this designation."
	single := "Q-SYS Designer 9.8 is the long-term support (LTS) release; LTS support ends March 2027."
	cases := []struct {
		name    string
		target  string
		text    string
		wantLTS bool
		wantEnd string
	}{
		{"multi-release article, LTS belongs to other release", "9.10", multi, false, ""},
		{"multi-release article, LTS belongs to queried release", "9.8", multi, true, "March 2027"},
		{"single-release article trusted whole", "9.8", single, true, "March 2027"},
		{"one sentence naming both releases, LTS belongs to the nearer", "9.10", "Q-SYS Designer 9.8 is the long-term support (LTS) release and is recommended over Designer 9.10; LTS support ends March 2027.", false, ""},
		{"one sentence naming both releases, queried release is the nearer", "9.8", "Q-SYS Designer 9.8 is the long-term support (LTS) release and is recommended over Designer 9.10; LTS support ends March 2027.", true, "March 2027"},
		{"one sentence designating two releases LTS, later designation kept", "10.0", "Q-SYS Designer 9.8 is LTS through March 2027 and Designer 10.0 is LTS through June 2028.", true, "June 2028"},
		{"one sentence designating two releases LTS, first designation kept", "9.8", "Q-SYS Designer 9.8 is LTS through March 2027 and Designer 10.0 is LTS through June 2028.", true, "March 2027"},
		{"multi-release, queried release far from LTS wording", "9.10", "Designer 9.8 is the LTS release. " + strings.Repeat("Filler sentence about unrelated topics. ", 20) + "Designer 9.10 adds the CX-Q series.", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			versions := articleVersions(tc.text)
			end, ok := detectLTSForVersion(tc.target, tc.text, versions)
			if ok != tc.wantLTS {
				t.Fatalf("detectLTSForVersion(%q) ok = %v, want %v", tc.target, ok, tc.wantLTS)
			}
			if end != tc.wantEnd {
				t.Fatalf("detectLTSForVersion(%q) end = %q, want %q", tc.target, end, tc.wantEnd)
			}
		})
	}
}

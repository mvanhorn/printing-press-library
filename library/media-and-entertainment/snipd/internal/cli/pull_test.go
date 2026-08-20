// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/internal/snipd"
)

// TestNovelPullHelpWires smoke-tests that the pull command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelPullHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"pull", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pull --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "pull"} {
		if !strings.Contains(help, want) {
			t.Fatalf("pull --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestFallbackSnipIDIsPositionalAndEditStable guards the UUID-less fallback id: it
// keys on the clip position (episode+start+end), so editing a snip's content does
// NOT change its id (a re-pull updates the row instead of duplicating it), while
// two snips at different clip spans still get different ids.
func TestFallbackSnipIDIsPositionalAndEditStable(t *testing.T) {
	base := snipd.Snip{EpisodeID: "ep1", Start: "15:11", End: "16:37", Note: "original", Title: "A"}

	// Editing content (or the title) must NOT change the id — position is identity.
	edited := base
	edited.Note = "edited note"
	edited.Quote = "new quote"
	edited.Title = "renamed"
	if fallbackSnipID(base, -1) != fallbackSnipID(edited, -1) {
		t.Error("editing a UUID-less snip's content changed its id; a re-pull would duplicate it")
	}

	// Two snips at different clip spans must get different ids.
	other := base
	other.End = "17:00"
	if fallbackSnipID(base, -1) == fallbackSnipID(other, -1) {
		t.Error("snips at different clip spans collided on the fallback id")
	}

	if !strings.HasPrefix(fallbackSnipID(base, -1), "ep1#") {
		t.Errorf("fallback id %q is not namespaced under the episode id", fallbackSnipID(base, -1))
	}
}

// TestFallbackSnipIDBothEmptyTimesDistinguishedByOrdinal guards the case Greptile
// flagged: when a UUID-less snip exports with no start AND no end, start+end alone
// collapses every such snip in an episode to one key, silently overwriting. The
// caller's per-episode ordinal must keep distinct snips distinct — while staying
// stable (a re-pull at the same position updates the row, it doesn't duplicate).
func TestFallbackSnipIDBothEmptyTimesDistinguishedByOrdinal(t *testing.T) {
	a := snipd.Snip{EpisodeID: "ep1", Start: "", End: "", Note: "first"}
	b := snipd.Snip{EpisodeID: "ep1", Start: "", End: "", Note: "second"}

	// Same episode, both timestamps empty, distinct snips → distinct ordinals → distinct ids.
	if fallbackSnipID(a, 0) == fallbackSnipID(b, 1) {
		t.Error("two both-empty snips in one episode collided; one would overwrite the other")
	}

	// Same position → same id (a re-pull updates, not duplicates).
	if fallbackSnipID(a, 0) != fallbackSnipID(a, 0) {
		t.Error("both-empty fallback id is not stable at a fixed ordinal")
	}

	// Editing content at a fixed position must not change the id.
	edited := a
	edited.Note = "edited"
	edited.Quote = "added"
	if fallbackSnipID(a, 0) != fallbackSnipID(edited, 0) {
		t.Error("editing a both-empty snip's content changed its id at the same ordinal")
	}
}

// TestTsLater guards the incremental cursor comparison: it must order by real
// instant, not lexically, so timestamps with different offsets or fractional
// precision can't park the cursor before episodes it should still cover.
func TestTsLater(t *testing.T) {
	cases := []struct {
		name, a, b string
		want       bool
	}{
		{"any ts beats the empty initial cursor", "2026-07-13T10:00:00Z", "", true},
		{"an earlier instant does not beat a later one", "2026-07-13T09:00:00Z", "2026-07-13T10:00:00Z", false},
		// Same instant, different offset + precision — lexical order would say true.
		{"same instant across offsets is not later", "2026-07-13T12:00:00.000+02:00", "2026-07-13T10:00:00Z", false},
		// Later instant whose string sorts lower than the Z form.
		{"a genuinely later instant across offsets", "2026-07-13T13:00:00+02:00", "2026-07-13T10:00:00Z", true},
	}
	for _, tc := range cases {
		if got := tsLater(tc.a, tc.b); got != tc.want {
			t.Errorf("%s: tsLater(%q, %q) = %v, want %v", tc.name, tc.a, tc.b, got, tc.want)
		}
	}
}

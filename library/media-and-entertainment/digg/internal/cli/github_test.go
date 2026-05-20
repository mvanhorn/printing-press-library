// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(digg-rankings-and-min-starrers): tests for the --min-starrers
// flag added to `github stars`. Pure unit tests against
// filterByMinStarrers; the flag wiring is covered by a tiny end-to-end
// "negative value rejected" test that exercises PreRunE.

package cli

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggparse"
)

func makeRepo(name string, distinct int, starrerNames ...string) diggparse.GithubRepoEntry {
	starrers := make([]diggparse.GithubStarrer, len(starrerNames))
	for i, n := range starrerNames {
		starrers[i] = diggparse.GithubStarrer{Username: n}
	}
	return diggparse.GithubRepoEntry{
		Repo: diggparse.GithubRepoMeta{
			FullName:         name,
			DistinctStarrers: distinct,
			Starrers:         starrers,
		},
	}
}

func TestFilterByMinStarrers_Threshold(t *testing.T) {
	repos := []diggparse.GithubRepoEntry{
		makeRepo("a/one", 1, "u1"),
		makeRepo("a/two", 2, "u1", "u2"),
		makeRepo("a/five", 5, "u1", "u2", "u3", "u4", "u5"),
	}

	cases := []struct {
		threshold int
		wantNames []string
	}{
		{0, []string{"a/one", "a/two", "a/five"}}, // no-op
		{1, []string{"a/one", "a/two", "a/five"}}, // no-op
		{2, []string{"a/two", "a/five"}},
		{5, []string{"a/five"}},
		{6, nil},
	}
	for _, tc := range cases {
		got := filterByMinStarrers(repos, tc.threshold)
		var names []string
		for _, r := range got {
			names = append(names, r.Repo.FullName)
		}
		if !equalStrings(names, tc.wantNames) {
			t.Errorf("threshold=%d: got %v, want %v", tc.threshold, names, tc.wantNames)
		}
	}
}

// TestFilterByMinStarrers_FallsBackToStarrersLen verifies that when
// upstream omits distinct_starrers (zero) but the parsed Starrers slice
// has entries, the filter uses the slice length. This is the
// belt-and-suspenders for older response shapes.
func TestFilterByMinStarrers_FallsBackToStarrersLen(t *testing.T) {
	repos := []diggparse.GithubRepoEntry{
		// distinct=0 but 3 starrers in the slice — should count as 3.
		makeRepo("a/three", 0, "u1", "u2", "u3"),
		// distinct=0 and no starrers — count is 0, filtered out at >=1.
		makeRepo("a/none", 0),
	}
	got := filterByMinStarrers(repos, 3)
	if len(got) != 1 || got[0].Repo.FullName != "a/three" {
		t.Errorf("got %+v, want exactly [a/three]", got)
	}
}

// TestFilterByMinStarrers_PrefersDistinctOverLen ensures we don't
// inflate the count by double-reading both fields. When
// DistinctStarrers is explicit (non-zero), we trust it even if the
// Starrers slice happens to be longer (truncated upstream views, e.g.).
func TestFilterByMinStarrers_PrefersDistinctOverLen(t *testing.T) {
	repos := []diggparse.GithubRepoEntry{
		// distinct=2 but slice has 5 — count is 2.
		makeRepo("a/two", 2, "u1", "u2", "u3", "u4", "u5"),
	}
	got := filterByMinStarrers(repos, 3)
	if len(got) != 0 {
		t.Errorf("got %+v, want empty (distinct=2 < threshold=3 even though len(starrers)=5)", got)
	}
}

// TestFilterByMinStarrers_NilSafety ensures the filter doesn't panic on
// zero-valued repos. A future parser change that emits an empty struct
// shouldn't crash the CLI.
func TestFilterByMinStarrers_NilSafety(t *testing.T) {
	repos := []diggparse.GithubRepoEntry{
		{}, // zero-valued — no Repo data, no Starrers
		makeRepo("a/has", 2, "u1", "u2"),
	}
	got := filterByMinStarrers(repos, 2)
	if len(got) != 1 || got[0].Repo.FullName != "a/has" {
		t.Errorf("got %+v, want exactly [a/has]", got)
	}
}

// TestFilterByMinStarrers_NoMutationOfInput is a defensive check that
// the filter doesn't share the backing array with the input in a way
// that would corrupt the caller's slice on subsequent appends. We
// allocate fresh via repos[:0:0] inside the filter; this test locks
// that invariant.
func TestFilterByMinStarrers_NoMutationOfInput(t *testing.T) {
	repos := []diggparse.GithubRepoEntry{
		makeRepo("a/one", 1, "u1"),
		makeRepo("a/two", 2, "u1", "u2"),
	}
	_ = filterByMinStarrers(repos, 2)
	// Original input untouched.
	if repos[0].Repo.FullName != "a/one" || repos[1].Repo.FullName != "a/two" {
		t.Errorf("input slice was mutated by filter: %+v", repos)
	}
}

// TestGithubStarsCmd_NegativeMinStarrersRejected exercises the PreRunE
// guard end-to-end. We don't need a network call — PreRunE runs before
// RunE, so flag validation errors before fetching.
func TestGithubStarsCmd_NegativeMinStarrersRejected(t *testing.T) {
	root := RootCmd()
	root.SetArgs([]string{
		"github", "stars", "--min-starrers", "-1",
		"--json", "--no-color", "--yes", "--no-input",
	})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for --min-starrers=-1, got nil")
	}
	if !strings.Contains(err.Error(), "--min-starrers must be >= 0") {
		t.Errorf("error = %q; expected '--min-starrers must be >= 0' phrase", err.Error())
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

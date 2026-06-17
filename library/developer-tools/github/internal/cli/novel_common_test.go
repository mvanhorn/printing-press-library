// Copyright 2026 Brandon Nye and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the pure aggregation logic behind the transcendence commands.

package cli

import (
	"encoding/json"
	"testing"
	"time"
)

func rawSlice(jsons ...string) [][]byte {
	out := make([][]byte, len(jsons))
	for i, j := range jsons {
		out[i] = []byte(j)
	}
	return out
}

func TestNvReviewerLoad(t *testing.T) {
	prs := rawSlice(
		`{"state":"open","requested_reviewers":[{"login":"alice"},{"login":"bob"}]}`,
		`{"state":"open","requested_reviewers":[{"login":"alice"}]}`,
		`{"state":"closed","requested_reviewers":[{"login":"alice"}]}`,
		`{"state":"open","requested_reviewers":[]}`,
	)
	got := nvReviewerLoad(prs, "open")
	if len(got) != 2 {
		t.Fatalf("want 2 reviewers, got %d (%+v)", len(got), got)
	}
	if got[0].Reviewer != "alice" || got[0].OpenPRs != 2 {
		t.Errorf("want alice=2 first, got %+v", got[0])
	}
	if got[1].Reviewer != "bob" || got[1].OpenPRs != 1 {
		t.Errorf("want bob=1, got %+v", got[1])
	}
	// closed reviewer should not leak into the open count
	if all := nvReviewerLoad(prs, "all"); all[0].OpenPRs != 3 {
		t.Errorf("all-state alice should be 3, got %+v", all[0])
	}
}

func TestNvStalePRs(t *testing.T) {
	now := time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)
	prs := rawSlice(
		`{"number":1,"state":"open","title":"old","updated_at":"2026-05-01T00:00:00Z"}`,
		`{"number":2,"state":"open","title":"fresh","updated_at":"2026-06-16T00:00:00Z"}`,
		`{"number":3,"state":"closed","title":"closed-old","updated_at":"2026-01-01T00:00:00Z"}`,
	)
	got := nvStalePRs(prs, 14*24*time.Hour, now)
	if len(got) != 1 {
		t.Fatalf("want 1 stale open PR, got %d (%+v)", len(got), got)
	}
	if got[0].Number != 1 {
		t.Errorf("want #1 stale, got #%d", got[0].Number)
	}
	if got[0].StaleDays < 40 {
		t.Errorf("stale_days for #1 should be ~47, got %d", got[0].StaleDays)
	}
}

func commitsFromJSON(t *testing.T, arr string) []any {
	t.Helper()
	var out []any
	if err := json.Unmarshal([]byte(arr), &out); err != nil {
		t.Fatalf("bad commits fixture: %v", err)
	}
	return out
}

func TestNvGroupCommitsByAuthor(t *testing.T) {
	commits := commitsFromJSON(t, `[
		{"sha":"aaaaaaa1","author":{"login":"alice"}},
		{"sha":"bbbbbbb2","author":{"login":"bob"}},
		{"sha":"ccccccc3","author":{"login":"alice"}},
		{"sha":"ddddddd4","commit":{"author":{"name":"Carol"}}}
	]`)
	got := nvGroupCommitsByAuthor(commits)
	if len(got) != 3 {
		t.Fatalf("want 3 authors, got %d (%+v)", len(got), got)
	}
	if got[0].Author != "alice" || got[0].Commits != 2 {
		t.Errorf("want alice=2 first, got %+v", got[0])
	}
	// falls back to commit.author.name when no login
	foundCarol := false
	for _, a := range got {
		if a.Author == "Carol" && a.Commits == 1 {
			foundCarol = true
		}
	}
	if !foundCarol {
		t.Errorf("want Carol=1 via commit.author.name fallback, got %+v", got)
	}
}

func TestNvWhoTouched(t *testing.T) {
	commits := commitsFromJSON(t, `[
		{"author":{"login":"alice"},"commit":{"author":{"date":"2026-01-10T00:00:00Z"}}},
		{"author":{"login":"alice"},"commit":{"author":{"date":"2026-03-20T00:00:00Z"}}},
		{"author":{"login":"bob"},"commit":{"author":{"date":"2026-02-01T00:00:00Z"}}}
	]`)
	got := nvWhoTouched(commits)
	if len(got) != 2 || got[0].Author != "alice" || got[0].Commits != 2 {
		t.Fatalf("want alice=2 first, got %+v", got)
	}
	if got[0].FirstSeen != "2026-01-10T00:00:00Z" || got[0].LastSeen != "2026-03-20T00:00:00Z" {
		t.Errorf("alice first/last wrong: %+v", got[0])
	}
}

func TestNvLabelCoverage(t *testing.T) {
	issues := rawSlice(
		`{"state":"open","labels":[{"name":"bug"},{"name":"p1"}]}`,
		`{"state":"closed","labels":[{"name":"bug"}]}`,
		`{"state":"open","labels":[]}`,
	)
	repoLabels := rawSlice(
		`{"name":"bug"}`, `{"name":"p1"}`, `{"name":"wontfix"}`,
	)
	rep := nvLabelCoverage(issues, repoLabels)
	var bug *labelStat
	for i := range rep.Labels {
		if rep.Labels[i].Label == "bug" {
			bug = &rep.Labels[i]
		}
	}
	if bug == nil || bug.Open != 1 || bug.Closed != 1 || bug.Total != 2 {
		t.Errorf("bug label want open1/closed1/total2, got %+v", bug)
	}
	if rep.UnlabeledOpen != 1 {
		t.Errorf("want 1 unlabeled open issue, got %d", rep.UnlabeledOpen)
	}
	if len(rep.UnusedLabels) != 1 || rep.UnusedLabels[0] != "wontfix" {
		t.Errorf("want unused=[wontfix], got %v", rep.UnusedLabels)
	}
}

func TestNvIssueRow(t *testing.T) {
	r := nvIssueRow([]byte(`{"number":42,"title":"crash","state":"open","html_url":"https://x/42"}`))
	if r.Number != 42 || r.Title != "crash" || r.State != "open" || r.URL != "https://x/42" {
		t.Errorf("unexpected issueRow: %+v", r)
	}
}

func TestNvMentionRow(t *testing.T) {
	commit := nvMentionRow("commits", []byte(`{"sha":"deadbeefcafe","commit":{"message":"fix ParseConfig\n\nbody"}}`))
	if commit.Type != "commit" || commit.Ref != "deadbeefcafe" || commit.Title != "fix ParseConfig" {
		t.Errorf("commit mention row wrong: %+v", commit)
	}
	pull := nvMentionRow("pulls", []byte(`{"number":7,"title":"PR title"}`))
	if pull.Type != "pull" || pull.Number != 7 {
		t.Errorf("pull mention row wrong: %+v", pull)
	}
	issue := nvMentionRow("issues", []byte(`{"number":9,"title":"issue title"}`))
	if issue.Type != "issue" || issue.Number != 9 {
		t.Errorf("issue mention row wrong: %+v", issue)
	}
}

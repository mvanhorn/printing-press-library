// Copyright 2026 Brandon Nye and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the pure aggregation logic behind the transcendence commands.

package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/github/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/github/internal/config"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/github/internal/store"
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

func TestNvIssueDupesKeepsOnlyOpenMatches(t *testing.T) {
	matches := []json.RawMessage{
		json.RawMessage(`{"number":1,"title":"closed match","state":"closed"}`),
		json.RawMessage(`{"number":2,"title":"open match","state":"open"}`),
	}
	rows := nvOpenIssueRows(matches, 10)
	if len(rows) != 1 || rows[0].Number != 2 {
		t.Fatalf("wanted only open duplicate candidate, got %+v", rows)
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

func TestNvCommitReferencesIssueHonorsNumberBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{name: "exact", message: "fix #12", want: true},
		{name: "punctuation", message: "closes (#12).", want: true},
		{name: "larger issue", message: "fix #120", want: false},
		{name: "identifier suffix", message: "fix #12abc", want: false},
		{name: "identifier prefix", message: "fixes#12", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := json.RawMessage(`{"commit":{"message":` + strconv.Quote(tt.message) + `}}`)
			if got := nvCommitReferencesIssue(raw, "12"); got != tt.want {
				t.Fatalf("nvCommitReferencesIssue(%q) = %v, want %v", tt.message, got, tt.want)
			}
		})
	}
}

func TestNvCommentMentionsEscapesWildcardsAndAppliesCutoff(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "mentions.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	fixtures := map[string]string{
		"literal-recent": `{"body":"my_function failed","updated_at":"2026-07-19T00:00:00Z","issue_url":"https://api.github.com/repos/o/r/issues/1"}`,
		"wildcard-only":  `{"body":"myXfunction failed","updated_at":"2026-07-19T00:00:00Z","issue_url":"https://api.github.com/repos/o/r/issues/2"}`,
		"literal-old":    `{"body":"my_function failed","updated_at":"2026-06-01T00:00:00Z","issue_url":"https://api.github.com/repos/o/r/issues/3"}`,
	}
	for id, raw := range fixtures {
		if err := st.Upsert("issues_comments", id, json.RawMessage(raw)); err != nil {
			t.Fatal(err)
		}
	}

	cutoff := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	hits, err := nvCommentMentions(st, "my_function", 25, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Ref != "#1" {
		t.Fatalf("wanted only recent literal match, got %+v", hits)
	}
}

func TestNvClearRepoScopedResources(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "repo-switch.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.Upsert("issues", "1", json.RawMessage(`{"number":1,"repository_url":"https://api.github.com/repos/old/repo"}`)); err != nil {
		t.Fatal(err)
	}
	if err := st.Upsert("unrelated", "1", json.RawMessage(`{"name":"keep"}`)); err != nil {
		t.Fatal(err)
	}
	if err := nvClearRepoScopedResources(st); err != nil {
		t.Fatal(err)
	}
	issues, err := st.List("issues", 0)
	if err != nil {
		t.Fatal(err)
	}
	unrelated, err := st.List("unrelated", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 || len(unrelated) != 1 {
		t.Fatalf("repo clear removed wrong rows: issues=%d unrelated=%d", len(issues), len(unrelated))
	}
}

func TestNvPopulateReportsPageCapAndMalformedResponses(t *testing.T) {
	t.Run("page cap", func(t *testing.T) {
		items := make([]map[string]any, 100)
		for i := range items {
			items[i] = map[string]any{"id": i + 1}
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if err := json.NewEncoder(w).Encode(items); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()

		st, err := store.Open(filepath.Join(t.TempDir(), "cap.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer st.Close()
		c := client.New(&config.Config{BaseURL: server.URL}, time.Second, 0)
		c.NoCache = true

		count, truncated, err := nvPopulate(context.Background(), c, st, "o", "r", "issues", 1)
		if err != nil {
			t.Fatal(err)
		}
		if count != 100 || !truncated {
			t.Fatalf("count=%d truncated=%v, want 100 true", count, truncated)
		}
	})

	t.Run("full page with PRs still paginates", func(t *testing.T) {
		page1 := make([]map[string]any, 100)
		for i := range page1 {
			item := map[string]any{"id": i + 1, "title": "issue"}
			if i%10 == 0 {
				item["pull_request"] = map[string]any{"url": "https://api.github.com/repos/o/r/pulls/" + strconv.Itoa(i+1)}
			}
			page1[i] = item
		}
		page2 := []map[string]any{{"id": 101, "title": "later"}}
		var hits int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits++
			page := r.URL.Query().Get("page")
			items := page1
			if page == "2" {
				items = page2
			}
			if err := json.NewEncoder(w).Encode(items); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()

		st, err := store.Open(filepath.Join(t.TempDir(), "paginate.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer st.Close()
		c := client.New(&config.Config{BaseURL: server.URL}, time.Second, 0)
		c.NoCache = true

		count, truncated, err := nvPopulate(context.Background(), c, st, "o", "r", "issues", 2)
		if err != nil {
			t.Fatal(err)
		}
		if hits != 2 {
			t.Fatalf("fetched %d pages, want 2 (raw page length 100 must continue)", hits)
		}
		if count != 91 || truncated {
			t.Fatalf("count=%d truncated=%v, want 91 false", count, truncated)
		}
	})

	t.Run("malformed response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"not":"an array"}`))
		}))
		defer server.Close()

		st, err := store.Open(filepath.Join(t.TempDir(), "malformed.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer st.Close()
		c := client.New(&config.Config{BaseURL: server.URL}, time.Second, 0)
		c.NoCache = true

		if _, _, err := nvPopulate(context.Background(), c, st, "o", "r", "issues", 1); err == nil {
			t.Fatal("expected malformed populate response to return an error")
		}
	})
}

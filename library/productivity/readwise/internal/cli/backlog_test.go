// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"time"
)

// TestNovelBacklogHelpWires smoke-tests that the backlog command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelBacklogHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"backlog", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("backlog --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "backlog"} {
		if !strings.Contains(help, want) {
			t.Fatalf("backlog --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelEstimateMinutes(t *testing.T) {
	tests := []struct {
		name        string
		readingTime string
		wordCount   int
		want        int
	}{
		{name: "reading time wins", readingTime: "4 mins", wordCount: 2000, want: 4},
		{name: "word count fallback rounds up", wordCount: 239, want: 2},
		{name: "minimum fallback", want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := novelEstimateMinutes(tt.readingTime, tt.wordCount); got != tt.want {
				t.Fatalf("novelEstimateMinutes() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNovelBacklogReportBucketsAndBudget(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	docs := []novelBacklogDoc{
		{ID: "a", Title: "A", Location: "later", EstMinutes: 10, SavedAt: now.Add(-2 * 24 * time.Hour)},
		{ID: "b", Title: "B", Location: "later", EstMinutes: 25, SavedAt: now.Add(-40 * 24 * time.Hour)},
		{ID: "c", Title: "C", Location: "new", EstMinutes: 5, ReadingProgress: 0.5, LastOpenedAt: now.Add(-40 * 24 * time.Hour)},
	}
	view := novelBacklogReport(docs, nil, 30*time.Minute, "length", now)
	if view.Totals.UnreadCount != 2 || view.Totals.EstimatedMinutes != 35 {
		t.Fatalf("totals = %+v, want unread=2 minutes=35", view.Totals)
	}
	if view.Abandoned != 1 {
		t.Fatalf("abandoned = %d, want 1", view.Abandoned)
	}
	if len(view.Worklist) != 2 || view.Worklist[0].ID != "c" || view.Worklist[1].ID != "a" {
		t.Fatalf("worklist = %+v, want started c and unread a to fit budget", view.Worklist)
	}
}

// sliceDStore seeds only an isolated temp database and applies fixture SQL.
func sliceDStore(t *testing.T, statements ...string) *store.Store {
	t.Helper()
	seedNovelCommandStore(t)
	db, err := store.OpenWithContext(context.Background(), defaultDBPath("readwise-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	for _, statement := range statements {
		if _, err := db.DB().Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	return db
}
func sliceDView(t *testing.T, out string, target any) {
	t.Helper()
	if err := json.Unmarshal([]byte(out), target); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}
func TestSliceDBacklogScopeLengthAndCaps(t *testing.T) {
	sliceDStore(t,
		"UPDATE documents SET location='archive' WHERE id='doc-read'",
		"UPDATE documents SET reading_progress=0 WHERE id='doc-read'",
		"UPDATE documents SET word_count=NULL, reading_time=NULL WHERE id='doc-2'",
	)
	var view novelBacklogView
	sliceDView(t, runNovelCommand(t, "backlog", "--limit", "1"), &view)
	if view.TotalDocuments != 3 || len(view.Worklist) != 1 || !view.Truncated || !strings.Contains(view.Note, "--limit") || strings.Join(view.Locations, ",") != "later,new,shortlist" {
		t.Fatalf("%+v", view)
	}
	sliceDView(t, runNovelCommand(t, "backlog", "--limit", "0"), &view)
	if len(view.Worklist) != 3 || view.Truncated {
		t.Fatalf("%+v", view)
	}
	found := false
	for _, item := range view.Worklist {
		if item.ID == "doc-2" {
			found = true
			if item.Minutes != nil || item.LengthKnown {
				t.Fatalf("%+v", item)
			}
		}
	}
	if !found {
		t.Fatal("unknown length missing")
	}
	sliceDView(t, runNovelCommand(t, "backlog", "--budget", "30m"), &view)
	if view.UnknownLengthExcluded != 1 || len(view.Worklist) != 2 {
		t.Fatalf("%+v", view)
	}
	sliceDView(t, runNovelCommand(t, "backlog", "--budget", "30m", "--include-unknown-length"), &view)
	if view.UnknownLengthExcluded != 0 || len(view.Worklist) != 3 {
		t.Fatalf("%+v", view)
	}
	sliceDView(t, runNovelCommand(t, "backlog", "--location", "archive"), &view)
	if view.TotalDocuments != 1 || view.Worklist[0].ID != "doc-read" {
		t.Fatalf("%+v", view)
	}
}
func TestSliceDBacklogPriorityMatchesDisplayedScore(t *testing.T) {
	sliceDStore(t,
		"UPDATE documents SET saved_at='2026-01-01T00:00:00Z', created_at=NULL, word_count=238, reading_time=NULL, location='new'",
		"UPDATE documents SET location='shortlist', reading_progress=0.5, word_count=2380 WHERE id='doc-3'",
	)
	var view novelBacklogView
	sliceDView(t, runNovelCommand(t, "backlog", "--budget", "30m", "--sort", "priority"), &view)
	if len(view.Worklist) != 3 || view.Worklist[0].ID != "doc-3" {
		t.Fatalf("%+v", view)
	}
	for i := 1; i < len(view.Worklist); i++ {
		if view.Worklist[i-1].Score < view.Worklist[i].Score {
			t.Fatal("score/sort mismatch")
		}
	}
}
func TestSliceDDBPathHelp(t *testing.T) {
	sliceDStore(t)
	for _, args := range [][]string{{"backlog"}, {"read-next"}, {"dedupe"}, {"triage", "stale"}, {"connections"}, {"ask"}, {"digest"}, {"mirror"}} {
		cmd := RootCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(append(args, "--help"))
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(out.String(), "\n") {
			if strings.Contains(line, "--db-path") && (strings.Contains(line, "(default:") || strings.Contains(line, "(default \"")) {
				t.Fatalf("duplicate default: %s", line)
			}
		}
	}
}

func TestSliceDDefaultFiftyCaps(t *testing.T) {
	sliceDStore(t,
		`WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<55)
 INSERT INTO documents(id,title,author,url,source_url,location,word_count,saved_at,data)
 SELECT 'cap-'||x,'Article '||x,'Writer','https://cap/'||x,'https://cap/'||x,'later',238,'2020-01-01T00:00:00Z','{}' FROM n`,
		`WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<55)
 INSERT INTO books(id,title,source_url,data) SELECT 'cap-'||x,'Article '||x,'https://cap/'||x,'{}' FROM n`,
	)
	var backlog novelBacklogView
	sliceDView(t, runNovelCommand(t, "backlog"), &backlog)
	if len(backlog.Worklist) != 50 || !backlog.Truncated || backlog.TotalDocuments != 58 {
		t.Fatalf("%+v", backlog)
	}
	var stale novelTriageStaleView
	sliceDView(t, runNovelCommand(t, "triage", "stale"), &stale)
	if len(stale.StaleDocuments) != 50 || len(stale.OrphanSources) != 50 || stale.TotalOrphanSources != 56 || !stale.Truncated {
		t.Fatalf("%+v", stale)
	}
	var dedupe novelDedupeView
	sliceDView(t, runNovelCommand(t, "dedupe"), &dedupe)
	if len(dedupe.Clusters) != 50 || dedupe.TotalClusters != 56 || !dedupe.Truncated {
		t.Fatalf("%+v", dedupe)
	}
}

func TestE3LimitMetadata(t *testing.T) {
	sliceDStore(t)
	for _, args := range [][]string{{"backlog", "--limit", "1"}, {"triage", "stale", "--never-opened", "--older-than", "1d", "--limit", "1"}} {
		out := runNovelCommand(t, args...)
		var view map[string]any
		sliceDView(t, out, &view)
		if view["limited"] != true || view["limit_note"] == nil || view["truncated"] != nil || view["note"] != nil {
			t.Fatalf("limit contract: %s", out)
		}
		if strings.HasPrefix(view["limit_note"].(string), " ") {
			t.Fatal(out)
		}
	}
}

func TestE3BacklogUnknownBudgetBound(t *testing.T) {
	sliceDStore(t, "UPDATE documents SET word_count=NULL,reading_time=NULL")
	var view novelBacklogView
	sliceDView(t, runNovelCommand(t, "backlog", "--budget", "30m", "--include-unknown-length", "--limit", "1"), &view)
	if len(view.Worklist) != 1 || !view.Truncated || view.TotalDocuments != 3 {
		t.Fatalf("%+v", view)
	}
}

func TestE3BacklogStartedBonus(t *testing.T) {
	now := time.Now()
	unread := novelBacklogDoc{ID: "a", Location: "later", EstMinutes: 5, SavedAt: now}
	started := unread
	started.ID = "z"
	started.ReadingProgress = 0.5
	view := novelBacklogReport([]novelBacklogDoc{unread, started}, nil, 0, "priority", now)
	if len(view.Worklist) != 2 || view.Worklist[0].ID != "z" || view.Worklist[0].Score-view.Worklist[1].Score != 5 {
		t.Fatalf("%+v", view.Worklist)
	}
}

func TestE3ReadingTimeAllocation(t *testing.T) {
	if !novelReadingTimeKnown("12 min") || novelReadingTimeKnown("unknown") {
		t.Fatal("parsing changed")
	}
	allocs := testing.AllocsPerRun(100, func() { novelReadingTimeKnown("12 min") })
	if allocs > 3 {
		t.Fatalf("reading-time regexp recompiled: %.0f allocations", allocs)
	}
}

// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"time"
)

// TestNovelTriageStaleHelpWires smoke-tests that the triage stale command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelTriageStaleHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"triage", "stale", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("triage stale --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "stale"} {
		if !strings.Contains(help, want) {
			t.Fatalf("triage stale --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelTriageStaleDurationParsing(t *testing.T) {
	got, err := cliutil.ParseDurationLoose("180d")
	if err != nil {
		t.Fatal(err)
	}
	if got != 180*24*time.Hour {
		t.Fatalf("duration = %s, want 180d equivalent", got)
	}
}

func TestNovelTriageStaleReportSeededStore(t *testing.T) {
	seedNovelCommandStore(t)
	out := runNovelCommand(t, "triage", "stale", "--older-than", "180d", "--never-opened", "--zero-highlights")
	var view novelTriageStaleView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("triage output is not JSON: %v\n%s", err, out)
	}
	if !hasStaleDocument(view.StaleDocuments, "doc-2") {
		t.Fatalf("stale documents = %+v, want old never-opened doc-2", view.StaleDocuments)
	}
	if !hasOrphanSource(view.OrphanSources, "2") {
		t.Fatalf("orphan sources = %+v, want zero-highlight book 2", view.OrphanSources)
	}
	if hasOrphanSource(view.OrphanSources, "1") {
		t.Fatalf("orphan sources = %+v, nonzero-highlight book 1 must not be an orphan", view.OrphanSources)
	}
	if hasOrphanSource(view.OrphanSources, "doc-3") {
		t.Fatalf("orphan sources = %+v, Reader document doc-3 must not be an orphan", view.OrphanSources)
	}
}

func hasStaleDocument(docs []novelStaleDocument, id string) bool {
	for _, doc := range docs {
		if doc.ID == id {
			return true
		}
	}
	return false
}

func hasOrphanSource(sources []novelOrphanSource, id string) bool {
	for _, source := range sources {
		if source.ID == id {
			return true
		}
	}
	return false
}

func TestSliceDStaleUsesLocalHighlightsAndLimits(t *testing.T) {
	sliceDStore(t,
		"UPDATE books SET data=json_set(data, '$.num_highlights', 0) WHERE id='1'",
		"INSERT INTO books(id,title,data) VALUES('3',NULL,'{}')",
	)
	var view novelTriageStaleView
	sliceDView(t, runNovelCommand(t, "triage", "stale", "--limit", "1"), &view)
	if view.TotalOrphanSources != 2 || view.TotalStaleDocuments < 2 || len(view.OrphanSources) != 1 || len(view.StaleDocuments) != 1 || !view.Truncated || !strings.Contains(view.Note, "--limit") {
		t.Fatalf("%+v", view)
	}
	sliceDView(t, runNovelCommand(t, "triage", "stale", "--limit", "0"), &view)
	if hasOrphanSource(view.OrphanSources, "1") || !hasOrphanSource(view.OrphanSources, "3") || len(view.OrphanSources) != 2 {
		t.Fatalf("%+v", view)
	}
	for _, row := range view.OrphanSources {
		if row.HighlightsLocal != 0 {
			t.Fatalf("%+v", row)
		}
	}
}

func TestE3TriageNotSynced(t *testing.T) {
	sliceDStore(t, "UPDATE books SET data=json_set(data,'$.num_highlights',7) WHERE id='2'")
	var view novelTriageStaleView
	sliceDView(t, runNovelCommand(t, "triage", "stale", "--zero-highlights"), &view)
	if hasOrphanSource(view.OrphanSources, "2") || !hasOrphanSource(view.NotSyncedSources, "2") || view.TotalNotSyncedSources != 1 || view.NotSyncedSources[0].Kind != "not_synced" {
		t.Fatalf("%+v", view)
	}
}

func TestE3TriageTimestampOrder(t *testing.T) {
	sliceDStore(t, "UPDATE documents SET saved_at='2026-06-01T00:00:00Z',last_opened_at='opened'", "UPDATE documents SET saved_at='2026-06-15T01:00:00+02:00',last_opened_at=NULL WHERE id='doc-1'", "UPDATE documents SET saved_at='2026-06-14T23:30:00-02:00',last_opened_at=NULL WHERE id='doc-2'")
	now := time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)
	view, err := novelTriageStaleReport(newNovelTriageStaleCmd(&rootFlags{}), 24*time.Hour, true, false, now)
	if err != nil {
		t.Fatal(err)
	}
	if !hasStaleDocument(view.StaleDocuments, "doc-1") || hasStaleDocument(view.StaleDocuments, "doc-2") {
		t.Fatalf("%+v", view)
	}
}

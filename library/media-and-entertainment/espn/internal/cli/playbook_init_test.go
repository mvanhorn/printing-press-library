// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/internal/cli/playbooks"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/internal/store"
)

func TestPlaybookInit_SeedsAllShippedPlaybooks(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	if err := installPlaybooksFromEmbed(context.Background(), s); err != nil {
		t.Fatalf("install: %v", err)
	}

	rows, err := s.ListPlaybooks()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	// Sentinel + per-family rows.
	if len(rows) < 4 {
		t.Errorf("expected at least 4 rows (sentinel + several playbooks); got %d", len(rows))
	}

	var foundSentinel bool
	var foundSeasonRecap bool
	var foundLeagueTopBottom bool
	for _, r := range rows {
		if r.QueryFamily == playbookSeedSentinelFamily {
			foundSentinel = true
			if r.NotesText != playbooks.SeedVersion {
				t.Errorf("sentinel notes_text = %q, want %q", r.NotesText, playbooks.SeedVersion)
			}
		}
		if strings.Contains(r.QueryFamily, "end") && strings.Contains(r.QueryFamily, "season") {
			foundSeasonRecap = true
			if !strings.Contains(r.NotesText, "teamShortName") {
				t.Errorf("season_recap notes should contain 'teamShortName' correction; got first 100 chars: %q", firstN(r.NotesText, 100))
			}
		}
		// The merged league_top_bottom playbook covers all leagues. Its
		// family is derived from "top 3 mlb teams in each division" which,
		// after the U1 stopword change (mlb/nba/nfl/nhl/mls all become
		// stopwords), normalizes to a family containing "division" and
		// "teams" but NOT "mlb"/"nba".
		if strings.Contains(r.QueryFamily, "division") && strings.Contains(r.QueryFamily, "teams") && strings.Contains(r.QueryFamily, "top") {
			foundLeagueTopBottom = true
			// Notes should carry both MLB and NBA division maps now.
			if !strings.Contains(r.NotesText, "MLB") || !strings.Contains(r.NotesText, "NBA") {
				t.Errorf("merged league_top_bottom notes should contain BOTH MLB and NBA division maps; got first 200 chars: %q", firstN(r.NotesText, 200))
			}
		}
	}
	if !foundSentinel {
		t.Error("sentinel row missing after install")
	}
	if !foundSeasonRecap {
		t.Error("season_recap playbook missing after install")
	}
	if !foundLeagueTopBottom {
		t.Error("merged league_top_bottom playbook missing after install")
	}
}

func TestPlaybookInit_Idempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	if err := installPlaybooksFromEmbed(context.Background(), s); err != nil {
		t.Fatalf("first install: %v", err)
	}
	firstRows, _ := s.ListPlaybooks()
	if err := installPlaybooksFromEmbed(context.Background(), s); err != nil {
		t.Fatalf("second install: %v", err)
	}
	secondRows, _ := s.ListPlaybooks()
	if len(firstRows) != len(secondRows) {
		t.Errorf("re-install drifted: first=%d second=%d", len(firstRows), len(secondRows))
	}
}

func TestPlaybookInit_ConcurrentSafe(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = installPlaybooksFromEmbed(context.Background(), s)
		}()
	}
	wg.Wait()

	rows, _ := s.ListPlaybooks()
	// Sentinel should be exactly 1; non-sentinel rows should match the
	// number of JSON files in the embed FS. Concurrent writers should
	// not produce duplicates (UpsertPlaybook handles the race).
	sentinelCount := 0
	for _, r := range rows {
		if r.QueryFamily == playbookSeedSentinelFamily {
			sentinelCount++
		}
	}
	if sentinelCount != 1 {
		t.Errorf("expected exactly 1 sentinel row, got %d", sentinelCount)
	}
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

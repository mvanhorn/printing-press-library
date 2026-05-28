// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

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
	// ListPlaybooks hides the sentinel row, so only per-family rows
	// surface here. Verify the sentinel exists separately via
	// GetPlaybookByFamily below.
	if len(rows) < 3 {
		t.Errorf("expected at least 3 user-facing playbook rows; got %d", len(rows))
	}
	for _, r := range rows {
		if r.QueryFamily == playbookSeedSentinelFamily {
			t.Errorf("ListPlaybooks should hide the seed sentinel; got family=%q", r.QueryFamily)
		}
	}

	// Sentinel is the durable signal that install completed; check it
	// directly rather than via the user-facing list.
	sentinel, ok, err := s.GetPlaybookByFamily(playbookSeedSentinelFamily)
	if err != nil {
		t.Fatalf("get sentinel: %v", err)
	}
	foundSentinel := ok
	if ok && sentinel.NotesText != playbooks.SeedVersion {
		t.Errorf("sentinel notes_text = %q, want %q", sentinel.NotesText, playbooks.SeedVersion)
	}

	var foundSeasonRecap bool
	var foundLeagueTopBottom bool
	for _, r := range rows {
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

	// Sentinel should exist exactly once; concurrent writers must not
	// duplicate it (UpsertPlaybook handles the race). The sentinel is
	// hidden from ListPlaybooks so we check via GetPlaybookByFamily.
	if _, ok, err := s.GetPlaybookByFamily(playbookSeedSentinelFamily); err != nil {
		t.Fatalf("get sentinel: %v", err)
	} else if !ok {
		t.Error("expected sentinel row after concurrent installs, got none")
	}
}

// TestPlaybookInit_ReseedReplacesNotesWithoutAmend confirms that a
// SeedVersion bump replaces the stored notes_text when no
// `[amend ...]` marker is present — otherwise the bump would have no
// effect on existing users (PreserveExistingNotes blocks the change).
// The seed loop's marker check is what lets corrected notes ship.
func TestPlaybookInit_ReseedReplacesNotesWithoutAmend(t *testing.T) {
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

	// Find any family row with non-empty notes_text we can mutate.
	rows, _ := s.ListPlaybooks()
	var target string
	for _, r := range rows {
		if r.NotesText != "" {
			target = r.QueryFamily
			break
		}
	}
	if target == "" {
		t.Fatal("no seeded row with notes to mutate; can't exercise the re-seed path")
	}

	// Overwrite the stored notes with stale content (no amend marker).
	if _, _, err := s.UpsertPlaybook(store.UpsertPlaybookInput{
		QueryFamily:  target,
		PlaybookJSON: "{}",
		NotesText:    "STALE PRE-CORRECTION CONTENT",
		Source:       store.LearningSourceTaught,
	}); err != nil {
		t.Fatalf("seed stale notes: %v", err)
	}
	// Reset the sentinel so the next install re-seeds.
	if _, _, err := s.UpsertPlaybook(store.UpsertPlaybookInput{
		QueryFamily: playbookSeedSentinelFamily,
		NotesText:   "old-version",
		Source:      "seeded",
	}); err != nil {
		t.Fatalf("reset sentinel: %v", err)
	}

	if err := installPlaybooksFromEmbed(context.Background(), s); err != nil {
		t.Fatalf("re-install: %v", err)
	}

	got, _, _ := s.GetPlaybookByFamily(target)
	if got.NotesText == "STALE PRE-CORRECTION CONTENT" {
		t.Errorf("re-seed should overwrite stale notes without amend marker; still got STALE content")
	}
}

// TestPlaybookInit_ReseedPreservesNotesWithAmend confirms the
// complementary path: when the stored notes contain a `[amend ...]`
// marker (an agent-authored correction), a SeedVersion bump preserves
// the existing content so the agent's work isn't lost.
func TestPlaybookInit_ReseedPreservesNotesWithAmend(t *testing.T) {
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

	rows, _ := s.ListPlaybooks()
	var target string
	for _, r := range rows {
		if r.NotesText != "" {
			target = r.QueryFamily
			break
		}
	}
	if target == "" {
		t.Fatal("no seeded row to mutate")
	}

	const amended = "base content\n\n[amend 2026-05-25T03:14Z]: agent gotcha"
	if _, _, err := s.UpsertPlaybook(store.UpsertPlaybookInput{
		QueryFamily:  target,
		PlaybookJSON: "{}",
		NotesText:    amended,
		Source:       store.LearningSourceTaught,
	}); err != nil {
		t.Fatalf("seed amended notes: %v", err)
	}
	if _, _, err := s.UpsertPlaybook(store.UpsertPlaybookInput{
		QueryFamily: playbookSeedSentinelFamily,
		NotesText:   "old-version",
		Source:      "seeded",
	}); err != nil {
		t.Fatalf("reset sentinel: %v", err)
	}

	if err := installPlaybooksFromEmbed(context.Background(), s); err != nil {
		t.Fatalf("re-install: %v", err)
	}

	got, _, _ := s.GetPlaybookByFamily(target)
	if got.NotesText != amended {
		t.Errorf("re-seed should preserve notes containing [amend ...] marker; got %q", firstN(got.NotesText, 200))
	}
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

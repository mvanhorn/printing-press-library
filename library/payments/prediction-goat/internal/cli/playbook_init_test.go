// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// playbook_init_test.go exercises the embed-FS auto-install path
// from U9. Tests inject an fstest.MapFS rather than reading the
// real playbooks.FS so scenarios are scoped to the file under test
// (and U10's hand-authored content can land independently).

package cli

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/cli/playbooks"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

// twoPlaybookFS returns an fstest.MapFS with two minimal playbooks +
// notes pairs. Both have query_family_examples that normalize to
// non-empty families, so installPlaybooksFromEmbed will upsert under
// each.
func twoPlaybookFS() fstest.MapFS {
	return fstest.MapFS{
		"odds_for_team.json": &fstest.MapFile{
			Data: []byte(`{
  "query_family_examples": ["chiefs super bowl odds", "lakers championship odds"],
  "steps": [{"cmd": "prediction-goat-pp-cli markets list --search $TEAM", "purpose": "find market"}],
  "entity_slots": ["$TEAM"],
  "expected_tool_calls": 2
}`),
		},
		"odds_for_team_notes.md": &fstest.MapFile{
			Data: []byte("# odds for team\n\nUse `markets list --search` to find the team-specific market.\n"),
		},
		"event_markets.json": &fstest.MapFile{
			Data: []byte(`{
  "query_family_examples": ["super bowl markets", "world cup markets"],
  "steps": [{"cmd": "prediction-goat-pp-cli events list --search $EVENT", "purpose": "find event"}],
  "entity_slots": ["$EVENT"],
  "expected_tool_calls": 2
}`),
		},
		"event_markets_notes.md": &fstest.MapFile{
			Data: []byte("# event markets\n\nUse `events list --search` to find the event.\n"),
		},
	}
}

func TestPlaybookInit_SeedsAllPlaybooks(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	if err := installPlaybooksFromEmbed(context.Background(), s, twoPlaybookFS()); err != nil {
		t.Fatalf("install: %v", err)
	}

	rows, err := s.ListPlaybooks()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	// Each JSON declares 2 examples → potentially 2 distinct families
	// per file. Worst case 4 rows, best case 2 (if examples normalize
	// to identical families). Either way, at least 2 non-sentinel rows.
	if len(rows) < 2 {
		t.Errorf("expected at least 2 playbook rows; got %d", len(rows))
	}
	for _, r := range rows {
		if r.QueryFamily == playbookSeedSentinelFamily {
			t.Errorf("ListPlaybooks should hide the seed sentinel; got family=%q", r.QueryFamily)
		}
	}

	// Sentinel exists with the correct SeedVersion.
	sentinel, ok, err := s.GetPlaybookByFamily(playbookSeedSentinelFamily)
	if err != nil {
		t.Fatalf("get sentinel: %v", err)
	}
	if !ok {
		t.Fatal("sentinel row missing after install")
	}
	if sentinel.NotesText != playbooks.SeedVersion {
		t.Errorf("sentinel notes_text = %q, want %q", sentinel.NotesText, playbooks.SeedVersion)
	}

	// Notes content should be present on the seeded rows.
	hasOddsNotes := false
	hasEventNotes := false
	for _, r := range rows {
		if strings.Contains(r.NotesText, "odds for team") {
			hasOddsNotes = true
		}
		if strings.Contains(r.NotesText, "event markets") {
			hasEventNotes = true
		}
	}
	if !hasOddsNotes {
		t.Error("expected odds_for_team notes to be seeded")
	}
	if !hasEventNotes {
		t.Error("expected event_markets notes to be seeded")
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

	embedFS := twoPlaybookFS()
	if err := installPlaybooksFromEmbed(context.Background(), s, embedFS); err != nil {
		t.Fatalf("first install: %v", err)
	}
	firstRows, _ := s.ListPlaybooks()
	if err := installPlaybooksFromEmbed(context.Background(), s, embedFS); err != nil {
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

	embedFS := twoPlaybookFS()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = installPlaybooksFromEmbed(context.Background(), s, embedFS)
		}()
	}
	wg.Wait()

	// Sentinel should exist exactly once after concurrent installs.
	if _, ok, err := s.GetPlaybookByFamily(playbookSeedSentinelFamily); err != nil {
		t.Fatalf("get sentinel: %v", err)
	} else if !ok {
		t.Error("expected sentinel row after concurrent installs, got none")
	}

	// No duplicate playbook rows: ListPlaybooks dedupes via the
	// query_family unique index. Sample one family to confirm.
	rows, err := s.ListPlaybooks()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	seen := make(map[string]int)
	for _, r := range rows {
		seen[r.QueryFamily]++
	}
	for fam, count := range seen {
		if count > 1 {
			t.Errorf("duplicate row for family=%q (count=%d)", fam, count)
		}
	}
}

// TestPlaybookInit_ReseedReplacesNotesWithoutAmend confirms that a
// SeedVersion bump replaces the stored notes_text when no
// `[amend ...]` marker is present.
func TestPlaybookInit_ReseedReplacesNotesWithoutAmend(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	embedFS := twoPlaybookFS()
	if err := installPlaybooksFromEmbed(context.Background(), s, embedFS); err != nil {
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
		t.Fatal("no seeded row with notes to mutate")
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

	if err := installPlaybooksFromEmbed(context.Background(), s, embedFS); err != nil {
		t.Fatalf("re-install: %v", err)
	}

	got, _, _ := s.GetPlaybookByFamily(target)
	if got.NotesText == "STALE PRE-CORRECTION CONTENT" {
		t.Errorf("re-seed should overwrite stale notes without amend marker; still got STALE content")
	}
}

// TestPlaybookInit_ReseedPreservesNotesWithAmend confirms the
// complementary path: stored notes containing a `[amend ...]` marker
// survive a SeedVersion bump.
func TestPlaybookInit_ReseedPreservesNotesWithAmend(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	embedFS := twoPlaybookFS()
	if err := installPlaybooksFromEmbed(context.Background(), s, embedFS); err != nil {
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

	// Use the canonical amend marker shape so the regex in
	// installPlaybooksFromEmbed fires preserve=true. The leading
	// newline is the durable anchor amendMarkerRe requires.
	amended := "base content\n[amend 2026-05-26T03:14Z]: agent gotcha"
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

	if err := installPlaybooksFromEmbed(context.Background(), s, embedFS); err != nil {
		t.Fatalf("re-install: %v", err)
	}

	got, _, _ := s.GetPlaybookByFamily(target)
	if got.NotesText != amended {
		t.Errorf("re-seed should preserve notes containing [amend ...] marker;\n got %q\nwant %q",
			firstNChars(got.NotesText, 200), amended)
	}
}

// TestPlaybookInit_AmendMarkerSpecificity ensures the regex does NOT
// match a literal "[amend " token that appears in user-authored
// notes content WITHOUT the timestamp anchor (Greptile round 4
// finding: the bare "[amend " heuristic was too loose).
func TestPlaybookInit_AmendMarkerSpecificity(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	embedFS := twoPlaybookFS()
	if err := installPlaybooksFromEmbed(context.Background(), s, embedFS); err != nil {
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

	// User-authored notes that mention "[amend ..." literally in
	// prose, NOT as the durable timestamped marker. The regex's
	// \n\[amend \d{4}- anchor should refuse to match this — the
	// content lacks the leading-newline+year-prefix anchor.
	loose := "this note explains how the [amend foo] flag works in some other tool"
	if _, _, err := s.UpsertPlaybook(store.UpsertPlaybookInput{
		QueryFamily:  target,
		PlaybookJSON: "{}",
		NotesText:    loose,
		Source:       store.LearningSourceTaught,
	}); err != nil {
		t.Fatalf("seed loose notes: %v", err)
	}
	if _, _, err := s.UpsertPlaybook(store.UpsertPlaybookInput{
		QueryFamily: playbookSeedSentinelFamily,
		NotesText:   "old-version",
		Source:      "seeded",
	}); err != nil {
		t.Fatalf("reset sentinel: %v", err)
	}

	if err := installPlaybooksFromEmbed(context.Background(), s, embedFS); err != nil {
		t.Fatalf("re-install: %v", err)
	}

	got, _, _ := s.GetPlaybookByFamily(target)
	if got.NotesText == loose {
		t.Errorf("re-seed should overwrite loose [amend literal that lacks timestamp anchor; got preserved content")
	}
}

// TestPlaybookInit_SkipsPlaybookWithoutExamples covers the Greptile
// round 2 finding: a JSON with no query_family_examples is unreachable
// at recall time, so the install path refuses to seed it and emits a
// stderr warning. Sentinel still writes because the skip is graceful.
func TestPlaybookInit_SkipsPlaybookWithoutExamples(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	noExamplesFS := fstest.MapFS{
		"empty_examples.json": &fstest.MapFile{
			Data: []byte(`{
  "steps": [{"cmd": "prediction-goat-pp-cli --help", "purpose": "no-op"}],
  "entity_slots": [],
  "expected_tool_calls": 1
}`),
		},
	}
	if err := installPlaybooksFromEmbed(context.Background(), s, noExamplesFS); err != nil {
		t.Fatalf("install: %v", err)
	}

	rows, _ := s.ListPlaybooks()
	if len(rows) != 0 {
		t.Errorf("expected 0 user-facing rows (skip unreachable); got %d", len(rows))
	}
	// Sentinel still writes because the skip is graceful — only
	// per-playbook errors block the sentinel, not policy skips.
	if _, ok, _ := s.GetPlaybookByFamily(playbookSeedSentinelFamily); !ok {
		t.Error("expected sentinel row after clean install with policy-skipped playbook")
	}
}

// TestPlaybookInit_FailureLeavesSentinelStale covers the Greptile
// round 4 finding: if any per-playbook upsert fails, the sentinel
// must NOT update so the next install retries. We can't easily make
// store.UpsertPlaybook fail in-process, so we exercise the parse
// failure path: a JSON syntax error counts as a per-file failure and
// flips upsertFailed = true.
func TestPlaybookInit_FailureLeavesSentinelStale(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	brokenFS := fstest.MapFS{
		"broken.json": &fstest.MapFile{
			Data: []byte(`{ this is not valid json `),
		},
		// Include a valid playbook too so we know the loop continues
		// across the broken one but still flags the overall install as
		// failed.
		"valid.json": &fstest.MapFile{
			Data: []byte(`{
  "query_family_examples": ["valid query family example"],
  "steps": [{"cmd": "prediction-goat-pp-cli --help", "purpose": "noop"}],
  "entity_slots": [],
  "expected_tool_calls": 1
}`),
		},
	}
	err = installPlaybooksFromEmbed(context.Background(), s, brokenFS)
	if err == nil {
		t.Fatal("expected install to return error when a per-file parse failed")
	}
	if !strings.Contains(err.Error(), "sentinel not updated") {
		t.Errorf("error should mention sentinel-not-updated; got %v", err)
	}

	// Sentinel must NOT exist — failure path leaves it stale so the
	// next install retries.
	if _, ok, _ := s.GetPlaybookByFamily(playbookSeedSentinelFamily); ok {
		t.Error("sentinel should NOT exist after a per-file failure; got a row")
	}
}

// TestPlaybookInit_HonorsContextCancel covers the Greptile round 4
// hygiene fix: ctx is honored at install entry and between bases.
func TestPlaybookInit_HonorsContextCancel(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before install begins

	err = installPlaybooksFromEmbed(ctx, s, twoPlaybookFS())
	if err == nil {
		t.Fatal("expected install to return error on canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	// Sentinel must NOT exist — we bailed before installing anything.
	if _, ok, _ := s.GetPlaybookByFamily(playbookSeedSentinelFamily); ok {
		t.Error("sentinel should NOT exist after canceled install")
	}
}

// firstNChars trims a string for test error messages; named to avoid
// colliding with topic.go's firstNonEmpty.
func firstNChars(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

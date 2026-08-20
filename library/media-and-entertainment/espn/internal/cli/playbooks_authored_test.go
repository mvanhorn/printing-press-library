// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// playbooks_authored_test.go ensures every JSON playbook + notes file
// shipped in internal/cli/playbooks/ parses cleanly and that the
// example queries collide on the expected query family.

package cli

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/internal/learn"

	_ "modernc.org/sqlite"
)

func TestAuthoredPlaybooks_AllParseCleanly(t *testing.T) {
	t.Parallel()
	dir := "playbooks"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read playbooks dir: %v", err)
	}
	jsonCount := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		jsonCount++
		path := filepath.Join(dir, e.Name())
		pb, err := learn.ParsePlaybookFile(path)
		if err != nil {
			t.Errorf("parse %s: %v", path, err)
			continue
		}
		if len(pb.Steps) == 0 && len(pb.EntitySlots) == 0 {
			t.Errorf("%s: empty playbook (no steps, no slots)", path)
		}
	}
	if jsonCount == 0 {
		t.Fatal("no .json playbooks found")
	}
}

func TestAuthoredPlaybooks_NotesFileExistsForEachJSON(t *testing.T) {
	t.Parallel()
	dir := "playbooks"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read playbooks dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".json")
		notesPath := filepath.Join(dir, base+"_notes.md")
		info, err := os.Stat(notesPath)
		if err != nil {
			t.Errorf("notes file missing for %s: %v", e.Name(), err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("notes file %s is empty", notesPath)
		}
	}
}

// TestAuthoredPlaybooks_SeasonRecapFamilyCollision verifies the
// season recap playbook's example queries collapse to the same
// structural family.
func TestAuthoredPlaybooks_SeasonRecapFamilyCollision(t *testing.T) {
	t.Parallel()
	// Open a tiny in-memory DB to seed entities the canonical resolver
	// expects.
	db := openInMemoryDBWithLookups(t)
	defer db.Close()
	for _, team := range [][]string{
		{"nba_team", "Golden State Warriors", "Warriors", "GSW"},
		{"nba_team", "Detroit Pistons", "Pistons", "DET"},
	} {
		for _, v := range team[1:] {
			if _, err := db.Exec(
				`INSERT INTO entity_lookups (kind, canonical, value, source) VALUES (?, ?, ?, 'seeded')`,
				team[0], team[1], v,
			); err != nil {
				t.Fatalf("seed: %v", err)
			}
		}
	}
	cfg := newLearnConfig()
	resolver := learn.NewCanonicalResolver(context.Background(), db)

	// Agents call recall with normalized-lowercase queries in
	// practice (Claude tokenizes before invoking the tool). Caps-stat
	// abbreviations like PPG/RPG/SPG are auto-promoted to entities by
	// the ALL-CAPS rule and would land in a different family. The
	// playbook's season_recap_notes.md documents the gotcha.
	queries := []string{
		"how did Warriors end the season who led in ppg rpg spg",
		"how did Pistons end the season who led in ppg rpg spg",
		"how did the Pistons end the season, who led in ppg, rpg, spg",
	}
	var families []string
	for _, q := range queries {
		n := learn.Normalize(q, cfg)
		n = learn.PromoteEntities(n, resolver)
		families = append(families, learn.QueryFamily(n))
	}
	for i := 1; i < len(families); i++ {
		if families[i] != families[0] {
			t.Errorf("families diverge: %q != %q (queries: %q vs %q)",
				families[i], families[0], queries[i], queries[0])
		}
	}
}

// openInMemoryDBWithLookups creates a tiny SQLite with the
// entity_lookups table for use in family-collision tests. Avoids the
// full store.Open path so this test doesn't depend on the full
// schema.
func openInMemoryDBWithLookups(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE entity_lookups (
		kind TEXT NOT NULL,
		canonical TEXT NOT NULL,
		value TEXT NOT NULL,
		source TEXT NOT NULL DEFAULT 'seeded',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (kind, canonical, value)
	)`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

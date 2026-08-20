// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildThemeFTSQuery tokenizes, lowercases, drops short tokens, and
// AND-joins. Lock the surface so multi-word themes don't silently widen
// to OR semantics (which would dilute the walk).
func TestBuildThemeFTSQuery(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"impermanence", `"impermanence"`},
		{"the moon", `"the" AND "moon"`},
		{"a stillness in form", `"stillness" AND "in" AND "form"`}, // "a" dropped (len<2)
		{`"quoted phrase"`, `"quoted" AND "phrase"`},
		{"  EXTRA   spaces  ", `"extra" AND "spaces"`},
	}
	for _, tc := range cases {
		got := buildThemeFTSQuery(tc.in)
		assert.Equal(t, tc.want, got, "input=%q", tc.in)
	}
}

// TestBuildThemeFTSQuery_AllShortTokens: when every token drops out as
// noise, we fall through to the raw trimmed theme so SearchWorks treats
// it as recent-works rather than crashing on an empty MATCH.
func TestBuildThemeFTSQuery_AllShortTokens(t *testing.T) {
	got := buildThemeFTSQuery("a i o")
	assert.Equal(t, "a i o", got)
}

// TestDiversityOrderedWalk_PrefersDiversity: with a pool that allows
// diversity, no two consecutive steps share source/region/medium.
func TestDiversityOrderedWalk_PrefersDiversity(t *testing.T) {
	pool := []store.Work{
		{ID: "aic:1", Source: "aic", CultureRegion: "Japan", Medium: "woodblock"},
		{ID: "aic:2", Source: "aic", CultureRegion: "Japan", Medium: "woodblock"},             // poor (same as 1)
		{ID: "met:3", Source: "met", CultureRegion: "France", Medium: "oil"},                  // great (3 axes)
		{ID: "harvard:4", Source: "harvard", CultureRegion: "Netherlands", Medium: "etching"}, // great
		{ID: "cleve:5", Source: "cleveland", CultureRegion: "USA", Medium: "photo"},           // great
	}
	walk := diversityOrderedWalk(pool, 4)
	require.Len(t, walk, 4)
	// First step is FTS-best (pool[0]).
	assert.Equal(t, "aic:1", walk[0].ID)
	// Subsequent steps avoid the same source/region/medium as the prior step.
	for i := 1; i < len(walk); i++ {
		prev := walk[i-1]
		this := walk[i]
		if prev.Source != "" && this.Source != "" {
			assert.NotEqual(t, prev.Source, this.Source, "step %d shares source with prior", i)
		}
		if prev.CultureRegion != "" && this.CultureRegion != "" {
			assert.NotEqual(t, prev.CultureRegion, this.CultureRegion, "step %d shares region with prior", i)
		}
		if prev.Medium != "" && this.Medium != "" {
			assert.NotEqual(t, prev.Medium, this.Medium, "step %d shares medium with prior", i)
		}
	}
}

// TestDiversityOrderedWalk_RespectsRankWhenTied: when multiple
// candidates tie on diversity, the higher-ranked (earlier in pool) one
// wins. The first step is always pool[0].
func TestDiversityOrderedWalk_RespectsRankWhenTied(t *testing.T) {
	pool := []store.Work{
		{ID: "aic:1", Source: "aic", CultureRegion: "Japan", Medium: "woodblock"},
		{ID: "met:2", Source: "met", CultureRegion: "France", Medium: "oil"},
		{ID: "harvard:3", Source: "harvard", CultureRegion: "Netherlands", Medium: "etching"},
	}
	walk := diversityOrderedWalk(pool, 2)
	require.Len(t, walk, 2)
	assert.Equal(t, "aic:1", walk[0].ID)
	// pool[1] and pool[2] both score 3 against the fingerprint. The
	// earlier one wins.
	assert.Equal(t, "met:2", walk[1].ID)
}

// TestDiversityOrderedWalk_TruncatesToPoolSize: requesting more steps
// than the pool holds returns a shorter walk, not duplicates.
func TestDiversityOrderedWalk_TruncatesToPoolSize(t *testing.T) {
	pool := []store.Work{
		{ID: "x:1", Source: "aic"},
		{ID: "x:2", Source: "met"},
	}
	walk := diversityOrderedWalk(pool, 10)
	assert.Len(t, walk, 2)
}

// TestDiversityOrderedWalk_EmptyPool: zero-element pool returns nil
// without panicking.
func TestDiversityOrderedWalk_EmptyPool(t *testing.T) {
	walk := diversityOrderedWalk(nil, 5)
	assert.Empty(t, walk)
}

// TestPathEnvelope: JSON shape (locked-in for agent consumers).
func TestPathEnvelope(t *testing.T) {
	walk := []store.Work{
		{ID: "aic:1", Source: "aic", CultureRegion: "Japan", Medium: "woodblock", Title: "Wave"},
		{ID: "met:2", Source: "met", CultureRegion: "France", Medium: "oil", Title: "Lilies"},
	}
	env := pathEnvelope("water", 5, walk)
	assert.Equal(t, "water", env["theme"])
	assert.Equal(t, 5, env["requested_steps"])
	assert.Equal(t, 2, env["actual_steps"])
	assert.ElementsMatch(t, []string{"aic", "met"}, env["distinct_sources"])
	assert.ElementsMatch(t, []string{"Japan", "France"}, env["distinct_regions"])
}

// TestPath_Integration drives the full command against a temp SQLite
// store with a tiny seeded corpus. Catches wiring bugs (the FTS5 query
// emitter, the SearchWorks → diversityOrderedWalk handoff, the
// rendered output) that unit tests don't cover.
func TestPath_Integration(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	ctx := context.Background()
	db, err := store.OpenWithContext(ctx, dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureArtGoatTables(ctx))

	now := time.Now().UTC()
	seed := []store.Work{
		{ID: "aic:1", Source: "aic", SourceID: "1", Title: "The Great Wave",
			Description:   "study of impermanence and water",
			CultureRegion: "Japan", Medium: "woodblock", SyncedAt: now},
		{ID: "met:2", Source: "met", SourceID: "2", Title: "Water Lilies",
			Description:   "stillness on water; impermanence as practice",
			CultureRegion: "France", Medium: "oil", SyncedAt: now},
		{ID: "harvard:3", Source: "harvard", SourceID: "3", Title: "Dutch Skies",
			Description:   "calm sky",
			CultureRegion: "Netherlands", Medium: "etching", SyncedAt: now},
	}
	for _, w := range seed {
		require.NoError(t, db.UpsertWork(ctx, w))
	}
	db.Close()

	buf := &bytes.Buffer{}
	cmd := newRootCmd(&rootFlags{})
	cmd.SetArgs([]string{"path", "--theme", "impermanence", "--steps", "3", "--db", dbPath, "--json"})
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	var env map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env), "output: %s", buf.String())
	assert.Equal(t, "impermanence", env["theme"])
	steps, ok := env["steps"].([]any)
	require.True(t, ok, "steps should be array")
	require.GreaterOrEqual(t, len(steps), 1)
	// The walk should at least include the FTS-matching works (aic:1 and met:2).
	stepIDs := make([]string, 0, len(steps))
	for _, s := range steps {
		m := s.(map[string]any)
		stepIDs = append(stepIDs, m["id"].(string))
	}
	joined := strings.Join(stepIDs, ",")
	assert.Contains(t, joined, "aic:1")
	assert.Contains(t, joined, "met:2")
}

// TestPath_MissingTheme: --theme is required; verify exits with usage error.
func TestPath_MissingTheme(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	buf := &bytes.Buffer{}
	cmd := newRootCmd(&rootFlags{})
	cmd.SetArgs([]string{"path", "--db", dbPath})
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Equal(t, 2, ExitCode(err))
}

// TestPath_CappedSteps: --steps > 25 is rejected to keep walks readable
// and to bound the over-fetch pool size.
func TestPath_CappedSteps(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	buf := &bytes.Buffer{}
	cmd := newRootCmd(&rootFlags{})
	cmd.SetArgs([]string{"path", "--theme", "x", "--steps", "100", "--db", dbPath})
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Equal(t, 2, ExitCode(err))
}

// TestVerifyEnvelope_Path mirrors the verify_envelope_test.go pattern.
func TestVerifyEnvelope_Path(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	buf := &bytes.Buffer{}
	cmd := newRootCmd(&rootFlags{})
	cmd.SetArgs([]string{"path", "--theme", "anything"})
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	var env map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
	assert.Equal(t, "path", env["command"])
	assert.Equal(t, true, env["verify_noop"])
}

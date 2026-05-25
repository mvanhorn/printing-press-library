// Copyright 2026 justinwfu. Licensed under Apache-2.0.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedWorksOnly populates a temp store with a tiny corpus suitable for
// the today-cache tests. Returns the dbPath the CLI should target via
// --db. No sits — the cache tests should not depend on journal state.
func seedWorksOnly(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	ctx := context.Background()
	db, err := store.OpenWithContext(ctx, dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureArtGoatTables(ctx))
	now := time.Now().UTC()
	seed := []store.Work{
		{ID: "aic:1", Source: "aic", SourceID: "1", Title: "Wave", Creator: "Hokusai", CreatorCanonical: "hokusai", DateStart: 1831, Medium: "woodblock", CultureRegion: "Japan", SyncedAt: now},
		{ID: "met:2", Source: "met", SourceID: "2", Title: "Lilies", Creator: "Monet", CreatorCanonical: "monet", DateStart: 1906, Medium: "oil", CultureRegion: "France", SyncedAt: now},
		{ID: "harvard:3", Source: "harvard", SourceID: "3", Title: "Self-portrait", Creator: "Rembrandt", CreatorCanonical: "rembrandt", DateStart: 1660, Medium: "oil", CultureRegion: "Netherlands", SyncedAt: now},
		{ID: "cleveland:4", Source: "cleveland", SourceID: "4", Title: "Landscape", DateStart: 1800, Medium: "ink", CultureRegion: "China", SyncedAt: now},
		{ID: "smithsonian:5", Source: "smithsonian", SourceID: "5", Title: "Quilt", DateStart: 1900, Medium: "textile", CultureRegion: "USA", SyncedAt: now},
	}
	for _, w := range seed {
		require.NoError(t, db.UpsertWork(ctx, w))
	}
	require.NoError(t, db.Close())
	return dbPath
}

func runTodayJSON(t *testing.T, dbPath string, extraArgs ...string) map[string]any {
	t.Helper()
	args := append([]string{"today", "--db", dbPath, "--json"}, extraArgs...)
	buf := &bytes.Buffer{}
	cmd := newRootCmd(&rootFlags{})
	cmd.SetArgs(args)
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute(), "today: %s", buf.String())
	var env map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env), "non-JSON output: %s", buf.String())
	return env
}

// TestTodayCache_IdempotentSameDay: two consecutive invocations on the
// same day with the same mode return the same work_id, why, and prompt.
// First call has cached=false; second has cached=true.
func TestTodayCache_IdempotentSameDay(t *testing.T) {
	dbPath := seedWorksOnly(t)

	first := runTodayJSON(t, dbPath)
	second := runTodayJSON(t, dbPath)

	assert.Equal(t, first["work_id"], second["work_id"], "same-day re-invocation must return same work_id")
	assert.Equal(t, first["why"], second["why"], "why must be cached verbatim")
	assert.Equal(t, first["prompt"], second["prompt"], "prompt must be cached verbatim")

	assert.Equal(t, false, first["cached"], "first invocation must not be cached")
	assert.Equal(t, true, second["cached"], "second invocation must report cached=true")

	// chosen_at on the second call equals the first call's chosen_at
	// (cache returned, not re-stamped).
	assert.Equal(t, first["chosen_at"], second["chosen_at"], "chosen_at must reflect original pick time")
}

// TestTodayCache_RerollClearsAndRolls: --reroll discards the cached
// pick. The resulting work may or may not differ (the picker's RNG +
// our small corpus can land on the same work), but the post-reroll
// invocation must report cached=false, and the *next* same-mode call
// must hit the cache and serve the rerolled work. chosen_at is not
// asserted directly because the envelope's RFC3339 formatting strips
// sub-second precision; within-second restamps look identical.
func TestTodayCache_RerollClearsAndRolls(t *testing.T) {
	dbPath := seedWorksOnly(t)

	_ = runTodayJSON(t, dbPath)
	rerolled := runTodayJSON(t, dbPath, "--reroll")
	assert.Equal(t, false, rerolled["cached"], "--reroll must mark cached=false")

	// A second call after the reroll should serve the rerolled pick from cache.
	after := runTodayJSON(t, dbPath)
	assert.Equal(t, rerolled["work_id"], after["work_id"], "post-reroll cache must serve the rerolled pick")
	assert.Equal(t, true, after["cached"])
	assert.Equal(t, rerolled["why"], after["why"], "why must be cached verbatim post-reroll")
}

// TestTodayCache_ModeIsolation: switching --mode rolls a fresh pick
// without disturbing the prior-mode cache.
func TestTodayCache_ModeIsolation(t *testing.T) {
	dbPath := seedWorksOnly(t)

	def := runTodayJSON(t, dbPath)
	bridge := runTodayJSON(t, dbPath, "--mode", "bridge-from-last")

	// Bridge mode is a separate cache row → first call for that mode → cached=false.
	assert.Equal(t, false, bridge["cached"])
	assert.Equal(t, "bridge-from-last", bridge["mode"])

	// Re-invoking default mode after bridge must still serve the original default pick.
	def2 := runTodayJSON(t, dbPath)
	assert.Equal(t, def["work_id"], def2["work_id"], "switching modes must not invalidate the default-mode cache")
	assert.Equal(t, true, def2["cached"])

	// Re-invoking bridge must hit its own cache.
	bridge2 := runTodayJSON(t, dbPath, "--mode", "bridge-from-last")
	assert.Equal(t, bridge["work_id"], bridge2["work_id"])
	assert.Equal(t, true, bridge2["cached"])
}

// TestTodayCache_PickDateInEnvelope: the envelope carries pick_date so
// agents reading the JSON know which calendar day the cache is keyed
// against. Defends against a future bug where the cache desyncs from
// the wall clock (e.g. across midnight).
func TestTodayCache_PickDateInEnvelope(t *testing.T) {
	dbPath := seedWorksOnly(t)
	env := runTodayJSON(t, dbPath)
	pickDate, ok := env["pick_date"].(string)
	require.True(t, ok)
	assert.Equal(t, time.Now().Format("2006-01-02"), pickDate)
}

// TestTodayCache_StalePickInvalidates: if the cached work_id no longer
// resolves in the works table, the next invocation must wipe the
// cache and pick fresh rather than return a phantom envelope.
func TestTodayCache_StalePickInvalidates(t *testing.T) {
	dbPath := seedWorksOnly(t)
	ctx := context.Background()

	// Manually write a cache row pointing at a work that doesn't exist.
	db, err := store.OpenWithContext(ctx, dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureArtGoatTables(ctx))
	require.NoError(t, db.SaveTodayPick(ctx, store.TodayPick{
		Date:     time.Now().Format("2006-01-02"),
		Mode:     "",
		WorkID:   "phantom:999",
		Why:      "stale",
		Prompt:   "stale",
		ChosenAt: time.Now().UTC(),
	}))
	require.NoError(t, db.Close())

	env := runTodayJSON(t, dbPath)
	assert.NotEqual(t, "phantom:999", env["work_id"], "stale cache must not be returned")
	assert.Equal(t, false, env["cached"], "stale cache must be treated as a miss")
}

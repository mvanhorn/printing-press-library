// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()
	db, err := OpenWithContext(ctx, filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	require.NoError(t, db.EnsureArtGoatTables(ctx))
	t.Cleanup(func() { db.Close() })
	return db
}

// TestTodayPick_RoundTrip: save, then read, returns the same fields.
// Catches column ordering / column-name drift in the INSERT/SELECT pair.
func TestTodayPick_RoundTrip(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, db.SaveTodayPick(ctx, TodayPick{
		Date:     "2026-05-21",
		Mode:     "",
		WorkID:   "aic:1",
		Why:      "anti-repeat brought this forward",
		Prompt:   "Where is the silence?",
		ChosenAt: now,
	}))
	got, err := db.GetTodayPick(ctx, "2026-05-21", "")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "aic:1", got.WorkID)
	assert.Equal(t, "Where is the silence?", got.Prompt)
	assert.Equal(t, "anti-repeat brought this forward", got.Why)
	assert.WithinDuration(t, now, got.ChosenAt.UTC(), time.Second)
}

// TestTodayPick_Miss: GetTodayPick with no matching row returns (nil, nil).
func TestTodayPick_Miss(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	got, err := db.GetTodayPick(ctx, "2026-05-21", "")
	require.NoError(t, err)
	assert.Nil(t, got)
}

// TestTodayPick_ModeIsolation: same date, different modes, different
// cached picks. A user toggling between modes shouldn't collide.
func TestTodayPick_ModeIsolation(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Now().UTC()
	require.NoError(t, db.SaveTodayPick(ctx, TodayPick{
		Date: "2026-05-21", Mode: "", WorkID: "aic:1", Why: "w", Prompt: "p", ChosenAt: now,
	}))
	require.NoError(t, db.SaveTodayPick(ctx, TodayPick{
		Date: "2026-05-21", Mode: "bridge-from-last", WorkID: "met:2", Why: "w2", Prompt: "p2", ChosenAt: now,
	}))
	a, err := db.GetTodayPick(ctx, "2026-05-21", "")
	require.NoError(t, err)
	require.NotNil(t, a)
	assert.Equal(t, "aic:1", a.WorkID)

	b, err := db.GetTodayPick(ctx, "2026-05-21", "bridge-from-last")
	require.NoError(t, err)
	require.NotNil(t, b)
	assert.Equal(t, "met:2", b.WorkID)
}

// TestTodayPick_Upsert: saving twice for the same (date, mode) replaces
// the prior pick (last write wins). Covers --reroll without --reroll
// (a stale write should not double-store).
func TestTodayPick_Upsert(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	now := time.Now().UTC()
	require.NoError(t, db.SaveTodayPick(ctx, TodayPick{
		Date: "2026-05-21", Mode: "", WorkID: "aic:1", Why: "first", Prompt: "p1", ChosenAt: now,
	}))
	require.NoError(t, db.SaveTodayPick(ctx, TodayPick{
		Date: "2026-05-21", Mode: "", WorkID: "met:2", Why: "second", Prompt: "p2", ChosenAt: now,
	}))
	got, err := db.GetTodayPick(ctx, "2026-05-21", "")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "met:2", got.WorkID)
	assert.Equal(t, "second", got.Why)
}

// TestTodayPick_Clear: clearing removes the row; the next get returns nil.
// Returns rows-affected so the CLI can tell user-requested clear from
// no-op clear.
func TestTodayPick_Clear(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	require.NoError(t, db.SaveTodayPick(ctx, TodayPick{
		Date: "2026-05-21", Mode: "", WorkID: "aic:1", Why: "w", Prompt: "p", ChosenAt: time.Now().UTC(),
	}))
	n, err := db.ClearTodayPick(ctx, "2026-05-21", "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	got, err := db.GetTodayPick(ctx, "2026-05-21", "")
	require.NoError(t, err)
	assert.Nil(t, got)

	// Clearing an absent row returns 0 — no error.
	n2, err := db.ClearTodayPick(ctx, "2026-05-21", "")
	require.NoError(t, err)
	assert.Equal(t, int64(0), n2)
}

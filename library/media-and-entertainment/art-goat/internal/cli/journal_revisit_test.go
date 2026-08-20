// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseHumanAge covers the supported suffixes and the rejection of
// malformed inputs. The "mo before m" precedence matters — without it
// "6mo" would alias to "6m" (which we don't support; time.ParseDuration
// has its own minutes semantics that don't fit a contemplative cadence).
func TestParseHumanAge(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"1d", 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"2w", 14 * 24 * time.Hour},
		{"6mo", 180 * 24 * time.Hour},
		{"1y", 365 * 24 * time.Hour},
		{"3y", 3 * 365 * 24 * time.Hour},
		// case-insensitive
		{"6MO", 180 * 24 * time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := parseHumanAge(tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseHumanAge_Rejects(t *testing.T) {
	bad := []string{"", "1", "abc", "-1y", "0d", "1m", "6 mo", "1.5y", "1yr"}
	for _, in := range bad {
		t.Run(in, func(t *testing.T) {
			_, err := parseHumanAge(in)
			assert.Error(t, err)
		})
	}
}

// TestSitNearestToDate writes three sits (one inside the window, two
// outside) and asserts the closest one comes back. Uses a temp DB so it
// runs hermetically and doesn't depend on a real journal.
func TestSitNearestToDate(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	ctx := context.Background()
	db, err := store.OpenWithContext(ctx, dbPath)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.EnsureArtGoatTables(ctx))

	now := time.Now().UTC()
	mkSit := func(daysAgo int, reflection string) int64 {
		started := now.AddDate(0, 0, -daysAgo)
		id, err := db.InsertSit(ctx, store.Sit{
			StartedAt:  started,
			EndedAt:    sql.NullTime{Time: started.Add(10 * time.Minute), Valid: true},
			Reflection: reflection,
			Mode:       "atomic",
		})
		require.NoError(t, err)
		return id
	}
	_ = mkSit(400, "way back") // outside window
	want := mkSit(365, "anchor sit")
	_ = mkSit(330, "just out of window") // outside ±7d
	_ = mkSit(10, "recent")              // outside

	target := now.AddDate(0, 0, -365)
	got, err := db.SitNearestToDate(ctx, target, 7)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, want, got.ID)
}

// TestSitNearestToDate_OutOfRange returns nil when nothing falls inside
// the window — the CLI converts that to a notFoundErr (exit code 3).
func TestSitNearestToDate_OutOfRange(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	ctx := context.Background()
	db, err := store.OpenWithContext(ctx, dbPath)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.EnsureArtGoatTables(ctx))

	now := time.Now().UTC()
	_, err = db.InsertSit(ctx, store.Sit{
		StartedAt: now,
		Mode:      "atomic",
	})
	require.NoError(t, err)

	target := now.AddDate(-5, 0, 0)
	got, err := db.SitNearestToDate(ctx, target, 7)
	require.NoError(t, err)
	assert.Nil(t, got)
}

// TestGetSit covers fetch-by-id and miss-by-id.
func TestGetSit(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	ctx := context.Background()
	db, err := store.OpenWithContext(ctx, dbPath)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.EnsureArtGoatTables(ctx))

	id, err := db.InsertSit(ctx, store.Sit{
		StartedAt:  time.Now().UTC(),
		Reflection: "hello",
		Mode:       "atomic",
	})
	require.NoError(t, err)
	got, err := db.GetSit(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "hello", got.Reflection)

	miss, err := db.GetSit(ctx, 99999)
	require.NoError(t, err)
	assert.Nil(t, miss)
}

// TestJournalCompareEnvelope: shape of the JSON output for `journal
// compare --json`. Kept tight to catch field-name drift.
func TestJournalCompareEnvelope(t *testing.T) {
	a := &store.Sit{ID: 1, Reflection: "ra", Mode: "atomic", StartedAt: time.Now().UTC()}
	b := &store.Sit{ID: 2, Reflection: "rb", Mode: "atomic", StartedAt: time.Now().UTC()}
	workA := &store.Work{ID: "aic:1", Title: "Wave", Source: "aic"}
	env := journalCompareEnvelope(a, workA, b, nil)
	assert.Contains(t, env, "a")
	assert.Contains(t, env, "b")
	envA, ok := env["a"].(map[string]any)
	require.True(t, ok)
	work, ok := envA["work"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "aic:1", work["id"])
	envB, ok := env["b"].(map[string]any)
	require.True(t, ok)
	_, hasWork := envB["work"]
	assert.False(t, hasWork, "b had no work_id, envelope should omit work")
}

// TestVerifyEnvelope_NewJournalCmds mirrors verify_envelope_test.go's
// existing pattern for the two new revisit/compare subcommands.
func TestVerifyEnvelope_NewJournalCmds(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Run("revisit", func(t *testing.T) {
		buf := &bytes.Buffer{}
		cmd := newRootCmd(&rootFlags{})
		cmd.SetArgs([]string{"journal", "revisit", "--age", "1y"})
		cmd.SetOut(buf)
		cmd.SetErr(&bytes.Buffer{})
		require.NoError(t, cmd.Execute())
		var env map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
		assert.Equal(t, "journal revisit", env["command"])
		assert.Equal(t, true, env["verify_noop"])
	})
	t.Run("compare", func(t *testing.T) {
		buf := &bytes.Buffer{}
		cmd := newRootCmd(&rootFlags{})
		cmd.SetArgs([]string{"journal", "compare", "1", "2"})
		cmd.SetOut(buf)
		cmd.SetErr(&bytes.Buffer{})
		require.NoError(t, cmd.Execute())
		var env map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
		assert.Equal(t, "journal compare", env["command"])
		assert.Equal(t, true, env["verify_noop"])
	})
}

// TestRevisit_MissingAge: --age is required; verify the CLI rejects an
// empty value with a usage error (exit code 2).
func TestRevisit_MissingAge(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	buf := &bytes.Buffer{}
	cmd := newRootCmd(&rootFlags{})
	cmd.SetArgs([]string{"journal", "revisit", "--db", dbPath})
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Equal(t, 2, ExitCode(err))
}

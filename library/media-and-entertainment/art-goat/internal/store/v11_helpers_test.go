// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

// Tests for the v11 art-goat-specific store helpers added alongside the
// `works`/`sits` schema: BrowseWorks, WorksByCreator,
// WorksByStructuredSimilarity, CompactAndReindex (works.go) and AllSits,
// CurrentStreak (sits.go). All cases use an in-memory SQLite store
// provisioned via EnsureArtGoatTables so each subtest is independent.

package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// newArtGoatTestStore opens an in-memory store, provisions the art-goat
// works + sits schema (including FTS5 indexes), and returns the handle.
// Each call yields a fresh independent DB so tests can't leak state.
func newArtGoatTestStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	ctx := context.Background()
	s, err := OpenWithContext(ctx, ":memory:")
	require.NoError(t, err, "open in-memory store")
	t.Cleanup(func() { _ = s.Close() })
	require.NoError(t, s.EnsureArtGoatTables(ctx), "ensure art-goat tables")
	return s, ctx
}

// fixtureWorks returns a small, deterministic mix of works spanning
// multiple sources, mediums, regions, creators, and date_start values.
// Used as the corpus for BrowseWorks / WorksByCreator /
// WorksByStructuredSimilarity / CompactAndReindex tests.
func fixtureWorks() []Work {
	now := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	return []Work{
		{
			ID: "aic:1", Source: "aic", SourceID: "1",
			Title: "Water Lilies", Creator: "Claude Monet", CreatorCanonical: "claude monet",
			DateStart: 1906, DateEnd: 1906, Medium: "Oil on canvas",
			CultureRegion: "European", Period: "Impressionist",
			Description: "Pond reflections", SyncedAt: now,
		},
		{
			ID: "aic:2", Source: "aic", SourceID: "2",
			Title: "Haystacks", Creator: "Claude Monet", CreatorCanonical: "claude monet",
			DateStart: 1890, DateEnd: 1891, Medium: "Oil on canvas",
			CultureRegion: "European", Period: "Impressionist",
			Description: "Series of haystacks at sunset", SyncedAt: now,
		},
		{
			ID: "met:1", Source: "met", SourceID: "1",
			Title: "Wave off Kanagawa", Creator: "Katsushika Hokusai", CreatorCanonical: "katsushika hokusai",
			DateStart: 1831, DateEnd: 1833, Medium: "Woodblock print",
			CultureRegion: "Japanese", Period: "Edo",
			Description: "Iconic wave", SyncedAt: now,
		},
		{
			ID: "met:2", Source: "met", SourceID: "2",
			Title: "Red Fuji", Creator: "Katsushika Hokusai", CreatorCanonical: "katsushika hokusai",
			DateStart: 1830, DateEnd: 1832, Medium: "Woodblock print",
			CultureRegion: "Japanese", Period: "Edo",
			Description: "Mount Fuji in red", SyncedAt: now,
		},
		{
			ID: "harvard:1", Source: "harvard", SourceID: "1",
			Title: "Untitled", Creator: "Mark Rothko", CreatorCanonical: "mark rothko",
			DateStart: 1957, DateEnd: 1957, Medium: "Oil on canvas",
			CultureRegion: "American", Period: "Abstract Expressionism",
			Description: "Color field", SyncedAt: now,
		},
	}
}

// seedWorks upserts the standard fixture batch into the store and asserts
// the count round-trips. Returned for callers that need to reference
// individual rows.
func seedWorks(t *testing.T, s *Store, ctx context.Context) []Work {
	t.Helper()
	works := fixtureWorks()
	n, err := s.UpsertWorksBatch(ctx, works)
	require.NoError(t, err, "seed works")
	require.Equal(t, len(works), n, "seeded row count")
	return works
}

// -----------------------------------------------------------------------
// BrowseWorks
// -----------------------------------------------------------------------

func TestBrowseWorks(t *testing.T) {
	t.Run("empty store returns no rows", func(t *testing.T) {
		s, ctx := newArtGoatTestStore(t)
		got, err := s.BrowseWorks(ctx, BrowseFilter{})
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("populated and filtered", func(t *testing.T) {
		s, ctx := newArtGoatTestStore(t)
		seedWorks(t, s, ctx)

		cases := []struct {
			name    string
			filter  BrowseFilter
			wantIDs []string // expected sorted by id ASC
		}{
			{
				name:    "no filter returns everything",
				filter:  BrowseFilter{},
				wantIDs: []string{"aic:1", "aic:2", "harvard:1", "met:1", "met:2"},
			},
			{
				name:    "source narrows to one collection",
				filter:  BrowseFilter{Source: "aic"},
				wantIDs: []string{"aic:1", "aic:2"},
			},
			{
				name:    "medium substring (case insensitive) matches",
				filter:  BrowseFilter{Medium: "WOODBLOCK"},
				wantIDs: []string{"met:1", "met:2"},
			},
			{
				name:    "region substring matches",
				filter:  BrowseFilter{Region: "japanese"},
				wantIDs: []string{"met:1", "met:2"},
			},
			{
				name:    "combined filters AND together",
				filter:  BrowseFilter{Source: "aic", Medium: "oil"},
				wantIDs: []string{"aic:1", "aic:2"},
			},
			{
				name:    "limit caps result set",
				filter:  BrowseFilter{Limit: 2},
				wantIDs: []string{"aic:1", "aic:2"},
			},
			{
				name:    "offset advances cursor",
				filter:  BrowseFilter{Limit: 2, Offset: 2},
				wantIDs: []string{"harvard:1", "met:1"},
			},
			{
				name:    "non-matching filter yields empty",
				filter:  BrowseFilter{Source: "does-not-exist"},
				wantIDs: nil,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := s.BrowseWorks(ctx, tc.filter)
				require.NoError(t, err)
				var ids []string
				for _, w := range got {
					ids = append(ids, w.ID)
				}
				assert.Equal(t, tc.wantIDs, ids)
			})
		}
	})
}

// -----------------------------------------------------------------------
// WorksByCreator
// -----------------------------------------------------------------------

func TestWorksByCreator(t *testing.T) {
	t.Run("empty store returns no rows", func(t *testing.T) {
		s, ctx := newArtGoatTestStore(t)
		got, err := s.WorksByCreator(ctx, "monet", "", 10)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("populated", func(t *testing.T) {
		s, ctx := newArtGoatTestStore(t)
		seedWorks(t, s, ctx)

		cases := []struct {
			name    string
			substr  string
			source  string
			limit   int
			wantIDs []string // ordered date_start ASC, id ASC
		}{
			{
				name:    "substring hits two works in chronological order",
				substr:  "monet",
				wantIDs: []string{"aic:2", "aic:1"}, // 1890 before 1906
			},
			{
				name:    "case insensitive match",
				substr:  "HOKUSAI",
				wantIDs: []string{"met:2", "met:1"}, // 1830 before 1831
			},
			{
				name:    "source filter narrows results",
				substr:  "monet",
				source:  "met", // no monet in met
				wantIDs: nil,
			},
			{
				name:    "source filter that matches still applies chronological order",
				substr:  "hokusai",
				source:  "met",
				wantIDs: []string{"met:2", "met:1"},
			},
			{
				name:    "limit truncates result set",
				substr:  "monet",
				limit:   1,
				wantIDs: []string{"aic:2"}, // earliest first
			},
			{
				name:    "non-matching substring yields empty",
				substr:  "picasso",
				wantIDs: nil,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := s.WorksByCreator(ctx, tc.substr, tc.source, tc.limit)
				require.NoError(t, err)
				var ids []string
				for _, w := range got {
					ids = append(ids, w.ID)
				}
				assert.Equal(t, tc.wantIDs, ids)
			})
		}
	})
}

// -----------------------------------------------------------------------
// WorksByStructuredSimilarity
// -----------------------------------------------------------------------

func TestWorksByStructuredSimilarity(t *testing.T) {
	t.Run("empty store returns no rows", func(t *testing.T) {
		s, ctx := newArtGoatTestStore(t)
		got, err := s.WorksByStructuredSimilarity(ctx, "Oil on canvas", "", "", "", 10)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("no dimensions provided returns nil", func(t *testing.T) {
		s, ctx := newArtGoatTestStore(t)
		seedWorks(t, s, ctx)
		got, err := s.WorksByStructuredSimilarity(ctx, "", "", "", "", 10)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("populated", func(t *testing.T) {
		s, ctx := newArtGoatTestStore(t)
		seedWorks(t, s, ctx)

		cases := []struct {
			name             string
			medium           string
			region           string
			creatorCanonical string
			excludeID        string
			limit            int
			wantIDs          []string // ordered date_start ASC, id ASC
		}{
			{
				name:    "medium-only OR clause finds all matching mediums",
				medium:  "Oil on canvas",
				wantIDs: []string{"aic:2", "aic:1", "harvard:1"}, // 1890, 1906, 1957
			},
			{
				name:    "region-only narrows to one collection family",
				region:  "Japanese",
				wantIDs: []string{"met:2", "met:1"},
			},
			{
				name:             "creator-only matches canonical exactly",
				creatorCanonical: "claude monet",
				wantIDs:          []string{"aic:2", "aic:1"},
			},
			{
				name:      "excludeID drops the seed work",
				medium:    "Oil on canvas",
				excludeID: "aic:1",
				wantIDs:   []string{"aic:2", "harvard:1"},
			},
			{
				name:             "OR-joins multiple dimensions",
				medium:           "Woodblock print",
				creatorCanonical: "mark rothko",
				wantIDs:          []string{"met:2", "met:1", "harvard:1"}, // 1830, 1831, 1957
			},
			{
				name:    "limit truncates",
				medium:  "Oil on canvas",
				limit:   1,
				wantIDs: []string{"aic:2"},
			},
			{
				name:    "non-matching dimension yields empty",
				medium:  "marble sculpture",
				wantIDs: nil,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := s.WorksByStructuredSimilarity(ctx, tc.medium, tc.region, tc.creatorCanonical, tc.excludeID, tc.limit)
				require.NoError(t, err)
				var ids []string
				for _, w := range got {
					ids = append(ids, w.ID)
				}
				assert.Equal(t, tc.wantIDs, ids)
			})
		}
	})
}

// -----------------------------------------------------------------------
// CompactAndReindex
// -----------------------------------------------------------------------

func TestCompactAndReindex(t *testing.T) {
	t.Run("empty store returns zero rows rebuilt", func(t *testing.T) {
		s, ctx := newArtGoatTestStore(t)
		n, err := s.CompactAndReindex(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, n)
	})

	t.Run("populated store rebuilds works_fts + sits_fts", func(t *testing.T) {
		s, ctx := newArtGoatTestStore(t)
		works := seedWorks(t, s, ctx)

		// Insert a handful of sits so the rebuild covers both tables.
		base := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
		sits := []Sit{
			{StartedAt: base, WorkID: works[0].ID, DurationSeconds: 600, Reflection: "felt the water", Prompt: "what color is the pond", Tags: "monet,water"},
			{StartedAt: base.AddDate(0, 0, 1), WorkID: works[1].ID, DurationSeconds: 600, Reflection: "haystack repetition", Prompt: "where does the light land", Tags: "monet,light"},
			{StartedAt: base.AddDate(0, 0, 2), WorkID: works[2].ID, DurationSeconds: 600, Reflection: "wave force", Prompt: "scale", Tags: "hokusai,wave"},
		}
		for _, sit := range sits {
			_, err := s.InsertSit(ctx, sit)
			require.NoError(t, err)
		}

		// Wipe the FTS tables out-of-band so we can prove CompactAndReindex
		// actually rebuilds them rather than reading a stale healthy index.
		_, err := s.DB().ExecContext(ctx, `DELETE FROM works_fts`)
		require.NoError(t, err)
		_, err = s.DB().ExecContext(ctx, `DELETE FROM sits_fts`)
		require.NoError(t, err)

		n, err := s.CompactAndReindex(ctx)
		require.NoError(t, err)
		assert.Equal(t, len(works)+len(sits), n, "rebuilt rows == works + sits count")

		// FTS still answers queries against the rebuilt index.
		hits, err := s.SearchWorks(ctx, "monet", 10)
		require.NoError(t, err)
		assert.NotEmpty(t, hits, "post-rebuild FTS should return monet hits")
		hits, err = s.SearchWorks(ctx, "hokusai", 10)
		require.NoError(t, err)
		assert.NotEmpty(t, hits, "post-rebuild FTS should return hokusai hits")

		// Sanity check sits_fts row count matches expectations.
		var sitsFTSCount int
		require.NoError(t, s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sits_fts`).Scan(&sitsFTSCount))
		assert.Equal(t, len(sits), sitsFTSCount)
	})
}

// -----------------------------------------------------------------------
// AllSits
// -----------------------------------------------------------------------

func TestAllSits(t *testing.T) {
	t.Run("empty store returns no rows", func(t *testing.T) {
		s, ctx := newArtGoatTestStore(t)
		got, err := s.AllSits(ctx, time.Time{})
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("populated returns oldest-first regardless of insert order", func(t *testing.T) {
		s, ctx := newArtGoatTestStore(t)

		// Build sits with non-chronological insert order so a passing
		// ordering assertion can't be a coincidence of insertion order.
		base := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
		inOrder := []Sit{
			{StartedAt: base.AddDate(0, 0, 2), Reflection: "third", Mode: "atomic"},
			{StartedAt: base, Reflection: "first", Mode: "atomic"},
			{StartedAt: base.AddDate(0, 0, 5), Reflection: "fourth", Mode: "atomic"},
			{StartedAt: base.AddDate(0, 0, 1), Reflection: "second", Mode: "atomic"},
		}
		for _, sit := range inOrder {
			_, err := s.InsertSit(ctx, sit)
			require.NoError(t, err)
		}

		t.Run("zero since returns everything oldest-first", func(t *testing.T) {
			got, err := s.AllSits(ctx, time.Time{})
			require.NoError(t, err)
			require.Len(t, got, 4)
			reflections := []string{got[0].Reflection, got[1].Reflection, got[2].Reflection, got[3].Reflection}
			assert.Equal(t, []string{"first", "second", "third", "fourth"}, reflections)
		})

		t.Run("since filter drops earlier sits", func(t *testing.T) {
			got, err := s.AllSits(ctx, base.AddDate(0, 0, 2))
			require.NoError(t, err)
			require.Len(t, got, 2)
			assert.Equal(t, "third", got[0].Reflection)
			assert.Equal(t, "fourth", got[1].Reflection)
		})

		t.Run("future since yields empty", func(t *testing.T) {
			got, err := s.AllSits(ctx, base.AddDate(0, 0, 30))
			require.NoError(t, err)
			assert.Empty(t, got)
		})
	})
}

// -----------------------------------------------------------------------
// CurrentStreak
// -----------------------------------------------------------------------

func TestCurrentStreak(t *testing.T) {
	t.Run("empty store yields zero", func(t *testing.T) {
		s, ctx := newArtGoatTestStore(t)
		got, err := s.CurrentStreak(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, got)
	})

	// computeStreak compares DATE(started_at) against today UTC. The
	// modernc.org/sqlite driver writes a time.Time passed to a TEXT
	// column as Go's default `2006-01-02 15:04:05 -0700 MST` format,
	// which SQLite's DATE() function refuses to parse (the trailing
	// "+0000 UTC" suffix isn't ISO8601). To exercise computeStreak's
	// own logic we bypass InsertSit and write the started_at value
	// directly in a SQLite-DATE-parseable form ("YYYY-MM-DD HH:MM:SS").
	today := time.Now().UTC().Truncate(24 * time.Hour).Add(12 * time.Hour)

	insertSitOnDay := func(t *testing.T, s *Store, ctx context.Context, day time.Time) {
		t.Helper()
		_, err := s.DB().ExecContext(ctx, `
INSERT INTO sits (started_at, ended_at, work_id, duration_seconds, prompt, reflection, mood, tags, mode)
VALUES (?, ?, '', ?, '', '', NULL, '', 'atomic')`,
			day.Format("2006-01-02 15:04:05"),
			day.Add(10*time.Minute).Format("2006-01-02 15:04:05"),
			600,
		)
		require.NoError(t, err)
	}

	t.Run("three consecutive days ending today yields streak 3", func(t *testing.T) {
		s, ctx := newArtGoatTestStore(t)
		insertSitOnDay(t, s, ctx, today.AddDate(0, 0, -2))
		insertSitOnDay(t, s, ctx, today.AddDate(0, 0, -1))
		insertSitOnDay(t, s, ctx, today)
		got, err := s.CurrentStreak(ctx)
		require.NoError(t, err)
		assert.Equal(t, 3, got)
	})

	t.Run("yesterday only yields one (grace period)", func(t *testing.T) {
		// computeStreak's grace branch fires on the very first iteration
		// when today has no sit but yesterday does: rows stream
		// most-recent-first, expecting=today, first row date=yesterday,
		// matches expecting.AddDate(0, 0, -1) → streak=1.
		s, ctx := newArtGoatTestStore(t)
		insertSitOnDay(t, s, ctx, today.AddDate(0, 0, -1))
		got, err := s.CurrentStreak(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, got)
	})

	t.Run("gap older than yesterday yields zero", func(t *testing.T) {
		s, ctx := newArtGoatTestStore(t)
		insertSitOnDay(t, s, ctx, today.AddDate(0, 0, -3))
		insertSitOnDay(t, s, ctx, today.AddDate(0, 0, -2))
		got, err := s.CurrentStreak(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, got)
	})
}

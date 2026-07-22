// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func newVagaroTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	require.NoError(t, s.EnsureVagaroTables(context.Background()))
	// Second call must be a no-op (idempotent lazy init).
	require.NoError(t, s.EnsureVagaroTables(context.Background()))
	return s
}

func TestUpsertBusinessAndLookup(t *testing.T) {
	ctx := context.Background()
	s := newVagaroTestStore(t)

	// Missing slug -> empty id, nil error.
	id, err := s.GetBusinessIDBySlug(ctx, "nope")
	require.NoError(t, err)
	assert.Equal(t, "", id)

	require.NoError(t, s.UpsertBusiness(ctx, BusinessRecord{
		Slug: "centralbarber", BusinessID: "93458", Name: "Central Barber Shop",
		City: "Seattle", State: "WA", Category: "barber",
	}))
	id, err = s.GetBusinessIDBySlug(ctx, "centralbarber")
	require.NoError(t, err)
	assert.Equal(t, "93458", id)

	// Upsert again updates in place (no duplicate row).
	require.NoError(t, s.UpsertBusiness(ctx, BusinessRecord{Slug: "centralbarber", BusinessID: "93458", Name: "Central Barber Shop v2"}))
	slugs, err := s.ListBusinessSlugs(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"centralbarber"}, slugs)

	// NULL-safe read: rating/review_count left NULL scan without error.
	var name string
	var rating, reviewCount interface{}
	err = s.DB().QueryRowContext(ctx,
		`SELECT name, rating, review_count FROM businesses WHERE slug=?`, "centralbarber").
		Scan(&name, &rating, &reviewCount)
	require.NoError(t, err)
	assert.Equal(t, "Central Barber Shop v2", name)
	assert.Nil(t, rating)
	assert.Nil(t, reviewCount)
}

func TestUpsertServicesProvidersReviews(t *testing.T) {
	ctx := context.Background()
	s := newVagaroTestStore(t)

	require.NoError(t, s.UpsertServices(ctx, "93458", []ServiceRecord{
		{ServiceID: "9433955", Title: "Men's Haircut", PriceText: "$52.00", PriceCents: 5200, Category: "Haircuts"},
		{ServiceID: "34098477", Title: "Skin Fade", PriceText: "$52.00", PriceCents: 5200, Category: "Haircuts"},
	}))
	require.NoError(t, s.UpsertProviders(ctx, "93458", []ProviderRecord{
		{ProviderID: "43931725", Name: "Ronnel Getz"},
		{ProviderID: "232533768", Name: "George Kuhar"},
	}))
	require.NoError(t, s.UpsertReviews(ctx, "93458", []ReviewRecord{
		{ReviewID: "1444419", Rating: 5.0, Text: "Awesome haircut!", Author: "Yan", Date: "2026-06-30"},
	}))

	count := func(table string) int {
		var n int
		require.NoError(t, s.DB().QueryRowContext(ctx, "SELECT count(*) FROM "+table+" WHERE business_id=?", "93458").Scan(&n))
		return n
	}
	assert.Equal(t, 2, count("services"))
	assert.Equal(t, 2, count("providers"))
	assert.Equal(t, 1, count("reviews"))

	// Re-upsert a service updates in place (PK on business_id+service_id).
	require.NoError(t, s.UpsertServices(ctx, "93458", []ServiceRecord{
		{ServiceID: "9433955", Title: "Men's Haircut (updated)", PriceText: "$55.00", PriceCents: 5500},
	}))
	assert.Equal(t, 2, count("services"))
	var title string
	require.NoError(t, s.DB().QueryRowContext(ctx,
		`SELECT title FROM services WHERE business_id=? AND service_id=?`, "93458", "9433955").Scan(&title))
	assert.Equal(t, "Men's Haircut (updated)", title)
}

// TestMigrateWatchBaselinesProvider verifies that a database created before the
// provider column existed is migrated so provider-scoped baselines work after
// an upgrade (regression for the "Baseline Schema Missing Migration" finding).
func TestMigrateWatchBaselinesProvider(t *testing.T) {
	ctx := context.Background()
	s := newVagaroTestStore(t)

	// Simulate a pre-provider database: drop the current table and recreate the
	// old provider-less schema with a row in it.
	_, err := s.db.ExecContext(ctx, `DROP TABLE watch_baselines`)
	require.NoError(t, err)
	_, err = s.db.ExecContext(ctx, `CREATE TABLE watch_baselines (
		slug TEXT NOT NULL, service_id TEXT NOT NULL,
		next_available TEXT, before_target TEXT, recorded_at TEXT NOT NULL,
		PRIMARY KEY (slug, service_id))`)
	require.NoError(t, err)
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO watch_baselines VALUES ('old','1','2026-01-01T00:00:00Z','','2026-01-01T00:00:00Z')`)
	require.NoError(t, err)

	// Re-running EnsureVagaroTables must migrate the stale table.
	require.NoError(t, s.EnsureVagaroTables(ctx))

	// The provider column now exists and provider-scoped writes/reads work.
	require.NoError(t, s.UpsertWatchBaseline(ctx, "centralbarber", "34098477", "43931725", "2026-07-25T11:00:00Z", ""))
	pb, found, err := s.GetWatchBaseline(ctx, "centralbarber", "34098477", "43931725")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "2026-07-25T11:00:00Z", pb.NextAvailable)

	// Idempotent: a second run over the now-current table is a no-op.
	require.NoError(t, s.EnsureVagaroTables(ctx))
	_, found, err = s.GetWatchBaseline(ctx, "centralbarber", "34098477", "43931725")
	require.NoError(t, err)
	assert.True(t, found)
}

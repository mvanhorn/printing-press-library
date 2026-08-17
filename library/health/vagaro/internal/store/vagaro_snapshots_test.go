// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceSnapshots(t *testing.T) {
	ctx := context.Background()
	s := newVagaroTestStore(t)

	require.NoError(t, s.InsertServiceSnapshot(ctx, "93458", "2026-07-01T00:00:00Z", []ServiceRecord{
		{ServiceID: "1", Title: "Cut", PriceCents: 5200},
	}))
	require.NoError(t, s.InsertServiceSnapshot(ctx, "93458", "2026-07-02T00:00:00Z", []ServiceRecord{
		{ServiceID: "1", Title: "Cut", PriceCents: 5500},
		{ServiceID: "2", Title: "Shave", PriceCents: 3000},
	}))

	times, err := s.RecentSnapshotTimes(ctx, "93458", 2)
	require.NoError(t, err)
	require.Len(t, times, 2)
	// Newest first.
	assert.Equal(t, "2026-07-02T00:00:00Z", times[0])
	assert.Equal(t, "2026-07-01T00:00:00Z", times[1])

	newer, err := s.SnapshotServices(ctx, "93458", times[0])
	require.NoError(t, err)
	require.Len(t, newer, 2)
	assert.Equal(t, "1", newer[0].ServiceID)
	assert.Equal(t, 5500, newer[0].PriceCents)

	older, err := s.SnapshotServices(ctx, "93458", times[1])
	require.NoError(t, err)
	require.Len(t, older, 1)
}

func TestWatchBaseline(t *testing.T) {
	ctx := context.Background()
	s := newVagaroTestStore(t)

	_, found, err := s.GetWatchBaseline(ctx, "centralbarber", "34098477", "")
	require.NoError(t, err)
	assert.False(t, found)

	require.NoError(t, s.UpsertWatchBaseline(ctx, "centralbarber", "34098477", "", "2026-07-24T10:00:00Z", "2026-07-30"))
	b, found, err := s.GetWatchBaseline(ctx, "centralbarber", "34098477", "")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "2026-07-24T10:00:00Z", b.NextAvailable)
	assert.Equal(t, "2026-07-30", b.BeforeTarget)

	// Upsert refreshes in place.
	require.NoError(t, s.UpsertWatchBaseline(ctx, "centralbarber", "34098477", "", "2026-07-20T09:00:00Z", ""))
	b, _, err = s.GetWatchBaseline(ctx, "centralbarber", "34098477", "")
	require.NoError(t, err)
	assert.Equal(t, "2026-07-20T09:00:00Z", b.NextAvailable)

	// Provider scopes the key: a provider-specific baseline is distinct from the
	// "any provider" baseline and from other providers on the same service.
	require.NoError(t, s.UpsertWatchBaseline(ctx, "centralbarber", "34098477", "43931725", "2026-07-25T11:00:00Z", ""))
	pb, found, err := s.GetWatchBaseline(ctx, "centralbarber", "34098477", "43931725")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "2026-07-25T11:00:00Z", pb.NextAvailable)

	// The "any provider" baseline is untouched by the provider-scoped upsert.
	b, _, err = s.GetWatchBaseline(ctx, "centralbarber", "34098477", "")
	require.NoError(t, err)
	assert.Equal(t, "2026-07-20T09:00:00Z", b.NextAvailable)

	// A different provider on the same service does not collide.
	_, found, err = s.GetWatchBaseline(ctx, "centralbarber", "34098477", "99999999")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestListBusinessesAndGet(t *testing.T) {
	ctx := context.Background()
	s := newVagaroTestStore(t)

	require.NoError(t, s.UpsertBusiness(ctx, BusinessRecord{
		Slug: "centralbarber", BusinessID: "93458", Name: "Central Barber",
		Rating: 4.9, ReviewCount: 212, PriceRange: "$$", City: "Seattle", State: "WA",
	}))
	b, ok, err := s.GetBusinessBySlug(ctx, "centralbarber")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, 4.9, b.Rating)
	assert.Equal(t, "$$", b.PriceRange)

	_, ok, err = s.GetBusinessBySlug(ctx, "missing")
	require.NoError(t, err)
	assert.False(t, ok)

	all, err := s.ListBusinesses(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, "Central Barber", all[0].Name)
}

// A partial upsert (e.g. child-resource sync carrying only slug + business_id)
// must not clobber previously-populated profile columns with empty values.
func TestUpsertBusinessPartialPreservesProfile(t *testing.T) {
	ctx := context.Background()
	s := newVagaroTestStore(t)

	require.NoError(t, s.UpsertBusiness(ctx, BusinessRecord{
		Slug: "centralbarber", BusinessID: "93458", Name: "Central Barber",
		Rating: 4.9, ReviewCount: 212, PriceRange: "$$", City: "Seattle", State: "WA",
		Address: "1 Main St", Phone: "555-1000", Category: "Barber",
	}))

	// Minimal record with only the key fields, as a `sync --resources services`
	// flow would produce.
	require.NoError(t, s.UpsertBusiness(ctx, BusinessRecord{
		Slug: "centralbarber", BusinessID: "93458",
	}))

	b, ok, err := s.GetBusinessBySlug(ctx, "centralbarber")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "Central Barber", b.Name)
	assert.Equal(t, 4.9, b.Rating)
	assert.Equal(t, 212, b.ReviewCount)
	assert.Equal(t, "$$", b.PriceRange)
	assert.Equal(t, "Seattle", b.City)
	assert.Equal(t, "WA", b.State)
	assert.Equal(t, "1 Main St", b.Address)
	assert.Equal(t, "555-1000", b.Phone)
	assert.Equal(t, "Barber", b.Category)

	// A populated field is still updated when a non-empty value arrives.
	require.NoError(t, s.UpsertBusiness(ctx, BusinessRecord{
		Slug: "centralbarber", BusinessID: "93458", Name: "Central Barber Co",
	}))
	b, _, err = s.GetBusinessBySlug(ctx, "centralbarber")
	require.NoError(t, err)
	assert.Equal(t, "Central Barber Co", b.Name)
	assert.Equal(t, "Seattle", b.City)
}

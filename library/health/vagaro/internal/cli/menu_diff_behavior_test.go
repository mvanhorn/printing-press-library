// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiffSnapshots(t *testing.T) {
	older := []store.SnapshotRow{
		{ServiceID: "1", Title: "Men's Haircut", PriceCents: 5200},
		{ServiceID: "2", Title: "Skin Fade", PriceCents: 5200},
		{ServiceID: "3", Title: "Beard Trim", PriceCents: 2000}, // will be removed
	}
	newer := []store.SnapshotRow{
		{ServiceID: "1", Title: "Men's Haircut", PriceCents: 5500},   // price up
		{ServiceID: "2", Title: "Skin Fade", PriceCents: 5200},       // unchanged
		{ServiceID: "4", Title: "Hot Towel Shave", PriceCents: 3000}, // added
	}
	var out menuDiffResult
	out.PriceChanges = []menuPriceChange{}
	out.Added = []menuServiceRef{}
	out.Removed = []menuServiceRef{}
	diffSnapshots(&out, older, newer)

	require.Len(t, out.PriceChanges, 1)
	assert.Equal(t, "1", out.PriceChanges[0].ServiceID)
	assert.Equal(t, "$52.00", out.PriceChanges[0].OldPrice)
	assert.Equal(t, "$55.00", out.PriceChanges[0].NewPrice)
	assert.Equal(t, "+$3.00", out.PriceChanges[0].DeltaText)

	require.Len(t, out.Added, 1)
	assert.Equal(t, "4", out.Added[0].ServiceID)

	require.Len(t, out.Removed, 1)
	assert.Equal(t, "3", out.Removed[0].ServiceID)
}

func TestDiffSnapshots_noChange(t *testing.T) {
	rows := []store.SnapshotRow{{ServiceID: "1", Title: "Cut", PriceCents: 5200}}
	var out menuDiffResult
	out.PriceChanges = []menuPriceChange{}
	out.Added = []menuServiceRef{}
	out.Removed = []menuServiceRef{}
	diffSnapshots(&out, rows, rows)
	assert.Empty(t, out.PriceChanges)
	assert.Empty(t, out.Added)
	assert.Empty(t, out.Removed)
}

func TestDeltaText(t *testing.T) {
	assert.Equal(t, "+$3.00", deltaText(5200, 5500))
	assert.Equal(t, "-$2.00", deltaText(5200, 5000))
	assert.Equal(t, "+$0.00", deltaText(5200, 5200))
}

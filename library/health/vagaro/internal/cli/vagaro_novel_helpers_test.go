// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/vagaro"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMedianCents(t *testing.T) {
	tests := []struct {
		name string
		in   []int
		want int
	}{
		{"empty", nil, 0},
		{"single", []int{5200}, 5200},
		{"odd", []int{3000, 5200, 4000}, 4000},
		{"even", []int{3000, 4000, 5000, 6000}, 4500},
		{"even-unsorted", []int{6000, 3000, 5000, 4000}, 4500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, medianCents(tt.in))
		})
	}
}

func TestResolveDay(t *testing.T) {
	// Reference "now" is a Wednesday: 2026-07-01.
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	fallback := now.AddDate(0, 0, 3)

	got, err := resolveDay("", now, fallback)
	require.NoError(t, err)
	assert.Equal(t, fallback, got)

	got, err = resolveDay("2026-07-24", now, fallback)
	require.NoError(t, err)
	assert.Equal(t, "2026-07-24", got.Format("2006-01-02"))

	// From Wednesday, next Friday is 2026-07-03.
	got, err = resolveDay("fri", now, fallback)
	require.NoError(t, err)
	assert.Equal(t, "2026-07-03", got.Format("2006-01-02"))

	// Same-day weekday resolves to today (delta 0).
	got, err = resolveDay("wed", now, fallback)
	require.NoError(t, err)
	assert.Equal(t, "2026-07-01", got.Format("2006-01-02"))

	_, err = resolveDay("someday", now, fallback)
	assert.Error(t, err)
}

func TestServiceMatchesQuery(t *testing.T) {
	assert.True(t, serviceMatchesQuery("Men's Haircut", "haircut"))
	assert.True(t, serviceMatchesQuery("Deep Tissue Massage", "massage"))
	assert.True(t, serviceMatchesQuery("60 Minute Swedish Massage", "swedish massage"))
	assert.False(t, serviceMatchesQuery("Beard Trim", "massage"))
	assert.False(t, serviceMatchesQuery("Skin Fade", ""))
}

func TestCheapestService(t *testing.T) {
	services := []vagaro.ServiceRow{
		{ServiceID: 1, ServiceTitle: "A", PriceCents: 6000},
		{ServiceID: 2, ServiceTitle: "B", PriceText: "$40.00"},
		{ServiceID: 3, ServiceTitle: "C", PriceCents: 5000},
	}
	svc, cents, ok := cheapestService(services)
	require.True(t, ok)
	assert.Equal(t, int64(2), svc.ServiceID)
	assert.Equal(t, 4000, cents)

	// No parseable price: returns the first service, ok=false.
	svc, _, ok = cheapestService([]vagaro.ServiceRow{{ServiceID: 9, ServiceTitle: "X", PriceText: "Varies"}})
	assert.False(t, ok)
	assert.Equal(t, int64(9), svc.ServiceID)
}

func TestDollarsFromCents(t *testing.T) {
	assert.Equal(t, "$52.00", dollarsFromCents(5200))
	assert.Equal(t, "$0.05", dollarsFromCents(5))
	assert.Equal(t, "$120.50", dollarsFromCents(12050))
}

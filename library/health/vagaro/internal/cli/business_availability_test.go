// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/vagaro"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAvailabilityPlanDefaultBehavior(t *testing.T) {
	services := []vagaro.ServiceRow{
		{ServiceID: 101, ServiceTitle: "Haircut"},
		{ServiceID: 202, ServiceTitle: "Color"},
	}
	now := time.Date(2026, 7, 8, 13, 30, 0, 0, time.UTC)

	plan, err := buildAvailabilityPlan(services, nil, availabilityOptions{}, now)
	require.NoError(t, err)

	assert.Equal(t, int64(101), plan.Service.ServiceID)
	assert.Equal(t, "", plan.ProviderID)
	assert.Equal(t, "", plan.ProviderName)
	assert.Equal(t, "2026-07-08", plan.From.Format("2006-01-02"))
	assert.Equal(t, "2026-07-14", plan.To.Format("2006-01-02"))
	require.Len(t, plan.WeekDates, 1)
	assert.Equal(t, "Wed Jul-08-2026", formatAvailabilityDate(plan.WeekDates[0]))
	assert.False(t, plan.ExplicitFrom)
	assert.False(t, plan.ExplicitTo)
}

func TestBuildAvailabilityPlanResolvesServiceProviderAndWeeks(t *testing.T) {
	services := []vagaro.ServiceRow{
		{ServiceID: 101, ServiceTitle: "Classic Haircut"},
		{ServiceID: 202, ServiceTitle: "Color"},
	}
	providers := []vagaro.Provider{
		{ServiceProviderID: 301, Name: "Alex Rivera"},
		{ServiceProviderID: 302, Name: "Jordan Lee"},
	}
	now := time.Date(2026, 7, 8, 13, 30, 0, 0, time.UTC)

	plan, err := buildAvailabilityPlan(services, providers, availabilityOptions{
		Service:  "haircut",
		Provider: "alex",
		From:     "2026-07-20",
		Weeks:    3,
	}, now)
	require.NoError(t, err)

	assert.Equal(t, int64(101), plan.Service.ServiceID)
	assert.Equal(t, "301", plan.ProviderID)
	assert.Equal(t, "Alex Rivera", plan.ProviderName)
	assert.Equal(t, "2026-07-20", plan.From.Format("2006-01-02"))
	assert.Equal(t, "2026-08-09", plan.To.Format("2006-01-02"))
	require.Len(t, plan.WeekDates, 3)
	assert.Equal(t, []string{"Mon Jul-20-2026", "Mon Jul-27-2026", "Mon Aug-03-2026"}, []string{
		formatAvailabilityDate(plan.WeekDates[0]),
		formatAvailabilityDate(plan.WeekDates[1]),
		formatAvailabilityDate(plan.WeekDates[2]),
	})
	assert.True(t, plan.ExplicitFrom)
	assert.False(t, plan.ExplicitTo)
}

func TestBuildAvailabilityPlanRejectsUnresolvedOrAmbiguousFilters(t *testing.T) {
	services := []vagaro.ServiceRow{
		{ServiceID: 101, ServiceTitle: "Kids Haircut"},
		{ServiceID: 202, ServiceTitle: "Adult Haircut"},
	}
	providers := []vagaro.Provider{
		{ServiceProviderID: 301, Name: "Alex Rivera"},
		{ServiceProviderID: 302, Name: "Alex Lee"},
	}
	now := time.Date(2026, 7, 8, 13, 30, 0, 0, time.UTC)

	_, err := buildAvailabilityPlan(services, providers, availabilityOptions{Service: "haircut"}, now)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "matched multiple services")

	_, err = buildAvailabilityPlan(services, providers, availabilityOptions{Service: "999"}, now)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service \"999\" was not found")

	_, err = buildAvailabilityPlan(services, providers, availabilityOptions{Provider: "alex"}, now)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "matched multiple providers")

	_, err = buildAvailabilityPlan(services, providers, availabilityOptions{Provider: "999"}, now)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider \"999\" was not found")
}

func TestBuildAvailabilityPlanDateValidation(t *testing.T) {
	services := []vagaro.ServiceRow{{ServiceID: 101, ServiceTitle: "Haircut"}}
	now := time.Date(2026, 7, 8, 13, 30, 0, 0, time.UTC)

	_, err := buildAvailabilityPlan(services, nil, availabilityOptions{From: "2026-07-20", To: "2026-07-19"}, now)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--to must be on or after --from")

	_, err = buildAvailabilityPlan(services, nil, availabilityOptions{To: "2026-07-20", Weeks: 2}, now)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use either --to or --weeks")

	_, err = buildAvailabilityPlan(services, nil, availabilityOptions{Weeks: -1}, now)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--weeks must be greater than 0")
}

func TestFilterAvailabilityGroupsHonorsDateWindow(t *testing.T) {
	groups := []vagaro.SlotGroup{
		{Date: "Mon Jul-20-2026", ProviderID: "301", Times: []string{"9:00 AM", "1:00 PM"}},
		{Date: "Mon Jul-27-2026", ProviderID: "301", Times: []string{"10:00 AM"}},
		{Date: "Mon Aug-03-2026", ProviderID: "301", Times: []string{"11:00 AM"}},
	}
	from := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)

	got := filterAvailabilityGroups(groups, "", from, to)
	require.Len(t, got, 1)
	assert.Equal(t, "Mon Jul-27-2026", got[0].Date)
	assert.Equal(t, []string{"10:00 AM"}, got[0].Times)
}

func TestFilterAvailabilityGroupsHonorsProvider(t *testing.T) {
	groups := []vagaro.SlotGroup{
		{Date: "Mon Jul-20-2026", ProviderID: "301", Times: []string{"9:00 AM"}},
		{Date: "Mon Jul-20-2026", ProviderID: "302", Times: []string{"10:00 AM"}},
		{Date: "Mon Jul-20-2026", Times: []string{"11:00 AM"}},
	}
	from := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)

	got := filterAvailabilityGroups(groups, "301", from, to)
	require.Len(t, got, 1)
	assert.Equal(t, "301", got[0].ProviderID)
	assert.Equal(t, []string{"9:00 AM"}, got[0].Times)

	got = filterAvailabilityGroups(groups, "", from, to)
	require.Len(t, got, 3)
}

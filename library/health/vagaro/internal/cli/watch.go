// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: track a business/service's next-available against a stored
// baseline and report when a slot opens sooner or crosses a --before target.
// generate --force preserves this body.

package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/store"
	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/vagaro"
	"github.com/spf13/cobra"
)

// watchStatus enumerates the outcomes of a watch check.
const (
	watchBaselineEstablished = "baseline_established"
	watchSooner              = "sooner"
	watchLater               = "later"
	watchNoChange            = "no_change"
	watchNoAvailability      = "no_availability"
)

type watchResult struct {
	Slug                  string `json:"slug"`
	BusinessID            string `json:"business_id"`
	ServiceID             string `json:"service_id"`
	Status                string `json:"status"`
	NextAvailable         string `json:"next_available,omitempty"`
	PreviousNextAvailable string `json:"previous_next_available,omitempty"`
	Before                string `json:"before,omitempty"`
	CrossedBefore         bool   `json:"crossed_before,omitempty"`
	Note                  string `json:"note,omitempty"`
}

// pp:data-source live
func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var (
		flagService  string
		flagProvider string
		flagBefore   string
	)

	cmd := &cobra.Command{
		Use:   "watch <slug>",
		Short: "Check one business/provider's next-available against a stored baseline and report if a slot opened up sooner.",
		Long: `Fetch the live next-available slot for a business + service and compare it to
the baseline stored on the previous run. Reports whether a slot opened sooner,
slipped later, or crossed a --before target date. The first run establishes the
baseline.`,
		Example:     "  vagaro-pp-cli watch centralbarber --service 34098477 --before 2026-07-05",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "slug=centralbarber;service=34098477"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			slug := strings.Trim(strings.TrimSpace(args[0]), "/")
			if slug == "" {
				return usageErr(fmt.Errorf("slug is required\nUsage: %s <slug> [--service <id>]", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			var beforeDay time.Time
			haveBefore := false
			if s := strings.TrimSpace(flagBefore); s != "" {
				t, err := time.Parse("2006-01-02", s)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --before %q (want YYYY-MM-DD): %w", s, err))
				}
				beforeDay = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.UTC)
				haveBefore = true
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newVagaroClient(flags)
			businessID, err := resolveBusinessID(ctx, c, flags, slug)
			if err != nil {
				return classifyVagaroError(err, flags)
			}
			serviceID := strings.TrimSpace(flagService)
			if serviceID == "" {
				services, err := c.Services(ctx, businessID)
				if err != nil {
					return classifyVagaroError(err, flags)
				}
				if len(services) == 0 {
					return apiErr(fmt.Errorf("business %s has no bookable services; pass --service <id>", slug))
				}
				serviceID = strconv.FormatInt(services[0].ServiceID, 10)
			}

			appDate, err := vagaro.FormatAppDate(time.Now().Format("2006-01-02"))
			if err != nil {
				return err
			}
			provider := strings.TrimSpace(flagProvider)
			groups, err := c.Availability(ctx, businessID, serviceID, provider, appDate)
			if err != nil {
				return classifyVagaroError(err, flags)
			}
			currentTime, currentLabel, currentValid := earliestSlot(groups)

			out := watchResult{Slug: slug, BusinessID: businessID, ServiceID: serviceID, Before: flagBefore}
			if currentLabel == "" {
				out.Status = watchNoAvailability
				out.Note = "no availability found for the current week"
				// Persist the empty baseline so a later opening registers as "sooner".
				_ = persistWatchBaseline(ctx, slug, serviceID, provider, "", flagBefore)
				return emitVagaro(cmd, flags, out)
			}
			out.NextAvailable = currentLabel
			if haveBefore && currentValid {
				out.CrossedBefore = !currentTime.After(beforeDay)
			}

			nextISO := ""
			if currentValid {
				nextISO = currentTime.UTC().Format(time.RFC3339)
			} else {
				nextISO = currentLabel
			}

			baseline, found, berr := readWatchBaseline(ctx, slug, serviceID, provider)
			if berr != nil || !found {
				out.Status = watchBaselineEstablished
				out.Note = "baseline established; re-run to detect changes"
				_ = persistWatchBaseline(ctx, slug, serviceID, provider, nextISO, flagBefore)
				return emitVagaro(cmd, flags, out)
			}
			out.PreviousNextAvailable = baseline.NextAvailable
			out.Status = classifyWatchChange(baseline.NextAvailable, nextISO, currentTime, currentValid)
			switch out.Status {
			case watchSooner:
				out.Note = "a slot opened up sooner than the previous baseline"
			case watchLater:
				out.Note = "the next-available slot slipped later than the baseline"
			default:
				out.Note = "next-available is unchanged from the baseline"
			}
			_ = persistWatchBaseline(ctx, slug, serviceID, provider, nextISO, flagBefore)
			return emitVagaro(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&flagService, "service", "", "Service ID to watch (default: first service)")
	cmd.Flags().StringVar(&flagProvider, "provider", "", "Provider ID to require (default: any)")
	cmd.Flags().StringVar(&flagBefore, "before", "", "Target date YYYY-MM-DD; report when next-available crosses it")
	return cmd
}

// earliestSlot returns the earliest parseable slot datetime plus a display
// label. valid=false when no slot date could be parsed (HTML-fragment
// fallback); label is still set from the first available time in that case.
func earliestSlot(groups []vagaro.SlotGroup) (time.Time, string, bool) {
	var best time.Time
	bestLabel := ""
	haveBest := false
	fallback := ""
	for _, g := range groups {
		for _, t := range g.Times {
			if fallback == "" {
				fallback = strings.TrimSpace(strings.TrimSpace(g.Date) + " " + t)
			}
			dt, ok := vagaro.ParseSlotDateTime(g.Date, t)
			if !ok {
				continue
			}
			if !haveBest || dt.Before(best) {
				best = dt
				bestLabel = formatSlotLabel(dt)
				haveBest = true
			}
		}
	}
	if haveBest {
		return best, bestLabel, true
	}
	return time.Time{}, fallback, false
}

// classifyWatchChange compares a stored baseline against the current slot.
// Falls back to string comparison when either side is not an RFC3339 time.
func classifyWatchChange(baselineISO, currentISO string, current time.Time, currentValid bool) string {
	baseTime, berr := time.Parse(time.RFC3339, baselineISO)
	if baselineISO == "" {
		// Previously no availability; any current slot is an opening.
		return watchSooner
	}
	if berr == nil && currentValid {
		switch {
		case current.Before(baseTime):
			return watchSooner
		case current.After(baseTime):
			return watchLater
		default:
			return watchNoChange
		}
	}
	if baselineISO == currentISO {
		return watchNoChange
	}
	return watchLater
}

func readWatchBaseline(ctx context.Context, slug, serviceID, provider string) (store.WatchBaseline, bool, error) {
	db, err := openStoreForRead(ctx, "vagaro-pp-cli")
	if err != nil || db == nil {
		return store.WatchBaseline{}, false, err
	}
	defer db.Close()
	return db.GetWatchBaseline(ctx, slug, serviceID, provider)
}

func persistWatchBaseline(ctx context.Context, slug, serviceID, provider, nextAvailable, before string) error {
	db, err := store.OpenWithContext(ctx, defaultDBPath("vagaro-pp-cli"))
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.EnsureVagaroTables(ctx); err != nil {
		return err
	}
	return db.UpsertWatchBaseline(ctx, slug, serviceID, provider, nextAvailable, before)
}

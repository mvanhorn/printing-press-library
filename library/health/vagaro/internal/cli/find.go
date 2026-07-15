// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: marketplace-wide "who can see me soonest" availability search.
// Scans metro listings, deep-fetches each business's matching service + slots,
// and ranks by soonest-available then rating. generate --force preserves this body.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/vagaro"
	"github.com/spf13/cobra"
)

// findMatch is one candidate business with its soonest matching-service slot.
type findMatch struct {
	Slug          string  `json:"slug"`
	Name          string  `json:"name"`
	Rating        float64 `json:"rating,omitempty"`
	ReviewCount   int     `json:"review_count,omitempty"`
	PriceRange    string  `json:"price_range,omitempty"`
	Service       string  `json:"service,omitempty"`
	ServiceID     string  `json:"service_id,omitempty"`
	PriceText     string  `json:"price_text,omitempty"`
	PriceCents    int     `json:"price_cents,omitempty"`
	Provider      string  `json:"provider,omitempty"`
	NextAvailable string  `json:"next_available,omitempty"`
	City          string  `json:"city,omitempty"`
	State         string  `json:"state,omitempty"`

	nextTime  time.Time
	nextValid bool
	matched   bool
	hasSlot   bool
}

type findFilters struct {
	MaxPrice  float64 `json:"max_price,omitempty"`
	MinRating float64 `json:"min_rating,omitempty"`
	From      string  `json:"from,omitempty"`
	To        string  `json:"to,omitempty"`
	Provider  string  `json:"provider,omitempty"`
}

type findResult struct {
	Service           string      `json:"service"`
	City              string      `json:"city"`
	ScannedBusinesses int         `json:"scanned_businesses"`
	MatchedBusinesses int         `json:"matched_businesses"`
	FetchFailures     int         `json:"fetch_failures"`
	MaxScanPages      int         `json:"max_scan_pages"`
	Filters           findFilters `json:"filters"`
	Results           []findMatch `json:"results"`
	Note              string      `json:"note,omitempty"`
	GeoNote           string      `json:"geo_note"`
}

// pp:data-source live
func newNovelFindCmd(flags *rootFlags) *cobra.Command {
	var (
		flagCity      string
		flagMaxPrice  float64
		flagMinRating float64
		flagFrom      string
		flagTo        string
		flagProvider  string
		flagMaxScan   int
		flagLimit     int
	)

	cmd := &cobra.Command{
		Use:   "find <service>",
		Short: "Find nearby businesses with a service open soonest, filtered by price, rating, and a date window.",
		Long: `Scan the metro's businesses for a service, fetch each one's matching service
and open appointment slots, and rank the results by soonest-available then
rating.

Geo note: the --city slug is advisory. Vagaro geo-locates by the caller's IP,
so results reflect YOUR metro regardless of the slug. Pass your own
city--state (e.g. seattle--washington); this environment always resolves to
San Francisco.`,
		Example:     "  vagaro-pp-cli find massage --city san-francisco--ca --max-price 120 --min-rating 4.5 --from thu --to sat",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "service=massage;city=san-francisco--ca", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			service := strings.TrimSpace(args[0])
			if service == "" {
				return usageErr(fmt.Errorf("service is required\nUsage: %s <service> [--city <city--state>]", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			city := strings.TrimSpace(flagCity)
			if city == "" {
				city = defaultMetroLocation
			}
			maxScan := flagMaxScan
			if maxScan <= 0 {
				maxScan = 5
			}
			if cliutil.IsDogfoodEnv() && maxScan > 1 {
				maxScan = 1
			}
			limit := flagLimit
			if limit <= 0 {
				limit = 10
			}
			provider := strings.TrimSpace(flagProvider)
			if strings.EqualFold(provider, "any") {
				provider = ""
			}

			now := time.Now()
			fromDate, err := resolveDay(flagFrom, now, now)
			if err != nil {
				return usageErr(err)
			}
			toDate, err := resolveDay(flagTo, now, fromDate.AddDate(0, 0, 6))
			if err != nil {
				return usageErr(err)
			}
			appDate, err := vagaro.FormatAppDate(fromDate.Format("2006-01-02"))
			if err != nil {
				return err
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newVagaroClient(flags)
			listings, err := c.Listings(ctx, service, city)
			if err != nil {
				return classifyVagaroError(err, flags)
			}
			// Populate the store so compare/market/price-check have this metro.
			upsertListingsToStore(ctx, listings)

			// Cheap pre-filter: drop businesses that already fail --min-rating
			// before spending three HTTP round-trips deep-scanning them.
			candidates := make([]vagaro.ListingBusiness, 0, len(listings))
			for _, b := range listings {
				if flagMinRating > 0 && b.Rating < flagMinRating {
					continue
				}
				candidates = append(candidates, b)
			}
			scanCap := maxScan * businessesPerScanPage
			if len(candidates) > scanCap {
				candidates = candidates[:scanCap]
			}

			results, errs := cliutil.FanoutRun(ctx, candidates,
				func(b vagaro.ListingBusiness) string { return b.Slug },
				func(ctx context.Context, b vagaro.ListingBusiness) (findMatch, error) {
					return scanFindCandidate(ctx, c, b, service, provider, appDate, fromDate, toDate)
				},
				cliutil.WithConcurrency(3),
			)
			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)

			out := findResult{
				Service:           service,
				City:              city,
				ScannedBusinesses: len(candidates),
				FetchFailures:     len(errs),
				MaxScanPages:      maxScan,
				Filters:           findFilters{MaxPrice: flagMaxPrice, MinRating: flagMinRating, From: flagFrom, To: flagTo, Provider: flagProvider},
				Results:           []findMatch{},
				GeoNote:           "results reflect the caller's IP metro; --city is advisory",
			}
			for _, r := range results {
				m := r.Value
				if !m.matched || !m.hasSlot {
					continue
				}
				if flagMaxPrice > 0 && m.PriceCents > 0 && float64(m.PriceCents)/100.0 > flagMaxPrice {
					continue
				}
				out.Results = append(out.Results, m)
			}
			sort.SliceStable(out.Results, func(i, j int) bool {
				a, b := out.Results[i], out.Results[j]
				if a.nextValid != b.nextValid {
					return a.nextValid // valid datetimes sort ahead of unparsed
				}
				if a.nextValid && !a.nextTime.Equal(b.nextTime) {
					return a.nextTime.Before(b.nextTime)
				}
				return a.Rating > b.Rating
			})
			out.MatchedBusinesses = len(out.Results)
			if len(out.Results) > limit {
				out.Results = out.Results[:limit]
			}
			if len(out.Results) == 0 {
				out.Note = fmt.Sprintf("no %s availability matched the filters across %d scanned business(es); "+
					"try widening --from/--to, raising --max-price, or lowering --min-rating", service, out.ScannedBusinesses)
			}
			return emitVagaro(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&flagCity, "city", "", "Metro as a city--state slug (advisory; server geo-locates by IP)")
	cmd.Flags().Float64Var(&flagMaxPrice, "max-price", 0, "Max matching-service price in dollars (0 = no cap)")
	cmd.Flags().Float64Var(&flagMinRating, "min-rating", 0, "Minimum business rating (0 = no floor)")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Window start: weekday (mon..sun) or YYYY-MM-DD (default today)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Window end: weekday (mon..sun) or YYYY-MM-DD (default +6 days)")
	cmd.Flags().StringVar(&flagProvider, "provider", "any", "Provider ID to require, or 'any'")
	cmd.Flags().IntVar(&flagMaxScan, "max-scan-pages", 1, "Scan cap; each page deep-scans ~6 businesses (1 under dogfood)")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Max businesses to return")
	return cmd
}

// scanFindCandidate deep-fetches one business: resolve businessID, load its
// services, match by keyword, and query slots for the cheapest match. Returns a
// findMatch (matched/hasSlot flags set) on success; an error only on a real
// fetch failure so partial failures are accounted, never phantom rows.
func scanFindCandidate(ctx context.Context, c *vagaro.Client, b vagaro.ListingBusiness, service, provider, appDate string, fromDate, toDate time.Time) (findMatch, error) {
	m := findMatch{
		Slug:        b.Slug,
		Name:        b.Name,
		Rating:      b.Rating,
		ReviewCount: b.ReviewCount,
		PriceRange:  b.PriceRange,
		City:        b.City,
		State:       b.State,
	}
	businessID, err := c.ResolveBusinessID(ctx, b.Slug)
	if err != nil {
		return findMatch{}, fmt.Errorf("resolve %s: %w", b.Slug, err)
	}
	services, err := c.Services(ctx, businessID)
	if err != nil {
		return findMatch{}, fmt.Errorf("services %s: %w", b.Slug, err)
	}
	matches := matchingServices(services, service)
	if len(matches) == 0 {
		return m, nil // scanned, no matching service — not a failure
	}
	svc, cents, _ := cheapestService(matches)
	m.matched = true
	m.Service = svc.ServiceTitle
	m.ServiceID = strconv.FormatInt(svc.ServiceID, 10)
	m.PriceText = svc.PriceText
	m.PriceCents = cents

	groups, err := c.Availability(ctx, businessID, m.ServiceID, provider, appDate)
	if err != nil {
		return findMatch{}, fmt.Errorf("slots %s: %w", b.Slug, err)
	}
	applyEarliestSlot(&m, groups, fromDate, toDate)
	return m, nil
}

// applyEarliestSlot fills the match's next-available fields from the earliest
// slot within [fromDate, toDate]. When slot dates are unparseable (HTML
// fallback), the first available time is used as-is and the window is not
// enforced.
func applyEarliestSlot(m *findMatch, groups []vagaro.SlotGroup, fromDate, toDate time.Time) {
	fromDay := time.Date(fromDate.Year(), fromDate.Month(), fromDate.Day(), 0, 0, 0, 0, time.UTC)
	toDay := time.Date(toDate.Year(), toDate.Month(), toDate.Day(), 23, 59, 59, 0, time.UTC)
	for _, g := range groups {
		for _, t := range g.Times {
			dt, ok := vagaro.ParseSlotDateTime(g.Date, t)
			if !ok {
				// No parseable date: accept the first time as next-available.
				if !m.hasSlot {
					m.hasSlot = true
					m.NextAvailable = strings.TrimSpace(strings.TrimSpace(g.Date) + " " + t)
					m.Provider = g.Provider
				}
				continue
			}
			if dt.Before(fromDay) || dt.After(toDay) {
				continue
			}
			if !m.nextValid || dt.Before(m.nextTime) {
				m.hasSlot = true
				m.nextValid = true
				m.nextTime = dt
				m.NextAvailable = formatSlotLabel(dt)
				m.Provider = g.Provider
			}
		}
	}
}

// formatSlotLabel renders a slot datetime like "Fri Jul 24 10:00 AM".
func formatSlotLabel(t time.Time) string {
	return t.Format("Mon Jan 2 3:04 PM")
}

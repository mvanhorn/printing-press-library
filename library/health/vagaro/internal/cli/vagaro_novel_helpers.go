// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Phase 3: shared helpers for the cross-business novel commands
// (find, price-check, market, compare). Keyword service matching, price-string
// math, day-window resolution, and best-effort store population from listings.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/store"
	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/vagaro"
)

// defaultMetroLocation is the advisory listings location used when the caller
// omits --city. The Vagaro server geo-locates by the caller's IP, so the slug
// is only a hint; a real user sees their own metro regardless of this value.
const defaultMetroLocation = "san-francisco--ca"

// businessesPerScanPage bounds how many businesses one unit of --max-scan-pages
// deep-scans (resolve + services + slots). Kept small because each business
// costs three sequential HTTP round-trips against a rate-limited host.
const businessesPerScanPage = 6

// weekdayNames maps lowercase weekday names/abbreviations to time.Weekday.
var weekdayNames = map[string]time.Weekday{
	"sun": time.Sunday, "sunday": time.Sunday,
	"mon": time.Monday, "monday": time.Monday,
	"tue": time.Tuesday, "tues": time.Tuesday, "tuesday": time.Tuesday,
	"wed": time.Wednesday, "weds": time.Wednesday, "wednesday": time.Wednesday,
	"thu": time.Thursday, "thur": time.Thursday, "thurs": time.Thursday, "thursday": time.Thursday,
	"fri": time.Friday, "friday": time.Friday,
	"sat": time.Saturday, "saturday": time.Saturday,
}

// resolveDay turns a --from/--to token into a concrete date. Accepts an empty
// string (returns fallback), a YYYY-MM-DD date, or a weekday name/abbreviation
// (returns the next occurrence on or after now).
func resolveDay(s string, now, fallback time.Time) (time.Time, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return fallback, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	if wd, ok := weekdayNames[s]; ok {
		delta := (int(wd) - int(now.Weekday()) + 7) % 7
		return now.AddDate(0, 0, delta), nil
	}
	return time.Time{}, fmt.Errorf("invalid day %q: use a weekday (mon..sun) or YYYY-MM-DD", s)
}

// serviceMatchesQuery reports whether a service title matches a free-text query
// by whole-word-ish substring: every non-trivial query token must appear in the
// lowercased title. Service IDs differ per business, so cross-business features
// match on title/keyword, not ID.
func serviceMatchesQuery(title, query string) bool {
	t := strings.ToLower(title)
	tokens := strings.Fields(strings.ToLower(query))
	if len(tokens) == 0 {
		return false
	}
	for _, tok := range tokens {
		if len(tok) < 3 {
			continue
		}
		if !strings.Contains(t, tok) {
			return false
		}
	}
	return true
}

// matchingServices returns the services whose title matches the query.
func matchingServices(services []vagaro.ServiceRow, query string) []vagaro.ServiceRow {
	out := make([]vagaro.ServiceRow, 0)
	for _, s := range services {
		if serviceMatchesQuery(s.ServiceTitle, query) {
			out = append(out, s)
		}
	}
	return out
}

// cheapestService returns the service with the lowest parseable price. When no
// service carries a parseable price, the first is returned so callers still get
// a service ID to query slots with.
func cheapestService(services []vagaro.ServiceRow) (vagaro.ServiceRow, int, bool) {
	best := vagaro.ServiceRow{}
	bestCents := -1
	for _, s := range services {
		cents := s.PriceCents
		if cents == 0 {
			if c, ok := vagaro.ParsePriceTextCents(s.PriceText); ok {
				cents = c
			}
		}
		if cents <= 0 {
			continue
		}
		if bestCents == -1 || cents < bestCents {
			bestCents = cents
			best = s
		}
	}
	if bestCents == -1 {
		if len(services) > 0 {
			return services[0], 0, false
		}
		return vagaro.ServiceRow{}, 0, false
	}
	return best, bestCents, true
}

// medianCents returns the median of a set of cent values. Empty input returns 0.
func medianCents(vals []int) int {
	if len(vals) == 0 {
		return 0
	}
	sorted := append([]int(nil), vals...)
	sort.Ints(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// dollarsFromCents renders integer cents as a "$52.00" string.
func dollarsFromCents(cents int) string {
	return fmt.Sprintf("$%d.%02d", cents/100, cents%100)
}

// businessRecordFromListing maps a listings JSON-LD row into a store row.
func businessRecordFromListing(b vagaro.ListingBusiness) store.BusinessRecord {
	return store.BusinessRecord{
		Slug:        b.Slug,
		BusinessID:  "", // listings does not expose businessID; filled on deep scan
		Name:        b.Name,
		Rating:      b.Rating,
		ReviewCount: b.ReviewCount,
		PriceRange:  b.PriceRange,
		City:        b.City,
		State:       b.State,
		Address:     b.Address,
		Phone:       b.Phone,
		Category:    b.Category,
	}
}

// upsertListingsToStore writes discovered businesses into the local store so
// compare/market/price-check have rating/price/address data without a re-scan.
// Best-effort: a store failure never fails the caller's live command. Rows with
// no businessID are skipped when a fuller row already exists, so a scan that
// later resolves the businessID is not clobbered by an id-less listings row.
func upsertListingsToStore(ctx context.Context, businesses []vagaro.ListingBusiness) {
	if len(businesses) == 0 {
		return
	}
	db, err := store.OpenWithContext(ctx, defaultDBPath("vagaro-pp-cli"))
	if err != nil {
		return
	}
	defer db.Close()
	if err := db.EnsureVagaroTables(ctx); err != nil {
		return
	}
	for _, b := range businesses {
		if b.Slug == "" {
			continue
		}
		rec := businessRecordFromListing(b)
		if rec.BusinessID == "" {
			// Preserve an already-resolved businessID from a prior deep scan.
			if existing, ok, _ := db.GetBusinessBySlug(ctx, b.Slug); ok && existing.BusinessID != "" {
				rec.BusinessID = existing.BusinessID
			}
		}
		_ = db.UpsertBusiness(ctx, rec)
	}
}

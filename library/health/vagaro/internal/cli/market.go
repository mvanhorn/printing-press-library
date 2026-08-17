// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: one-shot metro landscape. Aggregates business count, rating
// distribution, price-range histogram, and category breakdown. Prefers the
// synced local store; falls back to a live listings scan. generate --force
// preserves this body.

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/store"
	"github.com/spf13/cobra"
)

type marketResult struct {
	City               string         `json:"city"`
	Category           string         `json:"category,omitempty"`
	Source             string         `json:"source"`
	BusinessCount      int            `json:"business_count"`
	AvgRating          float64        `json:"avg_rating,omitempty"`
	RatingDistribution map[string]int `json:"rating_distribution"`
	PriceHistogram     map[string]int `json:"price_histogram"`
	ByCategory         map[string]int `json:"by_category"`
	Note               string         `json:"note,omitempty"`
	GeoNote            string         `json:"geo_note"`
}

// pp:data-source local
func newNovelMarketCmd(flags *rootFlags) *cobra.Command {
	var flagCategory string

	cmd := &cobra.Command{
		Use:   "market [<city--state>]",
		Short: "One-shot landscape of a metro: business count, rating distribution, and price ranges by category.",
		Long: `Aggregate the metro landscape: how many businesses, how they're rated, their
price-range spread, and a category breakdown.

Data source: --data-source local reads the synced store; auto/live scan the
live listings page (and populate the store). Run 'vagaro-pp-cli sync' or any
find/price-check first to enrich local data.

Geo note: the city--state slug is advisory (server geo-locates by IP).`,
		Example:     "  vagaro-pp-cli market san-francisco--ca --category massage",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "city=san-francisco--ca", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			city := defaultMetroLocation
			if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
				city = strings.TrimSpace(args[0])
			}
			if dryRunOK(flags) {
				return nil
			}
			category := strings.TrimSpace(flagCategory)
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			out := marketResult{
				City:               city,
				Category:           category,
				RatingDistribution: map[string]int{},
				PriceHistogram:     map[string]int{},
				ByCategory:         map[string]int{},
				GeoNote:            "results reflect the caller's IP metro; the slug is advisory",
			}

			// Local-first: read the synced store unless the user forced --data-source live.
			if flags.dataSource != "live" {
				if rows, ok := marketFromStore(ctx); ok && len(rows) > 0 {
					out.Source = "local"
					aggregateMarket(&out, rows)
					return emitVagaro(cmd, flags, out)
				}
				if flags.dataSource == "local" {
					out.Source = "local"
					out.Note = "no local data yet — run 'vagaro-pp-cli sync' or a find/price-check to populate the store"
					return emitVagaro(cmd, flags, out)
				}
			}

			// Live fallback: scan the listings page for the category term.
			svcTerm := category
			if svcTerm == "" {
				svcTerm = "beauty"
			}
			c := newVagaroClient(flags)
			listings, err := c.Listings(ctx, svcTerm, city)
			if err != nil {
				return classifyVagaroError(err, flags)
			}
			upsertListingsToStore(ctx, listings)
			out.Source = "live"
			rows := make([]store.BusinessRecord, 0, len(listings))
			for _, b := range listings {
				rows = append(rows, businessRecordFromListing(b))
			}
			aggregateMarket(&out, rows)
			if out.BusinessCount == 0 {
				out.Note = fmt.Sprintf("no businesses returned for %q in %s", svcTerm, city)
			}
			return emitVagaro(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&flagCategory, "category", "", "Category/service term to scope the landscape (used as the live listings query)")
	return cmd
}

// marketFromStore reads all synced businesses. ok=false when no store exists.
func marketFromStore(ctx context.Context) ([]store.BusinessRecord, bool) {
	db, err := openStoreForRead(ctx, "vagaro-pp-cli")
	if err != nil || db == nil {
		return nil, false
	}
	defer db.Close()
	rows, err := db.ListBusinesses(ctx)
	if err != nil {
		return nil, false
	}
	return rows, true
}

// aggregateMarket computes the distributions over a set of business rows.
//
// by_category is only as rich as the rows' Category field. The live listings
// JSON-LD tags every business as a generic schema.org LocalBusiness with no
// per-business category, so a live scan buckets under the --category query term
// (or "(uncategorized)"). Businesses whose Category was populated from a profile
// scan and persisted to the store surface their real category on the local path.
func aggregateMarket(out *marketResult, rows []store.BusinessRecord) {
	out.BusinessCount = len(rows)
	var ratingSum float64
	var ratingN int
	for _, b := range rows {
		out.RatingDistribution[ratingBucket(b.Rating)]++
		out.PriceHistogram[priceBucket(b.PriceRange)]++
		out.ByCategory[categoryLabel(b.Category, out.Category)]++
		if b.Rating > 0 {
			ratingSum += b.Rating
			ratingN++
		}
	}
	if ratingN > 0 {
		out.AvgRating = round1(ratingSum / float64(ratingN))
	}
}

func ratingBucket(r float64) string {
	switch {
	case r <= 0:
		return "unrated"
	case r >= 4.5:
		return "4.5+"
	case r >= 4.0:
		return "4.0-4.5"
	case r >= 3.0:
		return "3.0-4.0"
	default:
		return "<3.0"
	}
}

func priceBucket(pr string) string {
	pr = strings.TrimSpace(pr)
	switch pr {
	case "$", "$$", "$$$", "$$$$":
		return pr
	case "":
		return "unknown"
	default:
		// Some listings carry a numeric or descriptive price range; bucket by
		// the count of leading $ when present, else "other".
		if n := strings.Count(pr, "$"); n >= 1 && n <= 4 {
			return strings.Repeat("$", n)
		}
		return "other"
	}
}

func categoryLabel(cat, fallback string) string {
	cat = strings.TrimSpace(cat)
	if cat != "" {
		return strings.ToLower(cat)
	}
	if strings.TrimSpace(fallback) != "" {
		return strings.ToLower(strings.TrimSpace(fallback))
	}
	return "(uncategorized)"
}

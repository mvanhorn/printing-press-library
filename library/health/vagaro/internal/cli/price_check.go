// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: metro price spread for a service. Scans businesses, collects
// each one's matching-service price, and reports min/median/max plus which
// businesses sit below the median. generate --force preserves this body.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/vagaro"
	"github.com/spf13/cobra"
)

type priceQuote struct {
	Slug        string  `json:"slug"`
	Name        string  `json:"name,omitempty"`
	Service     string  `json:"service,omitempty"`
	PriceText   string  `json:"price_text,omitempty"`
	PriceCents  int     `json:"price_cents"`
	PriceDollar float64 `json:"price"`
	BelowMedian bool    `json:"below_median"`
	Rating      float64 `json:"rating,omitempty"`
}

type priceCheckResult struct {
	Service           string       `json:"service"`
	City              string       `json:"city"`
	ScannedBusinesses int          `json:"scanned_businesses"`
	QuotedBusinesses  int          `json:"quoted_businesses"`
	FetchFailures     int          `json:"fetch_failures"`
	MinPrice          float64      `json:"min_price,omitempty"`
	MedianPrice       float64      `json:"median_price,omitempty"`
	MaxPrice          float64      `json:"max_price,omitempty"`
	Quotes            []priceQuote `json:"quotes"`
	Note              string       `json:"note,omitempty"`
	GeoNote           string       `json:"geo_note"`
}

// pp:data-source live
func newNovelPriceCheckCmd(flags *rootFlags) *cobra.Command {
	var (
		flagCity    string
		flagMaxScan int
	)

	cmd := &cobra.Command{
		Use:   "price-check <service>",
		Short: "Show the price spread (min/median/max) for a service across a metro and flag who's below median.",
		Long: `Scan the metro's businesses for a service, collect each one's matching-service
price, and report the min / median / max spread. Businesses priced below the
median are flagged.

Geo note: the --city slug is advisory (server geo-locates by IP).`,
		Example:     "  vagaro-pp-cli price-check haircut --city san-francisco--ca",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "service=haircut;city=san-francisco--ca", "pp:no-error-path-probe": "true"},
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

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newVagaroClient(flags)
			listings, err := c.Listings(ctx, service, city)
			if err != nil {
				return classifyVagaroError(err, flags)
			}
			upsertListingsToStore(ctx, listings)

			candidates := listings
			scanCap := maxScan * businessesPerScanPage
			if len(candidates) > scanCap {
				candidates = candidates[:scanCap]
			}

			results, errs := cliutil.FanoutRun(ctx, candidates,
				func(b vagaro.ListingBusiness) string { return b.Slug },
				func(ctx context.Context, b vagaro.ListingBusiness) (priceQuote, error) {
					return quoteBusinessPrice(ctx, c, b, service)
				},
				cliutil.WithConcurrency(3),
			)
			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)

			out := priceCheckResult{
				Service:           service,
				City:              city,
				ScannedBusinesses: len(candidates),
				FetchFailures:     len(errs),
				Quotes:            []priceQuote{},
				GeoNote:           "results reflect the caller's IP metro; --city is advisory",
			}
			for _, r := range results {
				if r.Value.PriceCents > 0 {
					out.Quotes = append(out.Quotes, r.Value)
				}
			}
			out.QuotedBusinesses = len(out.Quotes)
			if len(out.Quotes) == 0 {
				out.Note = fmt.Sprintf("no priced %q service found across %d scanned business(es)", service, out.ScannedBusinesses)
				return emitVagaro(cmd, flags, out)
			}

			cents := make([]int, 0, len(out.Quotes))
			for _, q := range out.Quotes {
				cents = append(cents, q.PriceCents)
			}
			sort.Ints(cents)
			median := medianCents(cents)
			out.MinPrice = float64(cents[0]) / 100.0
			out.MaxPrice = float64(cents[len(cents)-1]) / 100.0
			out.MedianPrice = float64(median) / 100.0
			for i := range out.Quotes {
				out.Quotes[i].BelowMedian = out.Quotes[i].PriceCents < median
			}
			sort.SliceStable(out.Quotes, func(i, j int) bool {
				return out.Quotes[i].PriceCents < out.Quotes[j].PriceCents
			})
			return emitVagaro(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&flagCity, "city", "", "Metro as a city--state slug (advisory; server geo-locates by IP)")
	cmd.Flags().IntVar(&flagMaxScan, "max-scan-pages", 1, "Scan cap; each page deep-scans ~6 businesses (1 under dogfood)")
	return cmd
}

func quoteBusinessPrice(ctx context.Context, c *vagaro.Client, b vagaro.ListingBusiness, service string) (priceQuote, error) {
	q := priceQuote{Slug: b.Slug, Name: b.Name, Rating: b.Rating}
	businessID, err := c.ResolveBusinessID(ctx, b.Slug)
	if err != nil {
		return priceQuote{}, fmt.Errorf("resolve %s: %w", b.Slug, err)
	}
	services, err := c.Services(ctx, businessID)
	if err != nil {
		return priceQuote{}, fmt.Errorf("services %s: %w", b.Slug, err)
	}
	matches := matchingServices(services, service)
	if len(matches) == 0 {
		return q, nil // scanned, no matching service
	}
	svc, cents, ok := cheapestService(matches)
	if !ok {
		return q, nil // matched but no parseable price
	}
	q.Service = svc.ServiceTitle
	q.PriceText = svc.PriceText
	q.PriceCents = cents
	q.PriceDollar = float64(cents) / 100.0
	return q, nil
}

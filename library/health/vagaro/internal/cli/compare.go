// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: side-by-side comparison of named businesses. Reuses the
// foundation client for profile/services/reviews/next-available per slug.
// generate --force preserves this body.

package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/vagaro"
	"github.com/spf13/cobra"
)

// compareRow is one business column in the comparison.
type compareRow struct {
	Slug          string  `json:"slug"`
	Name          string  `json:"name,omitempty"`
	BusinessID    string  `json:"business_id,omitempty"`
	Rating        float64 `json:"rating,omitempty"`
	ReviewCount   int     `json:"review_count,omitempty"`
	PriceRange    string  `json:"price_range,omitempty"`
	Services      int     `json:"services"`
	MatchService  string  `json:"match_service,omitempty"`
	MatchPrice    string  `json:"match_price,omitempty"`
	NextAvailable string  `json:"next_available,omitempty"`
	City          string  `json:"city,omitempty"`
	State         string  `json:"state,omitempty"`
	Error         string  `json:"error,omitempty"`
}

type compareResult struct {
	Service       string       `json:"service,omitempty"`
	Businesses    []compareRow `json:"businesses"`
	FetchFailures int          `json:"fetch_failures"`
	Note          string       `json:"note,omitempty"`
}

// pp:data-source live
// pp:client-call -- buildCompareRow calls the real Vagaro client
// (FetchProfile/Services/Reviews/Availability); the client is passed in
// rather than constructed in the helper, so the one-hop detector can't see it.
func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var flagService string

	cmd := &cobra.Command{
		Use:   "compare <slugA> <slugB> [<slugC>...]",
		Short: "Compare named businesses side by side: rating, review count, price range, matching-service price, and next-available.",
		Long: `Fetch each business's profile, service menu, review rating, and next-available
slot, then render them side by side. Pass --service to pick which service's
price and availability to compare (defaults to the first service).`,
		Example:     "  vagaro-pp-cli compare centralbarber anotherbarber --service haircut",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "slugA=centralbarber;slugB=centralbarber"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			slugs := make([]string, 0, len(args))
			for _, a := range args {
				if s := strings.Trim(strings.TrimSpace(a), "/"); s != "" {
					slugs = append(slugs, s)
				}
			}
			if len(slugs) < 2 {
				return usageErr(fmt.Errorf("compare needs at least two business slugs\nUsage: %s <slugA> <slugB> [...]", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newVagaroClient(flags)
			service := strings.TrimSpace(flagService)

			results, errs := cliutil.FanoutRun(ctx, slugs,
				func(s string) string { return s },
				func(ctx context.Context, slug string) (compareRow, error) {
					return buildCompareRow(ctx, c, slug, service)
				},
				cliutil.WithConcurrency(3),
			)
			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)

			// Reassemble in the user's argument order regardless of completion.
			bySlug := map[string]compareRow{}
			for _, r := range results {
				bySlug[r.Value.Slug] = r.Value
			}
			for _, e := range errs {
				bySlug[e.Source] = compareRow{Slug: e.Source, Error: shortErr(e.Err)}
			}
			out := compareResult{Service: service, Businesses: make([]compareRow, 0, len(slugs)), FetchFailures: len(errs)}
			for _, s := range slugs {
				if row, ok := bySlug[s]; ok {
					out.Businesses = append(out.Businesses, row)
				}
			}
			if len(errs) > 0 {
				out.Note = fmt.Sprintf("%d of %d business(es) failed to fetch; see error fields", len(errs), len(slugs))
			}
			return emitVagaro(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&flagService, "service", "", "Service keyword to compare price/availability for (default: first service)")
	return cmd
}

func buildCompareRow(ctx context.Context, c *vagaro.Client, slug, service string) (compareRow, error) {
	row := compareRow{Slug: slug}
	prof, err := c.FetchProfile(ctx, slug)
	if err != nil {
		return compareRow{}, err
	}
	row.Name = prof.Name
	row.BusinessID = prof.BusinessID
	row.City = prof.City
	row.State = prof.State
	row.PriceRange = prof.PriceRange

	services, err := c.Services(ctx, prof.BusinessID)
	if err != nil {
		return compareRow{}, err
	}
	row.Services = len(services)

	var chosen vagaro.ServiceRow
	haveChosen := false
	if service != "" {
		if matches := matchingServices(services, service); len(matches) > 0 {
			chosen, _, _ = cheapestService(matches)
			haveChosen = true
		}
	} else if len(services) > 0 {
		chosen = services[0]
		haveChosen = true
	}
	if haveChosen {
		row.MatchService = chosen.ServiceTitle
		row.MatchPrice = chosen.PriceText
	}

	// Review rating: prefer the first page's average when the profile page did
	// not expose an aggregate rating.
	if reviews, err := c.Reviews(ctx, prof.BusinessID, "", 20); err == nil && len(reviews) > 0 {
		row.ReviewCount = len(reviews)
		var sum float64
		for _, rv := range reviews {
			sum += rv.Rating
		}
		row.Rating = round1(sum / float64(len(reviews)))
	}

	if haveChosen {
		appDate, derr := vagaro.FormatAppDate(time.Now().Format("2006-01-02"))
		if derr == nil {
			if groups, gerr := c.Availability(ctx, prof.BusinessID, strconv.FormatInt(chosen.ServiceID, 10), "", appDate); gerr == nil {
				row.NextAvailable = firstSlotLabel(groups)
			}
		}
	}
	return row, nil
}

// firstSlotLabel returns a "date time" label for the first available slot, or
// "none this week" when there is no availability.
func firstSlotLabel(groups []vagaro.SlotGroup) string {
	for _, g := range groups {
		if len(g.Times) > 0 {
			return strings.TrimSpace(strings.TrimSpace(g.Date) + " " + g.Times[0])
		}
	}
	return "none this week"
}

func round1(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}

func shortErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

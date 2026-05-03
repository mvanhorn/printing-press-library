package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/source/airbnb"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/source/vrbo"
	"github.com/spf13/cobra"
)

// pp:novel-static-reference
func newPlanCmd(flags *rootFlags) *cobra.Command {
	var checkin, checkout, backend string
	var guests, topN int
	var budget float64
	cmd := &cobra.Command{
		Use:         "plan <city>",
		Short:       "Search both platforms and rank options by direct-booking savings",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"city": args[0], "results": []any{}, "method": "dry_run"}, flags)
			}
			if checkin == "" || checkout == "" {
				return usageErr(fmt.Errorf("--checkin and --checkout are required"))
			}
			if err := validateDates(checkin, checkout); err != nil {
				return usageErr(err)
			}
			city := args[0]
			type source struct{ name string }
			// Per-listing timeouts are isolated: a slow vrbo or airbnb leg
			// no longer fails the whole call. Pre-fix, "context deadline
			// exceeded" returned results: [] for the entire response.
			results, errs := cliutil.FanoutRun(cmd.Context(), []source{{"airbnb"}, {"vrbo"}}, func(s source) string { return s.name }, func(ctx context.Context, s source) ([]string, error) {
				legCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
				defer cancel()
				switch s.name {
				case "airbnb":
					listings, _, err := airbnb.Search(legCtx, airbnb.SearchParams{Location: city, Checkin: checkin, Checkout: checkout, Adults: guests})
					var urls []string
					for _, l := range listings {
						if budget == 0 || (l.PriceBreakdown != nil && l.PriceBreakdown.Total <= budget) {
							urls = append(urls, l.URL)
						}
						if len(urls) >= top(topN) {
							break
						}
					}
					return urls, err
				default:
					return nil, vrbo.ErrDisabled
				}
			}, cliutil.WithConcurrency(2))
			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)
			failures := []map[string]any{}
			for _, e := range errs {
				reason := e.Err.Error()
				if vrbo.IsDisabled(e.Err) {
					reason = "vrbo_disabled"
				}
				failures = append(failures, map[string]any{"source": e.Source, "reason": reason})
			}
			var urls []string
			for _, r := range results {
				urls = append(urls, r.Value...)
			}
			cheapestLimit := top(topN)
			if cheapestLimit > 3 {
				cheapestLimit = 3
			}
			if len(urls) > cheapestLimit {
				urls = urls[:cheapestLimit]
			}
			cheapest, cerrs := cliutil.FanoutRun(cmd.Context(), urls, func(s string) string { return s }, func(ctx context.Context, u string) (map[string]any, error) {
				legCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
				ch, err := computeCheapest(legCtx, u, cheapestParams{Checkin: checkin, Checkout: checkout, Guests: guests, SearchBackend: backend, MaxDirectResults: 1})
				if err != nil {
					return nil, err
				}
				pt, _ := firstPlatformTotals(ch)
				dt := cheapestDirectTotal(ch)
				return map[string]any{"platform_url": u, "direct_url": directURL(ch), "savings": pt - dt, "cheapest": ch.Cheapest, "listing": ch.Listing}, nil
			}, cliutil.WithConcurrency(3))
			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), cerrs)
			for _, e := range cerrs {
				failures = append(failures, map[string]any{"source": e.Source, "reason": e.Err.Error()})
			}
			out := make([]map[string]any, 0, len(cheapest))
			for _, r := range cheapest {
				out = append(out, r.Value)
			}
			sortBySavings(out)
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"city": city, "results": out, "failures": failures}, flags)
		},
	}
	cmd.Flags().StringVar(&checkin, "checkin", "", "Arrival date YYYY-MM-DD")
	cmd.Flags().StringVar(&checkout, "checkout", "", "Departure date YYYY-MM-DD")
	cmd.Flags().IntVar(&guests, "guests", 1, "Guest count")
	cmd.Flags().Float64Var(&budget, "budget", 0, "Maximum platform total")
	cmd.Flags().IntVar(&topN, "top-n", 5, "Top listings per platform")
	cmd.Flags().StringVar(&backend, "search-backend", "", "Search backend")
	return cmd
}

func top(n int) int {
	if n <= 0 {
		return 5
	}
	return n
}

func directURL(ch *cheapestOutput) string {
	if m, ok := ch.Cheapest.(map[string]any); ok {
		if s, ok := m["url"].(string); ok {
			return s
		}
	}
	return ""
}

// Hand-written novel command. The Pointhound airport autocomplete lives at a
// separate base URL (scout.pointhound.com), so it can't be expressed cleanly
// in the primary spec.
//
//	// pp:client-call
//
// internal/scout calls the real scout.pointhound.com endpoint over HTTPS.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/pointhound/internal/scout"
)

func newAirportsCmd(flags *rootFlags) *cobra.Command {
	var minRating string
	var limit int
	var metroOnly bool
	var trackedOnly bool

	cmd := &cobra.Command{
		Use:   "airports <query>",
		Short: "Search airports and cities with Pointhound's deal-aware autocomplete",
		Long: strings.TrimSpace(`
Search airports and cities via Pointhound's Scout service. Each result carries
a dealRating ("high" or "low") and isTracked flag — the same signal Pointhound
uses to surface "high-frequency deals" hints inline on the website.

This is anonymous (no auth required) and is the recommended way to resolve
"SFO" → San Francisco Intl Airport before kicking off a search, or to discover
nearby airports for an exploration command like top-deals-matrix.
`),
		Example: strings.Trim(`
  pointhound-pp-cli airports SFO --json
  pointhound-pp-cli airports "san francisco" --limit 5 --metro
  pointhound-pp-cli airports lisbon --min-rating high --tracked-only
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			client := scout.New("")
			resp, err := client.Search(cmd.Context(), scout.SearchOptions{
				Query: query,
				Limit: limit,
				Metro: metroOnly,
				Bound: false,
				Live:  true,
			})
			if err != nil {
				return err
			}

			results := resp.Results
			if trackedOnly {
				filtered := results[:0]
				for _, r := range results {
					if r.IsTracked {
						filtered = append(filtered, r)
					}
				}
				results = filtered
			}
			if minRating != "" {
				filtered := results[:0]
				wanted := strings.ToLower(minRating)
				for _, r := range results {
					if strings.EqualFold(r.DealRating, wanted) {
						filtered = append(filtered, r)
					}
				}
				results = filtered
			}
			// Sort: tracked first, then dealRating high first, then rank
			sort.SliceStable(results, func(i, j int) bool {
				if results[i].IsTracked != results[j].IsTracked {
					return results[i].IsTracked
				}
				ri := dealWeight(results[i].DealRating)
				rj := dealWeight(results[j].DealRating)
				if ri != rj {
					return ri > rj
				}
				return results[i].Rank < results[j].Rank
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) && !humanFriendly {
				view := map[string]any{
					"results":      results,
					"searchStatus": resp.SearchStatus,
				}
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching airports.")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "CODE\tNAME\tCITY\tCOUNTRY\tDEAL\tTRACKED")
			for _, r := range results {
				tracked := "no"
				if r.IsTracked {
					tracked = "yes"
				}
				deal := r.DealRating
				if deal == "" {
					deal = "-"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Code, r.Name, r.City, r.CountryCode, deal, tracked)
			}
			_ = tw.Flush()
			return nil
		},
	}

	cmd.Flags().StringVar(&minRating, "min-rating", "", "Only show airports with this exact deal rating (e.g. high).")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum results to fetch from Scout (1-50).")
	cmd.Flags().BoolVar(&metroOnly, "metro", false, "Treat the query as a metro lookup (groups multi-airport cities).")
	cmd.Flags().BoolVar(&trackedOnly, "tracked-only", false, "Only show airports Pointhound actively tracks.")
	return cmd
}

func dealWeight(rating string) int {
	switch strings.ToLower(rating) {
	case "high":
		return 2
	case "low":
		return 1
	}
	return 0
}

// Encode the raw response unchanged for diagnostics if needed.
var _ = json.RawMessage(nil)

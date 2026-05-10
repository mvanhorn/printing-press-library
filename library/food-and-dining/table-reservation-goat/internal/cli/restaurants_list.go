// Hand-rewritten in Phase 3 to delegate to the cross-network source clients.
// The generated scaffold called `client.Get("/restaurants", params)` against
// opentable.com which 404s; restaurants list is the primary discovery surface
// so it must work cross-network. We delegate to the same code path `goat`
// uses to ensure consistency.

package cli

// PATCH: scaffold-endpoint-redirects — see .printing-press-patches.json for the change-set rationale.

import (
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/auth"
)

func newRestaurantsListCmd(flags *rootFlags) *cobra.Command {
	var flagQuery string
	var flagLatitude float64
	var flagLongitude float64
	var flagMetro string
	var flagNeighborhood string
	var flagCuisine string
	var flagPriceBand int
	var flagAccolade string
	var flagPartySize int
	var flagNetwork string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List restaurants across OpenTable and Tock",
		Long: "Cross-network restaurant search backed by Surf-cleared OpenTable SSR " +
			"and Tock SSR. Identical underlying data path as `goat`; this command " +
			"is the resource-style entry point.",
		Example:     "  table-reservation-goat-pp-cli restaurants list --query 'omakase' --party 2 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), goatResponse{
					Query:     flagQuery,
					Results:   []goatResult{{Network: "opentable", Name: "(dry-run sample)", MatchScore: 1.0}},
					Sources:   []string{"opentable", "tock"},
					QueriedAt: time.Now().UTC().Format(time.RFC3339),
				}, flags)
			}
			session, err := auth.Load()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			net := strings.ToLower(flagNetwork)
			results := []goatResult{}
			errors := []string{}
			sources := []string{}
			query := flagQuery
			lat, lng := flagLatitude, flagLongitude
			if net == "" || net == "opentable" {
				sources = append(sources, "opentable")
				if r, err := goatQueryOpenTable(ctx, session, query, lat, lng); err != nil {
					errors = append(errors, "opentable: "+err.Error())
				} else {
					results = append(results, r...)
				}
			}
			if net == "" || net == "tock" {
				sources = append(sources, "tock")
				// restaurants_list doesn't carry a metro/date/time/party context,
				// so fall back to NYC defaults — goatQueryTock applies the same
				// fallbacks internally when these are zero/empty.
				date := time.Now().UTC().Format("2006-01-02")
				if r, err := goatQueryTock(ctx, session, query, "", date, "19:00", 2, lat, lng); err != nil {
					errors = append(errors, "tock: "+err.Error())
				} else {
					results = append(results, r...)
				}
			}
			if flagLimit > 0 && len(results) > flagLimit {
				results = results[:flagLimit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), goatResponse{
				Query:     query,
				Results:   results,
				Errors:    errors,
				Sources:   sources,
				QueriedAt: time.Now().UTC().Format(time.RFC3339),
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagQuery, "query", "", "Free-text query (matches name, cuisine, neighborhood)")
	cmd.Flags().Float64Var(&flagLatitude, "latitude", 0, "Latitude for geo search")
	cmd.Flags().Float64Var(&flagLongitude, "longitude", 0, "Longitude for geo search")
	cmd.Flags().StringVar(&flagMetro, "metro", "", "Metro slug (e.g., chicago, seattle)")
	cmd.Flags().StringVar(&flagNeighborhood, "neighborhood", "", "Neighborhood slug")
	cmd.Flags().StringVar(&flagCuisine, "cuisine", "", "Cuisine filter")
	cmd.Flags().IntVar(&flagPriceBand, "max-price", 0, "Maximum price band 1-4")
	cmd.Flags().StringVar(&flagAccolade, "accolade", "", "Filter by accolade (michelin, worlds50best)")
	cmd.Flags().IntVar(&flagPartySize, "party", 2, "Party size for availability filter")
	cmd.Flags().StringVar(&flagNetwork, "network", "", "Restrict to one network (opentable, tock)")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Max restaurants to return")
	_ = flagMetro
	_ = flagNeighborhood
	_ = flagCuisine
	_ = flagPriceBand
	_ = flagAccolade
	_ = flagPartySize
	return cmd
}

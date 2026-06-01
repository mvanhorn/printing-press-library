// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/offerup"
)

type priceDropsResult struct {
	Query    string         `json:"query"`
	Location string         `json:"location,omitempty"`
	Since    string         `json:"since"`
	Count    int            `json:"count"`
	Drops    []offerup.Drop `json:"drops"`
}

// pp:data-source auto
func newNovelPriceDropsCmd(flags *rootFlags) *cobra.Command {
	lf := &locFlags{}
	var since string
	cmd := &cobra.Command{
		Use:   "price-drops <query>",
		Short: "Detect listings whose price fell between syncs — the same item, now cheaper — sorted by the size of the drop",
		Long: strings.Trim(`
Catch price cuts on items you're tracking. Searches OfferUp live and records a
price snapshot, then compares each listing's current price against the highest
price recorded for it within --since, returning the listings that dropped.
Needs at least two runs with a reduction between them, so the first run for a
query is expected to be empty.`, "\n"),
		Example:     "  offerup-pp-cli price-drops \"macbook pro\" --since 7d --zip 98101 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := args[0]
			_, cutoff, err := sinceCutoff(since)
			if err != nil {
				return usageErr(err)
			}
			result := priceDropsResult{Query: query, Location: lf.label(), Since: sinceLabel(since, "7d"), Drops: []offerup.Drop{}}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			client := newOfferupClient(flags)
			listings, err := client.Search(cmd.Context(), query, lf.searchOpts(0))
			if err != nil {
				return classifyOfferupError(err)
			}
			key := lf.storeKey(query)
			st, err := openOfferupStore()
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			if _, err := st.RecordSearch(key, listings); err != nil {
				return apiErr(err)
			}
			drops, err := st.Drops(key, cutoff)
			if err != nil {
				return apiErr(err)
			}
			if drops != nil {
				result.Drops = drops
			}
			result.Count = len(result.Drops)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&lf.zip, "zip", "", "ZIP code to scope the search (e.g. 98101)")
	cmd.Flags().StringVar(&lf.lat, "lat", "", "Latitude for precise location (use with --lon)")
	cmd.Flags().StringVar(&lf.lon, "lon", "", "Longitude for precise location (use with --lat)")
	cmd.Flags().StringVar(&lf.city, "city", "", "City name for the search location")
	cmd.Flags().StringVar(&lf.state, "state", "", "Two-letter state code (e.g. WA)")
	cmd.Flags().StringVar(&lf.category, "category", "", "OfferUp category id (cid) to scope the search")
	cmd.Flags().StringVar(&since, "since", "7d", "Compare against the highest price seen within this window (e.g. 7d, 24h, 1w)")
	return cmd
}

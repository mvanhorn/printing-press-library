// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/offerup"
)

type newSinceResult struct {
	Query    string                  `json:"query"`
	Location string                  `json:"location,omitempty"`
	Since    string                  `json:"since"`
	Count    int                     `json:"count"`
	Listings []offerup.StoredListing `json:"listings"`
}

// pp:data-source auto
func newNovelNewSinceCmd(flags *rootFlags) *cobra.Command {
	lf := &locFlags{}
	var since string
	cmd := &cobra.Command{
		Use:   "new-since <query>",
		Short: "Show only the listings that appeared since a cutoff for a saved search, so you never re-scan items you already saw",
		Long: strings.Trim(`
Track a saved search over time. Searches OfferUp live and records each
listing's first-seen time in the local store, then returns only the listings
first seen within the --since window — the items that are genuinely new since
you last looked. On the first run for a query, every result is new.`, "\n"),
		Example:     "  offerup-pp-cli new-since \"road bike\" --since 24h --zip 98101 --agent",
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
			result := newSinceResult{Query: query, Location: lf.label(), Since: sinceLabel(since, "24h"), Listings: []offerup.StoredListing{}}
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
			fresh, err := st.NewSince(key, cutoff)
			if err != nil {
				return apiErr(err)
			}
			if fresh != nil {
				result.Listings = fresh
			}
			result.Count = len(result.Listings)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&lf.zip, "zip", "", "ZIP code to scope the search (e.g. 98101)")
	cmd.Flags().StringVar(&lf.lat, "lat", "", "Latitude for precise location (use with --lon)")
	cmd.Flags().StringVar(&lf.lon, "lon", "", "Longitude for precise location (use with --lat)")
	cmd.Flags().StringVar(&lf.city, "city", "", "City name for the search location")
	cmd.Flags().StringVar(&lf.state, "state", "", "Two-letter state code (e.g. WA)")
	cmd.Flags().StringVar(&lf.category, "category", "", "OfferUp category id (cid) to scope the search")
	cmd.Flags().StringVar(&since, "since", "24h", "Only listings first seen within this window (e.g. 24h, 7d, 1w)")
	return cmd
}

// sinceLabel returns the since string or the default when empty.
func sinceLabel(since, def string) string {
	if strings.TrimSpace(since) == "" {
		return def
	}
	return since
}

// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/internal/airbnb"
	"github.com/spf13/cobra"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var p airbnb.SearchParams
	var roomTypes string
	cmd := &cobra.Command{
		Use:   "search [location]",
		Short: "Search Airbnb stays by location, dates, guests and price",
		Long: `Search Airbnb stays. Location is required (a city, address, or place name).
Results include listing id, name, nightly price, rating and coordinates — pipe
--json into other commands (e.g. 'contact', 'outreach') to act on hosts.`,
		Example: strings.Trim(`
  airbnb-outreach-pp-cli search "Berlin, Germany" --checkin 2026-08-10 --checkout 2026-08-14 --adults 2
  airbnb-outreach-pp-cli search "Lisbon" --price-max 120 --json --select id,name,price,rating
  airbnb-outreach-pp-cli search "Munich" --room-types "Entire home/apt" --min-bedrooms 2`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			p.Location = args[0]
			if roomTypes != "" {
				p.RoomTypes = splitCSV(roomTypes)
			}
			c := newAirbnbClient(flags)
			results, raw, err := c.Search(p)
			if err != nil {
				return classifyAirbnb(err, flags)
			}
			// Human table gets the flattened shape; --json gets the same typed
			// slice so --select works on stable field names.
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printSearchTable(cmd, results)
			}
			if flags.selectFields != "" || flags.compact || flags.csv {
				return flags.printJSON(cmd, results)
			}
			// Default machine output: rich raw payload.
			if len(raw) > 0 {
				return flags.printJSON(cmd, raw)
			}
			return flags.printJSON(cmd, results)
		},
	}
	cmd.Flags().StringVar(&p.Checkin, "checkin", "", "Check-in date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&p.Checkout, "checkout", "", "Check-out date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&p.Adults, "adults", 0, "Number of adults")
	cmd.Flags().IntVar(&p.Children, "children", 0, "Number of children")
	cmd.Flags().IntVar(&p.Infants, "infants", 0, "Number of infants")
	cmd.Flags().IntVar(&p.Pets, "pets", 0, "Number of pets")
	cmd.Flags().IntVar(&p.PriceMin, "price-min", 0, "Minimum nightly price")
	cmd.Flags().IntVar(&p.PriceMax, "price-max", 0, "Maximum nightly price")
	cmd.Flags().IntVar(&p.MinBedrooms, "min-bedrooms", 0, "Minimum number of bedrooms")
	cmd.Flags().IntVar(&p.ItemsPerGrid, "limit", 18, "Number of results per page")
	cmd.Flags().StringVar(&p.Cursor, "cursor", "", "Pagination cursor from a previous page")
	cmd.Flags().StringVar(&roomTypes, "room-types", "", "Comma-separated room types (e.g. \"Entire home/apt,Private room\")")
	return cmd
}

func printSearchTable(cmd *cobra.Command, results []airbnb.SearchResult) error {
	if len(results) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No results.")
		return nil
	}
	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(tw, bold("ID\tNAME\tPRICE\tRATING\tURL"))
	for _, r := range results {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.ID, truncate(r.Name, 40), r.Price, r.Rating, r.URL)
	}
	return tw.Flush()
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

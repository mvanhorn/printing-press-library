package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	srcebay "github.com/mvanhorn/printing-press-library/library/commerce/ebay/internal/source/ebay"

	"github.com/spf13/cobra"
)

func newAuctionsCmd(flags *rootFlags) *cobra.Command {
	var (
		query        string
		hasBids      bool
		minBids      int
		maxBids      int
		endingWithin time.Duration
		minPrice     float64
		maxPrice     float64
		category     string
		condition    string
		sort         string
		perPage      int
	)
	cmd := &cobra.Command{
		Use:   "auctions [query]",
		Short: "Search active auctions filtered by bid count and ending window",
		Long: `Search active eBay auctions and filter by bid count, ending window, and condition.
Buy It Now and zero-bid listings are excluded by default when --has-bids is set.

This is the "show me Steph Curry cards with at least 3 bids ending in the
next hour" query that the Browse API can no longer answer since the Finding
API was retired in February 2025.

Examples:
  ebay-pp-cli auctions "Steph Curry rookie" --has-bids --ending-within 1h
  ebay-pp-cli auctions "Rolex" --min-bids 5 --ending-within 30m --max-price 500
  ebay-pp-cli auctions "vintage camera" --has-bids --json --select item_id,title,price,bids,time_left`,
		Example: `  ebay-pp-cli auctions "Steph Curry rookie" --has-bids --ending-within 1h
  ebay-pp-cli auctions "Rolex" --min-bids 5 --ending-within 30m`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				query = strings.Join(args, " ")
			}
			if query == "" && !cmd.Flags().Changed("query") {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			src := srcebay.New(c)
			items := []srcebay.Listing{}
			fetched, err := src.FetchActive(context.Background(), srcebay.SearchOptions{
				Query:      query,
				Auction:    true, // auctions only — that's the whole point
				MinPrice:   minPrice,
				MaxPrice:   maxPrice,
				Category:   category,
				Condition:  condition,
				Sort:       defaultStr(sort, "ending-soonest"),
				PerPage:    perPage,
				HasBids:    hasBids,
				MinBids:    minBids,
				MaxBids:    maxBids,
				EndsWithin: endingWithin,
			})
			if err != nil {
				return fmt.Errorf("fetching auctions: %w", err)
			}
			if fetched != nil {
				items = fetched
			}
			data, err := json.Marshal(items)
			if err != nil {
				return err
			}
			if !flags.asJSON && !flags.agent && flags.selectFields == "" && !flags.csv {
				return printAuctionsHuman(cmd, items)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Keyword query (alternative to positional argument)")
	cmd.Flags().BoolVar(&hasBids, "has-bids", false, "Filter to auctions with at least 1 bid (sets --min-bids 1 if unset)")
	cmd.Flags().IntVar(&minBids, "min-bids", 0, "Minimum bid count filter")
	cmd.Flags().IntVar(&maxBids, "max-bids", 0, "Maximum bid count filter (0 = no max)")
	cmd.Flags().DurationVar(&endingWithin, "ending-within", 0, "Filter to auctions ending within this duration (e.g. 1h, 30m, 24h)")
	cmd.Flags().Float64Var(&minPrice, "min-price", 0, "Minimum current-bid price")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Maximum current-bid price")
	cmd.Flags().StringVar(&category, "category", "", "eBay category id filter (numeric)")
	cmd.Flags().StringVar(&condition, "condition", "", "Filter to a condition keyword: new, used, refurb, new-other")
	cmd.Flags().StringVar(&sort, "sort", "ending-soonest", "Sort: ending-soonest, newest, price-asc, price-desc, ending-latest")
	cmd.Flags().IntVar(&perPage, "per-page", 60, "Items per page (max 240)")
	return cmd
}

func printAuctionsHuman(cmd *cobra.Command, items []srcebay.Listing) error {
	w := cmd.OutOrStdout()
	if len(items) == 0 {
		fmt.Fprintln(w, "No auctions match the filters.")
		return nil
	}
	fmt.Fprintf(w, "%d active auctions:\n", len(items))
	for _, it := range items {
		fmt.Fprintf(w, "  $%-8.2f %3d bids  %-14s  %s\n", it.Price, it.Bids, it.TimeLeft, truncate(it.Title, 60))
		fmt.Fprintf(w, "             %s\n", it.URL)
	}
	return nil
}

func defaultStr(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

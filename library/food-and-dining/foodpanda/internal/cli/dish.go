// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command: cross-restaurant dish price search.
//
// foodpanda returns exactly one vendor's menu per call, so no upstream surface
// can answer "which restaurant near me sells the cheapest biryani". This sweeps
// the area listing, pulls menus for the nearest N vendors concurrently, and
// searches every product at once.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/foodpanda/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/foodpanda/internal/store"
)

type fpDishView struct {
	Query           string      `json:"query"`
	Matches         []fpProduct `json:"matches"`
	MatchCount      int         `json:"match_count"`
	VendorsSearched int         `json:"vendors_searched"`
	ProductsScanned int         `json:"products_scanned"`
	ScannedVendors  int         `json:"scanned_vendors"`
	MaxVendors      int         `json:"max_vendors"`
	FetchFailures   []string    `json:"fetch_failures,omitempty"`
	Note            string      `json:"note,omitempty"`
}

func newNovelDishCmd(flags *rootFlags) *cobra.Command {
	var (
		flagQuery   string
		lat, lng    float64
		limit       int
		maxVendors  int
		concurrency int
		maxPrice    float64
		sortKey     string
		country     string
		dbPath      string
		snapshot    bool
		allVendors  bool
		nameOnly    bool
	)

	cmd := &cobra.Command{
		Use:   "dish",
		Short: "Find which nearby restaurant sells a specific dish cheapest, searching every synced menu at once.",
		Long: "Search every nearby restaurant's menu at once for a dish.\n\n" +
			"foodpanda serves one vendor's menu per request, so this sweeps the area listing\n" +
			"and pulls menus for the nearest vendors concurrently before matching.\n" +
			"Do NOT use this to find restaurants by name; use 'find' for that.",
		Example: "  foodpanda-pp-cli dish --query 'chicken biryani' --max-price 600 --agent",
		// read-only by default: snapshots are opt-in via --snapshot, so a plain
		// search never writes. With --snapshot the write stays inside the CLI's
		// own SQLite store and never changes this command's output.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true", "pp:happy-args": "--query=biryani;--latitude=31.5204;--longitude=74.3587;--max-vendors=3;--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "dish")
			}
			query := strings.TrimSpace(flagQuery)
			if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
				query = strings.TrimSpace(args[0])
			}
			if query == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a dish query is required (positional or --query)"))
			}
			if lat == 0 || lng == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--latitude and --longitude are required"))
			}
			tokens := fpQueryTokens(query)

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if cliutil.IsDogfoodEnv() && maxVendors > 3 {
				maxVendors = 3
			}
			// Seed candidates from the SEARCH endpoint, not the plain listing.
			// The listing is not relevance-ordered, so taking its first N vendors
			// searches whoever happens to rank first (often unrelated cafes) and
			// misses the dish entirely. Search returns vendors upstream already
			// associates with the query, which is a far better candidate pool.
			seedURL := fpDiscoSearch
			seedQuery := fpQuery{Lat: lat, Lng: lng, Country: country, Query: query}
			if allVendors {
				seedURL, seedQuery.Query = fpDiscoVendors, ""
			}
			sweep, err := fpSweep(ctx, c, seedURL, seedQuery, 2)
			if err != nil {
				return err
			}
			// Prefer vendors whose own name/cuisine matches the query, so the
			// menu-fetch budget is spent on the most likely sellers first.
			if !allVendors {
				ranked := append([]fpVendor(nil), sweep.Vendors...)
				sort.SliceStable(ranked, func(i, j int) bool {
					return fpMatchScore(ranked[i], query) > fpMatchScore(ranked[j], query)
				})
				sweep.Vendors = ranked
			}
			candidates := fpTrim(sweep.Vendors, maxVendors)
			results := fpFetchMenus(ctx, c, country, candidates, concurrency)

			view := fpDishView{
				Query: query, ScannedVendors: sweep.Scanned, MaxVendors: maxVendors,
			}
			failures := make([]string, 0)
			matches := make([]fpProduct, 0, 32)
			for _, r := range results {
				if r.Err != nil {
					failures = append(failures, fmt.Sprintf("%s (%s): %s", r.VendorName, r.VendorCode, truncate(r.Err.Error(), 100)))
					continue
				}
				view.VendorsSearched++
				view.ProductsScanned += len(r.Products)
				for _, p := range r.Products {
					if !fpProductMatches(p, tokens) {
						continue
					}
					if maxPrice > 0 && p.Price > maxPrice {
						continue
					}
					p.MatchedOn = fpMatchedOn(p, tokens)
					if nameOnly && p.MatchedOn != "name" {
						continue
					}
					matches = append(matches, p)
				}
			}

			switch sortKey {
			case "price":
				sort.SliceStable(matches, func(i, j int) bool { return matches[i].Price < matches[j].Price })
				// Direct item-name hits outrank category-only context hits, so a
				// side dish in a "Biryani" category never outranks a real biryani.
				sort.SliceStable(matches, func(i, j int) bool {
					return matchRank(matches[i].MatchedOn) < matchRank(matches[j].MatchedOn)
				})
			case "name":
				sort.SliceStable(matches, func(i, j int) bool { return matches[i].Name < matches[j].Name })
			case "vendor":
				sort.SliceStable(matches, func(i, j int) bool { return matches[i].VendorName < matches[j].VendorName })
			}
			view.MatchCount = len(matches)
			view.Matches = fpTrim(matches, limit)
			if len(failures) > 0 {
				view.FetchFailures = failures
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: %d of %d menu fetches failed; results cover the remaining %d vendors\n",
					len(failures), len(results), view.VendorsSearched)
			}
			if len(matches) == 0 {
				view.Note = fmt.Sprintf("searched %d products across %d vendors without a match; raise --max-vendors or relax --max-price",
					view.ProductsScanned, view.VendorsSearched)
			}

			// Persist snapshots so menu-diff has history to compare against.
			if snapshot {
				if dbPath == "" {
					dbPath = defaultDBPath("foodpanda-pp-cli")
				}
				if s, err := store.OpenWithContext(ctx, dbPath); err == nil {
					for _, r := range results {
						if r.Err != nil || len(r.Raw) == 0 {
							continue
						}
						_ = s.SaveMenuSnapshot(ctx, r.VendorCode, r.VendorName, country, r.Raw)
					}
					_ = s.Close()
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Matches) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No menu item matched %q.\n%s\n", query, view.Note)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d matches for %q across %d vendors (%d products scanned)\n\n",
				view.MatchCount, query, view.VendorsSearched, view.ProductsScanned)
			out := make([][]string, 0, len(view.Matches))
			for _, m := range view.Matches {
				label := m.Name
				if m.Variation != "" && !strings.EqualFold(m.Variation, m.Name) {
					label = m.Name + " (" + m.Variation + ")"
				}
				out = append(out, []string{
					fmt.Sprintf("%.0f", m.Price), truncate(label, 40),
					truncate(m.VendorName, 28), m.VendorCode, truncate(m.Category, 18),
				})
			}
			return flags.printTable(cmd, []string{"PRICE", "ITEM", "RESTAURANT", "CODE", "CATEGORY"}, out)
		},
	}

	cmd.Flags().Float64Var(&lat, "latitude", 31.5204, "Latitude of the delivery point")
	cmd.Flags().Float64Var(&lng, "longitude", 74.3587, "Longitude of the delivery point")
	cmd.Flags().IntVar(&limit, "limit", 30, "Maximum matching items to return")
	cmd.Flags().IntVar(&maxVendors, "max-vendors", 12, "Maximum vendor menus to fetch and search")
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "Parallel menu fetches")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Drop items priced above this (0 = no cap)")
	cmd.Flags().StringVar(&sortKey, "sort", "price", "Sort by: price, name, vendor")
	cmd.Flags().StringVar(&country, "country", "pk", "Market code: pk, bd, sg, my, hk, th")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path for menu snapshots")
	cmd.Flags().BoolVar(&snapshot, "snapshot", false, "Also record fetched menus as menu-diff baselines (off by default: search stays a pure read)")
	cmd.Flags().BoolVar(&nameOnly, "name-only", false, "Only keep items whose own name matches (drops category-only context hits)")
	cmd.Flags().BoolVar(&allVendors, "all-vendors", false, "Search the nearest vendors instead of search-matched ones (slower, broader)")
	cmd.Flags().StringVar(&flagQuery, "query", "", "Dish to search for (alternative to the positional argument)")
	return cmd
}

// matchRank orders match provenance: direct name hits first.
func matchRank(m string) int {
	switch m {
	case "name":
		return 0
	case "category":
		return 1
	default:
		return 2
	}
}

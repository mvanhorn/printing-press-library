// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/bestfoodtrucks/internal/graphqlclient"
	"github.com/spf13/cobra"
)

type TruckDetailsResult struct {
	TruckID int
	Truck   *GqlTruckGetResult
	Error   error
}

type HotlistEntry struct {
	ID            GqlID   `json:"id"`
	Name          string  `json:"name"`
	AverageRating float64 `json:"averageRating"`
	RatingsCount  int     `json:"ratingsCount"`
}

type HotlistEnvelope struct {
	MarketQuery   string         `json:"market_query"`
	MarketName    string         `json:"market_name"`
	FetchFailures []string       `json:"fetch_failures,omitempty"`
	Results       []HotlistEntry `json:"results"`
}

func newNovelMarketHotlistCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "hotlist [city-or-id]",
		Short: "Ranks trucks operating in a city by review signal.",
		Long:  "Ranks trucks operating in a city by review signal — a cross-truck aggregate the site never computes.",
		Example: strings.Trim(`
  bestfoodtrucks-pp-cli market hotlist los-angeles --limit 5
  bestfoodtrucks-pp-cli market hotlist los-angeles --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "city=los-angeles"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "market hotlist")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			marketStr := "los-angeles" // default fallback
			if len(args) >= 1 {
				marketStr = args[0]
			}

			return runMarketHotlist(ctx, cmd, flags, marketStr, limit)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of trucks to show in ranking")
	return cmd
}

func runMarketHotlist(ctx context.Context, cmd *cobra.Command, flags *rootFlags, marketStr string, limit int) error {
	if limit < 1 {
		return usageErr(fmt.Errorf("--limit must be a positive integer, got %d", limit))
	}
	id, err := resolveMarketID(marketStr)
	if err != nil {
		return usageErr(err)
	}

	client := graphqlclient.New(flags.timeout)

	// Step 1: List all trucks in the market
	query := `
		query GetMarketTrucks($id: Int!) {
			market(id: $id) {
				id
				name
				trucks {
					records {
						id
						name
					}
				}
			}
		}
	`
	var mResult struct {
		Market *GqlMarketListResult `json:"market"`
	}
	err = client.Query(ctx, query, map[string]any{"id": id}, &mResult)
	if err != nil {
		return classifyAPIError(cmd.OutOrStdout(), fmt.Errorf("fetching market: %w", err), flags)
	}
	if mResult.Market == nil {
		return notFoundErr(fmt.Errorf("market not found: %d", id))
	}

	if mResult.Market.Trucks == nil || len(mResult.Market.Trucks.Records) == 0 {
		note := "No trucks found in this market to rank."
		if flags.asJSON {
			return printJSONFiltered(cmd.OutOrStdout(), HotlistEnvelope{
				MarketQuery: marketStr,
				MarketName:  mResult.Market.Name,
				Results:     []HotlistEntry{},
			}, flags)
		}
		fmt.Fprintln(cmd.OutOrStdout(), note)
		return nil
	}

	trucks := mResult.Market.Trucks.Records

	// Step 2: Concurrently query truck details with a pool of workers to fetch ratingInfo
	concurrencyLimit := 5
	sem := make(chan struct{}, concurrencyLimit)
	resultsChan := make(chan TruckDetailsResult, len(trucks))

	for _, tr := range trucks {
		go func(tID int) {
			sem <- struct{}{}
			defer func() { <-sem }()

			qDetails := `
				query GetTruckRating($id: Int!) {
					truck(id: $id) {
						id
						name
						averageRating
						ratingInfo {
							averageRating
							ratingsCount
						}
					}
				}
			`
			var tResult struct {
				Truck *GqlTruckGetResult `json:"truck"`
			}
			err := client.Query(ctx, qDetails, map[string]any{"id": tID}, &tResult)
			if err != nil {
				resultsChan <- TruckDetailsResult{TruckID: tID, Error: err}
				return
			}
			resultsChan <- TruckDetailsResult{TruckID: tID, Truck: tResult.Truck}
		}(tr.ID.Int())
	}

	var fetchFailures []string
	hotlist := make([]HotlistEntry, 0, len(trucks))

	for i := 0; i < len(trucks); i++ {
		res := <-resultsChan
		if res.Error != nil {
			fetchFailures = append(fetchFailures, fmt.Sprintf("truck %d: %v", res.TruckID, res.Error))
			continue
		}
		if res.Truck != nil {
			ratingVal := res.Truck.AverageRating
			countVal := 0
			if res.Truck.RatingInfo != nil {
				ratingVal = res.Truck.RatingInfo.AverageRating
				countVal = res.Truck.RatingInfo.RatingsCount
			}
			hotlist = append(hotlist, HotlistEntry{
				ID:            res.Truck.ID,
				Name:          res.Truck.Name,
				AverageRating: ratingVal,
				RatingsCount:  countVal,
			})
		}
	}

	// Sort: average rating descending, then ratings count descending
	sort.Slice(hotlist, func(i, j int) bool {
		if hotlist[i].AverageRating != hotlist[j].AverageRating {
			return hotlist[i].AverageRating > hotlist[j].AverageRating
		}
		return hotlist[i].RatingsCount > hotlist[j].RatingsCount
	})

	// Apply limit
	if limit > 0 && len(hotlist) > limit {
		hotlist = hotlist[:limit]
	}

	// Warn on stderr if some truck lookups failed
	if len(fetchFailures) > 0 && !flags.asJSON {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to fetch details for %d trucks\n", len(fetchFailures))
	}

	if flags.asJSON {
		envelope := HotlistEnvelope{
			MarketQuery:   marketStr,
			MarketName:    mResult.Market.Name,
			FetchFailures: fetchFailures,
			Results:       hotlist,
		}
		return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Market Hotlist for %s (%s):\n", bold(mResult.Market.Name), marketStr)
	fmt.Fprintln(w, strings.Repeat("-", 50))
	for idx, tr := range hotlist {
		reviewWord := "reviews"
		if tr.RatingsCount == 1 {
			reviewWord = "review"
		}
		fmt.Fprintf(w, "  %2d. %-30s %.1f★ (%d %s)\n", idx+1, green(tr.Name), tr.AverageRating, tr.RatingsCount, reviewWord)
	}

	return nil
}

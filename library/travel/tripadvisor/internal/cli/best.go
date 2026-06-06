// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored transcendence command. Preserved across regen.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type bestView struct {
	Query         string           `json:"query"`
	Category      string           `json:"category,omitempty"`
	Sort          string           `json:"sort"`
	ScannedCount  int              `json:"scanned_locations"`
	MaxScan       int              `json:"max_scan"`
	Results       []taDetail       `json:"results"`
	FetchFailures []taFetchFailure `json:"fetch_failures"`
	Note          string           `json:"note,omitempty"`
}

func newNovelBestCmd(flags *rootFlags) *cobra.Command {
	var (
		category string
		top      int
		sortKey  string
		maxScan  int
		language string
	)

	cmd := &cobra.Command{
		Use:   "best <query>",
		Short: "Search a place type and return the top hits ranked by rating, review count, or ranking",
		Long: "Search Tripadvisor, auto-fetch details for the matching locations (bounded by --max-scan " +
			"because the Content API is metered), and return them ranked by --sort. " +
			"Use this when the task is 'find the best X in Y' instead of calling find then show repeatedly.",
		Example: "  tripadvisor-pp-cli best \"Paris\" --category hotels --top 5 --sort rating --agent",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0",
			"pp:happy-args":       "<query>=Paris;--category=hotels",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("query is required"))
			}
			switch sortKey {
			case "rating", "reviews", "ranking":
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--sort must be one of rating, reviews, ranking"))
			}
			query := args[0]
			scan := taDogfoodScan(maxScan)

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			stubs, err := taSearch(cmd.Context(), c, query, category, "", language)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			ids := make([]string, 0, len(stubs))
			for _, s := range stubs {
				ids = append(ids, s.LocationID)
			}
			details, failures, scanned := taFetchDetailsBounded(cmd.Context(), c, ids, language, "", scan)
			taSortDetails(details, sortKey)
			details = taLimit(details, top)

			view := bestView{
				Query:         query,
				Category:      category,
				Sort:          sortKey,
				ScannedCount:  scanned,
				MaxScan:       scan,
				Results:       details,
				FetchFailures: failures,
			}
			if len(stubs) == 0 {
				view.Note = fmt.Sprintf("no locations matched %q; try a broader query or a different --category", query)
			} else if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d detail fetches failed; ranking computed over the remaining %d\n", len(failures), scanned, len(details))
			}
			return emitTANovel(cmd, flags, view, view.Results)
		},
	}

	cmd.Flags().StringVar(&category, "category", "", "Place type: hotels, restaurants, attractions, geos")
	cmd.Flags().IntVar(&top, "top", 5, "Number of ranked results to return")
	cmd.Flags().StringVar(&sortKey, "sort", "rating", "Rank by: rating, reviews, ranking")
	cmd.Flags().IntVar(&maxScan, "max-scan", taScanDefault, "Max locations to fetch details for before ranking (metered API)")
	cmd.Flags().StringVar(&language, "language", "en", "Language code")
	return cmd
}

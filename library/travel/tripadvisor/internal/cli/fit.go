// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored transcendence command. Preserved across regen.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type fitRow struct {
	taDetail
	TravelerShare float64 `json:"traveler_share"`
	FitScore      float64 `json:"fit_score"`
}

type fitView struct {
	Query         string           `json:"query"`
	Traveler      string           `json:"traveler"`
	Category      string           `json:"category,omitempty"`
	ScannedCount  int              `json:"scanned_locations"`
	MaxScan       int              `json:"max_scan"`
	Results       []fitRow         `json:"results"`
	FetchFailures []taFetchFailure `json:"fetch_failures"`
	Note          string           `json:"note,omitempty"`
}

func newNovelFitCmd(flags *rootFlags) *cobra.Command {
	var (
		category string
		traveler string
		top      int
		maxScan  int
		language string
	)

	cmd := &cobra.Command{
		Use:   "fit <query>",
		Short: "Rank search results by how well their traveler mix fits a profile (families, couples, solo, business)",
		Long: "Search Tripadvisor, fetch details up to --max-scan (metered API), and rank results by a fit score " +
			"that combines the share of reviews from your declared traveler type with the location's overall " +
			"rating. traveler_share is the raw share (0-1); fit_score weights that share by rating/5 so a place " +
			"that is both popular with your traveler type and well-rated ranks highest.",
		Example: "  tripadvisor-pp-cli fit \"Orlando\" --category hotels --traveler families --top 5 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "<query>=Orlando;--traveler=families;--category=hotels",
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
			key, ok := taTravelerProfiles[strings.ToLower(strings.TrimSpace(traveler))]
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--traveler must be one of families, couples, solo, business, friends"))
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

			rows := make([]fitRow, 0, len(details))
			for _, d := range details {
				total := 0
				for _, v := range d.TripTypes {
					total += v
				}
				share := 0.0
				if total > 0 {
					share = float64(d.TripTypes[key]) / float64(total)
				}
				// fit_score weights traveler share by overall rating (rating/5),
				// so a place popular with this traveler type AND well-rated ranks
				// highest. Falls back to raw share when no rating is available.
				fitScore := share
				if d.Rating > 0 {
					fitScore = share * (d.Rating / 5.0)
				}
				rows = append(rows, fitRow{taDetail: d, TravelerShare: round2(share), FitScore: round2(fitScore)})
			}
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].FitScore != rows[j].FitScore {
					return rows[i].FitScore > rows[j].FitScore
				}
				return rows[i].Rating > rows[j].Rating
			})
			if top > 0 && len(rows) > top {
				rows = rows[:top]
			}

			view := fitView{
				Query:         query,
				Traveler:      key,
				Category:      category,
				ScannedCount:  scanned,
				MaxScan:       scan,
				Results:       rows,
				FetchFailures: failures,
			}
			if len(stubs) == 0 {
				view.Note = fmt.Sprintf("no locations matched %q", query)
			} else if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d detail fetches failed; fit ranking computed over the rest\n", len(failures), scanned)
			}

			tableRows := make([]taDetail, 0, len(rows))
			for _, r := range rows {
				tableRows = append(tableRows, r.taDetail)
			}
			return emitTANovel(cmd, flags, view, tableRows)
		},
	}

	cmd.Flags().StringVar(&category, "category", "", "Place type: hotels, restaurants, attractions, geos")
	cmd.Flags().StringVar(&traveler, "traveler", "", "Traveler profile: families, couples, solo, business, friends")
	cmd.Flags().IntVar(&top, "top", 5, "Number of ranked results to return")
	cmd.Flags().IntVar(&maxScan, "max-scan", 15, "Max locations to fetch details for before ranking (metered API)")
	cmd.Flags().StringVar(&language, "language", "en", "Language code")
	return cmd
}

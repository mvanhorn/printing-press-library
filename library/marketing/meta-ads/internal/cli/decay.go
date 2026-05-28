// Copyright 2026 dhilip-subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads/internal/store"

	"github.com/spf13/cobra"
)

type decayView struct {
	CreativeID        string  `json:"creative_id"`
	FirstDate         string  `json:"first_date,omitempty"`
	LastDate          string  `json:"last_date,omitempty"`
	FirstCtr          float64 `json:"first_ctr"`
	LastCtr           float64 `json:"last_ctr"`
	CtrSlope          float64 `json:"ctr_slope"`
	DaysObserved      int     `json:"days_observed"`
	ProjectedDeadDate string  `json:"projected_dead_date,omitempty"`
	Verdict           string  `json:"verdict"`
	Note              string  `json:"note,omitempty"`
}

func newNovelDecayCmd(flags *rootFlags) *cobra.Command {
	var flagCreativeID string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "decay",
		Short: "Single-creative CTR decay curve with slope and projected dead-date.",
		Long: `For a creative, find every ad using it, aggregate daily CTR across all ads,
and compute the slope and projected dead-date (CTR=0 extrapolation).

Requires synced ad-level insights in the local store keyed to ads referencing this creative.`,
		Example: `  meta-ads-pp-cli decay --creative-id 120208734567 --agent
  meta-ads-pp-cli decay --creative-id 120208734567 --json --select ctr_slope,verdict`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute CTR decay curve from local insights")
				return nil
			}
			if flagCreativeID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--creative-id is required"))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("meta-ads-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			// Find ads that reference this creative
			adRows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT json_extract(data, '$.id') FROM resources
				 WHERE resource_type IN ('ads', 'adaccounts_ads', 'ad_accounts_ads')
				   AND (json_extract(data, '$.creative.id') = ? OR json_extract(data, '$.creative.creative_id') = ?)`,
				flagCreativeID, flagCreativeID)
			if err != nil {
				return fmt.Errorf("ads query: %w", err)
			}
			adIDs := make([]string, 0)
			for adRows.Next() {
				var id string
				if err := adRows.Scan(&id); err == nil && id != "" {
					adIDs = append(adIDs, id)
				}
			}
			if err := adRows.Err(); err != nil {
				_ = adRows.Close()
				return fmt.Errorf("iterating ad rows: %w", err)
			}
			_ = adRows.Close()

			if len(adIDs) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), decayView{
					CreativeID: flagCreativeID,
					Verdict:    "no-data",
					Note:       "no ads in local store reference this creative; run sync --resources ads first",
				}, flags)
			}

			// For each ad, sum impressions and clicks per day to derive CTR.
			type dayPoint struct {
				date        string
				impressions int64
				clicks      int64
			}
			byDate := make(map[string]*dayPoint)
			for _, adID := range adIDs {
				// Bail early if the parent context is cancelled — otherwise every
				// subsequent QueryContext returns context.Canceled, the loop exhausts
				// silently, and the command computes a slope from partial data with
				// exit 0 and no signal that the aggregate is incomplete.
				if ctxErr := cmd.Context().Err(); ctxErr != nil {
					return fmt.Errorf("aborted before scanning all ads: %w", ctxErr)
				}
				rows, err := db.DB().QueryContext(cmd.Context(),
					`SELECT data FROM resources
					 WHERE resource_type IN ('insights', 'ads_insights', 'adaccounts_insights')
					   AND json_extract(data, '$.ad_id') = ?
					 ORDER BY json_extract(data, '$.date_start') ASC`,
					adID)
				if err != nil {
					if ctxErr := cmd.Context().Err(); ctxErr != nil {
						return fmt.Errorf("aborted while scanning ad %s: %w", adID, ctxErr)
					}
					continue
				}
				for rows.Next() {
					var data []byte
					if err := rows.Scan(&data); err != nil {
						continue
					}
					var raw struct {
						DateStart   string `json:"date_start"`
						Impressions string `json:"impressions"`
						Clicks      string `json:"clicks"`
					}
					if err := json.Unmarshal(data, &raw); err != nil {
						continue
					}
					p, ok := byDate[raw.DateStart]
					if !ok {
						p = &dayPoint{date: raw.DateStart}
						byDate[raw.DateStart] = p
					}
					if n, err := strconv.ParseInt(raw.Impressions, 10, 64); err == nil {
						p.impressions += n
					}
					if n, err := strconv.ParseInt(raw.Clicks, 10, 64); err == nil {
						p.clicks += n
					}
				}
				if err := rows.Err(); err != nil {
					_ = rows.Close()
					return fmt.Errorf("iterating insights rows for ad %s: %w", adID, err)
				}
				_ = rows.Close()
			}

			// Compute CTR per day, sort, derive slope
			points := make([]fatigueDay, 0, len(byDate))
			for _, p := range byDate {
				ctr := 0.0
				if p.impressions > 0 {
					ctr = float64(p.clicks) / float64(p.impressions) * 100.0
				}
				points = append(points, fatigueDay{
					Date:        p.date,
					Impressions: p.impressions,
					Ctr:         ctr,
				})
			}
			// Sort by date
			sortByDate(points)

			view := decayView{CreativeID: flagCreativeID, DaysObserved: len(points)}
			if len(points) < 2 {
				view.Verdict = "insufficient-data"
				view.Note = "need at least 2 days of insights to compute decay; sync more history"
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			view.FirstDate = points[0].Date
			view.LastDate = points[len(points)-1].Date
			view.FirstCtr = points[0].Ctr
			view.LastCtr = points[len(points)-1].Ctr
			view.CtrSlope = slope(points, func(d fatigueDay) float64 { return d.Ctr })

			if view.FirstCtr > 0 && view.LastCtr < view.FirstCtr*0.5 {
				view.Verdict = "decayed"
			} else if view.CtrSlope < 0 {
				view.Verdict = "decaying"
			} else {
				view.Verdict = "stable"
			}

			// Project dead-date: when CTR would reach 0 given current slope.
			if view.CtrSlope < 0 && view.LastCtr > 0 {
				daysToDead := view.LastCtr / -view.CtrSlope
				view.ProjectedDeadDate = fmt.Sprintf("~%.0f days from %s", daysToDead, view.LastDate)
			}

			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagCreativeID, "creative-id", "", "Creative ID to analyze")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to ~/.meta-ads-pp-cli/data.db)")
	return cmd
}

func sortByDate(points []fatigueDay) {
	// simple in-place sort
	for i := 1; i < len(points); i++ {
		for j := i; j > 0 && points[j-1].Date > points[j].Date; j-- {
			points[j-1], points[j] = points[j], points[j-1]
		}
	}
}

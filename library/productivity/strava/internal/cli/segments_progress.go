// Copyright 2026 azaaron. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/strava/internal/cliutil"
)

type segmentProgressRow struct {
	EffortID    string  `json:"effort_id"`
	StartDate   string  `json:"start_date"`
	ElapsedSec  int     `json:"elapsed_seconds"`
	Elapsed     string  `json:"elapsed"`
	DeltaPRSec  int     `json:"delta_pr_seconds"`
	AvgWatts    float64 `json:"avg_watts,omitempty"`
	AvgHR       float64 `json:"avg_hr,omitempty"`
	IsPR        bool    `json:"is_pr"`
}

func newSegmentsProgressCmd(flags *rootFlags) *cobra.Command {
	var since string
	var limit int

	cmd := &cobra.Command{
		Use:         "progress <segment-id>",
		Short:       "See your full effort history on a segment to track progression",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Fetches all of your personal efforts on a segment from the Strava API
and shows them chronologically with elapsed time, delta from your PR, and
average power/HR (when available).

Requires activity:read_all scope for private efforts.`,
		Example: strings.Trim(`
  strava-pp-cli segments progress 229781
  strava-pp-cli segments progress 229781 --since 2025-01-01 --agent
  strava-pp-cli segments progress 229781 --json --select start_date,elapsed,delta_pr_seconds`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				sample := []segmentProgressRow{
					{EffortID: "1001", StartDate: "2025-06-01T08:00:00Z", ElapsedSec: 240, Elapsed: "4:00", DeltaPRSec: 0, AvgWatts: 320, AvgHR: 172, IsPR: true},
					{EffortID: "1002", StartDate: "2025-08-15T07:30:00Z", ElapsedSec: 235, Elapsed: "3:55", DeltaPRSec: -5, AvgWatts: 335, AvgHR: 175, IsPR: true},
				}
				return printJSONFiltered(cmd.OutOrStdout(), sample, flags)
			}

			segmentID := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Paginate until Strava returns an empty page (max 200 per page).
			// When --limit > 0 we stop early after collecting enough efforts.
			var efforts []map[string]any
			for page := 1; ; page++ {
				pageSize := 200
				if limit > 0 && limit-len(efforts) < pageSize {
					pageSize = limit - len(efforts)
					if pageSize <= 0 {
						break
					}
				}
				params := map[string]string{
					"per_page": fmt.Sprintf("%d", pageSize),
					"page":     fmt.Sprintf("%d", page),
				}
				if since != "" {
					params["start_date_local"] = since + "T00:00:00Z"
				}

				data, err := c.Get(cmd.Context(), "/segments/"+segmentID+"/all_efforts", params)
				if err != nil {
					return fmt.Errorf("fetching segment efforts (requires activity:read_all scope): %w", err)
				}
				var page_efforts []map[string]any
				if err := json.Unmarshal(data, &page_efforts); err != nil {
					return fmt.Errorf("parsing efforts: %w", err)
				}
				if len(page_efforts) == 0 {
					break
				}
				efforts = append(efforts, page_efforts...)
				if limit > 0 && len(efforts) >= limit {
					efforts = efforts[:limit]
					break
				}
				// Strava returns fewer than per_page on the last page
				if len(page_efforts) < pageSize {
					break
				}
			}

			if len(efforts) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), []segmentProgressRow{}, flags)
			}

			// Find PR (min elapsed_time)
			prSec := math.MaxFloat64
			for _, e := range efforts {
				if et := jsonFloat(e, "elapsed_time"); et > 0 && et < prSec {
					prSec = et
				}
			}

			var result []segmentProgressRow
			for _, e := range efforts {
				effortID := fmt.Sprintf("%.0f", jsonFloat(e, "id"))
				startDate, _ := e["start_date"].(string)
				elapsedSec := int(jsonFloat(e, "elapsed_time"))
				deltaPR := elapsedSec - int(prSec)

				row := segmentProgressRow{
					EffortID:   effortID,
					StartDate:  startDate,
					ElapsedSec: elapsedSec,
					Elapsed:    formatDuration(elapsedSec),
					DeltaPRSec: deltaPR,
					IsPR:       deltaPR == 0,
				}
				if aw := jsonFloat(e, "average_watts"); aw > 0 {
					row.AvgWatts = math.Round(aw)
				}
				if ah := jsonFloat(e, "average_heartrate"); ah > 0 {
					row.AvgHR = math.Round(ah)
				}
				result = append(result, row)
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Only show efforts since this date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of efforts to return (0 = all)")
	return cmd
}


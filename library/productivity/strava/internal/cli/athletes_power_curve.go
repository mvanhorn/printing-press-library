// Copyright 2026 azaaron. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/strava/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/strava/internal/store"
)

type powerCurveRow struct {
	Duration string  `json:"duration"`
	Seconds  int     `json:"seconds"`
	Watts    float64 `json:"watts"`
	WKg      float64 `json:"wkg,omitempty"`
}

var powerCurveWindows = []struct {
	label   string
	seconds int
}{
	{"1s", 1},
	{"5s", 5},
	{"30s", 30},
	{"1m", 60},
	{"5m", 300},
	{"20m", 1200},
	{"60m", 3600},
}

func newAthletesPowerCurveCmd(flags *rootFlags) *cobra.Command {
	var since string
	var weight float64
	var dbPath string

	cmd := &cobra.Command{
		Use:         "power-curve",
		Short:       "See your best mean power for each standard duration (1s to 60min)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Computes your best mean power (W) for each standard duration window by
fetching power streams live from the Strava API for activities in the local
store. Optionally normalizes to W/kg with --weight.

Activities must be synced first ('strava-pp-cli sync'). Stream data is fetched
live — one API call per activity with power data. Rate-limited to ~2 req/sec.`,
		Example: strings.Trim(`
  strava-pp-cli athlete power-curve
  strava-pp-cli athlete power-curve --since 2025-01-01 --weight 72 --agent
  strava-pp-cli athlete power-curve --json --select duration,watts,wkg`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				sample := []powerCurveRow{
					{Duration: "1m", Seconds: 60, Watts: 420, WKg: 5.8},
					{Duration: "5m", Seconds: 300, Watts: 360, WKg: 5.0},
					{Duration: "20m", Seconds: 1200, Watts: 310, WKg: 4.3},
				}
				return printJSONFiltered(cmd.OutOrStdout(), sample, flags)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("strava-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\nRun 'strava-pp-cli sync' first", err)
			}
			defer db.Close()

			// Fetch activity IDs from local store (sync populates this).
			// Filter to activities that have weighted_average_watts > 0 as a
			// cheap pre-filter — avoids fetching streams for non-power activities.
			query := `SELECT id FROM resources
WHERE resource_type IN ('athlete-activities', 'activities')
AND COALESCE(json_extract(data, '$.weighted_average_watts'), 0) > 0`
			var qargs []any
			if since != "" {
				query += ` AND COALESCE(json_extract(data, '$.start_date'), '') >= ?`
				qargs = append(qargs, since+"T00:00:00Z")
			}
			query += ` ORDER BY json_extract(data, '$.start_date') DESC`

			rows, err := db.DB().QueryContext(cmd.Context(), query, qargs...)
			if err != nil {
				return fmt.Errorf("querying activities: %w", err)
			}
			defer rows.Close()

			var activityIDs []string
			for rows.Next() {
				var id sql.NullString
				if err := rows.Scan(&id); err != nil || !id.Valid {
					continue
				}
				activityIDs = append(activityIDs, id.String)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading rows: %w", err)
			}

			if len(activityIDs) == 0 {
				return fmt.Errorf("no power-meter activities found in local store\n" +
					"Run 'strava-pp-cli sync' to populate activity data first.")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fetch watts streams live per activity; compute sliding-window max.
			bestWatts := make([]float64, len(powerCurveWindows))
			fetched := 0

			for i, actID := range activityIDs {
				if cliutil.IsDogfoodEnv() && i >= 2 {
					break
				}
				// Rate-limit: ~2 req/sec for stream fetches
				if i > 0 {
					time.Sleep(500 * time.Millisecond)
				}

				streamData, err := c.Get(cmd.Context(),
					"/activities/"+actID+"/streams",
					map[string]string{"keys": "watts", "key_by_type": "true"})
				if err != nil {
					continue // skip activities without stream access
				}

				wattsArray := extractStreamValues(string(streamData), "watts")
				if len(wattsArray) == 0 {
					continue
				}
				fetched++
				for j, win := range powerCurveWindows {
					best := slidingWindowMean(wattsArray, win.seconds)
					if best > bestWatts[j] {
						bestWatts[j] = best
					}
				}
			}

			if fetched == 0 {
				return fmt.Errorf("no power stream data returned for any activity\n" +
					"Requires activities recorded with a power meter and activity:read_all scope.")
			}

			var result []powerCurveRow
			for i, win := range powerCurveWindows {
				w := math.Round(bestWatts[i])
				row := powerCurveRow{
					Duration: win.label,
					Seconds:  win.seconds,
					Watts:    w,
				}
				if weight > 0 && w > 0 {
					row.WKg = math.Round((w/weight)*100) / 100
				}
				result = append(result, row)
			}

			_ = json.Marshal // keep import used via printJSONFiltered
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Only include activities since this date (YYYY-MM-DD)")
	cmd.Flags().Float64Var(&weight, "weight", 0, "Body weight in kg for W/kg normalization")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// slidingWindowMean computes the highest mean of any consecutive windowSec-length
// subarray in vals (treating vals as 1-second samples).
func slidingWindowMean(vals []float64, windowSec int) float64 {
	n := len(vals)
	if n < windowSec {
		if n == 0 {
			return 0
		}
		sum := 0.0
		for _, v := range vals {
			sum += v
		}
		return sum / float64(n)
	}
	sum := 0.0
	for i := 0; i < windowSec; i++ {
		sum += vals[i]
	}
	best := sum
	for i := windowSec; i < n; i++ {
		sum += vals[i] - vals[i-windowSec]
		if sum > best {
			best = sum
		}
	}
	return best / float64(windowSec)
}

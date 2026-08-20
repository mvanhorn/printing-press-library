// Copyright 2026 azaaron and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/strava/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/strava/internal/store"
	"github.com/spf13/cobra"
)

type gearStatusRow struct {
	GearID            string  `json:"gear_id"`
	Name              string  `json:"name"`
	Brand             string  `json:"brand,omitempty"`
	Model             string  `json:"model,omitempty"`
	TotalDistanceMi   float64 `json:"total_distance_mi"`
	TotalDistanceKm   float64 `json:"total_distance_km"`
	TotalHours        float64 `json:"total_hours"`
	ThresholdMi       float64 `json:"threshold_mi,omitempty"`
	PctThreshold      float64 `json:"pct_threshold,omitempty"`
	EstRetirementDate string  `json:"est_retirement_date,omitempty"`
	Status            string  `json:"status"`
}

func newGearStatusCmd(flags *rootFlags) *cobra.Command {
	var threshold string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "status",
		Short:       "See total mileage and retirement status for all gear",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Aggregates total distance and hours per gear item from synced activity data,
compares against user-configured retirement thresholds, and projects when each
item will reach its threshold based on recent usage rate.

Threshold format: --threshold run_shoes=400,bike_chain=3000
(values in miles). The gear_id prefix can be any substring of the gear's name.`,
		Example: strings.Trim(`
  strava-pp-cli gear status
  strava-pp-cli gear status --threshold run_shoes=400 --agent
  strava-pp-cli gear status --json --select gear_name,total_distance_mi,pct_threshold,est_retirement_date`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				sample := []gearStatusRow{
					{GearID: "g12345678", Name: "Nike Pegasus 40", Brand: "Nike",
						TotalDistanceMi: 312.4, TotalDistanceKm: 502.7, TotalHours: 52.3,
						ThresholdMi: 400, PctThreshold: 78.1,
						EstRetirementDate: "2026-08-15", Status: "ok"},
				}
				return printJSONFiltered(cmd.OutOrStdout(), sample, flags)
			}

			// Parse threshold map: "run_shoes=400,bike_chain=3000"
			thresholds := map[string]float64{}
			if threshold != "" {
				for _, part := range strings.Split(threshold, ",") {
					kv := strings.SplitN(part, "=", 2)
					if len(kv) != 2 {
						continue
					}
					var val float64
					if _, err := fmt.Sscanf(kv[1], "%f", &val); err == nil {
						thresholds[strings.TrimSpace(kv[0])] = val
					}
				}
			}

			if dbPath == "" {
				dbPath = defaultDBPath("strava-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\nRun 'strava-pp-cli sync' first", err)
			}
			defer db.Close()

			// Aggregate distance and time per gear_id from activities.
			// MIN(start_date) gives first use; MAX(start_date) gives last use —
			// lifespan = last - first is used for the daily rate projection.
			query := `SELECT
  json_extract(data, '$.gear_id') as gear_id,
  SUM(COALESCE(json_extract(data, '$.distance'), 0)) as total_distance_m,
  SUM(COALESCE(json_extract(data, '$.moving_time'), 0)) as total_seconds,
  COUNT(*) as activity_count,
  MIN(json_extract(data, '$.start_date')) as first_activity,
  MAX(json_extract(data, '$.start_date')) as last_activity
FROM resources
WHERE resource_type IN ('athlete-activities', 'activities')
  AND json_extract(data, '$.gear_id') IS NOT NULL
  AND json_extract(data, '$.gear_id') != ''
GROUP BY gear_id`

			rows, err := db.DB().QueryContext(cmd.Context(), query)
			if err != nil {
				return fmt.Errorf("querying gear totals: %w", err)
			}
			defer rows.Close()

			type gearAgg struct {
				gearID        string
				totalDistM    float64
				totalSeconds  float64
				activityCount int
				firstActivity string
				lastActivity  string
			}
			var aggs []gearAgg
			for rows.Next() {
				var gearID, firstActivity, lastActivity sql.NullString
				var distM, seconds sql.NullFloat64
				var count sql.NullInt64
				if err := rows.Scan(&gearID, &distM, &seconds, &count, &firstActivity, &lastActivity); err != nil {
					continue
				}
				if !gearID.Valid || gearID.String == "" {
					continue
				}
				aggs = append(aggs, gearAgg{
					gearID:        gearID.String,
					firstActivity: firstActivity.String,
					totalDistM:    distM.Float64,
					totalSeconds:  seconds.Float64,
					activityCount: int(count.Int64),
					lastActivity:  lastActivity.String,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading rows: %w", err)
			}

			// For each gear, fetch name from resources or API
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var result []gearStatusRow
			for i, agg := range aggs {
				// Sleep between gear metadata calls to avoid consuming the shared
				// Strava rate-limit budget (200 req/15 min). ~2 req/sec is gentle.
				if i > 0 {
					time.Sleep(500 * time.Millisecond)
				}
				// Fetch gear metadata from API
				gearData, getErr := c.Get(cmd.Context(), "/gear/"+agg.gearID, nil)
				gearName := agg.gearID
				gearBrand := ""
				gearModel := ""
				if getErr == nil {
					var gear map[string]any
					if json.Unmarshal(gearData, &gear) == nil {
						if n, _ := gear["name"].(string); n != "" {
							gearName = n
						}
						gearBrand, _ = gear["brand_name"].(string)
						gearModel, _ = gear["model_name"].(string)
					}
				}

				distMi := math.Round(agg.totalDistM/1609.34*10) / 10
				distKm := math.Round(agg.totalDistM/1000*10) / 10
				hours := math.Round(agg.totalSeconds/3600*10) / 10

				row := gearStatusRow{
					GearID:          agg.gearID,
					Name:            gearName,
					Brand:           gearBrand,
					Model:           gearModel,
					TotalDistanceMi: distMi,
					TotalDistanceKm: distKm,
					TotalHours:      hours,
					Status:          "ok",
				}

				// Match threshold by gear name substring
				for key, thresh := range thresholds {
					if strings.Contains(strings.ToLower(gearName), strings.ToLower(key)) {
						row.ThresholdMi = thresh
						row.PctThreshold = math.Round(distMi/thresh*1000) / 10
						if row.PctThreshold >= 90 {
							row.Status = "near_end"
						}
						if row.PctThreshold >= 100 {
							row.Status = "replace_now"
						}

						// Estimate retirement date from daily rate over the gear's active lifespan.
						if agg.firstActivity != "" && agg.lastActivity != "" {
							first, e1 := time.Parse(time.RFC3339, agg.firstActivity)
							last, e2 := time.Parse(time.RFC3339, agg.lastActivity)
							if e1 == nil && e2 == nil {
								lifespanDays := last.Sub(first).Hours() / 24
								if lifespanDays < 1 {
									lifespanDays = 1
								}
								miPerDay := distMi / lifespanDays
								if miPerDay > 0 {
									daysLeft := (thresh - distMi) / miPerDay
									retDate := time.Now().UTC().AddDate(0, 0, int(daysLeft))
									row.EstRetirementDate = retDate.Format("2006-01-02")
								}
							}
						}
						break
					}
				}

				result = append(result, row)
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&threshold, "threshold", "", "Retirement thresholds in miles (e.g. run_shoes=400,bike_chain=3000)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

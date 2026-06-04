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

type trainingLoadRow struct {
	Date  string  `json:"date"`
	TSS   float64 `json:"tss"`
	CTL   float64 `json:"ctl"`
	ATL   float64 `json:"atl"`
	TSB   float64 `json:"tsb"`
	Label string  `json:"label"`
}

func newTrainingLoadCmd(flags *rootFlags) *cobra.Command {
	var weeks int
	var ftp float64
	var activityType string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "load",
		Short: "See your CTL/ATL/TSB training load timeline",
		Long: `Computes Chronic Training Load (CTL), Acute Training Load (ATL), and
Training Stress Balance (TSB = CTL - ATL) from locally synced activity data.

TSS source (in priority order):
  1. Power-based TSS if --ftp is supplied and activity has weighted_average_watts
  2. Strava suffer_score (a.k.a. relative effort)
  3. Minutes of moving time as a proxy

Run 'strava-pp-cli sync' first to populate local data.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  strava-pp-cli training load --weeks 12
  strava-pp-cli training load --weeks 8 --ftp 285 --agent
  strava-pp-cli training load --weeks 12 --type Run --json --select date,ctl,tsb,label`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				sample := []trainingLoadRow{
					{Date: "2026-05-14", TSS: 72, CTL: 61.2, ATL: 68.5, TSB: -7.3, Label: "fatigued"},
					{Date: "2026-05-15", TSS: 0, CTL: 59.8, ATL: 58.7, TSB: 1.1, Label: "optimal"},
					{Date: "2026-05-16", TSS: 45, CTL: 59.0, ATL: 57.7, TSB: 1.3, Label: "optimal"},
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

			// Fetch activities from local store going back extra 42 days for CTL warmup
			cutoff := time.Now().UTC().AddDate(0, 0, -(weeks*7 + 42))

			query := `SELECT data FROM resources WHERE resource_type IN ('athlete-activities', 'activities')
AND COALESCE(json_extract(data, '$.start_date'), '') >= ?`
			qargs := []any{cutoff.Format("2006-01-02T15:04:05Z")}
			if activityType != "" {
				query += ` AND json_extract(data, '$.type') = ?`
				qargs = append(qargs, activityType)
			}
			query += ` ORDER BY json_extract(data, '$.start_date') ASC`

			rows, err := db.DB().QueryContext(cmd.Context(), query, qargs...)
			if err != nil {
				return fmt.Errorf("querying activities: %w", err)
			}
			defer rows.Close()

			// Aggregate TSS per day
			tssByDate := map[string]float64{}
			for rows.Next() {
				var dataRaw sql.NullString
				if err := rows.Scan(&dataRaw); err != nil || !dataRaw.Valid {
					continue
				}
				var act map[string]any
				if err := json.Unmarshal([]byte(dataRaw.String), &act); err != nil {
					continue
				}

				startRaw, _ := act["start_date"].(string)
				if startRaw == "" {
					startRaw, _ = act["start_date_local"].(string)
				}
				if startRaw == "" {
					continue
				}
				t, err := time.Parse(time.RFC3339, startRaw)
				if err != nil {
					continue
				}
				dk := t.UTC().Format("2006-01-02")

				var tss float64
				if ftp > 0 {
					if ww := jsonFloat(act, "weighted_average_watts"); ww > 0 {
						if mt := jsonFloat(act, "moving_time"); mt > 0 {
							intensity := ww / ftp
							tss = (mt / 3600) * intensity * intensity * 100
						}
					}
				}
				if tss == 0 {
					if ss := jsonFloat(act, "suffer_score"); ss > 0 {
						tss = ss
					} else if mt := jsonFloat(act, "moving_time"); mt > 0 {
						tss = mt / 60.0 // minutes as proxy
					}
				}
				if tss > 0 {
					tssByDate[dk] += tss
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading rows: %w", err)
			}

			// Warmup CTL/ATL over the 42-day period before the display window
			displayStart := time.Now().UTC().AddDate(0, 0, -(weeks * 7))
			ctl, atl := 0.0, 0.0
			for d := cutoff; d.Before(displayStart); d = d.AddDate(0, 0, 1) {
				dayTSS := tssByDate[d.Format("2006-01-02")]
				ctl += (dayTSS - ctl) / 42.0
				atl += (dayTSS - atl) / 7.0
			}

			// Build output rows
			var result []trainingLoadRow
			for d := displayStart; !d.After(time.Now().UTC()); d = d.AddDate(0, 0, 1) {
				dk := d.Format("2006-01-02")
				dayTSS := tssByDate[dk]
				ctl += (dayTSS - ctl) / 42.0
				atl += (dayTSS - atl) / 7.0
				tsb := ctl - atl

				label := "optimal"
				switch {
				case tsb < -30:
					label = "overreached"
				case tsb < -10:
					label = "fatigued"
				case tsb > 15:
					label = "fresh"
				}

				result = append(result, trainingLoadRow{
					Date:  dk,
					TSS:   math.Round(dayTSS),
					CTL:   math.Round(ctl*10) / 10,
					ATL:   math.Round(atl*10) / 10,
					TSB:   math.Round(tsb*10) / 10,
					Label: label,
				})
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().IntVar(&weeks, "weeks", 12, "Number of weeks to display")
	cmd.Flags().Float64Var(&ftp, "ftp", 0, "FTP in watts for power-based TSS (falls back to suffer_score if 0)")
	cmd.Flags().StringVar(&activityType, "type", "", "Filter by activity type (Run, Ride, Swim, etc.)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

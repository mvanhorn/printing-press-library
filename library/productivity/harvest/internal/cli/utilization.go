// Copyright 2026 dan-bronson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/harvest/internal/store"
)

type utilizationRow struct {
	UserID       string  `json:"user_id"`
	UserName     string  `json:"user_name"`
	ISOWeek      string  `json:"iso_week"`
	HoursTotal   float64 `json:"hours_total"`
	HoursBill    float64 `json:"hours_billable"`
	BillablePct  float64 `json:"billable_pct"`
	WeekCapacity float64 `json:"weekly_capacity,omitempty"`
}

func newUtilizationCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		userID string
		mine   bool
		weeks  int
	)

	cmd := &cobra.Command{
		Use:   "utilization",
		Short: "Per-user billable percentage by ISO week",
		Long: `Locally aggregates time_entries by user and ISO week, producing a rolling
billable% over the last N weeks. The agency-defining KPI that no Harvest CLI
exposes today.

LOCAL command: requires 'sync' with time-entries and users populated.`,
		Example: `  # Last 12 weeks for the team
  harvest-pp-cli utilization --weeks 12 --json --select user_name,iso_week,billable_pct

  # Just me
  harvest-pp-cli utilization --mine --weeks 8 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []utilizationRow{})
			}
			if weeks <= 0 || weeks > 104 {
				return fmt.Errorf("--weeks must be between 1 and 104")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("harvest-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'harvest-pp-cli sync' first.", err)
			}
			defer db.Close()

			if mine {
				userID, err = resolveCurrentUserID(cmd, flags, db)
				if err != nil {
					return err
				}
			}

			rows, err := computeUtilization(cmd, db, userID, weeks)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, rows)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&userID, "user", "", "Filter to one user (numeric ID)")
	cmd.Flags().BoolVar(&mine, "mine", false, "Filter to the authenticated user")
	cmd.Flags().IntVar(&weeks, "weeks", 12, "Number of trailing ISO weeks")
	return cmd
}

func computeUtilization(cmd *cobra.Command, db *store.Store, userID string, weeks int) ([]utilizationRow, error) {
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -7*weeks)
	from := start.Format("2006-01-02")

	sql := `SELECT
	  IFNULL(CAST(json_extract(data, '$.user.id') AS TEXT), '') AS uid,
	  IFNULL(json_extract(data, '$.user.name'), '') AS uname,
	  IFNULL(json_extract(data, '$.spent_date'), '') AS sd,
	  IFNULL(json_extract(data, '$.hours'), 0) AS h,
	  IFNULL(json_extract(data, '$.billable'), 0) AS b
	FROM time_entries
	WHERE json_extract(data, '$.spent_date') >= ?`
	params := []any{from}
	if userID != "" {
		sql += ` AND CAST(json_extract(data, '$.user.id') AS TEXT) = ?`
		params = append(params, userID)
	}

	rows, err := db.DB().QueryContext(cmd.Context(), sql, params...)
	if err != nil {
		return nil, fmt.Errorf("query time_entries: %w", err)
	}
	defer rows.Close()

	type key struct {
		uid, week string
	}
	agg := map[key]*utilizationRow{}
	for rows.Next() {
		var uid, uname, sd string
		var h float64
		var billable int64
		if err := rows.Scan(&uid, &uname, &sd, &h, &billable); err != nil {
			return nil, err
		}
		t, err := time.Parse("2006-01-02", sd)
		if err != nil {
			continue
		}
		year, w := t.ISOWeek()
		week := fmt.Sprintf("%04d-W%02d", year, w)
		k := key{uid, week}
		r := agg[k]
		if r == nil {
			r = &utilizationRow{UserID: uid, UserName: uname, ISOWeek: week}
			agg[k] = r
		}
		r.HoursTotal += h
		if billable != 0 {
			r.HoursBill += h
		}
	}

	out := make([]utilizationRow, 0)
	for _, r := range agg {
		if r.HoursTotal > 0 {
			r.BillablePct = math.Round((r.HoursBill/r.HoursTotal)*1000) / 10
		}
		r.HoursTotal = math.Round(r.HoursTotal*100) / 100
		r.HoursBill = math.Round(r.HoursBill*100) / 100
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UserID != out[j].UserID {
			return out[i].UserID < out[j].UserID
		}
		return out[i].ISOWeek < out[j].ISOWeek
	})
	return out, nil
}

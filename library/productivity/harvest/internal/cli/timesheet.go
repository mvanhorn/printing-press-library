// Copyright 2026 dan-bronson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/harvest/internal/store"
)

func newTimesheetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "timesheet",
		Short: "Inspect timesheet completeness and gaps",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
	}
	cmd.AddCommand(newTimesheetGapsCmd(flags))
	return cmd
}

type timesheetGap struct {
	UserID     string  `json:"user_id"`
	UserName   string  `json:"user_name"`
	Date       string  `json:"date"`
	DayOfWeek  string  `json:"day_of_week"`
	TotalHours float64 `json:"total_hours"`
	MinHours   float64 `json:"min_hours"`
	Deficit    float64 `json:"deficit"`
}

func newTimesheetGapsCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath   string
		userID   string
		mine     bool
		from     string
		to       string
		minHours float64
		weekdays bool
	)

	cmd := &cobra.Command{
		Use:   "gaps",
		Short: "List workdays where logged hours fall below a threshold",
		Long: `Joins users with time_entries to find dates where a user logged less than
the threshold (default 6 hours). Use this to chase missing time at week-end.

LOCAL command: requires 'sync --resource time-entries' first.`,
		Example: `  # Friday chase: who has gaps this week?
  harvest-pp-cli timesheet gaps --from 2026-05-11 --to 2026-05-15 --min-hours 6 --json

  # My own gaps for last week
  harvest-pp-cli timesheet gaps --mine --from 2026-05-04 --to 2026-05-08 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []timesheetGap{})
			}
			if from == "" || to == "" {
				return fmt.Errorf("--from and --to are required (YYYY-MM-DD)")
			}
			fromDate, err := time.Parse("2006-01-02", from)
			if err != nil {
				return fmt.Errorf("parse --from: %w", err)
			}
			toDate, err := time.Parse("2006-01-02", to)
			if err != nil {
				return fmt.Errorf("parse --to: %w", err)
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

			gaps, err := computeTimesheetGaps(cmd, db, userID, fromDate, toDate, minHours, weekdays)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, gaps)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&userID, "user", "", "Filter to one user (numeric ID)")
	cmd.Flags().BoolVar(&mine, "mine", false, "Filter to the authenticated user")
	cmd.Flags().StringVar(&from, "from", "", "First date (YYYY-MM-DD, required)")
	cmd.Flags().StringVar(&to, "to", "", "Last date (YYYY-MM-DD, required)")
	cmd.Flags().Float64Var(&minHours, "min-hours", 6.0, "Minimum hours per workday")
	cmd.Flags().BoolVar(&weekdays, "weekdays-only", true, "Skip Saturday/Sunday")
	return cmd
}

func computeTimesheetGaps(cmd *cobra.Command, db *store.Store, userID string, from, to time.Time, minHours float64, weekdaysOnly bool) ([]timesheetGap, error) {
	// Pull totals per (user, date) in the range.
	sql := `SELECT
	  json_extract(data, '$.user.id') AS uid,
	  json_extract(data, '$.user.name') AS uname,
	  json_extract(data, '$.spent_date') AS sd,
	  SUM(IFNULL(json_extract(data, '$.hours'), 0)) AS total
	FROM time_entries
	WHERE json_extract(data, '$.spent_date') >= ?
	  AND json_extract(data, '$.spent_date') <= ?`
	params := []any{from.Format("2006-01-02"), to.Format("2006-01-02")}
	if userID != "" {
		sql += ` AND CAST(json_extract(data, '$.user.id') AS TEXT) = ?`
		params = append(params, userID)
	}
	sql += ` GROUP BY uid, sd`

	rows, err := db.DB().QueryContext(cmd.Context(), sql, params...)
	if err != nil {
		return nil, fmt.Errorf("query time_entries: %w", err)
	}
	defer rows.Close()

	type key struct{ user, date string }
	totals := map[key]timesheetGap{}
	for rows.Next() {
		var uid, uname, sd *string
		var total *float64
		if err := rows.Scan(&uid, &uname, &sd, &total); err != nil {
			return nil, err
		}
		if uid == nil || sd == nil {
			continue
		}
		k := key{*uid, *sd}
		var hours float64
		if total != nil {
			hours = *total
		}
		name := ""
		if uname != nil {
			name = *uname
		}
		totals[k] = timesheetGap{
			UserID: *uid, UserName: name, Date: *sd, TotalHours: hours,
		}
	}

	// Enumerate every active user × every date in range, fill gaps.
	users, err := db.List("users", 0)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	type u struct {
		id     string
		name   string
		active bool
	}
	var userList []u
	for _, raw := range users {
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		var id string
		switch v := obj["id"].(type) {
		case float64:
			id = fmt.Sprintf("%d", int64(v))
		case string:
			id = v
		}
		if userID != "" && id != userID {
			continue
		}
		name, _ := obj["first_name"].(string)
		last, _ := obj["last_name"].(string)
		if last != "" {
			name += " " + last
		}
		active, _ := obj["is_active"].(bool)
		userList = append(userList, u{id, name, active})
	}

	out := make([]timesheetGap, 0)
	for _, usr := range userList {
		if !usr.active {
			continue
		}
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			if weekdaysOnly && (d.Weekday() == time.Saturday || d.Weekday() == time.Sunday) {
				continue
			}
			k := key{usr.id, d.Format("2006-01-02")}
			t, has := totals[k]
			if !has {
				t = timesheetGap{UserID: usr.id, UserName: usr.name, Date: d.Format("2006-01-02"), TotalHours: 0}
			}
			if t.TotalHours < minHours {
				t.MinHours = minHours
				t.Deficit = minHours - t.TotalHours
				t.DayOfWeek = d.Weekday().String()
				if t.UserName == "" {
					t.UserName = usr.name
				}
				out = append(out, t)
			}
		}
	}
	return out, nil
}

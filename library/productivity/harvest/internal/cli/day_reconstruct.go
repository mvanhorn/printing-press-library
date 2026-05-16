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

type dayReconstructStub struct {
	UserID      int64   `json:"user_id"`
	ProjectID   int64   `json:"project_id"`
	TaskID      int64   `json:"task_id"`
	SpentDate   string  `json:"spent_date"`
	Hours       float64 `json:"hours"`
	Notes       string  `json:"notes"`
	ProjectName string  `json:"project_name,omitempty"`
	TaskName    string  `json:"task_name,omitempty"`
}

func newDayCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "day",
		Short: "Day-level helpers (reconstruct missing hours)",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
	}
	cmd.AddCommand(newDayReconstructCmd(flags))
	return cmd
}

func newDayReconstructCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath      string
		userID      string
		mine        bool
		date        string
		targetHours float64
		lookback    int
	)

	cmd := &cobra.Command{
		Use:   "reconstruct",
		Short: "Emit JSON stub entries to fill a day to target hours",
		Long: `Reads the user's existing entries for the date, computes the deficit, and
generates stub entries proportional to that user's recent project/task mix.
Output is JSON ready to pipe into 'time-entries create --stdin' (after editing
notes).

LOCAL command: requires 'sync' with time-entries populated.`,
		Example: `  # Generate stubs filling Friday to 8h, based on last 14 days
  harvest-pp-cli day reconstruct --mine --date 2026-05-14 --target-hours 8 --json

  # Stub-then-create pipeline (dry-run)
  harvest-pp-cli day reconstruct --mine --date 2026-05-14 --target-hours 8 --json \
    | harvest-pp-cli time-entries create --stdin --dry-run`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []dayReconstructStub{})
			}
			if date == "" {
				return fmt.Errorf("--date is required (YYYY-MM-DD)")
			}
			if targetHours <= 0 {
				return fmt.Errorf("--target-hours must be > 0")
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
			if userID == "" {
				return fmt.Errorf("--user (numeric ID) or --mine is required")
			}

			stubs, err := buildDayReconstructStubs(cmd, db, userID, date, targetHours, lookback)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, stubs)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&userID, "user", "", "User numeric ID")
	cmd.Flags().BoolVar(&mine, "mine", false, "Use the authenticated user")
	cmd.Flags().StringVar(&date, "date", "", "Target date (YYYY-MM-DD, required)")
	cmd.Flags().Float64Var(&targetHours, "target-hours", 8.0, "Desired total hours for the day")
	cmd.Flags().IntVar(&lookback, "lookback-days", 14, "Days of recent history for project/task mix")
	return cmd
}

func buildDayReconstructStubs(cmd *cobra.Command, db *store.Store, userID, date string, target float64, lookback int) ([]dayReconstructStub, error) {
	uidInt, err := parseInt64(userID)
	if err != nil {
		return nil, fmt.Errorf("user ID must be numeric: %w", err)
	}

	// Step 1: hours already logged on the date.
	already := 0.0
	{
		row := db.DB().QueryRowContext(cmd.Context(), `SELECT IFNULL(SUM(IFNULL(json_extract(data, '$.hours'), 0)), 0)
		FROM time_entries
		WHERE CAST(json_extract(data, '$.user.id') AS TEXT) = ?
		  AND json_extract(data, '$.spent_date') = ?`, userID, date)
		if err := row.Scan(&already); err != nil {
			return nil, fmt.Errorf("aggregate existing: %w", err)
		}
	}
	deficit := target - already
	if deficit <= 0 {
		return []dayReconstructStub{}, nil
	}

	// Step 2: recent project/task mix.
	target_t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("parse --date: %w", err)
	}
	lookbackStart := target_t.AddDate(0, 0, -lookback).Format("2006-01-02")
	lookbackEnd := target_t.AddDate(0, 0, -1).Format("2006-01-02")

	mixRows, err := db.DB().QueryContext(cmd.Context(), `SELECT
		IFNULL(CAST(json_extract(data, '$.project.id') AS INTEGER), 0) AS pid,
		IFNULL(json_extract(data, '$.project.name'), '') AS pname,
		IFNULL(CAST(json_extract(data, '$.task.id') AS INTEGER), 0) AS tid,
		IFNULL(json_extract(data, '$.task.name'), '') AS tname,
		IFNULL(json_extract(data, '$.hours'), 0) AS h
	FROM time_entries
	WHERE CAST(json_extract(data, '$.user.id') AS TEXT) = ?
	  AND json_extract(data, '$.spent_date') >= ?
	  AND json_extract(data, '$.spent_date') <= ?`, userID, lookbackStart, lookbackEnd)
	if err != nil {
		return nil, fmt.Errorf("read recent mix: %w", err)
	}
	defer mixRows.Close()

	type slot struct {
		pid, tid     int64
		pname, tname string
		hours        float64
	}
	mix := map[string]*slot{}
	totalMix := 0.0
	for mixRows.Next() {
		var s slot
		if err := mixRows.Scan(&s.pid, &s.pname, &s.tid, &s.tname, &s.hours); err != nil {
			return nil, err
		}
		if s.pid == 0 || s.tid == 0 || s.hours <= 0 {
			continue
		}
		key := fmt.Sprintf("%d/%d", s.pid, s.tid)
		existing := mix[key]
		if existing == nil {
			existing = &slot{pid: s.pid, tid: s.tid, pname: s.pname, tname: s.tname}
			mix[key] = existing
		}
		existing.hours += s.hours
		totalMix += s.hours
	}
	if totalMix <= 0 {
		return nil, fmt.Errorf("no recent entries in the last %d days — provide --user with history, or sync first", lookback)
	}

	var stubs []dayReconstructStub
	for _, s := range mix {
		share := s.hours / totalMix * deficit
		share = math.Round(share*4) / 4 // snap to 15 minutes
		if share <= 0 {
			continue
		}
		stubs = append(stubs, dayReconstructStub{
			UserID:      uidInt,
			ProjectID:   s.pid,
			TaskID:      s.tid,
			SpentDate:   date,
			Hours:       share,
			ProjectName: s.pname,
			TaskName:    s.tname,
			Notes:       "(reconstructed — edit before committing)",
		})
	}
	sort.Slice(stubs, func(i, j int) bool { return stubs[i].Hours > stubs[j].Hours })

	// Adjust the largest stub so totals match deficit.
	currentTotal := 0.0
	for _, s := range stubs {
		currentTotal += s.Hours
	}
	if len(stubs) > 0 {
		stubs[0].Hours += math.Round((deficit-currentTotal)*4) / 4
		if stubs[0].Hours < 0 {
			stubs[0].Hours = 0
		}
	}
	return stubs, nil
}

func parseInt64(s string) (int64, error) {
	var v int64
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

// Copyright 2026 adam-birddog and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored transcendence command: capacity utilization.
//
// pp:data-source local
package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type utilizationRow struct {
	Key      string  `json:"key"`
	Classes  int     `json:"classes"`
	Capacity int     `json:"capacity"`
	Booked   int     `json:"booked"`
	FillRate float64 `json:"fill_rate"`
}

func newNovelUtilizationCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var groupBy string
	var start, end string
	var limit int

	cmd := &cobra.Command{
		Use:   "utilization",
		Short: "Compute fill ratio (booked vs capacity) per class, instructor, time-slot, or location over a date window.",
		Long: `Compute capacity utilization (booked vs max capacity) from synced classes.

Groups your locally synced classes by class, instructor, time-slot (hour of
day), or location and reports the fill ratio. Optionally bound the window with
--start / --end (ISO 8601). Results are ordered by fill rate ascending so
under-filled slots surface first.

Run 'sutra-fitness-pp-cli sync' first to populate classes.

Use this command for fill ratio across many classes or slots. For one class's
attendees use the reservations list; for teacher performance use 'scorecard'.`,
		Example:     "  sutra-fitness-pp-cli utilization --group-by instructor\n  sutra-fitness-pp-cli utilization --group-by timeslot --start 2026-06-01 --end 2026-06-30",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			groupBy = strings.ToLower(strings.TrimSpace(groupBy))
			if groupBy == "" {
				groupBy = "class"
			}

			var keyExpr string
			switch groupBy {
			case "class":
				keyExpr = "COALESCE(NULLIF(c.name,''), c.id)"
			case "instructor":
				keyExpr = "COALESCE(NULLIF(c.instructor_name,''),'(unassigned)')"
			case "timeslot":
				keyExpr = "COALESCE(substr(c.start_time,12,2)||':00', '(unknown)')"
			case "location":
				keyExpr = "COALESCE(NULLIF(l.name,''), c.location_id, '(unknown)')"
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--group-by must be one of: class, instructor, timeslot, location"))
			}

			db, ready, err := openAnalyticsStore(cmd.Context(), cmd, dbPath)
			if err != nil {
				return err
			}
			if !ready {
				return emitAnalytics(cmd, flags, []utilizationRow{})
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "classes") {
				hintIfStale(cmd, db, "classes", flags.maxAge)
			}

			where := []string{"COALESCE(c.canceled,0)=0", "COALESCE(c.deleted,0)=0"}
			var queryArgs []any
			if start != "" {
				where = append(where, "c.start_time >= ?")
				queryArgs = append(queryArgs, start)
			}
			if end != "" {
				where = append(where, "c.start_time <= ?")
				queryArgs = append(queryArgs, end)
			}

			query := fmt.Sprintf(`
				SELECT %s AS grp,
				       COUNT(*) AS classes,
				       COALESCE(SUM(c.max_capacity),0) AS capacity,
				       COALESCE(SUM(c.total_booked),0) AS booked
				FROM classes c
				LEFT JOIN locations l ON c.location_id = l.id
				WHERE %s
				GROUP BY grp`, keyExpr, strings.Join(where, " AND "))

			rows, err := db.DB().QueryContext(cmd.Context(), query, queryArgs...)
			if err != nil {
				return fmt.Errorf("querying classes: %w", err)
			}
			defer rows.Close()

			out := make([]utilizationRow, 0)
			for rows.Next() {
				var key string
				var classes, capacity, booked int
				if err := rows.Scan(&key, &classes, &capacity, &booked); err != nil {
					continue
				}
				out = append(out, utilizationRow{
					Key:      key,
					Classes:  classes,
					Capacity: capacity,
					Booked:   booked,
					FillRate: pct(booked, capacity),
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating classes: %w", err)
			}
			sort.SliceStable(out, func(i, j int) bool {
				if out[i].FillRate != out[j].FillRate {
					return out[i].FillRate < out[j].FillRate
				}
				return out[i].Key < out[j].Key
			})
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}

			return emitAnalytics(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&groupBy, "group-by", "class", "Group by: class, instructor, timeslot, or location")
	cmd.Flags().StringVar(&start, "start", "", "Only classes starting on/after this ISO 8601 time")
	cmd.Flags().StringVar(&end, "end", "", "Only classes starting on/before this ISO 8601 time")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")
	return cmd
}

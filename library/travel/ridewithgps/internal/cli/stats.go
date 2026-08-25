// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source local
//
// `stats` aggregates the locally synced trips mirror into time-windowed or
// activity-type training totals. Hand-authored transcendence command.
package cli

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/ridewithgps/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/ridewithgps/internal/store"
	"github.com/spf13/cobra"
)

type statsBucket struct {
	Period         string  `json:"period"`
	Rides          int     `json:"rides"`
	DistanceKM     float64 `json:"distance_km"`
	DistanceMI     float64 `json:"distance_mi"`
	ElevationGainM float64 `json:"elevation_gain_m"`
	MovingTime     string  `json:"moving_time"`
}

type statsView struct {
	GroupedBy string        `json:"grouped_by"`
	Periods   []statsBucket `json:"periods"`
	Note      string        `json:"note,omitempty"`
}

func newNovelStatsCmd(flags *rootFlags) *cobra.Command {
	var period string
	var by string
	var since string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Time-windowed distance, elevation, and moving-time totals with activity-type breakdowns over your logged rides.",
		Long: `Aggregate your locally synced trips into training totals.

Group by a time window (week, month, year) or by activity type. Reads the local
SQLite mirror only — run 'ridewithgps-pp-cli sync --resources trips' first.

For all-time best single efforts use 'records'; stats reports period sums.`,
		Example: strings.Trim(`
  ridewithgps-pp-cli stats --period month
  ridewithgps-pp-cli stats --period year --json
  ridewithgps-pp-cli stats --by activity-type --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would aggregate local trips into training stats")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			groupByActivity := strings.EqualFold(by, "activity-type") || strings.EqualFold(by, "activity_type")
			if !groupByActivity {
				switch period {
				case "week", "month", "year":
				case "":
					period = "month"
				default:
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--period must be one of week, month, year"))
				}
			}

			var sinceClause string
			var sinceArgs []any
			if since != "" {
				d, err := cliutil.ParseDurationLoose(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
				}
				sinceClause = " AND departed_at >= datetime('now', ?)"
				sinceArgs = append(sinceArgs, fmt.Sprintf("-%d seconds", int64(d.Seconds())))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("ridewithgps-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: ridewithgps-pp-cli sync --resources trips --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "trips", flags.maxAge)

			bucketExpr := "strftime('%Y-%m', departed_at)"
			label := "month"
			if groupByActivity {
				bucketExpr = "COALESCE(NULLIF(activity_type,''), 'unknown')"
				label = "activity_type"
			} else {
				switch period {
				case "week":
					bucketExpr = "strftime('%Y-W%W', departed_at)"
					label = "week"
				case "year":
					bucketExpr = "strftime('%Y', departed_at)"
					label = "year"
				}
			}

			query := fmt.Sprintf(`SELECT %s AS bucket,
					COUNT(*) AS rides,
					COALESCE(SUM(distance),0) AS dist,
					COALESCE(SUM(elevation_gain),0) AS elev,
					COALESCE(SUM(COALESCE(moving_time, duration)),0) AS moving
				FROM trips
				WHERE departed_at IS NOT NULL%s
				GROUP BY bucket
				ORDER BY bucket DESC`, bucketExpr, sinceClause)

			rows, err := db.DB().QueryContext(ctx, query, sinceArgs...)
			if err != nil {
				return fmt.Errorf("querying trips: %w", err)
			}
			defer rows.Close()

			view := statsView{GroupedBy: label, Periods: make([]statsBucket, 0)}
			for rows.Next() {
				var bucket sql.NullString
				var rides sql.NullInt64
				var dist, elev, moving sql.NullFloat64
				if err := rows.Scan(&bucket, &rides, &dist, &elev, &moving); err != nil {
					continue
				}
				view.Periods = append(view.Periods, statsBucket{
					Period:         bucket.String,
					Rides:          int(rides.Int64),
					DistanceKM:     roundN(metersToKM(dist.Float64), 1),
					DistanceMI:     roundN(metersToMiles(dist.Float64), 1),
					ElevationGainM: roundN(elev.Float64, 0),
					MovingTime:     secondsToHMS(moving.Float64),
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading trips: %w", err)
			}
			if len(view.Periods) == 0 {
				view.Note = "no trips found in the local mirror; run 'ridewithgps-pp-cli sync --resources trips'"
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			defer tw.Flush()
			header := strings.ToUpper(label[:1]) + label[1:]
			fmt.Fprintf(tw, "%s\tRides\tDistance\tElevation\tMoving time\n", header)
			fmt.Fprintf(tw, "------\t-----\t--------\t---------\t-----------\n")
			for _, p := range view.Periods {
				fmt.Fprintf(tw, "%s\t%d\t%.1f km / %.1f mi\t%.0f m\t%s\n",
					p.Period, p.Rides, p.DistanceKM, p.DistanceMI, p.ElevationGainM, p.MovingTime)
			}
			if view.Note != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&period, "period", "month", "Time bucket: week, month, or year")
	cmd.Flags().StringVar(&by, "by", "", "Group by 'activity-type' instead of a time window")
	cmd.Flags().StringVar(&since, "since", "", "Only include rides since this duration ago (e.g. 30d, 12w, 1y)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: local mirror)")
	return cmd
}

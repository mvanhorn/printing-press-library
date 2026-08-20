// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source local
//
// `records` surfaces all-time best efforts from the locally synced trips
// mirror. Hand-authored transcendence command.
package cli

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/ridewithgps/internal/store"
	"github.com/spf13/cobra"
)

type recordEntry struct {
	TripID     string  `json:"trip_id"`
	Name       string  `json:"name"`
	DepartedAt string  `json:"departed_at,omitempty"`
	Value      float64 `json:"value"`
	Unit       string  `json:"unit"`
	Display    string  `json:"display"`
}

type recordsView struct {
	Metric  string        `json:"metric"`
	Records []recordEntry `json:"records"`
	Note    string        `json:"note,omitempty"`
}

// metric -> (column, unit, formatter). Columns are promoted trip-summary fields.
var recordMetrics = map[string]struct {
	column string
	unit   string
	format func(v float64) string
}{
	"distance":  {"distance", "km", func(v float64) string { return fmt.Sprintf("%.1f km / %.1f mi", metersToKM(v), metersToMiles(v)) }},
	"elevation": {"elevation_gain", "m", func(v float64) string { return fmt.Sprintf("%.0f m / %.0f ft", v, v*3.28084) }},
	"speed":     {"max_speed", "km/h", func(v float64) string { return fmt.Sprintf("%.1f km/h", v) }},
	"power":     {"max_watts", "W", func(v float64) string { return fmt.Sprintf("%.0f W", v) }},
	"duration":  {"COALESCE(moving_time, duration)", "time", func(v float64) string { return secondsToHMS(v) }},
}

func newNovelRecordsCmd(flags *rootFlags) *cobra.Command {
	var metric string
	var top int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "records",
		Short: "All-time best efforts — longest ride, most climbing, fastest average speed, biggest power — from your trip metrics.",
		Long: `Surface your all-time best single rides from the local trips mirror.

Metrics: distance, elevation, speed (max), power (max watts), duration. Reads the
local SQLite mirror only — run 'ridewithgps-pp-cli sync --resources trips' first.

For period totals and averages use 'stats'; records returns top-N extremes.`,
		Example: strings.Trim(`
  ridewithgps-pp-cli records --metric distance --top 10
  ridewithgps-pp-cli records --metric elevation --json
  ridewithgps-pp-cli records --metric power --top 5 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute personal records from local trips")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if metric == "" {
				metric = "distance"
			}
			spec, ok := recordMetrics[strings.ToLower(metric)]
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--metric must be one of distance, elevation, speed, power, duration"))
			}
			if top <= 0 {
				top = 10
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

			query := fmt.Sprintf(`SELECT id, COALESCE(name,''), departed_at, %s AS metric
				FROM trips
				WHERE %s IS NOT NULL AND %s > 0
				ORDER BY metric DESC
				LIMIT ?`, spec.column, spec.column, spec.column)
			rows, err := db.DB().QueryContext(ctx, query, top)
			if err != nil {
				return fmt.Errorf("querying trips: %w", err)
			}
			defer rows.Close()

			view := recordsView{Metric: strings.ToLower(metric), Records: make([]recordEntry, 0)}
			for rows.Next() {
				var id, name sql.NullString
				var departed sql.NullString
				var val sql.NullFloat64
				if err := rows.Scan(&id, &name, &departed, &val); err != nil {
					continue
				}
				view.Records = append(view.Records, recordEntry{
					TripID:     id.String,
					Name:       name.String,
					DepartedAt: departed.String,
					Value:      roundN(val.Float64, 2),
					Unit:       spec.unit,
					Display:    spec.format(val.Float64),
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading trips: %w", err)
			}
			if len(view.Records) == 0 {
				view.Note = fmt.Sprintf("no trips with a %s value in the local mirror; run 'ridewithgps-pp-cli sync --resources trips'", view.Metric)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			defer tw.Flush()
			fmt.Fprintf(tw, "#\t%s\tRide\tDate\n", strings.ToUpper(view.Metric[:1])+view.Metric[1:])
			fmt.Fprintf(tw, "-\t-----\t----\t----\n")
			for i, r := range view.Records {
				date := r.DepartedAt
				if len(date) >= 10 {
					date = date[:10]
				}
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", i+1, r.Display, truncate(r.Name, 40), date)
			}
			if view.Note != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&metric, "metric", "distance", "Best-effort metric: distance, elevation, speed, power, duration")
	cmd.Flags().IntVar(&top, "top", 10, "Number of top records to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: local mirror)")
	return cmd
}

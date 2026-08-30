// Copyright 2026 Olivier and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: punctuality.
//
// Reads only the local observation store. Every competing Belgian rail tool is
// stateless, so "is the 08:12 always late" is unanswerable without this.

package cli

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/irail/internal/store"
)

type punctualityRow struct {
	Vehicle        string  `json:"vehicle"`
	VehicleShort   string  `json:"vehicle_short,omitempty"`
	Direction      string  `json:"direction,omitempty"`
	Samples        int     `json:"samples"`
	OnTime         int     `json:"on_time"`
	Late           int     `json:"late"`
	Canceled       int     `json:"canceled"`
	AvgDelaySec    int     `json:"avg_delay_seconds"`
	MaxDelaySec    int     `json:"max_delay_seconds"`
	OnTimePercent  float64 `json:"on_time_percent"`
	PlatformChange int     `json:"platform_changes"`
}

type punctualityView struct {
	Station        string           `json:"station,omitempty"`
	BoardType      string           `json:"board_type"`
	Direction      string           `json:"direction,omitempty"`
	WindowDays     int              `json:"window_days"`
	LateThresholdS int              `json:"late_threshold_seconds"`
	Samples        int              `json:"samples"`
	Trains         []punctualityRow `json:"trains"`
	Note           string           `json:"note,omitempty"`
}

func newNovelPunctualityCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string
	var flagStation string
	var flagVehicle string
	var flagBoardType string
	var flagDays int
	var flagLateAfter int
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "punctuality",
		Short: "Historical delay statistics for trains you have observed",
		Long: "Shows how reliable a train or route has actually been, from delay observations\n" +
			"recorded on this machine by 'observe'.\n\n" +
			"Use this command for questions about the past, such as chronic lateness. Do NOT\n" +
			"use it for today's live delay; use 'board' or 'route' for that.\n\n" +
			"Departure, arrival and route captures are separate histories and are never\n" +
			"aggregated together; pick one with --board-type. Route captures recorded by\n" +
			"'observe' with --from and --to need --board-type route, and must name the\n" +
			"destination with --to so several routes out of one origin are not averaged\n" +
			"into a figure that describes no single journey.\n\n" +
			"This command never calls the API. It is empty until 'observe' has run.",
		Example: `  irail-pp-cli punctuality --station Brussels-Central
  irail-pp-cli punctuality --station Brussels-Central --board-type arrival
  irail-pp-cli punctuality --from Ghent-Sint-Pieters --to Brussels-Central --board-type route --agent
  irail-pp-cli punctuality --station Leuven --days 30 --late-after 120`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would summarise locally recorded delay observations")
				return nil
			}
			boardType, err := normalizeBoardType(flagBoardType)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			// A station is usually the origin of several observed routes.
			// Averaging them together produces a reliability figure for no
			// single journey, so a route summary has to name its destination.
			if boardType == "route" && flagTo == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--to is required with --board-type route"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("irail-pp-cli")
			}
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"no local mirror at %s\nrun: irail-pp-cli observe --station <station> --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			station := resolveStationName(flagStation)
			if flagFrom != "" {
				station = resolveStationName(flagFrom)
			}
			direction := resolveStationName(flagTo)

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer func() { _ = db.Close() }()
			if err := db.EnsureIrailSchema(ctx); err != nil {
				return err
			}

			since := time.Now().AddDate(0, 0, -flagDays).Unix()
			query := `
				SELECT vehicle,
				       COALESCE(vehicle_short, ''),
				       COALESCE(direction, ''),
				       delay_seconds,
				       canceled,
				       platform_normal
				FROM irail_observations
				WHERE observed_at >= ? AND board_type = ?`
			// board_type is always filtered: departure, arrival and route rows
			// describe different things, so averaging them together produces a
			// statistic that belongs to no real journey.
			argv := []any{since, boardType}
			if station != "" {
				query += ` AND station = ?`
				argv = append(argv, station)
			}
			if direction != "" {
				query += ` AND direction = ?`
				argv = append(argv, direction)
			}
			if flagVehicle != "" {
				query += ` AND (vehicle = ? OR vehicle_short = ?)`
				argv = append(argv, flagVehicle, flagVehicle)
			}

			rows, err := db.DB().QueryContext(ctx, query, argv...)
			if err != nil {
				return fmt.Errorf("querying observations: %w", err)
			}

			agg := map[string]*punctualityRow{}
			total := 0
			for rows.Next() {
				var vehicle string
				var short, dir sql.NullString
				var delay, canceled, platformNormal sql.NullInt64
				if err := rows.Scan(&vehicle, &short, &dir, &delay, &canceled, &platformNormal); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning observation: %w", err)
				}
				total++
				r, ok := agg[vehicle]
				if !ok {
					r = &punctualityRow{Vehicle: vehicle, VehicleShort: short.String, Direction: dir.String}
					agg[vehicle] = r
				}
				r.Samples++
				d := int(delay.Int64)
				if canceled.Int64 == 1 {
					r.Canceled++
				} else if d >= flagLateAfter {
					r.Late++
				} else {
					r.OnTime++
				}
				r.AvgDelaySec += d
				if d > r.MaxDelaySec {
					r.MaxDelaySec = d
				}
				// platform_normal is stored 1 = usual platform, 0 = changed.
				if platformNormal.Valid && platformNormal.Int64 == 0 {
					r.PlatformChange++
				}
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating observations: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("closing observation rows: %w", err)
			}

			view := punctualityView{
				Station:        station,
				BoardType:      boardType,
				Direction:      direction,
				WindowDays:     flagDays,
				LateThresholdS: flagLateAfter,
				Samples:        total,
				Trains:         make([]punctualityRow, 0, len(agg)),
			}
			for _, r := range agg {
				if r.Samples > 0 {
					r.AvgDelaySec /= r.Samples
					r.OnTimePercent = float64(r.OnTime) / float64(r.Samples) * 100
				}
				view.Trains = append(view.Trains, *r)
			}
			// Worst first: most cancellations, then highest average delay.
			sort.Slice(view.Trains, func(i, j int) bool {
				a, b := view.Trains[i], view.Trains[j]
				if a.Canceled != b.Canceled {
					return a.Canceled > b.Canceled
				}
				if a.AvgDelaySec != b.AvgDelaySec {
					return a.AvgDelaySec > b.AvgDelaySec
				}
				return a.Vehicle < b.Vehicle
			})
			if flagLimit > 0 && len(view.Trains) > flagLimit {
				view.Trains = view.Trains[:flagLimit]
			}

			if total == 0 {
				view.Note = fmt.Sprintf(
					"no %s observations recorded in the last %d day(s) for this filter; run 'irail-pp-cli observe' first, then check back",
					boardType, flagDays)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if total == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-22s %7s %7s %8s %8s %7s\n",
				"TRAIN", "DIRECTION", "SAMPLES", "ONTIME%", "AVGDELAY", "MAXDELAY", "CANCEL")
			for _, r := range view.Trains {
				name := r.VehicleShort
				if name == "" {
					name = r.Vehicle
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-22s %7d %6.0f%% %8s %8s %7d\n",
					name, truncate(r.Direction, 22), r.Samples, r.OnTimePercent,
					humanDuration(r.AvgDelaySec), humanDuration(r.MaxDelaySec), r.Canceled)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d observation(s) over %d day(s); late means %s or more\n",
				total, flagDays, humanDuration(flagLateAfter))
			return nil
		},
	}

	cmd.Flags().StringVar(&flagFrom, "from", "", "Origin station (alias for --station when analysing a route)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Destination: names the route with --board-type route, otherwise filters departures or arrivals by headsign")
	cmd.Flags().StringVar(&flagStation, "station", "", "Station whose recorded observations to summarise")
	cmd.Flags().StringVar(&flagVehicle, "vehicle", "", "Restrict to one train, e.g. IC 2843 or BE.NMBS.IC2843")
	cmd.Flags().StringVar(&flagBoardType, "board-type", "departure", "Which recorded history to summarise: departure, arrival or route")
	cmd.Flags().IntVar(&flagDays, "days", 30, "How many days of history to include")
	cmd.Flags().IntVar(&flagLateAfter, "late-after", 60, "Seconds of delay before a train counts as late")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum trains to report")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

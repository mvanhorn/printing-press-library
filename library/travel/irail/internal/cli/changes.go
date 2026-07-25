// Copyright 2026 Olivier and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: changes.
//
// Diffs the two most recent observation rounds for a station. Surfaces the
// undocumented platforminfo.normal flag, which is the only machine-readable
// signal that a platform changed from its usual assignment.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/irail/internal/store"
)

type changeRow struct {
	Vehicle      string `json:"vehicle"`
	VehicleShort string `json:"vehicle_short,omitempty"`
	Direction    string `json:"direction,omitempty"`
	ScheduledAt  string `json:"scheduled_at"`
	Kind         string `json:"kind"`
	Detail       string `json:"detail"`
	FromValue    string `json:"from,omitempty"`
	ToValue      string `json:"to,omitempty"`
}

type changesView struct {
	Station    string      `json:"station"`
	PreviousAt string      `json:"previous_observation,omitempty"`
	LatestAt   string      `json:"latest_observation,omitempty"`
	Changes    []changeRow `json:"changes"`
	Note       string      `json:"note,omitempty"`
}

// observationSnapshot is one recorded board row, keyed for diffing.
type observationSnapshot struct {
	vehicle      string
	vehicleShort string
	direction    string
	scheduledAt  int64
	delay        int
	canceled     bool
	platform     string
	normal       bool
}

func newNovelChangesCmd(flags *rootFlags) *cobra.Command {
	var flagStation string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "changes",
		Short: "New delays, cancellations and platform changes since your last observation",
		Long: "Diffs the two most recent observation rounds recorded by 'observe'.\n\n" +
			"Use this command for deltas since you last looked. Do NOT use it for a full\n" +
			"current board; use 'board' for that.\n\n" +
			"This command never calls the API. It needs at least two 'observe' rounds.",
		Example: `  irail-pp-cli changes --station Brussels-Central
  irail-pp-cli changes --station Brussels-Central --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would diff the two most recent local observation rounds")
				return nil
			}
			if flagStation == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--station is required"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("irail-pp-cli")
			}
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"no local mirror at %s\nrun: irail-pp-cli observe --station %s --db %s\n",
					dbPath, flagStation, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			station := resolveStationName(flagStation)
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer func() { _ = db.Close() }()
			if err := db.EnsureIrailSchema(ctx); err != nil {
				return err
			}

			// Drain the two most recent observation instants first; SQLite uses a
			// single connection, so no follow-up query may run while rows is open.
			stamps, err := recentObservationStamps(ctx, db, station, 2)
			if err != nil {
				return err
			}

			view := changesView{Station: station, Changes: make([]changeRow, 0)}
			if len(stamps) < 2 {
				view.Note = fmt.Sprintf(
					"only %d observation round(s) recorded for %s; run 'irail-pp-cli observe --station %s' again to compare",
					len(stamps), station, station)
				if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}

			latestAt, prevAt := stamps[0], stamps[1]
			view.LatestAt = time.Unix(latestAt, 0).In(belgiumTZ()).Format(time.RFC3339)
			view.PreviousAt = time.Unix(prevAt, 0).In(belgiumTZ()).Format(time.RFC3339)

			latest, err := observationsAt(ctx, db, station, latestAt)
			if err != nil {
				return err
			}
			previous, err := observationsAt(ctx, db, station, prevAt)
			if err != nil {
				return err
			}

			view.Changes = diffObservations(previous, latest)
			if len(view.Changes) == 0 {
				view.Note = "nothing changed between the last two observation rounds"
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Changes) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			for _, ch := range view.Changes {
				name := ch.VehicleShort
				if name == "" {
					name = ch.Vehicle
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-18s %-16s %s\n", name, ch.Kind, clockOf(ch.ScheduledAt), ch.Detail)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d change(s) between %s and %s\n",
				len(view.Changes), clockOf(view.PreviousAt), clockOf(view.LatestAt))
			return nil
		},
	}

	cmd.Flags().StringVar(&flagStation, "station", "", "Station to diff (name, telegraph code or id)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// recentObservationStamps returns the most recent distinct observation
// instants for a station, newest first.
func recentObservationStamps(ctx context.Context, db *store.Store, station string, n int) ([]int64, error) {
	rows, err := db.DB().QueryContext(ctx,
		`SELECT DISTINCT observed_at FROM irail_observations
		 WHERE station = ? ORDER BY observed_at DESC LIMIT ?`, station, n)
	if err != nil {
		return nil, fmt.Errorf("querying observation rounds: %w", err)
	}
	out := make([]int64, 0, n)
	for rows.Next() {
		var ts int64
		if err := rows.Scan(&ts); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning observation round: %w", err)
		}
		out = append(out, ts)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating observation rounds: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing observation rounds: %w", err)
	}
	return out, nil
}

// observationsAt loads one observation round keyed by vehicle + scheduled time.
func observationsAt(ctx context.Context, db *store.Store, station string, at int64) (map[string]observationSnapshot, error) {
	rows, err := db.DB().QueryContext(ctx,
		`SELECT vehicle, COALESCE(vehicle_short,''), COALESCE(direction,''),
		        scheduled_at, delay_seconds, canceled, COALESCE(platform,''), platform_normal
		 FROM irail_observations WHERE station = ? AND observed_at = ?`, station, at)
	if err != nil {
		return nil, fmt.Errorf("querying observations: %w", err)
	}
	out := map[string]observationSnapshot{}
	for rows.Next() {
		var s observationSnapshot
		var short, dir, platform sql.NullString
		var scheduled, delay, canceled, normal sql.NullInt64
		if err := rows.Scan(&s.vehicle, &short, &dir, &scheduled, &delay, &canceled, &platform, &normal); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning observation: %w", err)
		}
		s.vehicleShort = short.String
		s.direction = dir.String
		s.scheduledAt = scheduled.Int64
		s.delay = int(delay.Int64)
		s.canceled = canceled.Int64 == 1
		s.platform = platform.String
		s.normal = !normal.Valid || normal.Int64 == 1
		out[fmt.Sprintf("%s|%d", s.vehicle, s.scheduledAt)] = s
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating observations: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing observations: %w", err)
	}
	return out, nil
}

// diffObservations reports what changed between two observation rounds.
// Exported behaviour is pure so it can be unit tested without a database.
func diffObservations(previous, latest map[string]observationSnapshot) []changeRow {
	out := make([]changeRow, 0)
	for key, now := range latest {
		before, existed := previous[key]
		sched := time.Unix(now.scheduledAt, 0).In(belgiumTZ()).Format(time.RFC3339)
		base := changeRow{
			Vehicle:      now.vehicle,
			VehicleShort: now.vehicleShort,
			Direction:    now.direction,
			ScheduledAt:  sched,
		}

		if !existed {
			base.Kind = "new-departure"
			base.Detail = "appeared on the board since the last check"
			out = append(out, base)
			continue
		}
		if now.canceled && !before.canceled {
			c := base
			c.Kind = "canceled"
			c.Detail = "this train is now cancelled"
			out = append(out, c)
		}
		if now.delay > before.delay {
			c := base
			c.Kind = "delay-increased"
			c.Detail = fmt.Sprintf("delay rose from %s to %s", humanDuration(before.delay), humanDuration(now.delay))
			c.FromValue = humanDuration(before.delay)
			c.ToValue = humanDuration(now.delay)
			out = append(out, c)
		}
		if now.platform != before.platform && now.platform != "" && before.platform != "" {
			c := base
			c.Kind = "platform-changed"
			c.Detail = fmt.Sprintf("platform moved from %s to %s", before.platform, now.platform)
			c.FromValue = before.platform
			c.ToValue = now.platform
			out = append(out, c)
		} else if !now.normal && before.normal {
			// platforminfo.normal flipped to 0: iRail is signalling the platform
			// is no longer the usual one even though the number may be unchanged.
			c := base
			c.Kind = "platform-not-normal"
			c.Detail = fmt.Sprintf("platform %s is not the usual one for this train", now.platform)
			out = append(out, c)
		}
	}
	sortChangeRows(out)
	return out
}

func sortChangeRows(rows []changeRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j].ScheduledAt < rows[j-1].ScheduledAt; j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

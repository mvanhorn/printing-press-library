// Copyright 2026 Olivier and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: changes.
//
// Diffs the two most recent observation rounds for a station. Surfaces the
// undocumented platforminfo.normal flag, which is the only machine-readable
// signal that a platform changed from its usual assignment.

package cli

import (
	"context"
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
	BoardType  string      `json:"board_type"`
	Direction  string      `json:"direction,omitempty"`
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
	var flagBoardType string
	var flagTo string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "changes",
		Short: "New delays, cancellations and platform changes since your last observation",
		Long: "Diffs the two most recent observation rounds recorded by 'observe'.\n\n" +
			"Use this command for deltas since you last looked. Do NOT use it for a full\n" +
			"current board; use 'board' for that.\n\n" +
			"Departure, arrival and route captures are separate histories, so the diff is\n" +
			"scoped to one --board-type at a time. Route captures also need --to, because a\n" +
			"station can be the origin of several observed routes.\n\n" +
			"This command never calls the API. It needs at least two 'observe' rounds.",
		Example: `  irail-pp-cli changes --station Brussels-Central
  irail-pp-cli changes --station Brussels-Central --board-type arrival
  irail-pp-cli changes --station Ghent-Sint-Pieters --board-type route --to Brussels-Central
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
			boardType, err := normalizeBoardType(flagBoardType)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if boardType == "route" && flagTo == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--to is required with --board-type route"))
			}
			if boardType != "route" && flagTo != "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--to only applies to --board-type route"))
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
			direction := ""
			if boardType == "route" {
				direction = resolveStationName(flagTo)
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer func() { _ = db.Close() }()
			if err := db.EnsureIrailSchema(ctx); err != nil {
				return err
			}

			// Drain the two most recent rounds first; SQLite uses a single
			// connection, so no follow-up query may run while rows is open.
			rounds, err := db.RecentObservationRounds(ctx, station, boardType, direction, 2)
			if err != nil {
				return err
			}

			view := changesView{
				Station:   station,
				BoardType: boardType,
				Direction: direction,
				Changes:   make([]changeRow, 0),
			}
			if len(rounds) < 2 {
				view.Note = fmt.Sprintf(
					"only %d %s observation round(s) recorded for %s; run 'irail-pp-cli observe --station %s' again to compare",
					len(rounds), boardType, station, station)
				if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}

			view.LatestAt = time.Unix(rounds[0].ObservedAt, 0).In(belgiumTZ()).Format(time.RFC3339)
			view.PreviousAt = time.Unix(rounds[1].ObservedAt, 0).In(belgiumTZ()).Format(time.RFC3339)

			latest, err := snapshotsInRound(ctx, db, rounds[0].ID)
			if err != nil {
				return err
			}
			previous, err := snapshotsInRound(ctx, db, rounds[1].ID)
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
	cmd.Flags().StringVar(&flagBoardType, "board-type", "departure", "Which recorded history to diff: departure, arrival or route")
	cmd.Flags().StringVar(&flagTo, "to", "", "Destination of the recorded route, required with --board-type route")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// normalizeBoardType validates the recorded history a command should read.
// Departure, arrival and route captures of the same station are distinct
// histories; mixing them reports every train in one as appearing or vanishing
// from the other.
func normalizeBoardType(v string) (string, error) {
	switch v {
	case "", "departure":
		return "departure", nil
	case "arrival", "route":
		return v, nil
	default:
		return "", fmt.Errorf("unknown --board-type %q: want departure, arrival or route", v)
	}
}

// snapshotsInRound loads one capture round keyed by vehicle + scheduled time.
// The round id already pins station, board type and route destination, so the
// key does not need to repeat them.
func snapshotsInRound(ctx context.Context, db *store.Store, roundID string) (map[string]observationSnapshot, error) {
	obs, err := db.ObservationsInRound(ctx, roundID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]observationSnapshot, len(obs))
	for _, o := range obs {
		out[fmt.Sprintf("%s|%d", o.Vehicle, o.ScheduledAt)] = observationSnapshot{
			vehicle:      o.Vehicle,
			vehicleShort: o.VehicleShort,
			direction:    o.Direction,
			scheduledAt:  o.ScheduledAt,
			delay:        o.DelaySeconds,
			canceled:     o.Canceled,
			platform:     o.Platform,
			normal:       o.PlatformNormal,
		}
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

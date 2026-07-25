// Copyright 2026 Olivier and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: observe.
//
// Records what the live board says right now into local SQLite. The iRail API
// has no historical endpoint, so this is the only source of delay history.

package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/irail/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/irail/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/irail/internal/store"
)

type observeResult struct {
	Station   string `json:"station"`
	BoardType string `json:"board_type"`
	Direction string `json:"direction,omitempty"`
	Seen      int    `json:"seen"`
	Recorded  int    `json:"recorded"`
	Error     string `json:"error,omitempty"`
}

type observeView struct {
	ObservedAt        string          `json:"observed_at"`
	Targets           []observeResult `json:"targets"`
	TotalRecorded     int             `json:"total_recorded"`
	TotalObservations int             `json:"total_observations_stored"`
	FetchFailures     []observeResult `json:"fetch_failures,omitempty"`
	Note              string          `json:"note,omitempty"`
}

func newNovelObserveCmd(flags *rootFlags) *cobra.Command {
	var flagStation string
	var flagFrom string
	var flagTo string
	var flagBoardType string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "observe",
		Short: "Record the current board into local SQLite so delay history accumulates",
		Long: "Records what the live board says right now into local SQLite.\n\n" +
			"Use this command to build history; run it on a schedule. Do NOT use it to read\n" +
			"history back; use 'punctuality' or 'changes' for that.\n\n" +
			"With no flags it observes every saved route (see 'saved add').",
		Example: `  irail-pp-cli observe --station Brussels-Central
  irail-pp-cli observe --from Ghent-Sint-Pieters --to Brussels-Central
  irail-pp-cli observe --json`,
		// Reads the API and writes only the local cache; no external state changes.
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 && !hasSavedRoutes(cmd.Context(), dbPath) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would record the current board into the local store")
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("irail-pp-cli")
			}

			type target struct {
				station   string
				to        string
				boardType string
			}
			var targets []target

			switch {
			case flagFrom != "" && flagTo != "":
				targets = append(targets, target{
					station:   resolveStationName(flagFrom),
					to:        resolveStationName(flagTo),
					boardType: "route",
				})
			case flagStation != "":
				targets = append(targets, target{
					station:   resolveStationName(flagStation),
					boardType: flagBoardType,
				})
			default:
				saved, err := loadSavedRoutes(ctx, dbPath)
				if err != nil {
					return err
				}
				for _, r := range saved {
					if r.ToStation != "" {
						targets = append(targets, target{station: r.FromStation, to: r.ToStation, boardType: "route"})
						continue
					}
					targets = append(targets, target{station: r.FromStation, boardType: flagBoardType})
				}
			}

			if len(targets) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--station or --from with --to is required when no saved routes exist"))
			}
			// Live dogfood runs against the real API under a flat per-command
			// timeout; observing one target proves the path without risking it.
			if cliutil.IsDogfoodEnv() && len(targets) > 1 {
				targets = targets[:1]
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer func() { _ = db.Close() }()
			if err := db.EnsureIrailSchema(ctx); err != nil {
				return err
			}

			observedAt := time.Now().Unix()
			view := observeView{
				ObservedAt:    time.Unix(observedAt, 0).In(belgiumTZ()).Format(time.RFC3339),
				Targets:       make([]observeResult, 0, len(targets)),
				FetchFailures: make([]observeResult, 0),
			}

			for _, t := range targets {
				var obs []store.Observation
				var fetchErr error
				if t.boardType == "route" {
					obs, fetchErr = observeRoute(ctx, c, t.station, t.to, observedAt)
				} else {
					obs, fetchErr = observeBoard(ctx, c, t.station, t.boardType, observedAt)
				}
				res := observeResult{Station: t.station, BoardType: t.boardType, Direction: t.to, Seen: len(obs)}
				if fetchErr != nil {
					// A failed fetch is not an empty board. Keep it out of the
					// recorded totals and surface it explicitly.
					res.Error = fetchErr.Error()
					view.FetchFailures = append(view.FetchFailures, res)
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: observing %s failed: %v\n", t.station, fetchErr)
					continue
				}
				inserted, err := db.InsertObservations(ctx, obs)
				if err != nil {
					return err
				}
				res.Recorded = inserted
				view.TotalRecorded += inserted
				view.Targets = append(view.Targets, res)
			}

			total, err := db.ObservationCount(ctx)
			if err != nil {
				return err
			}
			view.TotalObservations = total
			if total < 2 {
				view.Note = "run observe again later; punctuality and changes need at least two observation rounds to compare"
			}
			if len(view.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: %d of %d target(s) failed; totals cover only the remaining %d\n",
					len(view.FetchFailures), len(targets), len(view.Targets))
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			for _, t := range view.Targets {
				label := t.Station
				if t.Direction != "" {
					label = t.Station + " -> " + t.Direction
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-46s seen %3d  recorded %3d\n", label, t.Seen, t.Recorded)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d new observation(s); %d stored in total\n",
				view.TotalRecorded, view.TotalObservations)
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagStation, "station", "", "Station whose board to record (name, telegraph code or id)")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Origin station, to record a route's departure leg instead of a board")
	cmd.Flags().StringVar(&flagTo, "to", "", "Destination station, used with --from")
	cmd.Flags().StringVar(&flagBoardType, "board-type", "departure", "Record the departures or arrivals board")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// observeBoard fetches a liveboard and converts it to typed observations.
// pp:client-call
func observeBoard(ctx context.Context, c *client.Client, station, boardType string, observedAt int64) ([]store.Observation, error) {
	if boardType == "" {
		boardType = "departure"
	}
	env, err := irailFetch(ctx, c, "/v1/liveboard?format=json", map[string]string{
		"station": station,
		"arrdep":  boardType,
		"lang":    "en",
	})
	if err != nil {
		return nil, err
	}

	key, inner := "departures", "departure"
	if boardType == "arrival" {
		key, inner = "arrivals", "arrival"
	}
	rows := sliceAt(env, key, inner)
	out := make([]store.Observation, 0, len(rows))
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, observationFromRow(row, station, boardType, "", observedAt))
	}
	return out, nil
}

// observeRoute records the departure leg of each connection on a route.
// pp:client-call
func observeRoute(ctx context.Context, c *client.Client, from, to string, observedAt int64) ([]store.Observation, error) {
	env, err := irailFetch(ctx, c, "/v1/connections?format=json", map[string]string{
		"from": from,
		"to":   to,
		"lang": "en",
	})
	if err != nil {
		return nil, err
	}
	conns := sliceAt(env, "connection")
	out := make([]store.Observation, 0, len(conns))
	for _, raw := range conns {
		conn, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		dep := mapAt(conn, "departure")
		if dep == nil {
			continue
		}
		out = append(out, observationFromRow(dep, from, "route", to, observedAt))
	}
	return out, nil
}

// observationFromRow coerces one iRail board row into a typed observation.
// iRail returns every scalar as a JSON string, so nothing here is a direct cast.
func observationFromRow(row map[string]any, station, boardType, direction string, observedAt int64) store.Observation {
	o := store.Observation{
		ObservedAt:          observedAt,
		Station:             station,
		BoardType:           boardType,
		Vehicle:             irailString(row["vehicle"]),
		ScheduledAt:         irailInt64(row["time"]),
		DelaySeconds:        irailInt(row["delay"]),
		Canceled:            irailBool(row["canceled"]),
		Left:                irailBool(row["left"]),
		Platform:            irailString(row["platform"]),
		PlatformNormal:      true,
		DepartureConnection: irailString(row["departureConnection"]),
		Direction:           direction,
	}
	if vi := mapAt(row, "vehicleinfo"); vi != nil {
		o.VehicleShort = irailString(vi["shortname"])
	}
	if pi := mapAt(row, "platforminfo"); pi != nil {
		// platforminfo.normal == "0" means the platform differs from the usual
		// one. It is absent from the published docs but present in live data.
		if _, present := pi["normal"]; present {
			o.PlatformNormal = irailBool(pi["normal"])
		}
		if name := irailString(pi["name"]); name != "" {
			o.Platform = name
		}
	}
	if occ := mapAt(row, "occupancy"); occ != nil {
		o.Occupancy = irailString(occ["name"])
	}
	if o.Direction == "" {
		if dir := mapAt(row, "direction"); dir != nil {
			o.Direction = irailString(dir["name"])
		}
	}
	return o
}

// loadSavedRoutes reads saved shortcuts, returning nothing when the local store
// does not exist yet.
func loadSavedRoutes(ctx context.Context, dbPath string) ([]store.SavedRoute, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("irail-pp-cli")
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, nil
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = db.Close() }()
	if err := db.EnsureIrailSchema(ctx); err != nil {
		return nil, err
	}
	return db.ListSavedRoutes(ctx)
}

// hasSavedRoutes reports whether a bare `observe` has anything to act on.
func hasSavedRoutes(ctx context.Context, dbPath string) bool {
	routes, err := loadSavedRoutes(ctx, dbPath)
	return err == nil && len(routes) > 0
}

// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
//
// `gear` computes per-bike mileage by fan-out over trip details (the gear
// object is only on the full trip, not the summary) joined with locally synced
// trip distances. Hand-authored transcendence command.
package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/travel/ridewithgps/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/ridewithgps/internal/store"
)

type gearTotal struct {
	GearID     string  `json:"gear_id"`
	Name       string  `json:"name"`
	Rides      int     `json:"rides"`
	DistanceKM float64 `json:"distance_km"`
	DistanceMI float64 `json:"distance_mi"`
	Due        bool    `json:"due,omitempty"`
	distanceM  float64
}

type gearFetchFailure struct {
	TripID string `json:"trip_id"`
	Error  string `json:"error"`
}

type gearView struct {
	ScannedTrips   int                `json:"scanned_trips"`
	DueThresholdKM float64            `json:"due_threshold_km,omitempty"`
	Gear           []gearTotal        `json:"gear"`
	FetchFailures  []gearFetchFailure `json:"fetch_failures"`
	Note           string             `json:"note,omitempty"`
}

func newNovelGearCmd(flags *rootFlags) *cobra.Command {
	var bike string
	var dueKM float64
	var maxScanTrips int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "gear",
		Short: "Per-bike accumulated mileage from your logged rides, plus maintenance-due flags against wear thresholds.",
		Long: `Roll up per-bike mileage from your logged trips.

Gear is attached to the full trip detail (not the summary), so this scans up to
--max-scan-trips synced trips, fetches each detail to read its gear, and sums the
distance per bike. Pass --due-km to flag bikes past a wear threshold (e.g. a chain
replacement interval). Run 'ridewithgps-pp-cli sync --resources trips' first.`,
		Example: strings.Trim(`
  ridewithgps-pp-cli gear
  ridewithgps-pp-cli gear --due-km 4000 --json
  ridewithgps-pp-cli gear --bike "Allied" --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would roll up per-bike mileage from trip details")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if maxScanTrips <= 0 {
				maxScanTrips = 100
			}
			if cliutil.IsDogfoodEnv() && maxScanTrips > 3 {
				maxScanTrips = 3
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
			maybeEmitSyncHints(cmd, db, "trips", flags.maxAge)

			rows, err := db.DB().QueryContext(ctx, `SELECT id, COALESCE(distance,0) FROM trips
				WHERE stationary IS NULL OR stationary = 0
				ORDER BY departed_at DESC LIMIT ?`, maxScanTrips)
			if err != nil {
				_ = db.Close()
				return fmt.Errorf("listing trips: %w", err)
			}
			type tripRow struct {
				id    string
				distM float64
			}
			var trips []tripRow
			for rows.Next() {
				var id sql.NullString
				var dist sql.NullFloat64
				if err := rows.Scan(&id, &dist); err == nil && id.String != "" {
					trips = append(trips, tripRow{id: id.String, distM: dist.Float64})
				}
			}
			rowsErr := rows.Err()
			_ = rows.Close()
			_ = db.Close()
			if rowsErr != nil {
				return fmt.Errorf("reading trips: %w", rowsErr)
			}

			view := gearView{ScannedTrips: len(trips), DueThresholdKM: dueKM, Gear: make([]gearTotal, 0), FetchFailures: make([]gearFetchFailure, 0)}
			if len(trips) == 0 {
				view.Note = "no trips in the local mirror; run 'ridewithgps-pp-cli sync --resources trips'"
				return printJSONOrTableGear(cmd, view, flags, bike)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Bounded fan-out; preserve per-fetch errors (principle: parallel-fetch
			// partial failures must not become phantom zero rows in the aggregate).
			type result struct {
				tripID   string
				gearID   string
				gearName string
				distM    float64
				err      error
			}
			sem := make(chan struct{}, 5)
			resultsCh := make(chan result, len(trips))
			var wg sync.WaitGroup
			for _, t := range trips {
				wg.Add(1)
				sem <- struct{}{}
				go func(tr tripRow) {
					defer wg.Done()
					defer func() { <-sem }()
					raw, err := c.Get(ctx, fmt.Sprintf("/api/v1/trips/%s.json", tr.id), nil)
					if err != nil {
						resultsCh <- result{tripID: tr.id, err: err}
						return
					}
					detail, err := unwrapAssetDetail(raw, "trip")
					if err != nil {
						resultsCh <- result{tripID: tr.id, err: err}
						return
					}
					dist := tr.distM
					if dist <= 0 {
						dist = detail.Distance
					}
					r := result{tripID: tr.id, distM: dist}
					if detail.Gear != nil && detail.Gear.ID.String() != "" {
						r.gearID = detail.Gear.ID.String()
						r.gearName = strings.TrimSpace(detail.Gear.Make + " " + detail.Gear.Model)
						if r.gearName == "" {
							r.gearName = "gear " + r.gearID
						}
					}
					resultsCh <- r
				}(t)
			}
			go func() { wg.Wait(); close(resultsCh) }()

			totals := map[string]*gearTotal{}
			for r := range resultsCh {
				if r.err != nil {
					view.FetchFailures = append(view.FetchFailures, gearFetchFailure{TripID: r.tripID, Error: classifyGearErr(r.err)})
					continue
				}
				key := r.gearID
				name := r.gearName
				if key == "" {
					key = "unassigned"
					name = "(no gear assigned)"
				}
				gt, ok := totals[key]
				if !ok {
					gt = &gearTotal{GearID: key, Name: name}
					totals[key] = gt
				}
				gt.Rides++
				gt.distanceM += r.distM
			}

			for _, gt := range totals {
				gt.DistanceKM = roundN(metersToKM(gt.distanceM), 1)
				gt.DistanceMI = roundN(metersToMiles(gt.distanceM), 1)
				if dueKM > 0 && gt.DistanceKM >= dueKM {
					gt.Due = true
				}
				if bike != "" && !strings.Contains(strings.ToLower(gt.Name), strings.ToLower(bike)) {
					continue
				}
				view.Gear = append(view.Gear, *gt)
			}
			sort.Slice(view.Gear, func(i, j int) bool { return view.Gear[i].distanceM > view.Gear[j].distanceM })

			if len(view.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d trip detail fetches failed; mileage computed over the remaining %d\n",
					len(view.FetchFailures), len(trips), len(trips)-len(view.FetchFailures))
			}
			if len(view.Gear) == 0 && view.Note == "" {
				view.Note = "no gear found across the scanned trips"
			}
			return printJSONOrTableGear(cmd, view, flags, bike)
		},
	}
	cmd.Flags().StringVar(&bike, "bike", "", "Filter to bikes whose make/model contains this text")
	cmd.Flags().Float64Var(&dueKM, "due-km", 0, "Flag bikes at or past this many km (maintenance-due)")
	cmd.Flags().IntVar(&maxScanTrips, "max-scan-trips", 100, "Max recent trips to scan for gear")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: local mirror)")
	return cmd
}

func printJSONOrTableGear(cmd *cobra.Command, view gearView, flags *rootFlags, bike string) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	if len(view.Gear) == 0 {
		if view.Note != "" {
			fmt.Fprintln(cmd.OutOrStdout(), view.Note)
		}
		return nil
	}
	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintf(tw, "Bike\tRides\tDistance\tDue\n")
	fmt.Fprintf(tw, "----\t-----\t--------\t---\t\n")
	for _, g := range view.Gear {
		due := ""
		if g.Due {
			due = "DUE"
		}
		fmt.Fprintf(tw, "%s\t%d\t%.0f km / %.0f mi\t%s\n", truncate(g.Name, 40), g.Rides, g.DistanceKM, g.DistanceMI, due)
	}
	_ = tw.Flush()
	fmt.Fprintf(cmd.OutOrStdout(), "\nScanned %d trips.\n", view.ScannedTrips)
	return nil
}

func classifyGearErr(err error) string {
	if err == nil {
		return ""
	}
	var rle *cliutil.RateLimitError
	if errors.As(err, &rle) {
		return "rate limited"
	}
	return truncate(err.Error(), 120)
}

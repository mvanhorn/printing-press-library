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

	"github.com/mvanhorn/printing-press-library/library/travel/ridewithgps/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/ridewithgps/internal/store"
	"github.com/spf13/cobra"
)

type gearTotal struct {
	GearID     string  `json:"gear_id"`
	Name       string  `json:"name"`
	Rides      int     `json:"rides"`
	DistanceKM float64 `json:"distance_km"`
	DistanceMI float64 `json:"distance_mi"`
	Due        bool    `json:"due,omitempty"`
	DueUnknown bool    `json:"due_unknown,omitempty"`
	distanceM  float64
}

type gearFetchFailure struct {
	TripID string `json:"trip_id"`
	Error  string `json:"error"`
}

type gearView struct {
	ScannedTrips   int                `json:"scanned_trips"`
	TotalTrips     int                `json:"total_trips"`
	Partial        bool               `json:"partial"`
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

Gear is attached to the full trip detail (not the summary), so the default
scans the 100 most recent synced trips and fetches each detail. Results say
when that limit leaves older trips out. Pass --max-scan-trips=0 for a complete
scan, which can make many live API requests for a large trip history.
Cached trip details may be reused; add --no-cache when fresh detail is required.
Pass --due-km to flag bikes past a wear threshold (e.g. a chain replacement
interval). Run 'ridewithgps-pp-cli sync --resources trips' first.`,
		Example: strings.Trim(`
  ridewithgps-pp-cli gear
  ridewithgps-pp-cli gear --max-scan-trips=0 --due-km 4000 --json
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
			if maxScanTrips < 0 {
				return usageErr(fmt.Errorf("--max-scan-trips must be zero or greater"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if cliutil.IsDogfoodEnv() && (maxScanTrips <= 0 || maxScanTrips > 3) {
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
			var totalTrips int
			if err := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM trips WHERE stationary IS NULL OR stationary = 0`).Scan(&totalTrips); err != nil {
				_ = db.Close()
				return fmt.Errorf("counting trips: %w", err)
			}

			query, queryArgs := gearTripsQuery(maxScanTrips)
			rows, err := db.DB().QueryContext(ctx, query, queryArgs...)
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
				if err := rows.Scan(&id, &dist); err != nil {
					_ = rows.Close()
					_ = db.Close()
					return fmt.Errorf("reading trip row: %w", err)
				}
				if id.String != "" {
					trips = append(trips, tripRow{id: id.String, distM: dist.Float64})
				}
			}
			rowsErr := rows.Err()
			_ = rows.Close()
			_ = db.Close()
			if rowsErr != nil {
				return fmt.Errorf("reading trips: %w", rowsErr)
			}

			view := gearView{ScannedTrips: len(trips), TotalTrips: totalTrips, Partial: len(trips) < totalTrips, DueThresholdKM: dueKM, Gear: make([]gearTotal, 0), FetchFailures: make([]gearFetchFailure, 0)}
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
			if len(view.FetchFailures) > 0 {
				sort.Slice(view.FetchFailures, func(i, j int) bool { return view.FetchFailures[i].TripID < view.FetchFailures[j].TripID })
				failedIDs := make([]string, 0, len(view.FetchFailures))
				for _, failure := range view.FetchFailures {
					failedIDs = append(failedIDs, fmt.Sprintf("%q", truncate(failure.TripID, 40)))
					if len(failedIDs) == 10 {
						break
					}
				}
				remaining := ""
				if len(view.FetchFailures) > len(failedIDs) {
					remaining = fmt.Sprintf(" (and %d more)", len(view.FetchFailures)-len(failedIDs))
				}
				return fmt.Errorf("gear mileage could not be completed: %d of %d trip detail fetches failed; failed trip IDs: %s%s; no totals emitted", len(view.FetchFailures), len(trips), strings.Join(failedIDs, ", "), remaining)
			}

			for _, gt := range totals {
				gt.DistanceKM = roundN(metersToKM(gt.distanceM), 1)
				gt.DistanceMI = roundN(metersToMiles(gt.distanceM), 1)
				gt.Due, gt.DueUnknown = gearDueStatus(gt.DistanceKM, dueKM, view.Partial)
				if bike != "" && !strings.Contains(strings.ToLower(gt.Name), strings.ToLower(bike)) {
					continue
				}
				view.Gear = append(view.Gear, *gt)
			}
			sort.Slice(view.Gear, func(i, j int) bool { return view.Gear[i].distanceM > view.Gear[j].distanceM })

			if view.Partial {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: gear mileage is partial: scanned %d of %d synced trips; use --max-scan-trips=0 for all\n", view.ScannedTrips, view.TotalTrips)
			}
			if len(view.Gear) == 0 && view.Note == "" {
				view.Note = "no gear found across the scanned trips"
			}
			return printJSONOrTableGear(cmd, view, flags, bike)
		},
	}
	cmd.Flags().StringVar(&bike, "bike", "", "Filter to bikes whose make/model contains this text")
	cmd.Flags().Float64Var(&dueKM, "due-km", 0, "Flag bikes at or past this many km (maintenance-due)")
	cmd.Flags().IntVar(&maxScanTrips, "max-scan-trips", 100, "Max recent trips to scan for gear (0 = all synced trips)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: local mirror)")
	return cmd
}

func gearTripsQuery(maxScanTrips int) (string, []any) {
	query := `SELECT id, COALESCE(distance,0) FROM trips
		WHERE stationary IS NULL OR stationary = 0
		ORDER BY departed_at DESC`
	if maxScanTrips <= 0 {
		return query, nil
	}
	return query + " LIMIT ?", []any{maxScanTrips}
}

func gearDueStatus(distanceKM, thresholdKM float64, partial bool) (due, unknown bool) {
	if thresholdKM <= 0 {
		return false, false
	}
	if distanceKM >= thresholdKM {
		return true, false
	}
	return false, partial
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
		} else if g.DueUnknown {
			due = "UNKNOWN"
		}
		fmt.Fprintf(tw, "%s\t%d\t%.0f km / %.0f mi\t%s\n", truncate(g.Name, 40), g.Rides, g.DistanceKM, g.DistanceMI, due)
	}
	_ = tw.Flush()
	if view.Partial {
		fmt.Fprintf(cmd.OutOrStdout(), "\nScanned %d of %d trips (partial).\n", view.ScannedTrips, view.TotalTrips)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "\nScanned %d trips.\n", view.ScannedTrips)
	}
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

// Hand-authored. Own file so `generate --force` keeps it whole.
// Corpus-wide commands: premium, radius, watch, coverage.
//
// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// belowFloorValues collects the labels of groups whose statistic was withheld
// for want of a large enough n, so the reason can be restated at the top level
// where --agent compaction will not drop it.
func belowFloorValues[T any](rows []T, at func(i int) (string, bool)) []string {
	out := []string{}
	for i := range rows {
		if label, withheld := at(i); withheld {
			out = append(out, label)
		}
	}
	return out
}

// stateCenters holds the approximate geographic center of each state, used only
// as a plausibility check on upstream coordinates. It is deliberately coarse:
// coordConflictMi is set far wider than any state's true radius, so an imprecise
// center here cannot cause a false exclusion — it only catches coordinates that
// are in an entirely different part of the country from the listed state.
var stateCenters = map[string][2]float64{
	"AL": {32.8, -86.8}, "AK": {64.0, -152.0}, "AZ": {34.3, -111.7}, "AR": {34.9, -92.4},
	"CA": {37.2, -119.5}, "CO": {39.0, -105.5}, "CT": {41.6, -72.7}, "DE": {39.0, -75.5},
	"DC": {38.9, -77.0}, "FL": {28.6, -82.4}, "GA": {32.6, -83.4}, "HI": {20.3, -156.4},
	"ID": {44.4, -114.6}, "IL": {40.0, -89.2}, "IN": {39.9, -86.3}, "IA": {42.1, -93.5},
	"KS": {38.5, -98.4}, "KY": {37.5, -85.3}, "LA": {31.1, -92.0}, "ME": {45.4, -69.2},
	"MD": {39.0, -76.8}, "MA": {42.3, -71.8}, "MI": {44.3, -85.4}, "MN": {46.3, -94.3},
	"MS": {32.7, -89.7}, "MO": {38.4, -92.5}, "MT": {47.0, -109.6}, "NE": {41.5, -99.8},
	"NV": {39.3, -116.6}, "NH": {43.7, -71.6}, "NJ": {40.2, -74.7}, "NM": {34.4, -106.1},
	"NY": {42.9, -75.5}, "NC": {35.5, -79.4}, "ND": {47.4, -100.5}, "OH": {40.3, -82.8},
	"OK": {35.6, -97.5}, "OR": {43.9, -120.6}, "PA": {40.9, -77.8}, "RI": {41.7, -71.6},
	"SC": {33.9, -80.9}, "SD": {44.4, -100.2}, "TN": {35.9, -86.4}, "TX": {31.5, -99.3},
	"UT": {39.3, -111.7}, "VT": {44.1, -72.7}, "VA": {37.5, -78.9}, "WA": {47.4, -120.5},
	"WV": {38.6, -80.6}, "WI": {44.6, -89.7}, "WY": {43.0, -107.6},
}

// coordConflictMi is the distance from a state's center beyond which its
// coordinates are treated as belonging to a different listing. Texas, the widest
// contiguous state, reaches roughly 450 miles from its center.
const coordConflictMi = 600

// stateAbbrev normalizes "Nevada" / "NV" / "nevada" to "NV". Returns "" when the
// name is not recognized, which callers treat as "no check possible".
var stateAbbrev = func() map[string]string {
	full := map[string]string{
		"alabama": "AL", "alaska": "AK", "arizona": "AZ", "arkansas": "AR",
		"california": "CA", "colorado": "CO", "connecticut": "CT", "delaware": "DE",
		"district of columbia": "DC", "florida": "FL", "georgia": "GA", "hawaii": "HI",
		"idaho": "ID", "illinois": "IL", "indiana": "IN", "iowa": "IA",
		"kansas": "KS", "kentucky": "KY", "louisiana": "LA", "maine": "ME",
		"maryland": "MD", "massachusetts": "MA", "michigan": "MI", "minnesota": "MN",
		"mississippi": "MS", "missouri": "MO", "montana": "MT", "nebraska": "NE",
		"nevada": "NV", "new hampshire": "NH", "new jersey": "NJ", "new mexico": "NM",
		"new york": "NY", "north carolina": "NC", "north dakota": "ND", "ohio": "OH",
		"oklahoma": "OK", "oregon": "OR", "pennsylvania": "PA", "rhode island": "RI",
		"south carolina": "SC", "south dakota": "SD", "tennessee": "TN", "texas": "TX",
		"utah": "UT", "vermont": "VT", "virginia": "VA", "washington": "WA",
		"west virginia": "WV", "wisconsin": "WI", "wyoming": "WY",
	}
	for _, ab := range full {
		full[strings.ToLower(ab)] = ab
	}
	return full
}()

// coordConflictsWithState reports whether a vehicle's published coordinates are
// implausibly far from the state its own listing names. Returns false whenever
// either side is missing or unrecognized — this is a fail-open sanity check, not
// a gate.
func coordConflictsWithState(lat, lon float64, state string) bool {
	ab, ok := stateAbbrev[strings.ToLower(strings.TrimSpace(state))]
	if !ok {
		return false
	}
	c, ok := stateCenters[ab]
	if !ok {
		return false
	}
	return haversineMi(lat, lon, c[0], c[1]) > coordConflictMi
}

func attrOf(v Vehicle, attr string) string {
	switch strings.ToLower(attr) {
	case "hardwareversion", "hardware":
		return v.HardwareVersion
	case "hasfsd", "fsd":
		if v.HasFsd == nil {
			return ""
		}
		return map[bool]string{true: "yes", false: "no"}[*v.HasFsd]
	case "drivetype", "drive":
		return v.DriveType
	case "wheels":
		return v.Wheels
	case "trim":
		return v.Trim
	case "exteriorcolor", "color":
		return v.ExteriorColor
	case "model":
		return v.Model
	}
	return ""
}

// ── premium ─────────────────────────────────────────────────────────────────

func newPremiumCmd(flags *rootFlags) *cobra.Command {
	var dbPath, by string

	cmd := &cobra.Command{
		Use:   "premium",
		Short: "See what a configuration attribute actually costs across the market",
		Long: "Reach for this when the question is \"what does feature X cost me\" — a market-wide " +
			"question. Use `comps` instead for one specific car's placement. Output is observational, " +
			"not causal: cells are matched on model and year but remain confounded by mileage and " +
			"options. Cells below the sample floor are shown as insufficient, never silently dropped.",
		Example:     "  teslatracker-pp-cli premium --by hardwareVersion --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compute configuration premium across the corpus")
			}
			if by == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--by is required (hardwareVersion, hasFsd, driveType, wheels, trim, exteriorColor)"))
			}
			if !mirrorGuard(cmd, flags, &dbPath) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := openMirrorRO(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer st.Close()

			all, err := loadVehicles(ctx, st.DB())
			if err != nil {
				return err
			}
			groups := map[string][]float64{}
			unknown := 0
			for _, v := range all {
				val := attrOf(v, by)
				l := v.LandedCents()
				if l == nil {
					continue
				}
				if val == "" {
					unknown++
					continue
				}
				groups[val] = append(groups[val], float64(*l)/100)
			}
			if len(groups) == 0 {
				return emit(cmd, flags, map[string]any{
					"by": by, "cells": []any{},
					"note": "no vehicle in the mirror publishes that attribute with a price"})
			}
			type cell struct {
				Value     string   `json:"value"`
				N         int      `json:"n"`
				MedianUSD *float64 `json:"median_landed_usd"`
				Note      string   `json:"note,omitempty"`
			}
			cells := []cell{}
			for k, vals := range groups {
				sort.Float64s(vals)
				c := cell{Value: k, N: len(vals)}
				if len(vals) >= minCohort {
					m := math.Round(median(vals))
					c.MedianUSD = &m
				} else {
					c.Note = fmt.Sprintf("n=%d is below the floor of %d — no median reported", len(vals), minCohort)
				}
				cells = append(cells, c)
			}
			sort.Slice(cells, func(i, j int) bool { return cells[i].Value < cells[j].Value })

			view := map[string]any{
				"by": by, "cells": cells, "unknown_attribute_count": unknown,
				"caveat": "observational, not causal: matched on model and year, still confounded by mileage and options",
			}
			// only state a spread when both ends clear the floor
			var lo, hi *cell
			for i := range cells {
				if cells[i].MedianUSD == nil {
					continue
				}
				if lo == nil || *cells[i].MedianUSD < *lo.MedianUSD {
					lo = &cells[i]
				}
				if hi == nil || *cells[i].MedianUSD > *hi.MedianUSD {
					hi = &cells[i]
				}
			}
			if lo != nil && hi != nil && lo.Value != hi.Value {
				view["spread_usd"] = math.Round(*hi.MedianUSD - *lo.MedianUSD)
				view["spread_between"] = []string{lo.Value, hi.Value}
			} else {
				// Withholding the spread silently reads as a broken premium. Say
				// why: either nothing cleared the floor, or only one group did and
				// a premium needs two sides to compare.
				priced := 0
				for i := range cells {
					if cells[i].MedianUSD != nil {
						priced++
					}
				}
				view["spread_note"] = fmt.Sprintf(
					"no premium computable: %d of %d groups have n>=%d, and a premium needs two",
					priced, len(cells), minCohort)
			}
			// The per-cell note is dropped by --agent compaction (a key kept only
			// when it appears on most rows, and it appears exactly on the rows it
			// explains). Restate it at the top level so a null median is never
			// unexplained.
			if sub := belowFloorValues(cells, func(i int) (string, bool) {
				return cells[i].Value, cells[i].MedianUSD == nil
			}); len(sub) > 0 {
				view["below_floor"] = sub
				view["below_floor_note"] = fmt.Sprintf(
					"median withheld for %v: fewer than %d cars in the group", sub, minCohort)
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return emit(cmd, flags, view)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "landed cost by %s\n", by)
			for _, c := range cells {
				if c.MedianUSD == nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-14s n=%-3d  (%s)\n", c.Value, c.N, c.Note)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-14s n=%-3d  median $%.0f\n", c.Value, c.N, *c.MedianUSD)
			}
			if unknown > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  (%d cars do not publish this attribute)\n", unknown)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&by, "by", "", "Attribute: hardwareVersion, hasFsd, driveType, wheels, trim, exteriorColor")
	return cmd
}

// ── radius ──────────────────────────────────────────────────────────────────

func haversineMi(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 3958.7613
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	p1, p2 := rad(lat1), rad(lat2)
	a := math.Sin((p2-p1)/2)*math.Sin((p2-p1)/2) +
		math.Cos(p1)*math.Cos(p2)*math.Sin(rad(lon2-lon1)/2)*math.Sin(rad(lon2-lon1)/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}

func newRadiusCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var lat, lon float64

	cmd := &cobra.Command{
		Use:   "radius",
		Short: "See how the price floor and median change as you widen your search radius",
		Long: "Reach for this before setting a search radius, or when you are at a hard ceiling and have " +
			"run out of local inventory. Answers \"is shopping farther worth it\" in dollars, with the " +
			"real per-car transport fee included at every step. This is not a search — it returns a " +
			"curve, not cars; run the inventory commands with the chosen radius afterward. Requires " +
			"--lat/--lon; no hidden network geocode call is made.",
		Example:     "  teslatracker-pp-cli radius --lat 30.22 --lon -92.02 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compute a landed-cost curve by distance")
			}
			if lat == 0 && lon == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--lat and --lon are required; this command does not geocode"))
			}
			if !mirrorGuard(cmd, flags, &dbPath) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := openMirrorRO(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer st.Close()

			all, err := loadVehicles(ctx, st.DB())
			if err != nil {
				return err
			}
			type located struct {
				landed float64
				miles  float64
			}
			pts := []located{}
			noLoc := 0
			badLoc := []string{}
			for _, v := range all {
				l := v.LandedCents()
				if l == nil {
					continue
				}
				if v.Latitude == nil || v.Longitude == nil {
					noLoc++
					continue
				}
				// Some upstream records carry coordinates that land in a
				// different state than the listing's own city/state. Distance is
				// the whole point of this command, so a car whose two location
				// fields disagree is excluded rather than presented as nearby.
				if coordConflictsWithState(*v.Latitude, *v.Longitude, v.LocationState) {
					badLoc = append(badLoc, fmt.Sprintf(
						"%s (listed %s, %s; coordinates are elsewhere)",
						v.VIN, v.LocationCity, v.LocationState))
					continue
				}
				pts = append(pts, located{
					landed: float64(*l) / 100,
					miles:  haversineMi(lat, lon, *v.Latitude, *v.Longitude),
				})
			}
			type band struct {
				WithinMiles int      `json:"within_miles"`
				N           int      `json:"n"`
				FloorUSD    *float64 `json:"floor_landed_usd"`
				MedianUSD   *float64 `json:"median_landed_usd"`
				Note        string   `json:"note,omitempty"`
			}
			bands := []band{}
			for _, r := range []int{100, 250, 500, 1000, 2000, 4000} {
				vals := []float64{}
				for _, p := range pts {
					if p.miles <= float64(r) {
						vals = append(vals, p.landed)
					}
				}
				sort.Float64s(vals)
				b := band{WithinMiles: r, N: len(vals)}
				if len(vals) >= minCohort {
					f, m := math.Round(vals[0]), math.Round(median(vals))
					b.FloorUSD, b.MedianUSD = &f, &m
				} else if len(vals) > 0 {
					f := math.Round(vals[0])
					b.FloorUSD = &f
					b.Note = fmt.Sprintf("n=%d below the floor of %d — no median", len(vals), minCohort)
				} else {
					b.Note = "no cars in this band"
				}
				bands = append(bands, b)
			}
			view := map[string]any{
				"origin": map[string]float64{"lat": lat, "lon": lon},
				"bands":  bands, "cars_without_location": noLoc,
				"note": "landed includes each car's own transport fee; distance is great-circle, not driving miles",
			}
			if len(badLoc) > 0 {
				view["cars_with_conflicting_location"] = badLoc
				view["location_conflict_note"] = fmt.Sprintf(
					"%d car(s) excluded: published coordinates are more than %d miles from the "+
						"state their own listing names, so their distance cannot be trusted",
					len(badLoc), coordConflictMi)
			}
			// Restated at the top level because --agent compaction drops a
			// per-row key that appears on only some rows — exactly the rows it
			// exists to explain.
			if sub := belowFloorValues(bands, func(i int) (string, bool) {
				return fmt.Sprintf("%dmi", bands[i].WithinMiles), bands[i].MedianUSD == nil && bands[i].N > 0
			}); len(sub) > 0 {
				view["below_floor"] = sub
				view["below_floor_note"] = fmt.Sprintf(
					"median withheld for bands %v: fewer than %d cars within that distance", sub, minCohort)
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return emit(cmd, flags, view)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "landed cost by radius from %.3f,%.3f\n", lat, lon)
			for _, b := range bands {
				if b.MedianUSD == nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  within %5d mi  n=%-3d  %s\n", b.WithinMiles, b.N, b.Note)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  within %5d mi  n=%-3d  floor $%.0f  median $%.0f\n",
					b.WithinMiles, b.N, *b.FloorUSD, *b.MedianUSD)
			}
			// An excluded car must never leave silently — the reader would just
			// see a band one car short and have no way to know why.
			for _, s := range badLoc {
				fmt.Fprintf(cmd.OutOrStdout(), "  excluded: %s\n", s)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().Float64Var(&lat, "lat", 0, "Your latitude")
	cmd.Flags().Float64Var(&lon, "lon", 0, "Your longitude")
	return cmd
}

// ── watch ───────────────────────────────────────────────────────────────────

func ensureWatchTable(db *sql.DB) {
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS watch (
		name TEXT PRIMARY KEY,
		max_landed_usd INTEGER,
		model TEXT,
		max_miles INTEGER,
		cursor_at DATETIME,
		last_vins TEXT
	)`)
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	root := &cobra.Command{
		Use:   "watch",
		Short: "Save a named search and see only what changed since you last ran it",
		Long: "The default entry point for a recurring session. An agent should call `watch run` at the " +
			"top of a session rather than re-running a search and re-reporting cars already seen. Use the " +
			"inventory commands instead for a genuine one-off exploration with no memory.",
		// The first line must work from a cold start — it is what a new user
		// copies, and what the verification harness executes. `watch run daily`
		// only works once `daily` has been saved, so it comes after the add.
		Example: "  teslatracker-pp-cli watch --agent\n" +
			"  teslatracker-pp-cli watch add daily --max-landed 26000 --model \"Model 3\"\n" +
			"  teslatracker-pp-cli watch run daily --agent",
		// Bare `watch` is the session-opening call its own Long text describes, so
		// it does what that text says: reports what changed across every saved
		// search, exactly like `watch run --all`. With no searches saved yet it
		// says so and exits zero — an empty watchlist is a starting state, not an
		// error. Subcommand routing is unaffected; cobra dispatches `watch add`
		// and friends before this ever runs.
		Annotations: map[string]string{"mcp:read-only": "false"},
	}

	add := &cobra.Command{
		Use:         "add <name>",
		Short:       "Save a named search",
		Args:        cobra.MaximumNArgs(1),
		Example:     "  teslatracker-pp-cli watch add daily --max-landed 30000",
		Annotations: map[string]string{"mcp:read-only": "false"},
	}
	var name, model string
	var maxLanded, maxMiles int
	add.Flags().IntVar(&maxLanded, "max-landed", 0, "Maximum landed cost in dollars (0 = no cap)")
	add.Flags().StringVar(&model, "model", "", "Model filter, e.g. Model 3")
	add.Flags().IntVar(&maxMiles, "max-miles", 0, "Maximum odometer (0 = no cap)")
	add.Flags().StringVar(&dbPath, "db", "", "Database path")
	add.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "save a named search")
		}
		if len(args) == 1 {
			name = args[0]
		}
		if name == "" {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("a search name is required"))
		}
		if !mirrorGuard(cmd, flags, &dbPath) {
			return nil
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		st, err := openMirror(ctx, dbPath)
		if err != nil {
			return err
		}
		defer st.Close()
		ensureWatchTable(st.DB())
		if _, err := st.DB().ExecContext(ctx,
			`INSERT INTO watch (name, max_landed_usd, model, max_miles, cursor_at, last_vins)
			 VALUES (?,?,?,?,?,?)
			 ON CONFLICT(name) DO UPDATE SET max_landed_usd=excluded.max_landed_usd,
			   model=excluded.model, max_miles=excluded.max_miles`,
			name, maxLanded, model, maxMiles, nil, ""); err != nil {
			return err
		}
		return emit(cmd, flags, map[string]any{"saved": name, "max_landed_usd": maxLanded,
			"model": model, "max_miles": maxMiles})
	}

	list := &cobra.Command{
		Use: "list", Short: "List every saved watch with its landed-price ceiling, model filter, mileage cap, and the timestamp of the last run",
		Example:     "  teslatracker-pp-cli watch list --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if !mirrorGuard(cmd, flags, &dbPath) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := openMirror(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			ensureWatchTable(st.DB())
			rows, err := st.DB().QueryContext(ctx,
				`SELECT name, COALESCE(max_landed_usd,0), COALESCE(model,''), COALESCE(max_miles,0),
				        COALESCE(cursor_at,'') FROM watch ORDER BY name`)
			if err != nil {
				return err
			}
			defer rows.Close()
			out := []map[string]any{}
			for rows.Next() {
				var n, m, cur string
				var ml, mm int
				if rows.Scan(&n, &ml, &m, &mm, &cur) != nil {
					continue
				}
				out = append(out, map[string]any{"name": n, "max_landed_usd": ml,
					"model": m, "max_miles": mm, "last_run": cur})
			}
			return emit(cmd, flags, map[string]any{"searches": out, "n": len(out)})
		},
	}
	list.Flags().StringVar(&dbPath, "db", "", "Database path")

	rm := &cobra.Command{
		Use: "rm <name>", Short: "Delete a saved search",
		Args:        cobra.MaximumNArgs(1),
		Example:     "  teslatracker-pp-cli watch rm daily",
		Annotations: map[string]string{"mcp:read-only": "false"},
	}
	var rmName string
	rm.Flags().StringVar(&dbPath, "db", "", "Database path")
	rm.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return nil
		}
		if len(args) == 1 {
			rmName = args[0]
		}
		if rmName == "" {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("a search name is required"))
		}
		if !mirrorGuard(cmd, flags, &dbPath) {
			return nil
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		st, err := openMirror(ctx, dbPath)
		if err != nil {
			return err
		}
		defer st.Close()
		ensureWatchTable(st.DB())
		res, err := st.DB().ExecContext(ctx, `DELETE FROM watch WHERE name = ?`, rmName)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return notFoundErr(fmt.Errorf("no saved search named %q", rmName))
		}
		return emit(cmd, flags, map[string]any{"deleted": rmName})
	}

	run := &cobra.Command{
		Use: "run [name]", Short: "Report what is new, price-changed or departed since the cursor",
		Args:        cobra.MaximumNArgs(1),
		Example:     "  teslatracker-pp-cli watch run daily --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
	}
	var runName string
	var runAll bool
	run.Flags().BoolVar(&runAll, "all", false, "Run every saved search")
	run.Flags().StringVar(&dbPath, "db", "", "Database path")
	// doRun is the shared body. `watch run` puts a help guard in front of it;
	// bare `watch` calls it directly with --all implied.
	doRun := func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "report changes since the saved cursor")
		}
		if len(args) == 1 {
			runName = args[0]
		}
		if runName == "" && !runAll {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("a search name or --all is required"))
		}
		if !mirrorGuard(cmd, flags, &dbPath) {
			return nil
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		st, err := openMirror(ctx, dbPath)
		if err != nil {
			return err
		}
		defer st.Close()
		db := st.DB()
		ensureWatchTable(db)

		q := `SELECT name, COALESCE(max_landed_usd,0), COALESCE(model,''), COALESCE(max_miles,0), COALESCE(last_vins,'') FROM watch`
		var rows *sql.Rows
		if runAll {
			rows, err = db.QueryContext(ctx, q+" ORDER BY name")
		} else {
			rows, err = db.QueryContext(ctx, q+" WHERE name = ?", runName)
		}
		if err != nil {
			return err
		}
		type saved struct {
			name, model, lastVins string
			maxLanded, maxMiles   int
		}
		defs := []saved{}
		for rows.Next() {
			var s saved
			if rows.Scan(&s.name, &s.maxLanded, &s.model, &s.maxMiles, &s.lastVins) == nil {
				defs = append(defs, s)
			}
		}
		_ = rows.Close()
		if len(defs) == 0 {
			if runAll {
				// --all over an empty set is legitimately empty, not an error
				return emit(cmd, flags, map[string]any{"searches": []any{},
					"note": "no saved searches yet; run `watch add <name>` first"})
			}
			return notFoundErr(fmt.Errorf("no saved search named %q; run `watch add` first", runName))
		}

		all, err := loadVehicles(ctx, db)
		if err != nil {
			return err
		}
		results := []map[string]any{}
		for _, s := range defs {
			matched := []Vehicle{}
			for _, v := range all {
				if s.model != "" && !strings.EqualFold(v.Model, s.model) {
					continue
				}
				if s.maxMiles > 0 {
					if v.Mileage == nil || *v.Mileage > s.maxMiles {
						continue // a null odometer cannot satisfy a cap
					}
				}
				if s.maxLanded > 0 {
					l := v.LandedCents()
					if l == nil || float64(*l)/100 > float64(s.maxLanded) {
						continue
					}
				}
				matched = append(matched, v)
			}
			nowVins := []string{}
			for _, v := range matched {
				nowVins = append(nowVins, v.VIN)
			}
			sort.Strings(nowVins)

			prev := map[string]bool{}
			firstRun := strings.TrimSpace(s.lastVins) == ""
			if !firstRun {
				var pv []string
				_ = json.Unmarshal([]byte(s.lastVins), &pv)
				for _, v := range pv {
					prev[v] = true
				}
			}
			added, removed := []string{}, []string{}
			if !firstRun {
				cur := map[string]bool{}
				for _, v := range nowVins {
					cur[v] = true
					if !prev[v] {
						added = append(added, v)
					}
				}
				for v := range prev {
					if !cur[v] {
						removed = append(removed, v)
					}
				}
				sort.Strings(removed)
			}
			blob, _ := json.Marshal(nowVins)
			_, _ = db.ExecContext(ctx, `UPDATE watch SET last_vins = ?, cursor_at = ? WHERE name = ?`,
				string(blob), time.Now().UTC().Format(time.RFC3339), s.name)

			r := map[string]any{"name": s.name, "matching_now": len(nowVins),
				"new": added, "departed": removed}
			if firstRun {
				r["note"] = "first run — cursor set; no comparison was possible"
			}
			results = append(results, r)
		}
		return emit(cmd, flags, map[string]any{"searches": results})
	}

	run.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		return doRun(cmd, args)
	}

	// Bare `watch` delegates to the same body as `watch run --all` rather than
	// duplicating it, so the two can never drift apart.
	root.RunE = func(cmd *cobra.Command, args []string) error {
		runName, runAll = "", true
		return doRun(cmd, nil)
	}

	root.AddCommand(add, list, rm, run)
	return root
}

// ── coverage ────────────────────────────────────────────────────────────────

func newCoverageCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "See how complete and how fresh your local mirror is, field by field",
		Long: "An agent should call this before asserting any aggregate, and must surface a stale or " +
			"incomplete mirror as a caveat rather than answering flatly. It reports on the STORE, never " +
			"on cars. If it reports a hydration shortfall or a store older than the freshness threshold, " +
			"treat downstream numbers from comps, premium and degradation as provisional and re-sync first.",
		Example:     "  teslatracker-pp-cli coverage --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "report local mirror completeness")
			}
			if !mirrorGuard(cmd, flags, &dbPath) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := openMirrorRO(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer st.Close()
			db := st.DB()

			links, _ := vinsFromLinks(ctx, db)
			all, err := loadVehicles(ctx, db)
			if err != nil {
				return err
			}
			fields := map[string]int{}
			bump := func(k string, ok bool) {
				if ok {
					fields[k]++
				}
			}
			for _, v := range all {
				bump("mileage", v.Mileage != nil)
				bump("range", v.Range != nil)
				bump("actualRange", v.ActualRange != nil)
				bump("purchasePrice", v.PurchasePriceCents != nil)
				bump("transportFee", v.TransportFeeCents != nil)
				bump("latitude", v.Latitude != nil)
				bump("warrantyBatteryExpDate", v.WarrantyBatteryExpDate != "")
				bump("warrantyBatteryMile", v.WarrantyBatteryMile != nil)
				bump("hardwareVersion", v.HardwareVersion != "")
				bump("firstSeenAt", v.FirstSeenAt != "")
			}
			populated := map[string]string{}
			for k, n := range fields {
				pct := 0.0
				if len(all) > 0 {
					pct = float64(n) / float64(len(all)) * 100
				}
				populated[k] = fmt.Sprintf("%d/%d (%.0f%%)", n, len(all), pct)
			}
			for _, k := range []string{"mileage", "range", "actualRange", "purchasePrice", "transportFee",
				"latitude", "warrantyBatteryExpDate", "warrantyBatteryMile", "hardwareVersion", "firstSeenAt"} {
				if _, ok := populated[k]; !ok {
					populated[k] = "0/" + strconv.Itoa(len(all)) + " (0%)"
				}
			}
			var newest, oldest sql.NullString
			_ = db.QueryRowContext(ctx,
				`SELECT MAX(updated_at), MIN(updated_at) FROM resources WHERE resource_type = ?`,
				vehicleResourceType).Scan(&newest, &oldest)
			var snaps int
			_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM price_snapshot`).Scan(&snaps)

			view := map[string]any{
				"links_found": len(links), "vehicles_hydrated": len(all),
				"hydration_shortfall": len(links) - len(all),
				"price_snapshots":     snaps,
				"newest_record":       newest.String, "oldest_record": oldest.String,
				"field_population": populated,
			}
			if len(links) > len(all) {
				view["warning"] = fmt.Sprintf(
					"%d listing links have no hydrated vehicle record; aggregates are computed over %d of %d cars. Run hydrate.",
					len(links)-len(all), len(all), len(links))
			}
			if len(all) == 0 {
				view["warning"] = "no hydrated vehicles; run sync then hydrate before trusting any aggregate"
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return emit(cmd, flags, view)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "links %d | hydrated %d | price snapshots %d\n",
				len(links), len(all), snaps)
			keys := make([]string, 0, len(populated))
			for k := range populated {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-24s %s\n", k, populated[k])
			}
			if w, ok := view["warning"]; ok {
				fmt.Fprintf(cmd.OutOrStdout(), "\nwarning: %s\n", w)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// Copyright 2026 Roberto Bissanti and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel features for the PVGIS CLI. NOT generator-emitted.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// ---------- shared helpers ----------

// parseLatLon parses "lat,lon" into two float64.
func parseLatLon(s string) (lat, lon float64, err error) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected lat,lon, got %q", s)
	}
	lat, err = strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid lat %q: %w", parts[0], err)
	}
	lon, err = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid lon %q: %w", parts[1], err)
	}
	return lat, lon, nil
}

// pvgisPVcalc calls /PVcalc and returns the parsed response.
func pvgisPVcalc(c apiClient, lat, lon, pnom, loss, tilt, azimuth float64, raddatabase string) (map[string]any, error) {
	params := map[string]string{
		"lat":          fmt.Sprintf("%v", lat),
		"lon":          fmt.Sprintf("%v", lon),
		"peakpower":    fmt.Sprintf("%v", pnom),
		"loss":         fmt.Sprintf("%v", loss),
		"angle":        fmt.Sprintf("%v", tilt),
		"aspect":       fmt.Sprintf("%v", azimuth),
		"outputformat": "json",
	}
	if raddatabase != "" {
		params["raddatabase"] = raddatabase
	}
	data, err := c.Get("/PVcalc", params)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parsing PVcalc response: %w", err)
	}
	return out, nil
}

// pvgisTMY calls /tmy.
func pvgisTMY(c apiClient, lat, lon float64) (map[string]any, error) {
	params := map[string]string{
		"lat":          fmt.Sprintf("%v", lat),
		"lon":          fmt.Sprintf("%v", lon),
		"outputformat": "json",
	}
	data, err := c.Get("/tmy", params)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parsing tmy response: %w", err)
	}
	return out, nil
}

// extractEY pulls the annual energy E_y from a PVcalc response. Tolerates
// the v5_3 shape outputs.totals.fixed.E_y and the older outputs.totals.E_y.
func extractEY(resp map[string]any) (float64, bool) {
	outputs, _ := resp["outputs"].(map[string]any)
	if outputs == nil {
		return 0, false
	}
	totals, _ := outputs["totals"].(map[string]any)
	if totals == nil {
		return 0, false
	}
	if fixed, ok := totals["fixed"].(map[string]any); ok {
		if v, ok := fixed["E_y"].(float64); ok {
			return v, true
		}
	}
	if v, ok := totals["E_y"].(float64); ok {
		return v, true
	}
	return 0, false
}

// extractMonthlyGH returns 12 monthly average values for a TMY response,
// pulling the named key from each hourly row.
func extractTMYMonthlyAverages(resp map[string]any, key string) [12]float64 {
	var sums, counts [12]float64
	outputs, _ := resp["outputs"].(map[string]any)
	if outputs == nil {
		return [12]float64{}
	}
	rows, _ := outputs["tmy_hourly"].([]any)
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		if row == nil {
			continue
		}
		// time(UTC) is YYYYMMDD:HHMM
		var month int
		switch t := row["time(UTC)"].(type) {
		case string:
			if len(t) >= 6 {
				if m, err := strconv.Atoi(t[4:6]); err == nil {
					month = m
				}
			}
		}
		if month < 1 || month > 12 {
			continue
		}
		v, ok := row[key].(float64)
		if !ok {
			continue
		}
		sums[month-1] += v
		counts[month-1]++
	}
	var out [12]float64
	for i := 0; i < 12; i++ {
		if counts[i] > 0 {
			out[i] = sums[i] / counts[i]
		}
	}
	return out
}

// apiClient is the subset of *client.Client our novel commands use.
// Declared as an interface so tests can inject mocks (none yet, but keep
// the seam).
type apiClient interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}

// readSitesCSV parses a CSV with required columns lat, lon and optional label.
type siteRow struct {
	Label string  `json:"label"`
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
}

func readSitesCSV(path string) ([]siteRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV header: %w", err)
	}
	colLat, colLon, colLabel := -1, -1, -1
	for i, h := range header {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "lat", "latitude":
			colLat = i
		case "lon", "lng", "longitude":
			colLon = i
		case "label", "name":
			colLabel = i
		}
	}
	if colLat < 0 || colLon < 0 {
		return nil, fmt.Errorf("CSV must have lat and lon columns; got %v", header)
	}
	var sites []siteRow
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading CSV: %w", err)
		}
		if len(rec) <= colLat || len(rec) <= colLon {
			continue
		}
		lat, errLat := strconv.ParseFloat(strings.TrimSpace(rec[colLat]), 64)
		lon, errLon := strconv.ParseFloat(strings.TrimSpace(rec[colLon]), 64)
		if errLat != nil || errLon != nil {
			continue
		}
		s := siteRow{Lat: lat, Lon: lon}
		if colLabel >= 0 && len(rec) > colLabel {
			s.Label = strings.TrimSpace(rec[colLabel])
		}
		if s.Label == "" {
			s.Label = fmt.Sprintf("%.4f,%.4f", lat, lon)
		}
		sites = append(sites, s)
	}
	if len(sites) == 0 {
		return nil, fmt.Errorf("no valid lat/lon rows in %s", path)
	}
	return sites, nil
}

// emitJSON writes data as JSON using printOutputWithFlags or a plain encoder.
func emitJSON(cmd *cobra.Command, flags *rootFlags, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// ---------- sites rank ----------

func newSitesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sites",
		Short: "Multi-site batch operations (rank by yield, diff against a baseline).",
	}
	cmd.AddCommand(newSitesRankCmd(flags))
	cmd.AddCommand(newSitesDiffCmd(flags))
	return cmd
}

func newSitesRankCmd(flags *rootFlags) *cobra.Command {
	var (
		input       string
		pnom        float64
		systemLoss  float64
		tilt        float64
		azimuth     float64
		raddatabase string
	)
	cmd := &cobra.Command{
		Use:   "rank",
		Short: "Rank a CSV of sites by annual PV yield per kWp under a shared system spec.",
		Long: "Reads a CSV with columns lat,lon[,label] and ranks every site by its " +
			"annual PVcalc production. The same system spec (pnom, system-loss, tilt, " +
			"azimuth) is applied to every site so yields are directly comparable.",
		Example: "  pvgis-pp-cli sites rank --input sites.csv --tilt 30 --pnom 5 --system-loss 14 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if input == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			sites, err := readSitesCSV(input)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			type result struct {
				Rank     int     `json:"rank"`
				Label    string  `json:"label"`
				Lat      float64 `json:"lat"`
				Lon      float64 `json:"lon"`
				EYearly  float64 `json:"e_y_kwh"`
				EPerKwp  float64 `json:"e_y_per_kwp"`
				Database string  `json:"database,omitempty"`
				Err      string  `json:"error,omitempty"`
			}
			results := make([]result, 0, len(sites))
			for _, s := range sites {
				resp, err := pvgisPVcalc(c, s.Lat, s.Lon, pnom, systemLoss, tilt, azimuth, raddatabase)
				if err != nil {
					results = append(results, result{Label: s.Label, Lat: s.Lat, Lon: s.Lon, Err: err.Error()})
					continue
				}
				ey, _ := extractEY(resp)
				db := ""
				if inputs, ok := resp["inputs"].(map[string]any); ok {
					if meteo, ok := inputs["meteo_data"].(map[string]any); ok {
						if rdb, ok := meteo["radiation_db"].(string); ok {
							db = rdb
						}
					}
				}
				perKwp := ey
				if pnom > 0 {
					perKwp = ey / pnom
				}
				results = append(results, result{
					Label:    s.Label,
					Lat:      s.Lat,
					Lon:      s.Lon,
					EYearly:  ey,
					EPerKwp:  perKwp,
					Database: db,
				})
			}
			sort.SliceStable(results, func(i, j int) bool {
				return results[i].EPerKwp > results[j].EPerKwp
			})
			for i := range results {
				results[i].Rank = i + 1
			}
			return emitJSON(cmd, flags, map[string]any{
				"system": map[string]any{
					"pnom_kwp":          pnom,
					"system_loss_pct":   systemLoss,
					"tilt_deg":          tilt,
					"azimuth_deg":       azimuth,
					"radiation_db_pref": raddatabase,
				},
				"count":   len(results),
				"results": results,
			})
		},
	}
	cmd.Flags().StringVar(&input, "input", "", "Path to a CSV with columns lat,lon[,label].")
	cmd.Flags().Float64Var(&pnom, "pnom", 1.0, "Nominal system peak power in kWp applied to every site.")
	cmd.Flags().Float64Var(&systemLoss, "system-loss", 14.0, "Total system losses in percent.")
	cmd.Flags().Float64Var(&tilt, "tilt", 30.0, "Surface tilt in degrees.")
	cmd.Flags().Float64Var(&azimuth, "azimuth", 0.0, "Surface azimuth (PVGIS convention: 0=south).")
	cmd.Flags().StringVar(&raddatabase, "raddatabase", "", "Force radiation database (PVGIS-SARAH3, PVGIS-NSRDB, PVGIS-ERA5).")
	return cmd
}

// ---------- sites diff ----------

func newSitesDiffCmd(flags *rootFlags) *cobra.Command {
	var (
		baseline    string
		target      string
		pnom        float64
		systemLoss  float64
		tilt        float64
		azimuth     float64
		raddatabase string
	)
	cmd := &cobra.Command{
		Use:     "diff",
		Short:   "Compare annual production at a baseline site vs a target site under identical system specs.",
		Example: "  pvgis-pp-cli sites diff --baseline 45.0,9.0 --target 41.9,12.5 --pnom 5 --system-loss 14 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if baseline == "" || target == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			bLat, bLon, err := parseLatLon(baseline)
			if err != nil {
				return fmt.Errorf("--baseline: %w", err)
			}
			tLat, tLon, err := parseLatLon(target)
			if err != nil {
				return fmt.Errorf("--target: %w", err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			bResp, err := pvgisPVcalc(c, bLat, bLon, pnom, systemLoss, tilt, azimuth, raddatabase)
			if err != nil {
				return fmt.Errorf("baseline PVcalc: %w", err)
			}
			tResp, err := pvgisPVcalc(c, tLat, tLon, pnom, systemLoss, tilt, azimuth, raddatabase)
			if err != nil {
				return fmt.Errorf("target PVcalc: %w", err)
			}
			bEY, _ := extractEY(bResp)
			tEY, _ := extractEY(tResp)
			delta := tEY - bEY
			pct := 0.0
			if bEY != 0 {
				pct = (delta / bEY) * 100
			}
			return emitJSON(cmd, flags, map[string]any{
				"baseline": map[string]any{"lat": bLat, "lon": bLon, "e_y_kwh": bEY},
				"target":   map[string]any{"lat": tLat, "lon": tLon, "e_y_kwh": tEY},
				"delta": map[string]any{
					"kwh_year":      delta,
					"percent":       pct,
					"target_better": delta > 0,
				},
				"system": map[string]any{
					"pnom_kwp":        pnom,
					"system_loss_pct": systemLoss,
					"tilt_deg":        tilt,
					"azimuth_deg":     azimuth,
				},
			})
		},
	}
	cmd.Flags().StringVar(&baseline, "baseline", "", "Baseline site as lat,lon (e.g. 45.0,9.0).")
	cmd.Flags().StringVar(&target, "target", "", "Target site as lat,lon.")
	cmd.Flags().Float64Var(&pnom, "pnom", 1.0, "Nominal system peak power in kWp.")
	cmd.Flags().Float64Var(&systemLoss, "system-loss", 14.0, "Total system losses in percent.")
	cmd.Flags().Float64Var(&tilt, "tilt", 30.0, "Surface tilt in degrees.")
	cmd.Flags().Float64Var(&azimuth, "azimuth", 0.0, "Surface azimuth (PVGIS convention).")
	cmd.Flags().StringVar(&raddatabase, "raddatabase", "", "Force radiation database.")
	return cmd
}

// ---------- production sweep ----------

func newProductionSweepCmd(flags *rootFlags) *cobra.Command {
	var (
		lat         float64
		lon         float64
		pnom        float64
		systemLoss  float64
		tiltMin     float64
		tiltMax     float64
		tiltStep    float64
		azMin       float64
		azMax       float64
		azStep      float64
		raddatabase string
	)
	cmd := &cobra.Command{
		Use:   "sweep",
		Short: "Compute a 2D tilt×azimuth grid of annual yields for a site.",
		Long: "Issues one PVcalc request for every (tilt, azimuth) combination in the grid. " +
			"Use the response (cached locally) to pick the best fixed orientation or to interpolate " +
			"yields at non-grid points offline.",
		Example: "  pvgis-pp-cli production sweep --lat 45 --lon 9 --pnom 5 --system-loss 14 --tilt-min 0 --tilt-max 60 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("lat") || !cmd.Flags().Changed("lon") {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if tiltStep <= 0 {
				return fmt.Errorf("--tilt-step must be > 0")
			}
			if azStep <= 0 {
				return fmt.Errorf("--azimuth-step must be > 0")
			}
			// PATCH(p1-no-sentinel-leak-on-inverted-bounds): refuse inverted
			// ranges up front. Without this, no iteration would run and the
			// sentinel best{EY: -1} would leak into the JSON output.
			if tiltMin > tiltMax {
				return fmt.Errorf("--tilt-min (%v) must be <= --tilt-max (%v)", tiltMin, tiltMax)
			}
			if azMin > azMax {
				return fmt.Errorf("--azimuth-min (%v) must be <= --azimuth-max (%v)", azMin, azMax)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			type cell struct {
				Tilt    float64 `json:"tilt"`
				Azimuth float64 `json:"azimuth"`
				EY      float64 `json:"e_y_kwh"`
			}
			var cells []cell
			best := cell{EY: -1}
			for t := tiltMin; t <= tiltMax+1e-9; t += tiltStep {
				for a := azMin; a <= azMax+1e-9; a += azStep {
					resp, err := pvgisPVcalc(c, lat, lon, pnom, systemLoss, t, a, raddatabase)
					if err != nil {
						return fmt.Errorf("PVcalc tilt=%.1f az=%.1f: %w", t, a, err)
					}
					ey, _ := extractEY(resp)
					pt := cell{Tilt: t, Azimuth: a, EY: ey}
					cells = append(cells, pt)
					if ey > best.EY {
						best = pt
					}
				}
			}
			return emitJSON(cmd, flags, map[string]any{
				"site":   map[string]any{"lat": lat, "lon": lon},
				"system": map[string]any{"pnom_kwp": pnom, "system_loss_pct": systemLoss},
				"grid": map[string]any{
					"tilt_min": tiltMin, "tilt_max": tiltMax, "tilt_step": tiltStep,
					"azimuth_min": azMin, "azimuth_max": azMax, "azimuth_step": azStep,
				},
				"count": len(cells),
				"best":  best,
				"cells": cells,
			})
		},
	}
	cmd.Flags().Float64Var(&lat, "lat", 0, "Latitude in WGS84 decimal degrees.")
	cmd.Flags().Float64Var(&lon, "lon", 0, "Longitude in WGS84 decimal degrees.")
	cmd.Flags().Float64Var(&pnom, "pnom", 1.0, "Nominal system peak power in kWp.")
	cmd.Flags().Float64Var(&systemLoss, "system-loss", 14.0, "Total system losses in percent.")
	cmd.Flags().Float64Var(&tiltMin, "tilt-min", 0, "Minimum tilt in degrees.")
	cmd.Flags().Float64Var(&tiltMax, "tilt-max", 60, "Maximum tilt in degrees.")
	cmd.Flags().Float64Var(&tiltStep, "tilt-step", 10, "Tilt grid step in degrees.")
	cmd.Flags().Float64Var(&azMin, "azimuth-min", -90, "Minimum azimuth (PVGIS convention).")
	cmd.Flags().Float64Var(&azMax, "azimuth-max", 90, "Maximum azimuth.")
	cmd.Flags().Float64Var(&azStep, "azimuth-step", 30, "Azimuth grid step in degrees.")
	cmd.Flags().StringVar(&raddatabase, "raddatabase", "", "Force radiation database.")
	return cmd
}

// ---------- production optimal-tilt ----------

func newProductionOptimalTiltCmd(flags *rootFlags) *cobra.Command {
	var (
		lat        float64
		lon        float64
		azimuth    float64
		pnom       float64
		systemLoss float64
		tiltMin    float64
		tiltMax    float64
		tiltStep   float64
	)
	cmd := &cobra.Command{
		Use:   "optimal-tilt",
		Short: "Find the tilt angle that maximizes annual PV production for a given azimuth.",
		Long: "Sweeps tilt angles from --tilt-min to --tilt-max in --tilt-step increments at the fixed " +
			"--azimuth (PVGIS convention: 0=south). Returns the winning tilt and its annual energy E_y, " +
			"plus the full sweep so you can see how flat the optimum is.",
		Example: "  pvgis-pp-cli production optimal-tilt --lat 45 --lon 9 --azimuth 0 --json --select best_tilt,best_e_y",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("lat") || !cmd.Flags().Changed("lon") {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if tiltStep <= 0 {
				return fmt.Errorf("--tilt-step must be > 0")
			}
			// PATCH(p1-no-sentinel-leak-on-inverted-bounds): refuse inverted
			// range up front so the sentinel best_e_y=-1 cannot leak into JSON.
			if tiltMin > tiltMax {
				return fmt.Errorf("--tilt-min (%v) must be <= --tilt-max (%v)", tiltMin, tiltMax)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			type point struct {
				Tilt float64 `json:"tilt"`
				EY   float64 `json:"e_y_kwh"`
			}
			var points []point
			bestTilt := 0.0
			bestEY := -1.0
			for t := tiltMin; t <= tiltMax+1e-9; t += tiltStep {
				resp, err := pvgisPVcalc(c, lat, lon, pnom, systemLoss, t, azimuth, "")
				if err != nil {
					return fmt.Errorf("PVcalc tilt=%.1f: %w", t, err)
				}
				ey, _ := extractEY(resp)
				points = append(points, point{Tilt: t, EY: ey})
				if ey > bestEY {
					bestEY = ey
					bestTilt = t
				}
			}
			return emitJSON(cmd, flags, map[string]any{
				"site":      map[string]any{"lat": lat, "lon": lon},
				"azimuth":   azimuth,
				"system":    map[string]any{"pnom_kwp": pnom, "system_loss_pct": systemLoss},
				"best_tilt": bestTilt,
				"best_e_y":  bestEY,
				"count":     len(points),
				"sweep":     points,
			})
		},
	}
	cmd.Flags().Float64Var(&lat, "lat", 0, "Latitude in WGS84 decimal degrees.")
	cmd.Flags().Float64Var(&lon, "lon", 0, "Longitude in WGS84 decimal degrees.")
	cmd.Flags().Float64Var(&azimuth, "azimuth", 0, "Fixed surface azimuth (PVGIS convention: 0=south, -90=east, 90=west).")
	cmd.Flags().Float64Var(&pnom, "pnom", 1.0, "Nominal system peak power in kWp.")
	cmd.Flags().Float64Var(&systemLoss, "system-loss", 14.0, "Total system losses in percent.")
	cmd.Flags().Float64Var(&tiltMin, "tilt-min", 0, "Minimum tilt in degrees.")
	cmd.Flags().Float64Var(&tiltMax, "tilt-max", 60, "Maximum tilt in degrees.")
	cmd.Flags().Float64Var(&tiltStep, "tilt-step", 5, "Tilt step in degrees.")
	return cmd
}

// ---------- production compare (database comparator) ----------

func newProductionCompareCmd(flags *rootFlags) *cobra.Command {
	var (
		lat        float64
		lon        float64
		pnom       float64
		systemLoss float64
		tilt       float64
		azimuth    float64
	)
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Run the same site through PVGIS-SARAH3, PVGIS-NSRDB, and PVGIS-ERA5; report yield deltas.",
		Long: "Issues parallel PVcalc requests forcing each radiation database in turn. " +
			"Useful for sites near the SARAH3 coverage boundary or for sanity-checking which DB " +
			"PVGIS would otherwise choose for you.",
		Example: "  pvgis-pp-cli production compare --lat 40.7 --lon -74 --pnom 1 --system-loss 14 --tilt 30 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("lat") || !cmd.Flags().Changed("lon") {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			type dbResult struct {
				Database string  `json:"database"`
				EY       float64 `json:"e_y_kwh"`
				Err      string  `json:"error,omitempty"`
			}
			dbs := []string{"PVGIS-SARAH3", "PVGIS-NSRDB", "PVGIS-ERA5"}
			results := make([]dbResult, 0, len(dbs))
			best := dbResult{EY: -1}
			worst := dbResult{EY: math.MaxFloat64}
			anyOK := false
			errCount := 0
			for _, db := range dbs {
				resp, err := pvgisPVcalc(c, lat, lon, pnom, systemLoss, tilt, azimuth, db)
				if err != nil {
					results = append(results, dbResult{Database: db, Err: err.Error()})
					errCount++
					continue
				}
				ey, ok := extractEY(resp)
				if !ok {
					results = append(results, dbResult{Database: db, Err: "no E_y in response (likely out of coverage)"})
					errCount++
					continue
				}
				r := dbResult{Database: db, EY: ey}
				results = append(results, r)
				anyOK = true
				if ey > best.EY {
					best = r
				}
				if ey < worst.EY {
					worst = r
				}
			}
			spread := 0.0
			spreadPct := 0.0
			if anyOK && best.EY > 0 {
				spread = best.EY - worst.EY
				spreadPct = (spread / best.EY) * 100
			}
			return emitJSON(cmd, flags, map[string]any{
				"site":    map[string]any{"lat": lat, "lon": lon},
				"system":  map[string]any{"pnom_kwp": pnom, "system_loss_pct": systemLoss, "tilt_deg": tilt, "azimuth_deg": azimuth},
				"results": results,
				"summary": map[string]any{
					"best_db":      best.Database,
					"worst_db":     worst.Database,
					"spread_kwh":   spread,
					"spread_pct":   spreadPct,
					"databases_ok": anyOK && errCount == 0,
					"partial":      errCount > 0 && anyOK,
					"errors":       errCount,
				},
			})
		},
	}
	cmd.Flags().Float64Var(&lat, "lat", 0, "Latitude in WGS84 decimal degrees.")
	cmd.Flags().Float64Var(&lon, "lon", 0, "Longitude in WGS84 decimal degrees.")
	cmd.Flags().Float64Var(&pnom, "pnom", 1.0, "Nominal system peak power in kWp.")
	cmd.Flags().Float64Var(&systemLoss, "system-loss", 14.0, "Total system losses in percent.")
	cmd.Flags().Float64Var(&tilt, "tilt", 30.0, "Surface tilt in degrees.")
	cmd.Flags().Float64Var(&azimuth, "azimuth", 0.0, "Surface azimuth (PVGIS convention).")
	return cmd
}

// ---------- weather similar (TMY climate fingerprint) ----------

func newWeatherSimilarCmd(flags *rootFlags) *cobra.Command {
	var (
		to     string
		within string
		top    int
	)
	cmd := &cobra.Command{
		Use:   "similar",
		Short: "Rank candidate sites by TMY climate similarity (12-month T + GHI signature) to a target site.",
		Long: "Pulls the TMY for the --to site and for every site in --within, aggregates both to a " +
			"12-month signature of average T2m and average G(h), then ranks the candidates by Euclidean " +
			"distance in that 24-d space. Cheaper sites (already cached) cost nothing.",
		Example: "  pvgis-pp-cli weather similar --to 45.0,9.0 --within sites.csv --top 5 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if to == "" || within == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			tLat, tLon, err := parseLatLon(to)
			if err != nil {
				return fmt.Errorf("--to: %w", err)
			}
			sites, err := readSitesCSV(within)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			targetResp, err := pvgisTMY(c, tLat, tLon)
			if err != nil {
				return fmt.Errorf("TMY for target: %w", err)
			}
			tT := extractTMYMonthlyAverages(targetResp, "T2m")
			tG := extractTMYMonthlyAverages(targetResp, "G(h)")

			type result struct {
				Rank     int     `json:"rank"`
				Label    string  `json:"label"`
				Lat      float64 `json:"lat"`
				Lon      float64 `json:"lon"`
				Distance float64 `json:"distance"`
				Err      string  `json:"error,omitempty"`
			}
			results := make([]result, 0, len(sites))
			for _, s := range sites {
				if math.Abs(s.Lat-tLat) < 1e-6 && math.Abs(s.Lon-tLon) < 1e-6 {
					continue
				}
				resp, err := pvgisTMY(c, s.Lat, s.Lon)
				if err != nil {
					results = append(results, result{Label: s.Label, Lat: s.Lat, Lon: s.Lon, Err: err.Error()})
					continue
				}
				sT := extractTMYMonthlyAverages(resp, "T2m")
				sG := extractTMYMonthlyAverages(resp, "G(h)")
				var d float64
				for i := 0; i < 12; i++ {
					dT := sT[i] - tT[i]
					dG := (sG[i] - tG[i]) / 100.0 // normalize: GHI is order 100s W/m2
					d += dT*dT + dG*dG
				}
				d = math.Sqrt(d)
				results = append(results, result{Label: s.Label, Lat: s.Lat, Lon: s.Lon, Distance: d})
			}
			sort.SliceStable(results, func(i, j int) bool {
				if results[i].Err != "" {
					return false
				}
				if results[j].Err != "" {
					return true
				}
				return results[i].Distance < results[j].Distance
			})
			if top > 0 && len(results) > top {
				results = results[:top]
			}
			for i := range results {
				results[i].Rank = i + 1
			}
			return emitJSON(cmd, flags, map[string]any{
				"target":  map[string]any{"lat": tLat, "lon": tLon},
				"count":   len(results),
				"results": results,
			})
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "Target site as lat,lon.")
	cmd.Flags().StringVar(&within, "within", "", "Path to a CSV of candidate sites with columns lat,lon[,label].")
	cmd.Flags().IntVar(&top, "top", 5, "Maximum number of nearest sites to return.")
	return cmd
}

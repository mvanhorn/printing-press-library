// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newCompareCmd(flags *rootFlags) *cobra.Command {
	var (
		regionA string
		regionB string
		periodA string
		periodB string
		region  string
		window  string
		minMag  float64
		dataSrc string
	)
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Side-by-side comparison of two regions OR two time periods (counts, max mag, total energy)",
		Long: `Compare earthquake activity between two regions OR two time periods.

Region mode: --region-a "W,S,E,N" --region-b "W,S,E,N" --window 30d
Period mode: --region "W,S,E,N" --period-a "2020-01-01:2021-01-01" --period-b "2024-01-01:2025-01-01"

For each side, computes event count, max magnitude, and total seismic energy
(in joules, derived from log10 E = 1.5M + 4.8). Outputs the delta as both
absolute and percentage.`,
		Example: strings.Trim(`
  # Compare two cities' activity over the past 30 days
  usgs-earthquakes-pp-cli compare --region-a -122.5,37.5,-122.0,37.9 --region-b -118.5,33.8,-118.0,34.2 --window 30d --json

  # Has California gotten quieter? 2020 vs 2024
  usgs-earthquakes-pp-cli compare --region -125,32,-114,42 --period-a 2020-01-01:2021-01-01 --period-b 2024-01-01:2025-01-01 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()

			var sideA, sideB *compareSide
			var err error
			switch {
			case regionA != "" && regionB != "":
				wA, sA, eA, nA, perr := parseBBox(regionA)
				if perr != nil {
					return usageErr(fmt.Errorf("--region-a: %w", perr))
				}
				wB, sB, eB, nB, perr := parseBBox(regionB)
				if perr != nil {
					return usageErr(fmt.Errorf("--region-b: %w", perr))
				}
				start, perr := parseSinceArg(window)
				if perr != nil {
					return usageErr(perr)
				}
				sideA = &compareSide{Label: regionA, BboxW: wA, BboxS: sA, BboxE: eA, BboxN: nA, StartTime: start, EndTime: time.Now().UTC()}
				sideB = &compareSide{Label: regionB, BboxW: wB, BboxS: sB, BboxE: eB, BboxN: nB, StartTime: start, EndTime: time.Now().UTC()}
			case region != "" && periodA != "" && periodB != "":
				w, s, e, n, perr := parseBBox(region)
				if perr != nil {
					return usageErr(fmt.Errorf("--region: %w", perr))
				}
				stA, etA, perr := parsePeriod(periodA)
				if perr != nil {
					return usageErr(fmt.Errorf("--period-a: %w", perr))
				}
				stB, etB, perr := parsePeriod(periodB)
				if perr != nil {
					return usageErr(fmt.Errorf("--period-b: %w", perr))
				}
				sideA = &compareSide{Label: "period-a:" + periodA, BboxW: w, BboxS: s, BboxE: e, BboxN: n, StartTime: stA, EndTime: etA}
				sideB = &compareSide{Label: "period-b:" + periodB, BboxW: w, BboxS: s, BboxE: e, BboxN: n, StartTime: stB, EndTime: etB}
			default:
				return usageErr(fmt.Errorf("either --region-a + --region-b (with --window) or --region + --period-a + --period-b is required"))
			}

			err = computeSide(ctx, flags, sideA, minMag, dataSrc)
			if err != nil {
				return err
			}
			err = computeSide(ctx, flags, sideB, minMag, dataSrc)
			if err != nil {
				return err
			}

			deltaCount := sideB.Count - sideA.Count
			var deltaPct float64
			if sideA.Count > 0 {
				deltaPct = round2(float64(deltaCount) / float64(sideA.Count) * 100)
			}
			energyRatio := 0.0
			if sideA.TotalEnergy > 0 {
				energyRatio = round2(sideB.TotalEnergy / sideA.TotalEnergy)
			}

			out := map[string]any{
				"side_a":                sideA,
				"side_b":                sideB,
				"delta_count":           deltaCount,
				"delta_pct":             deltaPct,
				"energy_ratio_b_over_a": energyRatio,
				"params": map[string]any{
					"min_mag": minMag,
				},
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "SIDE\tLABEL\tCOUNT\tMAX_MAG\tTOTAL_ENERGY_J\tWINDOW")
			fmt.Fprintf(w, "A\t%s\t%d\tM%.1f\t%.2e\t%s — %s\n",
				sideA.Label, sideA.Count, sideA.MaxMag, sideA.TotalEnergy,
				sideA.StartTime.Format(time.RFC3339), sideA.EndTime.Format(time.RFC3339))
			fmt.Fprintf(w, "B\t%s\t%d\tM%.1f\t%.2e\t%s — %s\n",
				sideB.Label, sideB.Count, sideB.MaxMag, sideB.TotalEnergy,
				sideB.StartTime.Format(time.RFC3339), sideB.EndTime.Format(time.RFC3339))
			fmt.Fprintf(w, "\nDelta count (B-A)\t%d (%.1f%%)\n", deltaCount, deltaPct)
			fmt.Fprintf(w, "Energy ratio (B/A)\t%.2f\n", energyRatio)
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&regionA, "region-a", "", `Region A bbox "W,S,E,N" (region-mode)`)
	cmd.Flags().StringVar(&regionB, "region-b", "", `Region B bbox "W,S,E,N" (region-mode)`)
	cmd.Flags().StringVar(&periodA, "period-a", "", `Period A "<start>:<end>" or single year "2020" (period-mode)`)
	cmd.Flags().StringVar(&periodB, "period-b", "", `Period B "<start>:<end>" or single year "2024" (period-mode)`)
	cmd.Flags().StringVar(&region, "region", "", `Single region bbox "W,S,E,N" (period-mode)`)
	cmd.Flags().StringVar(&window, "window", "30d", "Lookback window (region-mode)")
	cmd.Flags().Float64Var(&minMag, "min-mag", 2.5, "Minimum magnitude")
	cmd.Flags().StringVar(&dataSrc, "data-source", "auto", "Data source: auto, live, local")
	return cmd
}

type compareSide struct {
	Label            string    `json:"label"`
	BboxW            float64   `json:"bbox_w"`
	BboxS            float64   `json:"bbox_s"`
	BboxE            float64   `json:"bbox_e"`
	BboxN            float64   `json:"bbox_n"`
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
	Count            int       `json:"count"`
	MaxMag           float64   `json:"max_mag"`
	TotalEnergy      float64   `json:"total_energy_j"`
	EnergyTruncated  bool      `json:"energy_truncated,omitempty"`
	EnergySampleSize int       `json:"energy_sample_size,omitempty"`
}

// parsePeriod accepts "YYYY-MM-DD:YYYY-MM-DD", "YYYY:YYYY", or single year "YYYY".
func parsePeriod(s string) (time.Time, time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("empty period")
	}
	if !strings.Contains(s, ":") {
		// Single year shorthand.
		t, err := time.Parse("2006", s)
		if err == nil {
			return t, t.AddDate(1, 0, 0), nil
		}
		return time.Time{}, time.Time{}, fmt.Errorf("could not parse period %q (expected YYYY-MM-DD:YYYY-MM-DD or YYYY)", s)
	}
	parts := strings.SplitN(s, ":", 2)
	start, err1 := parseDateOrYear(parts[0])
	end, err2 := parseDateOrYear(parts[1])
	if err1 != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("could not parse period start %q", parts[0])
	}
	if err2 != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("could not parse period end %q", parts[1])
	}
	return start, end, nil
}

func parseDateOrYear(s string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02", "2006-01", "2006"} {
		if t, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unparseable: %s", s)
}

func computeSide(ctx context.Context, flags *rootFlags, side *compareSide, minMag float64, dataSrc string) error {
	// Try local first when allowed.
	if dataSrc != "live" {
		db, err := openLocalStore(ctx)
		if err == nil {
			defer db.Close()
			rows, err := db.DB().QueryContext(ctx, `
				SELECT json_extract(data, '$.properties.mag') AS mag,
				       json_extract(data, '$.geometry.coordinates[0]') AS lon,
				       json_extract(data, '$.geometry.coordinates[1]') AS lat,
				       json_extract(data, '$.properties.time') AS t
				FROM resources
				WHERE resource_type='events'
				  AND CAST(t AS INTEGER) BETWEEN ? AND ?
			`, side.StartTime.UnixMilli(), side.EndTime.UnixMilli())
			if err == nil {
				defer rows.Close()
				count := 0
				var max, energy float64
				for rows.Next() {
					var mag, lat, lon sql.NullFloat64
					var t sql.NullInt64
					if rows.Scan(&mag, &lon, &lat, &t) != nil {
						continue
					}
					if !mag.Valid || mag.Float64 < minMag {
						continue
					}
					if !lat.Valid || !lon.Valid {
						continue
					}
					if lon.Float64 < side.BboxW || lon.Float64 > side.BboxE {
						continue
					}
					if lat.Float64 < side.BboxS || lat.Float64 > side.BboxN {
						continue
					}
					count++
					if mag.Float64 > max {
						max = mag.Float64
					}
					energy += math.Pow(10, 1.5*mag.Float64+4.8)
				}
				if count > 0 || dataSrc == "local" {
					side.Count = count
					side.MaxMag = max
					side.TotalEnergy = energy
					return nil
				}
			}
		}
		if dataSrc == "local" {
			return nil
		}
	}

	// Live fallback uses two FDSN calls:
	//   1. /count (text response, no 20k cap) → exact event count regardless
	//      of how many events fall in the window
	//   2. /query (capped at FDSN's hard 20000 ceiling) → top-magnitude
	//      sample for max_mag and total_energy
	// When count > 20000, total_energy reflects only the top-magnitude
	// sample; EnergyTruncated=true signals that to the consumer. Counting
	// via len(features) on a single /query call (the prior shape) silently
	// capped count at 20000 and miscounted both energy and the delta for
	// any seismically active region.
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	filterParams := map[string]string{
		"starttime":    fdsnTimeFormat(side.StartTime),
		"endtime":      fdsnTimeFormat(side.EndTime),
		"minlongitude": strconv.FormatFloat(side.BboxW, 'f', -1, 64),
		"minlatitude":  strconv.FormatFloat(side.BboxS, 'f', -1, 64),
		"maxlongitude": strconv.FormatFloat(side.BboxE, 'f', -1, 64),
		"maxlatitude":  strconv.FormatFloat(side.BboxN, 'f', -1, 64),
		"minmagnitude": strconv.FormatFloat(minMag, 'f', -1, 64),
	}

	// Step 1: exact count via /count (text response). FDSN's /count has no
	// 20000 cap.
	countParams := map[string]string{"format": "text"}
	for k, v := range filterParams {
		countParams[k] = v
	}
	countData, err := c.Get("/count", countParams)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	// /count?format=text returns the integer literal (possibly with trailing
	// newline) or a small JSON envelope wrapping it depending on the client.
	exactCount, err := strconv.Atoi(strings.TrimSpace(string(countData)))
	if err != nil {
		// Fall back to numeric extraction if the body is wrapped.
		exactCount = 0
		for _, b := range countData {
			if b >= '0' && b <= '9' {
				exactCount = exactCount*10 + int(b-'0')
			} else if exactCount > 0 {
				break
			}
		}
	}
	side.Count = exactCount

	// Step 2: top-magnitude sample for max + energy via /query.
	queryParams := map[string]string{
		"format":  "geojson",
		"limit":   "20000",
		"orderby": "magnitude",
	}
	for k, v := range filterParams {
		queryParams[k] = v
	}
	data, err := c.Get("/query", queryParams)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	var fc struct {
		Features []map[string]any `json:"features"`
	}
	if json.Unmarshal(data, &fc) != nil {
		return fmt.Errorf("parse compare response")
	}
	side.EnergySampleSize = len(fc.Features)
	if exactCount > len(fc.Features) {
		side.EnergyTruncated = true
	}
	for _, f := range fc.Features {
		props, _ := f["properties"].(map[string]any)
		mag, _ := props["mag"].(float64)
		if mag > side.MaxMag {
			side.MaxMag = mag
		}
		side.TotalEnergy += math.Pow(10, 1.5*mag+4.8)
	}
	return nil
}

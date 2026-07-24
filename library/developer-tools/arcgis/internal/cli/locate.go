// Copyright 2026 togorashi45 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: bind a coordinate (or a CSV of coordinates) to the containing
// feature via a point-intersects query. The RE signal-to-parcel bind.

package cli

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/arcgis/internal/arcgis"

	"github.com/spf13/cobra"
)

// pp:client-call — hits the ArcGIS REST API via c.QueryPage (arcgis client), which the reimplementation regex does not recognize.
// pp:data-source live
func newNovelLocateCmd(flags *rootFlags) *cobra.Command {
	var (
		point   string
		csvPath string
		lonCol  string
		latCol  string
		fields  string
		geom    bool
	)
	cmd := &cobra.Command{
		Use:   "locate <layer-url>",
		Short: "Bind a coordinate (or a CSV of coordinates) to the feature that contains it",
		Long: `Find the feature(s) in a layer that contain a point, via one intersects query.

Use this to attach a lat/lng distress signal (obituary, court record, permit) to
the exact parcel polygon that contains it, instead of fuzzy address matching.

Provide a single point with --point lon,lat, or a batch with --csv pointing at a
file that has longitude and latitude columns (defaults: lon/latitude column names
are auto-detected, or set --lon-col/--lat-col).`,
		Example: strings.Trim(`
  arcgis-pp-cli locate <parcel-layer-url> --point -101.87,33.58 --agent
  arcgis-pp-cli locate <parcel-layer-url> --csv signals.csv --lon-col lng --lat-col lat`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "url=https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer/0;--point=-71.46568,42.42269"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would locate the containing feature(s) for the given point(s)")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a layer URL is required"))
			}
			if point == "" && csvPath == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide --point lon,lat or --csv <file>"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newArcClient(flags)
			if err != nil {
				return err
			}
			layerURL := args[0]

			type binding struct {
				Lon      float64          `json:"lon"`
				Lat      float64          `json:"lat"`
				Matched  int              `json:"matched"`
				Features []arcgis.Feature `json:"features"`
				Error    string           `json:"error,omitempty"`
			}

			locateOne := func(lon, lat float64) binding {
				q := arcgis.QueryOptions{
					Where:        "1=1",
					OutFields:    orDefault(fields, "*"),
					OutSR:        "4326",
					ReturnGeom:   geom,
					Geometry:     fmt.Sprintf("%g,%g", lon, lat),
					GeometryType: "esriGeometryPoint",
					SpatialRel:   "esriSpatialRelIntersects",
				}
				res, err := c.QueryPage(ctx, layerURL, q)
				b := binding{Lon: lon, Lat: lat}
				if err != nil {
					b.Error = err.Error()
					return b
				}
				b.Features = res.Features
				b.Matched = len(res.Features)
				return b
			}

			var results []binding
			if point != "" {
				lon, lat, err := parsePoint(point)
				if err != nil {
					return usageErr(err)
				}
				results = append(results, locateOne(lon, lat))
			}
			if csvPath != "" {
				pts, err := readPointsCSV(csvPath, lonCol, latCol)
				if err != nil {
					return err
				}
				for _, p := range pts {
					results = append(results, locateOne(p[0], p[1]))
				}
			}

			if len(results) == 1 && point != "" && csvPath == "" {
				// Single-point convenience: return just its binding object.
				return printJSONFiltered(cmd.OutOrStdout(), results[0], flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&point, "point", "", "a single point as lon,lat")
	cmd.Flags().StringVar(&csvPath, "csv", "", "CSV file of points to locate")
	cmd.Flags().StringVar(&lonCol, "lon-col", "", "longitude column name in the CSV (auto-detected if blank)")
	cmd.Flags().StringVar(&latCol, "lat-col", "", "latitude column name in the CSV (auto-detected if blank)")
	cmd.Flags().StringVar(&fields, "fields", "*", "comma-separated outFields to return from matched features")
	cmd.Flags().BoolVar(&geom, "geometry", false, "include matched feature geometry")
	return cmd
}

func parsePoint(s string) (lon, lat float64, err error) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("--point must be lon,lat (got %q)", s)
	}
	lon, err = strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("bad longitude %q: %w", parts[0], err)
	}
	lat, err = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("bad latitude %q: %w", parts[1], err)
	}
	return lon, lat, nil
}

func readPointsCSV(path, lonCol, latCol string) ([][2]float64, error) {
	f, err := os.Open(path) // #nosec G304 -- path is the user-supplied --points-csv input file; reading it is the command's purpose.
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV header: %w", err)
	}
	li, la := -1, -1
	for i, h := range header {
		hl := strings.ToLower(strings.TrimSpace(h))
		if lonCol != "" {
			if hl == strings.ToLower(lonCol) {
				li = i
			}
		} else if li == -1 && (hl == "lon" || hl == "lng" || hl == "long" || hl == "longitude" || hl == "x") {
			li = i
		}
		if latCol != "" {
			if hl == strings.ToLower(latCol) {
				la = i
			}
		} else if la == -1 && (hl == "lat" || hl == "latitude" || hl == "y") {
			la = i
		}
	}
	if li == -1 || la == -1 {
		return nil, fmt.Errorf("could not find longitude/latitude columns in %s (set --lon-col/--lat-col)", path)
	}
	var pts [][2]float64
	line := 1
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		line++
		if li >= len(rec) || la >= len(rec) {
			continue
		}
		lon, e1 := strconv.ParseFloat(strings.TrimSpace(rec[li]), 64)
		lat, e2 := strconv.ParseFloat(strings.TrimSpace(rec[la]), 64)
		if e1 != nil || e2 != nil {
			continue
		}
		pts = append(pts, [2]float64{lon, lat})
	}
	if len(pts) == 0 {
		return nil, fmt.Errorf("no valid coordinate rows in %s", path)
	}
	return pts, nil
}

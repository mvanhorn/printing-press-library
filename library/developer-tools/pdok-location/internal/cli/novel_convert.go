// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// newConvertCmd is the parent of all offline geometry/coordinate converters.
// Pure-math; no API calls. Useful when an agent has WKT or RD coords from
// PDOK responses and needs GeoJSON / WGS84 for downstream tooling.
func newConvertCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert",
		Short: "Offline coordinate and geometry conversions (RD ↔ WGS84, WKT ↔ GeoJSON)",
		Long: "Pure-math conversions between Dutch RD (EPSG:28992) coordinates and " +
			"WGS84 (EPSG:4326), and between WKT and GeoJSON geometries. All " +
			"sub-commands run entirely offline (no API call).",
		Example: "  pdok-location-pp-cli convert rd-to-ll 121200 488000\n" +
			"  pdok-location-pp-cli convert ll-to-rd 4.8922 52.3731\n" +
			"  pdok-location-pp-cli convert wkt-to-geojson 'POINT(4.76 52.64)'",
	}
	cmd.AddCommand(newConvertRDToLLCmd(flags))
	cmd.AddCommand(newConvertLLToRDCmd(flags))
	cmd.AddCommand(newConvertWKTToGeoJSONCmd(flags))
	cmd.AddCommand(newConvertGeoJSONToWKTCmd(flags))
	return cmd
}

func newConvertRDToLLCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rd-to-ll [x] [y]",
		Short: "Convert Dutch RD (EPSG:28992) x/y to WGS84 lon/lat",
		Long: "Convert a Dutch RD coordinate pair (EPSG:28992, used by every PDOK " +
			"`centroide_rd` field) to WGS84 lon/lat. Reads pairs from positional " +
			"args, or one pair per stdin line when no args are given.",
		Example: "  pdok-location-pp-cli convert rd-to-ll 121200 488000 --json\n" +
			"  echo '155000 463000' | pdok-location-pp-cli convert rd-to-ll --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			pairs, err := readCoordPairs(args, cmd.InOrStdin())
			if err != nil {
				return usageErr(err)
			}
			if len(pairs) == 0 {
				return cmd.Help()
			}
			results := make([]map[string]any, 0, len(pairs))
			for _, p := range pairs {
				lon, lat := rdToWGS84(p[0], p[1])
				results = append(results, map[string]any{
					"rd":    map[string]float64{"x": p[0], "y": p[1]},
					"wgs84": map[string]float64{"lon": lon, "lat": lat},
				})
			}
			return emitConverted(cmd, flags, results)
		},
	}
	return cmd
}

func newConvertLLToRDCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ll-to-rd [lon] [lat]",
		Short: "Convert WGS84 lon/lat to Dutch RD (EPSG:28992) x/y",
		Long: "Convert a WGS84 (lon, lat) pair to Dutch RD x/y. WGS84 input order " +
			"is lon-first to match PDOK's `centroide_ll` ordering. Reads pairs " +
			"from positional args, or one pair per stdin line when no args are " +
			"given.",
		Example: "  pdok-location-pp-cli convert ll-to-rd 4.8922 52.3731 --json\n" +
			"  echo '5.387206 52.155174' | pdok-location-pp-cli convert ll-to-rd",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			pairs, err := readCoordPairs(args, cmd.InOrStdin())
			if err != nil {
				return usageErr(err)
			}
			if len(pairs) == 0 {
				return cmd.Help()
			}
			results := make([]map[string]any, 0, len(pairs))
			for _, p := range pairs {
				x, y := wgs84ToRD(p[0], p[1])
				results = append(results, map[string]any{
					"wgs84": map[string]float64{"lon": p[0], "lat": p[1]},
					"rd":    map[string]float64{"x": x, "y": y},
				})
			}
			return emitConverted(cmd, flags, results)
		},
	}
	return cmd
}

func newConvertWKTToGeoJSONCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wkt-to-geojson [wkt]",
		Short: "Convert a WKT geometry literal to GeoJSON",
		Long: "Parse a WKT geometry literal (POINT, MULTIPOINT, LINESTRING, " +
			"MULTILINESTRING, POLYGON, MULTIPOLYGON) and emit it as a GeoJSON " +
			"geometry object. Coordinate order is preserved (WKT POINT(x y) " +
			"becomes [x, y]; the caller must know whether x/y mean lon/lat or " +
			"RD). Reads from positional arg or stdin.",
		Example: "  pdok-location-pp-cli convert wkt-to-geojson 'POINT(4.76 52.64)'\n" +
			"  pdok-location-pp-cli lookup --id <id> --json | jq -r '.results.response.docs[0].geometrie_ll' | pdok-location-pp-cli convert wkt-to-geojson",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			wkt, err := readSingleStringArg(args, cmd.InOrStdin())
			if err != nil {
				return usageErr(err)
			}
			if wkt == "" {
				return cmd.Help()
			}
			g, err := wktToGeoJSON(wkt)
			if err != nil {
				return usageErr(err)
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(g)
		},
	}
	return cmd
}

func newConvertGeoJSONToWKTCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "geojson-to-wkt [geojson]",
		Short: "Convert a GeoJSON geometry to its WKT literal",
		Long: "Parse a GeoJSON geometry object (Point, MultiPoint, LineString, " +
			"MultiLineString, Polygon, MultiPolygon) and emit the equivalent WKT " +
			"literal. Reads JSON from positional arg or stdin.",
		Example: "  pdok-location-pp-cli convert geojson-to-wkt '{\"type\":\"Point\",\"coordinates\":[4.76,52.64]}'\n" +
			"  cat poly.json | pdok-location-pp-cli convert geojson-to-wkt",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			raw, err := readSingleStringArg(args, cmd.InOrStdin())
			if err != nil {
				return usageErr(err)
			}
			if raw == "" {
				return cmd.Help()
			}
			var g map[string]any
			if err := json.Unmarshal([]byte(raw), &g); err != nil {
				return usageErr(fmt.Errorf("invalid JSON: %w", err))
			}
			wkt, err := geoJSONToWKT(g)
			if err != nil {
				return usageErr(err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), wkt)
			return nil
		},
	}
	return cmd
}

// readCoordPairs collects (x, y) pairs from positional args (interpreted as
// alternating x y x y ...) or from stdin (one "x y" pair per line). Returns
// empty slice when neither source provides any.
func readCoordPairs(args []string, stdin interface{ Read([]byte) (int, error) }) ([][2]float64, error) {
	if len(args) > 0 {
		if len(args)%2 != 0 {
			return nil, fmt.Errorf("expected pairs of coordinates, got %d values", len(args))
		}
		out := make([][2]float64, 0, len(args)/2)
		for i := 0; i < len(args); i += 2 {
			x, err := strconv.ParseFloat(args[i], 64)
			if err != nil {
				return nil, fmt.Errorf("invalid coord %q: %w", args[i], err)
			}
			y, err := strconv.ParseFloat(args[i+1], 64)
			if err != nil {
				return nil, fmt.Errorf("invalid coord %q: %w", args[i+1], err)
			}
			out = append(out, [2]float64{x, y})
		}
		return out, nil
	}
	if stdin == nil {
		return nil, nil
	}
	scanner := bufio.NewScanner(stdin)
	var out [][2]float64
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("invalid stdin line %q: need two numbers", line)
		}
		x, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			return nil, err
		}
		y, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return nil, err
		}
		out = append(out, [2]float64{x, y})
	}
	return out, scanner.Err()
}

// readSingleStringArg returns the first positional arg or, when there are
// none, the entirety of stdin (trimmed). Empty when neither source has data.
func readSingleStringArg(args []string, stdin interface{ Read([]byte) (int, error) }) (string, error) {
	if len(args) > 0 {
		return strings.TrimSpace(args[0]), nil
	}
	if stdin == nil {
		return "", nil
	}
	buf := make([]byte, 0, 1024)
	tmp := make([]byte, 1024)
	for {
		n, err := stdin.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return strings.TrimSpace(string(buf)), nil
}

func emitConverted(cmd *cobra.Command, flags *rootFlags, results []map[string]any) error {
	// Always default to JSON for converters — they're machine-shaped.
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) || len(results) > 1 {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}
	// Human single-pair output.
	r := results[0]
	if rd, ok := r["rd"].(map[string]float64); ok {
		fmt.Fprintf(cmd.OutOrStdout(), "rd: x=%.3f y=%.3f\n", rd["x"], rd["y"])
	}
	if w, ok := r["wgs84"].(map[string]float64); ok {
		fmt.Fprintf(cmd.OutOrStdout(), "wgs84: lon=%.7f lat=%.7f\n", w["lon"], w["lat"])
	}
	_ = os.Stderr
	return nil
}

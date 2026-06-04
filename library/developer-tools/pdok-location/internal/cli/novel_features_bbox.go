// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/pdok-location/internal/cliutil"
	"github.com/spf13/cobra"
)

// validCollections lists the Location API collections supported by `features
// search`. Mirrors the spec's enumerated collections so a typo fails fast.
var validCollections = map[string]bool{
	"adres": true, "functioneel_gebied": true, "gebouw": true,
	"gemeentegebied": true, "geografisch_gebied": true, "inrichtingselement": true,
	"perceel": true, "plaats": true, "provinciegebied": true, "spoorbaandeel": true,
	"waterdeel": true, "wegdeel": true, "woonplaats": true,
}

type bboxFeature struct {
	Collection string                     `json:"collection"`
	ID         any                        `json:"id"`
	Properties map[string]json.RawMessage `json:"properties,omitempty"`
	Geometry   any                        `json:"geometry,omitempty"`
	BBox       any                        `json:"bbox,omitempty"`
}

// newFeaturesCmd parents Location API feature commands.
func newFeaturesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "features",
		Short: "Kadaster Location API (OGC features) helpers",
	}
	cmd.AddCommand(newFeaturesSearchCmd(flags))
	return cmd
}

// newFeaturesSearchCmd: friendly wrapper around the Location API `/search`
// endpoint. The raw endpoint requires per-collection opt-in via `?<col>[version]=1`
// syntax and a mandatory `q` text query. This command takes a single `--query`
// plus a comma-separated `--collections` list, optionally a `--bbox`, and emits
// either GeoJSON-style JSON or a flat CSV across all matched collections.
func newFeaturesSearchCmd(flags *rootFlags) *cobra.Command {
	var bbox string
	var collectionsCSV string
	var query string
	var limit int
	var crs string
	var asCSV bool
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search across multiple OGC collections (with optional bbox filter)",
		Long: "Friendly wrapper around the Location API `/search` endpoint. The raw " +
			"endpoint requires both a `q` query term AND per-collection opt-in via " +
			"a bracketed `<col>[version]=1` syntax — this command takes a single " +
			"`--query` plus a comma-separated `--collections` list and handles the " +
			"encoding. With `--bbox` the search is spatially filtered; without it " +
			"the search is national. The value-add over hitting `/search` directly " +
			"is the friendly collections flag, the optional CSV flatten across " +
			"collections, and CRS friendly-name resolution.",
		Example: "  pdok-location-pp-cli features search --query Damrak --collections adres --limit 5\n" +
			"  pdok-location-pp-cli features search --query amsterdam --collections adres,perceel --bbox 4.85,52.36,4.92,52.40 --csv",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("query") || !cmd.Flags().Changed("collections") {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			cols := strings.Split(collectionsCSV, ",")
			for i, c := range cols {
				cols[i] = strings.TrimSpace(c)
				if !validCollections[cols[i]] {
					return usageErr(fmt.Errorf("unknown collection %q (valid: %s)", cols[i], collectionNames()))
				}
			}
			if bbox != "" {
				parts := strings.Split(bbox, ",")
				if len(parts) != 4 {
					return usageErr(fmt.Errorf("--bbox must be 'minlon,minlat,maxlon,maxlat' (got %q)", bbox))
				}
				for _, p := range parts {
					if _, err := strconv.ParseFloat(strings.TrimSpace(p), 64); err != nil {
						return usageErr(fmt.Errorf("--bbox values must be numbers: %q is not a valid float", strings.TrimSpace(p)))
					}
				}
			}
			if limit < 1 {
				limit = 25
			}
			if cliutil.IsDogfoodEnv() {
				limit = 5
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			crsURI := resolveCRS(crs)

			// Build query parameters in the exact form `/search` expects.
			// Each collection contributes `<col>[version]=1`.
			params := url.Values{}
			params.Set("q", query)
			params.Set("limit", fmt.Sprintf("%d", limit))
			params.Set("f", "json")
			if bbox != "" {
				params.Set("bbox", bbox)
				if crsURI != "" {
					params.Set("bbox-crs", crsURI)
				}
			}
			if crsURI != "" {
				params.Set("crs", crsURI)
			}
			for _, col := range cols {
				params.Set(fmt.Sprintf("%s[version]", col), "1")
			}

			// We can't reuse c.Get(map[string]string{}) because its param
			// shape doesn't allow bracketed keys; encode the path with the
			// query string baked in.
			pathWithQuery := "/kadaster/location-api/v1/search?" + params.Encode()
			data, err := c.Get(pathWithQuery, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp struct {
				Features []bboxFeature `json:"features"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return apiErr(err)
			}
			// Decorate each feature with its collection from the response
			// properties (the `/search` endpoint stamps `collection_id`).
			perCollection := map[string]int{}
			for _, c := range cols {
				perCollection[c] = 0
			}
			for i := range resp.Features {
				if v, ok := resp.Features[i].Properties["collection_id"]; ok {
					var s string
					if json.Unmarshal(v, &s) == nil {
						resp.Features[i].Collection = s
						perCollection[s]++
					}
				}
			}
			// Surface collections that the user asked for but produced zero
			// matches, so silent fan-out drops are visible. Only emit when
			// at least one requested collection returned nothing AND output
			// is not the agent/machine JSON envelope (stderr is the wrong
			// surface for agents that pipe stdout).
			if !flags.agent && !flags.quiet {
				var empties []string
				for _, c := range cols {
					if perCollection[c] == 0 {
						empties = append(empties, c)
					}
				}
				if len(empties) > 0 && len(empties) < len(cols) {
					fmt.Fprintf(cmd.ErrOrStderr(), "note: 0 matches for: %s\n", strings.Join(empties, ", "))
				}
			}

			if asCSV {
				return writeBboxCSV(cmd.OutOrStdout(), resp.Features)
			}
			out := map[string]any{
				"query":       query,
				"collections": cols,
				"count":       len(resp.Features),
				"features":    resp.Features,
			}
			if bbox != "" {
				out["bbox"] = bbox
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Text query (required by the /search endpoint)")
	cmd.Flags().StringVar(&bbox, "bbox", "", "Bounding box 'minlon,minlat,maxlon,maxlat' (WGS84 by default; change with --crs)")
	cmd.Flags().StringVar(&collectionsCSV, "collections", "", "Comma-separated OGC collections (adres, perceel, gebouw, gemeentegebied, ...)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max results")
	cmd.Flags().StringVar(&crs, "crs", "", "CRS shorthand: wgs84, rd, webmerc, etrs89")
	cmd.Flags().BoolVar(&asCSV, "csv", false, "Flatten properties to a CSV")
	return cmd
}

func collectionNames() string {
	names := make([]string, 0, len(validCollections))
	for k := range validCollections {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func resolveCRS(name string) string {
	switch strings.ToLower(name) {
	case "", "wgs84":
		return "http://www.opengis.net/def/crs/OGC/1.3/CRS84"
	case "rd", "epsg:28992":
		return "http://www.opengis.net/def/crs/EPSG/0/28992"
	case "webmerc", "epsg:3857":
		return "http://www.opengis.net/def/crs/EPSG/0/3857"
	case "etrs89", "epsg:4258":
		return "http://www.opengis.net/def/crs/EPSG/0/4258"
	}
	return name
}

func writeBboxCSV(w io.Writer, items []bboxFeature) error {
	if len(items) == 0 {
		fmt.Fprintln(w, "collection,id")
		return nil
	}
	keys := map[string]struct{}{}
	for _, it := range items {
		for k := range it.Properties {
			keys[k] = struct{}{}
		}
	}
	colNames := make([]string, 0, len(keys))
	for k := range keys {
		colNames = append(colNames, k)
	}
	sort.Strings(colNames)
	cw := csv.NewWriter(w)
	header := append([]string{"collection", "id"}, colNames...)
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, it := range items {
		row := []string{it.Collection, fmt.Sprintf("%v", it.ID)}
		for _, k := range colNames {
			if v, ok := it.Properties[k]; ok {
				var s string
				if json.Unmarshal(v, &s) == nil {
					// /search wraps matched spans in <b>...</b> for highlight
					// fields (e.g. `highlight_term`). Strip the markup for
					// CSV consumers — spreadsheets and downstream tools see
					// literal `<b>` otherwise. JSON output keeps the markup
					// so callers who want the highlight spans still have them.
					row = append(row, stripHighlights(s))
				} else {
					row = append(row, string(v))
				}
			} else {
				row = append(row, "")
			}
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

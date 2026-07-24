// Copyright 2026 togorashi45 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: full-layer extraction with correct pagination, OID fallback,
// bbox tiling, and GeoJSON/CSV/JSONL output.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/arcgis/internal/arcgis"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelQueryCmd(flags *rootFlags) *cobra.Command {
	var (
		where    string
		fields   string
		outSR    string
		orderBy  string
		bbox     string
		geom     bool
		format   string
		outPath  string
		pager    string
		pageSize int
		limit    int
		maxTiles int
	)
	cmd := &cobra.Command{
		Use:   "query <layer-url>",
		Short: "Pull features from a layer with automatic pagination and CSV/GeoJSON output",
		Long: `Pull every feature matching --where from an ArcGIS layer, correctly paginated.

Pagination is chosen automatically: resultOffset paging when the server supports
it, else OBJECTID chunking. Use --tile (or --pager tile) to subdivide the extent
for dense layers that exceed the transfer limit without pagination support.

Output formats: geojson (default), csv, jsonl (newline-delimited GeoJSON), json
(raw esri features). Geometry is included for geojson/jsonl by default and can be
turned on for other formats with --geometry.`,
		Example: strings.Trim(`
  arcgis-pp-cli query <layer-url> --format csv --out parcels.csv
  arcgis-pp-cli query <layer-url> --where "LANDUSE='VACANT'" --fields APN,OWNER
  arcgis-pp-cli query <layer-url> --bbox -101.9,33.5,-101.8,33.6 --format geojson
  arcgis-pp-cli query <layer-url> --tile --out big_layer.geojson`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "url=https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer/0;--limit=3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would pull features from the given layer URL")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a layer URL is required"))
			}
			format = strings.ToLower(format)
			switch format {
			case "geojson", "csv", "jsonl", "json":
			default:
				return usageErr(fmt.Errorf("unknown --format %q (use geojson, csv, jsonl, or json)", format))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newArcClient(flags)
			if err != nil {
				return err
			}
			layerURL := args[0]
			meta, err := c.LayerMeta(ctx, layerURL)
			if err != nil {
				return fmt.Errorf("reading layer metadata: %w", err)
			}

			returnGeom := geom || format == "geojson" || format == "jsonl"
			q := arcgis.QueryOptions{
				Where:      where,
				OutFields:  orDefault(fields, "*"),
				OutSR:      orDefault(outSR, "4326"),
				ReturnGeom: returnGeom,
				OrderBy:    orderBy,
			}
			if bbox != "" {
				q.Geometry = bbox
				q.GeometryType = "esriGeometryEnvelope"
				q.SpatialRel = "esriSpatialRelIntersects"
			}
			mode := arcgis.PagerMode(strings.ToLower(pager))
			if cmd.Flags().Changed("tile") {
				mode = arcgis.PagerTile
			}

			// Collect features (bounded by --limit). Streaming to disk for huge
			// layers is possible, but collecting keeps GeoJSON/CSV framing simple;
			// --limit plus tiling caps memory in practice.
			var feats []arcgis.Feature
			opts := arcgis.ExtractOptions{
				Query:    q,
				Mode:     mode,
				PageSize: pageSize,
				Limit:    limit,
				MaxTiles: maxTiles,
			}
			n, err := c.ExtractAll(ctx, layerURL, meta, opts, func(f arcgis.Feature) error {
				feats = append(feats, f)
				return nil
			})
			if err != nil {
				return fmt.Errorf("extracting features (%d pulled before error): %w", n, err)
			}

			w, closeOut, err := openOut(cmd, outPath)
			if err != nil {
				return err
			}
			defer closeOut()

			switch format {
			case "geojson":
				if err := writeFeatureCollection(w, feats, meta.GeometryType); err != nil {
					return err
				}
			case "jsonl":
				if err := writeJSONL(w, feats, meta.GeometryType); err != nil {
					return err
				}
			case "csv":
				var sample map[string]any
				if len(feats) > 0 {
					sample = feats[0].Attributes
				}
				cols := columnsFor(fields, meta.Fields, sample)
				if err := writeCSV(w, feats, cols); err != nil {
					return err
				}
			case "json":
				if err := printJSONFiltered(w, feats, flags); err != nil {
					return err
				}
			}
			if outPath != "" && outPath != "-" {
				fmt.Fprintf(cmd.ErrOrStderr(), "wrote %d features to %s\n", n, outPath)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&where, "where", "1=1", "SQL where clause to match features")
	cmd.Flags().StringVar(&fields, "fields", "*", "comma-separated outFields (default all)")
	cmd.Flags().StringVar(&outSR, "out-sr", "4326", "output spatial reference WKID (default 4326 lon/lat)")
	cmd.Flags().StringVar(&orderBy, "order-by", "", "orderByFields (defaults to the object-id field for stable paging)")
	cmd.Flags().StringVar(&bbox, "bbox", "", "spatial filter envelope: xmin,ymin,xmax,ymax (in lon/lat)")
	cmd.Flags().BoolVar(&geom, "geometry", false, "include geometry for csv/json output (always on for geojson/jsonl)")
	cmd.Flags().StringVar(&format, "format", "geojson", "output format: geojson, csv, jsonl, json")
	cmd.Flags().StringVar(&outPath, "out", "", "write to a file instead of stdout")
	cmd.Flags().StringVar(&pager, "pager", "auto", "pagination strategy: auto, offset, oid, tile")
	cmd.Flags().Bool("tile", false, "subdivide the extent into quadrants (for dense layers without pagination)")
	cmd.Flags().IntVar(&pageSize, "page-size", 0, "rows per request (default the layer maxRecordCount)")
	cmd.Flags().IntVar(&limit, "limit", 0, "stop after this many features (0 = all)")
	cmd.Flags().IntVar(&maxTiles, "max-tiles", 4096, "safety cap on tile subdivisions in tile mode")
	return cmd
}

// columnsFor resolves CSV column order from --fields, else the layer schema,
// else the sample feature keys.
func columnsFor(fieldsFlag string, layerFields []arcgis.Field, sample map[string]any) []string {
	if fieldsFlag != "" && fieldsFlag != "*" {
		parts := strings.Split(fieldsFlag, ",")
		cols := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				cols = append(cols, s)
			}
		}
		if len(cols) > 0 {
			return cols
		}
	}
	return arcgis.FieldNames(layerFields, sample)
}

// Copyright 2026 togorashi45 and contributors. Licensed under Apache-2.0. See LICENSE.
// URL-taking sync: mirror an ArcGIS layer's features into the local SQLite store
// so 'sql' and 'diff' can run offline. Replaces the generated base-URL sync.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/arcgis/internal/arcgis"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/arcgis/internal/store"

	"github.com/spf13/cobra"
)

// storedFeature is the row shape written to the local store for each feature.
type storedFeature struct {
	ID         string          `json:"id"`
	LayerURL   string          `json:"layer_url"`
	OID        int64           `json:"oid"`
	Attributes map[string]any  `json:"attributes"`
	Geometry   json.RawMessage `json:"geometry,omitempty"`
}

func featureStoreID(layerURL string, oid int64) string {
	return fmt.Sprintf("%s#%d", layerURL, oid)
}

func newArcSyncCmd(flags *rootFlags) *cobra.Command {
	var (
		where    string
		fields   string
		dbPath   string
		pager    string
		pageSize int
		limit    int
		geom     bool
	)
	cmd := &cobra.Command{
		Use:   "sync <layer-url>",
		Short: "Mirror an ArcGIS layer into the local SQLite store for offline sql/diff",
		Long: `Pull a layer's features into the local SQLite store, keyed by layer URL and
OBJECTID. Run this once, then use 'sql' to query offline and 'diff' to detect
attribute changes on the next sync. Re-running sync refreshes the baseline.`,
		Example:     "  arcgis-pp-cli sync <layer-url> --fields APN,OWNER,LANDUSE",
		Annotations: map[string]string{"pp:happy-args": "url=https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer/0;--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would sync the given layer URL into the local store")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a layer URL is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newArcClient(flags)
			if err != nil {
				return err
			}
			layerURL := arcgis.NormalizeLayerURL(args[0])
			meta, err := c.LayerMeta(ctx, layerURL)
			if err != nil {
				return fmt.Errorf("reading layer metadata: %w", err)
			}
			oidField := meta.ObjectIDField
			if oidField == "" {
				oidField = "OBJECTID"
			}

			if dbPath == "" {
				dbPath = defaultDBPath("arcgis-pp-cli")
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()
			if err := ensureFeaturesView(ctx, db); err != nil {
				return err
			}

			q := arcgis.QueryOptions{
				Where:      where,
				OutFields:  orDefault(fields, "*"),
				OutSR:      "4326",
				ReturnGeom: geom,
			}
			opts := arcgis.ExtractOptions{
				Query:    q,
				Mode:     arcgis.PagerMode(pager),
				PageSize: pageSize,
				Limit:    limit,
			}
			synced := 0
			n, err := c.ExtractAll(ctx, layerURL, meta, opts, func(f arcgis.Feature) error {
				oid, _ := f.OID(oidField)
				sf := storedFeature{
					ID:         featureStoreID(layerURL, oid),
					LayerURL:   layerURL,
					OID:        oid,
					Attributes: f.Attributes,
					Geometry:   f.Geometry,
				}
				data, err := json.Marshal(sf)
				if err != nil {
					return err
				}
				if err := db.Upsert("feature", sf.ID, data); err != nil {
					return err
				}
				synced++
				return nil
			})
			if err != nil {
				return fmt.Errorf("syncing features (%d written before error): %w", synced, err)
			}
			if flags.asJSON || flags.agent {
				b, _ := json.Marshal(map[string]any{"synced": n, "layer_url": layerURL, "db": dbPath})
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "synced %d features from %s into %s\n", n, layerURL, dbPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&where, "where", "1=1", "SQL where clause to match features")
	cmd.Flags().StringVar(&fields, "fields", "*", "comma-separated outFields to store")
	cmd.Flags().StringVar(&dbPath, "db", "", "store path (default the standard local db)")
	cmd.Flags().StringVar(&pager, "pager", "auto", "pagination strategy: auto, offset, oid")
	cmd.Flags().IntVar(&pageSize, "page-size", 0, "rows per request (default the layer maxRecordCount)")
	cmd.Flags().IntVar(&limit, "limit", 0, "stop after this many features (0 = all)")
	cmd.Flags().BoolVar(&geom, "geometry", false, "store feature geometry as well as attributes")
	return cmd
}

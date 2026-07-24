// Copyright 2026 togorashi45 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: re-query a layer and diff a tracked field against the last
// local sync to surface changes (e.g. ownership transfers).

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/arcgis/internal/arcgis"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/arcgis/internal/store"

	"github.com/spf13/cobra"
)

// pp:data-source auto — re-queries the layer live and diffs against the last local sync baseline in the store.
func newNovelDiffCmd(flags *rootFlags) *cobra.Command {
	var (
		key    string
		track  string
		where  string
		dbPath string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "diff <layer-url>",
		Short: "Re-query a layer and diff a tracked field against the last local sync",
		Long: `Compare a layer's current values against the local sync baseline and report which
records changed a tracked field. Surfaces ownership transfers, value changes, or
status flips between two pulls without any change feed from the server.

Run 'sync <layer-url>' first to establish the baseline, then 'diff' to see what
moved. --key is the record id field (default OBJECTID); --track is the field to
watch (e.g. OWNER).`,
		Example:     "  arcgis-pp-cli diff <layer-url> --track OWNER --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "url=https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer/0;--track=FLD_ZONE"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would diff current layer values against the local sync baseline")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a layer URL is required"))
			}
			if track == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--track <field> is required"))
			}
			if key == "" {
				key = "OBJECTID"
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			layerURL := arcgis.NormalizeLayerURL(args[0])

			if dbPath == "" {
				dbPath = defaultDBPath("arcgis-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local baseline at %s\nrun: arcgis-pp-cli sync %s --db %s\n", dbPath, layerURL, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			// Load baseline tracked values keyed by OID.
			baseline, err := loadBaseline(ctx, db, layerURL, key, track)
			if err != nil {
				return err
			}
			if len(baseline) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no baseline features for %s; run sync first\n", layerURL)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			// Pull current values for the key + tracked field only.
			c, err := newArcClient(flags)
			if err != nil {
				return err
			}
			meta, err := c.LayerMeta(ctx, layerURL)
			if err != nil {
				return fmt.Errorf("reading layer metadata: %w", err)
			}
			q := arcgis.QueryOptions{
				Where:     where,
				OutFields: fmt.Sprintf("%s,%s", key, track),
				OutSR:     "4326",
			}
			type change struct {
				Key  any `json:"key"`
				From any `json:"from"`
				To   any `json:"to"`
			}
			var changes []change
			seen := map[string]bool{}
			_, err = c.ExtractAll(ctx, layerURL, meta, arcgis.ExtractOptions{Query: q}, func(f arcgis.Feature) error {
				oid, ok := f.OID(key)
				if !ok {
					return nil
				}
				oidStr := fmt.Sprintf("%d", oid)
				seen[oidStr] = true
				cur := f.Attributes[track]
				old, had := baseline[oidStr]
				if had && !valuesEqual(old, cur) {
					changes = append(changes, change{Key: oid, From: old, To: cur})
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("pulling current values: %w", err)
			}
			if limit > 0 && len(changes) > limit {
				changes = changes[:limit]
			}
			view := map[string]any{
				"layer_url":      layerURL,
				"tracked_field":  track,
				"baseline_count": len(baseline),
				"changed":        len(changes),
				"changes":        changes,
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d of %d tracked records changed %s\n", len(changes), len(baseline), track)
			for _, ch := range changes {
				fmt.Fprintf(cmd.OutOrStdout(), "  %v: %v -> %v\n", ch.Key, ch.From, ch.To)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&key, "key", "OBJECTID", "record id field to match baseline against")
	cmd.Flags().StringVar(&track, "track", "", "field to watch for changes (required)")
	cmd.Flags().StringVar(&where, "where", "1=1", "SQL where clause limiting the current pull")
	cmd.Flags().StringVar(&dbPath, "db", "", "store path (default the standard local db)")
	cmd.Flags().IntVar(&limit, "limit", 0, "cap the number of changes returned (0 = all)")
	return cmd
}

// loadBaseline returns tracked-field values keyed by the record key string, from
// features previously synced for this layer.
func loadBaseline(ctx context.Context, db *store.Store, layerURL, key, track string) (map[string]any, error) {
	rows, err := db.DB().QueryContext(ctx,
		`SELECT data FROM resources WHERE resource_type='feature' AND json_extract(data,'$.layer_url')=?`, layerURL)
	if err != nil {
		return nil, fmt.Errorf("loading baseline: %w", err)
	}
	defer rows.Close()
	out := map[string]any{}
	for rows.Next() {
		var data sql.RawBytes
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var sf storedFeature
		if err := json.Unmarshal([]byte(data), &sf); err != nil {
			continue
		}
		// Match the current-pull side, which keys by f.OID(key). Prefer the
		// tracked key field when present in attributes, else the stored OID.
		keyStr := fmt.Sprintf("%d", sf.OID)
		if kv, ok := sf.Attributes[key]; ok && key != "OBJECTID" {
			keyStr = arcgis.AttrToString(kv)
		}
		out[keyStr] = sf.Attributes[track]
	}
	return out, rows.Err()
}

func valuesEqual(a, b any) bool {
	return arcgis.AttrToString(a) == arcgis.AttrToString(b)
}

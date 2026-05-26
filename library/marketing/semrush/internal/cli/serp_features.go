// Copyright 2026 charles-garrison. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature #10 — SERP feature monitor (time-series).

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newSerpFeaturesCmd(flags *rootFlags) *cobra.Command {
	var since string
	var database string

	cmd := &cobra.Command{
		Use:         "serp-features [keyword]",
		Short:       "Time-series of which SERP features (featured snippet, PAA, video) have appeared for a keyword.",
		Long:        "serp-features reads the SERP-feature flag columns (Fk, Fp) persisted by 'keyword organic-serp' and emits, per snapshot, which features were present.",
		Example:     "  semrush-pp-cli serp-features 'best running shoes' --since 30d",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openNovelStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			recordBalanceSnapshotForCmd(ctx, db, flags, cmd.CommandPath(), cmd.ErrOrStderr())

			if !hintIfUnsynced(cmd, db, "keyword") {
				hintIfStale(cmd, db, "keyword", flags.maxAge)
			}

			keyword := args[0]
			window, err := parseSince(since)
			if err != nil {
				return usageErr(err)
			}
			cutoff := time.Now().Add(-window)

			rows, err := db.DB().QueryContext(ctx,
				`SELECT COALESCE(json_extract(data, '$.Fk'), json_extract(data, '$.keyword_serp_features'), '') AS fk,
				        COALESCE(json_extract(data, '$.Fp'), json_extract(data, '$.serp_features'), '') AS fp,
				        synced_at,
				        COALESCE(json_extract(data, '$.database'), '') AS dbc
				 FROM resources
				 WHERE resource_type IN ('keyword', 'keyword_organic_serp')
				   AND (json_extract(data, '$.Ph') = ? OR json_extract(data, '$.phrase') = ?)
				   AND (? = '' OR json_extract(data, '$.database') = ? OR json_extract(data, '$.database') IS NULL)
				 ORDER BY synced_at ASC`,
				keyword, keyword, database, database)
			if err != nil {
				return fmt.Errorf("query keyword_organic_serp: %w", err)
			}
			defer rows.Close()

			type tick struct {
				TS        string   `json:"ts"`
				Features  []string `json:"features"`
				PageLevel []string `json:"page_features"`
				Database  string   `json:"database"`
			}
			featureFirstSeen := map[string]time.Time{}
			featureLastSeen := map[string]time.Time{}
			var series []tick

			for rows.Next() {
				var fk, fp, dbc string
				var when time.Time
				if err := rows.Scan(&fk, &fp, &when, &dbc); err != nil {
					return fmt.Errorf("scan serp row: %w", err)
				}
				if when.Before(cutoff) {
					continue
				}
				parsePresent := func(raw string) []string {
					raw = strings.TrimSpace(raw)
					if raw == "" || raw == "0" {
						return nil
					}
					// Semrush returns a comma-list of digits where each digit
					// is the index of a SERP feature. We pass through raw so
					// agents can interpret per Semrush docs.
					return strings.Split(raw, ",")
				}
				features := parsePresent(fk)
				pageFeatures := parsePresent(fp)
				for _, f := range append(features, pageFeatures...) {
					if _, ok := featureFirstSeen[f]; !ok {
						featureFirstSeen[f] = when
					}
					featureLastSeen[f] = when
				}
				series = append(series, tick{
					TS:        when.UTC().Format(time.RFC3339),
					Features:  features,
					PageLevel: pageFeatures,
					Database:  dbc,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate serp rows: %w", err)
			}

			type featureWindow struct {
				Feature   string `json:"feature"`
				FirstSeen string `json:"first_seen"`
				LastSeen  string `json:"last_seen"`
			}
			var windows []featureWindow
			for f, fs := range featureFirstSeen {
				windows = append(windows, featureWindow{
					Feature:   f,
					FirstSeen: fs.UTC().Format(time.RFC3339),
					LastSeen:  featureLastSeen[f].UTC().Format(time.RFC3339),
				})
			}
			sort.SliceStable(windows, func(i, j int) bool { return windows[i].Feature < windows[j].Feature })

			out := map[string]any{
				"keyword":   keyword,
				"since":     since,
				"database":  database,
				"snapshots": series,
				"windows":   windows,
			}
			raw, err := json.Marshal(out)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "Window to inspect (e.g. 30d, 12w)")
	cmd.Flags().StringVar(&database, "database", "us", "Semrush database/country to filter on; empty matches all")
	return cmd
}

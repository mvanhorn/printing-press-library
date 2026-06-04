// Copyright 2026 Matt and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: Phase 3 transcendence — T6 coverage-drift.

package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newCoverageDriftCmd(flags *rootFlags) *cobra.Command {
	var (
		field string
		days  int
	)

	cmd := &cobra.Command{
		Use:         "coverage-drift <site>",
		Short:       "URLs whose inspection state flipped (indexed → not indexed, robots changed, canonical changed)",
		Long:        "Pulls url_inspections snapshots ordered by (inspection_url, snapshot_at) for the last --days, then reports URLs whose chosen --field differs between the oldest and newest snapshot in the window. The API returns only the current state, so this is the only way to see flips.",
		Example:     "  google-search-console-pp-cli coverage-drift sc-domain:example.com --field indexingState --days 30 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			site := args[0]
			col, err := mapInspectionField(field)
			if err != nil {
				return usageErr(err)
			}
			startDate := daysAgo(days)
			ctx := context.Background()
			s, err := openStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer s.Close()

			var inspCount int64
			if err := s.DB().QueryRowContext(ctx,
				`SELECT COUNT(*) FROM url_inspections WHERE site_url = ?`, site).Scan(&inspCount); err != nil {
				return apiErr(err)
			}
			if inspCount == 0 {
				return notFoundErr(fmt.Errorf("local store has no url_inspections for %q; run `google-search-console-pp-cli url-inspection inspect-batch --file <urls.txt> --site %s` first to populate snapshots", site, site))
			}

			q := fmt.Sprintf(`
SELECT inspection_url, snapshot_at, COALESCE(%s,'') AS val
FROM url_inspections
WHERE site_url = ? AND snapshot_at >= ?
ORDER BY inspection_url, snapshot_at`, col)
			rows, err := s.DB().QueryContext(ctx, q, site, startDate)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()

			type entry struct{ ts, val string }
			byURL := map[string][]entry{}
			for rows.Next() {
				var url, ts, val string
				if err := rows.Scan(&url, &ts, &val); err != nil {
					return err
				}
				byURL[url] = append(byURL[url], entry{ts, val})
			}
			if err := rows.Err(); err != nil {
				return err
			}
			out := []map[string]any{}
			for url, entries := range byURL {
				if len(entries) < 2 {
					continue
				}
				oldest := entries[0]
				newest := entries[len(entries)-1]
				if oldest.val == newest.val {
					continue
				}
				out = append(out, map[string]any{
					"inspection_url": url,
					"field":          field,
					"old_value":      oldest.val,
					"new_value":      newest.val,
					"old_snapshot":   oldest.ts,
					"new_snapshot":   newest.ts,
				})
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&field, "field", "indexingState", "Inspection field to diff (verdict, coverage_state, robots_txt_state, indexing_state, page_fetch_state)")
	cmd.Flags().IntVar(&days, "days", 30, "Window in days")
	return cmd
}

// mapInspectionField translates user-facing names to actual column names.
func mapInspectionField(s string) (string, error) {
	switch s {
	case "indexingState", "indexing_state":
		return "indexing_state", nil
	case "verdict":
		return "verdict", nil
	case "coverageState", "coverage_state":
		return "coverage_state", nil
	case "robotsTxtState", "robots_txt_state":
		return "robots_txt_state", nil
	case "pageFetchState", "page_fetch_state":
		return "page_fetch_state", nil
	case "googleCanonical", "google_canonical":
		return "google_canonical", nil
	case "userCanonical", "user_canonical":
		return "user_canonical", nil
	case "crawledAs", "crawled_as":
		return "crawled_as", nil
	}
	return "", fmt.Errorf("unknown --field %q", s)
}

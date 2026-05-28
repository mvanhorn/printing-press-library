// Copyright 2026 Matt and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: Phase 3 transcendence — T9 sitemap-watch.

package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newSitemapWatchCmd(flags *rootFlags) *cobra.Command {
	var since string

	cmd := &cobra.Command{
		Use:         "sitemap-watch <site>",
		Short:       "Diff sitemap state between snapshots — surface new errors, new warnings, content drops",
		Long:        "Compares oldest-in-window snapshot vs newest-in-window for each (site_url, feedpath) and flags deltas: errors increased, warnings increased, content count drops, last-downloaded staleness. The API returns current state only; this is the only way to see regressions.",
		Example:     "  google-search-console-pp-cli sitemap-watch sc-domain:example.com --since 7d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			site := args[0]
			dur, err := parseLast(since)
			if err != nil {
				return usageErr(err)
			}
			startTS, _ := dateRange(dur)

			ctx := context.Background()
			s, err := openStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer s.Close()

			var sitemapCount int64
			if err := s.DB().QueryRowContext(ctx,
				`SELECT COUNT(*) FROM sitemaps WHERE site_url = ?`, site).Scan(&sitemapCount); err != nil {
				return apiErr(err)
			}
			if sitemapCount == 0 {
				return notFoundErr(fmt.Errorf("local store has no sitemaps for %q; run `google-search-console-pp-cli sync --site %s --last 30d` first to capture snapshots", site, site))
			}

			rows, err := s.DB().QueryContext(ctx, `
SELECT feedpath, snapshot_at, COALESCE(errors,0), COALESCE(warnings,0),
       COALESCE(last_downloaded,''), COALESCE(contents_json,'')
FROM sitemaps
WHERE site_url = ? AND snapshot_at >= ?
ORDER BY feedpath, snapshot_at`, site, startTS)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type entry struct {
				ts             string
				errors         int64
				warnings       int64
				lastDownloaded string
				contents       string
			}
			byFeed := map[string][]entry{}
			for rows.Next() {
				var fp string
				var e entry
				if err := rows.Scan(&fp, &e.ts, &e.errors, &e.warnings, &e.lastDownloaded, &e.contents); err != nil {
					return err
				}
				byFeed[fp] = append(byFeed[fp], e)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			out := []map[string]any{}
			for fp, entries := range byFeed {
				if len(entries) < 2 {
					continue
				}
				oldest := entries[0]
				newest := entries[len(entries)-1]
				if newest.errors > oldest.errors {
					out = append(out, map[string]any{
						"feedpath":    fp,
						"change_type": "errors_increased",
						"old_value":   oldest.errors,
						"new_value":   newest.errors,
					})
				}
				if newest.warnings > oldest.warnings {
					out = append(out, map[string]any{
						"feedpath":    fp,
						"change_type": "warnings_increased",
						"old_value":   oldest.warnings,
						"new_value":   newest.warnings,
					})
				}
				// last_downloaded stale: didn't move between oldest and newest snapshot
				if oldest.lastDownloaded != "" && oldest.lastDownloaded == newest.lastDownloaded && len(entries) >= 2 {
					out = append(out, map[string]any{
						"feedpath":    fp,
						"change_type": "last_downloaded_unchanged",
						"old_value":   oldest.lastDownloaded,
						"new_value":   newest.lastDownloaded,
					})
				}
				if len(oldest.contents) > 0 && len(newest.contents) > 0 && len(newest.contents) < len(oldest.contents)/2 {
					out = append(out, map[string]any{
						"feedpath":    fp,
						"change_type": "contents_dropped",
						"old_value":   len(oldest.contents),
						"new_value":   len(newest.contents),
					})
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Snapshot window to compare across")
	return cmd
}

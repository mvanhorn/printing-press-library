// Copyright 2026 Mohammed Al Khamis and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type formatRow struct {
	MediaProductType     string  `json:"media_product_type"`
	Posts                int64   `json:"posts"`
	AvgReach             float64 `json:"avg_reach"`
	AvgTotalInteractions float64 `json:"avg_total_interactions"`
	AvgReelsWatchTime    float64 `json:"avg_reels_watch_time"`
}

func newNovelFormatsCmd(flags *rootFlags) *cobra.Command {
	var dbFlag, accountFlag string

	cmd := &cobra.Command{
		Use:   "formats",
		Short: "Compare Reels vs Feed vs Story vs Carousel by reach, engagement, and Reels watch-time.",
		Long: `Group stored media by media_product_type and report post count, average
reach, average total_interactions, and average Reels watch-time (meaningful
only for the REELS row).

Reads the local store. Run 'instagram-pp-cli pull' first to populate media.`,
		Example:     "  instagram-pp-cli formats --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare formats from local store")
				return nil
			}
			db, err := ensureAnalyticsSchema(cmd.Context(), resolveDBPath(dbFlag))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()

			query := `
				SELECT COALESCE(NULLIF(media_product_type,''),'UNKNOWN') AS pt,
				       COUNT(*) AS posts,
				       AVG(COALESCE(reach,0)) AS avg_reach,
				       AVG(COALESCE(total_interactions,0)) AS avg_inter,
				       AVG(COALESCE(reels_avg_watch_time,0)) AS avg_watch
				FROM ig_brand_media`
			args2 := []any{}
			if accountFlag != "" {
				query += ` WHERE slug = ?`
				args2 = append(args2, slugify(accountFlag))
			}
			query += ` GROUP BY pt ORDER BY posts DESC`

			rows, err := db.DB().QueryContext(cmd.Context(), query, args2...)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()

			out := make([]formatRow, 0)
			for rows.Next() {
				var r formatRow
				if err := rows.Scan(&r.MediaProductType, &r.Posts, &r.AvgReach, &r.AvgTotalInteractions, &r.AvgReelsWatchTime); err != nil {
					return apiErr(err)
				}
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return apiErr(err)
			}

			if flags.asJSON {
				env := map[string]any{"formats": out, "count": len(out)}
				if len(out) == 0 {
					env["note"] = "no media in store; run 'instagram-pp-cli pull' first"
				}
				return printJSONFiltered(cmd.OutOrStdout(), env, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No media in store. Run 'instagram-pp-cli pull' first.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "PRODUCT_TYPE\tPOSTS\tAVG_REACH\tAVG_INTERACTIONS\tAVG_REELS_WATCH")
			for _, r := range out {
				fmt.Fprintf(tw, "%s\t%d\t%.1f\t%.1f\t%.1f\n", r.MediaProductType, r.Posts, r.AvgReach, r.AvgTotalInteractions, r.AvgReelsWatchTime)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Path to the local store (defaults to the standard data dir)")
	cmd.Flags().StringVar(&accountFlag, "account", "", "Limit to a single brand slug")
	return cmd
}

// Copyright 2026 Mohammed Al Khamis and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type hashtagPerfRow struct {
	Slug               string `json:"slug"`
	Hashtag            string `json:"hashtag"`
	TopMediaEngagement int64  `json:"top_media_engagement"`
	TopMediaReach      int64  `json:"top_media_reach"`
	TopMediaCount      int64  `json:"top_media_count"`
	CapturedAt         string `json:"captured_at"`
}

func newNovelHashtagPerfCmd(flags *rootFlags) *cobra.Command {
	var dbFlag, accountFlag string
	var limitFlag int

	cmd := &cobra.Command{
		Use:   "hashtag-perf",
		Short: "Rank the hashtags you track by the reach and engagement of their top media.",
		Long: `Rank tracked hashtags by the engagement of their top media, using the latest
snapshot per (brand, hashtag). top_media_reach is 0 where the hashtag API does
not expose reach on hashtag media.

Reads the local store. Track hashtags with 'brands track-hashtag <slug>
<hashtag>', then run 'instagram-pp-cli pull' to collect snapshots.`,
		Example:     "  instagram-pp-cli hashtag-perf --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank hashtags from local store")
				return nil
			}
			if limitFlag <= 0 {
				limitFlag = 20
			}
			db, err := ensureAnalyticsSchema(cmd.Context(), resolveDBPath(dbFlag))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()

			// Latest snapshot per (slug, hashtag), ranked by engagement.
			query := `
				SELECT h.slug, h.hashtag,
				       COALESCE(h.top_media_engagement,0),
				       COALESCE(h.top_media_reach,0),
				       COALESCE(h.top_media_count,0),
				       COALESCE(h.captured_at,'')
				FROM ig_hashtag_snapshots h
				JOIN (
					SELECT slug, hashtag, MAX(id) AS mx
					FROM ig_hashtag_snapshots
					GROUP BY slug, hashtag
				) latest ON latest.slug = h.slug AND latest.hashtag = h.hashtag AND latest.mx = h.id`
			args2 := []any{}
			if accountFlag != "" {
				query += ` WHERE h.slug = ?`
				args2 = append(args2, slugify(accountFlag))
			}
			query += ` ORDER BY COALESCE(h.top_media_engagement,0) DESC, h.hashtag LIMIT ?`
			args2 = append(args2, limitFlag)

			rows, err := db.DB().QueryContext(cmd.Context(), query, args2...)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()

			out := make([]hashtagPerfRow, 0)
			for rows.Next() {
				var r hashtagPerfRow
				if err := rows.Scan(&r.Slug, &r.Hashtag, &r.TopMediaEngagement, &r.TopMediaReach, &r.TopMediaCount, &r.CapturedAt); err != nil {
					return apiErr(err)
				}
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return apiErr(err)
			}

			if flags.asJSON {
				env := map[string]any{"hashtags": out, "count": len(out)}
				if len(out) == 0 {
					env["note"] = "no hashtag snapshots; run 'brands track-hashtag' then 'instagram-pp-cli pull'"
				}
				return printJSONFiltered(cmd.OutOrStdout(), env, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No hashtag snapshots. Run 'brands track-hashtag <slug> <hashtag>' then 'instagram-pp-cli pull'.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "SLUG\tHASHTAG\tENGAGEMENT\tREACH\tTOP_MEDIA\tCAPTURED_AT")
			for _, r := range out {
				fmt.Fprintf(tw, "%s\t#%s\t%d\t%d\t%d\t%s\n", r.Slug, r.Hashtag, r.TopMediaEngagement, r.TopMediaReach, r.TopMediaCount, r.CapturedAt)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Path to the local store (defaults to the standard data dir)")
	cmd.Flags().StringVar(&accountFlag, "account", "", "Limit to a single brand slug")
	cmd.Flags().IntVar(&limitFlag, "limit", 20, "Maximum hashtags to return")
	return cmd
}

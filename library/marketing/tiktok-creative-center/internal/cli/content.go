// Copyright 2026 Jon and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-implemented transcendence command for the TikTok Creative Center CLI.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelContentCmd(flags *rootFlags) *cobra.Command {
	var flagRegion string
	var flagDays string
	var flagTop string

	cmd := &cobra.Command{
		Use:   "content [niche]",
		Short: "One ranked feed of trending/viral content across Top Ads, hashtag videos, and creators.",
		Long: "Pulls trending/viral content from three sources into one ranked feed: the Top Ads library, " +
			"representative videos in hashtag detail, and top creators' items. The web UI silos these; " +
			"this command joins them. Optional niche keyword filters by hashtag/industry/ad text. " +
			"Reads the local store; run 'sync' first.",
		Example:     "  tiktok-creative-center-pp-cli content \"marvel rivals\" --region US --days 7 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			niche := ""
			if len(args) > 0 {
				niche = args[0]
			}
			ctx := cmd.Context()
			_ = flagDays
			db, err := novelOpenStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := loadHashtagRows(ctx, db, flagRegion)
			if err != nil {
				return err
			}
			ads, err := loadTopAdRows(ctx, db, flagRegion)
			if err != nil {
				return err
			}
			if len(rows) == 0 && len(ads) == 0 {
				return fmt.Errorf("%s", syncFirstHint)
			}

			items := buildContentFeed(niche, rows, ads)
			if len(items) == 0 {
				return fmt.Errorf("no content matched niche %q in local store; try a broader keyword or re-run sync", niche)
			}
			items = rankContentByPopularity(items, parseIntFlag(flagTop, 10))
			return flags.printJSON(cmd, items)
		},
	}
	cmd.Flags().StringVar(&flagRegion, "region", "US", "ISO country code to filter synced data")
	cmd.Flags().StringVar(&flagDays, "days", "7", "Time range in days (informational; applied at sync time)")
	cmd.Flags().StringVar(&flagTop, "top", "20", "Number of content items to return")
	return cmd
}

// buildContentFeed joins the three content sources, optionally filtering by a
// niche keyword. Pure for testability.
func buildContentFeed(niche string, rows []hashtagRow, ads []topAdRow) []contentItem {
	nl := strings.ToLower(niche)
	items := make([]contentItem, 0, len(rows)+len(ads))
	for _, r := range rows {
		if niche != "" && !matchNiche(r, niche) {
			continue
		}
		items = append(items, hashtagVideoItems(r)...)
	}
	for _, a := range ads {
		if niche != "" && !adMatchesNiche(a, nl) {
			continue
		}
		items = append(items, topAdContentItem(a))
	}
	return items
}

// adMatchesNiche checks whether a top ad relates to a lowercased niche keyword.
func adMatchesNiche(a topAdRow, nl string) bool {
	if nl == "" {
		return true
	}
	if strings.Contains(strings.ToLower(a.Title), nl) {
		return true
	}
	if strings.Contains(strings.ToLower(a.AdText), nl) {
		return true
	}
	if strings.Contains(strings.ToLower(a.Author), nl) {
		return true
	}
	for _, k := range a.Keywords {
		if strings.Contains(strings.ToLower(k), nl) {
			return true
		}
	}
	return false
}

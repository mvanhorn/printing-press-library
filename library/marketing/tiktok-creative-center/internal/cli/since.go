// Copyright 2026 Jon and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-implemented transcendence command for the TikTok Creative Center CLI.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// sinceResult is the set of newly-trending items within a window.
type sinceResult struct {
	Window      string        `json:"window"`
	NewHashtags []sinceHashtag `json:"newHashtags"`
	NewTopAds   []sinceTopAd   `json:"newTopContent"`
}

type sinceHashtag struct {
	Hashtag    string  `json:"hashtag"`
	Popularity float64 `json:"popularity"`
	SyncedAt   string  `json:"syncedAt,omitempty"`
}

type sinceTopAd struct {
	Title    string  `json:"title"`
	Author   string  `json:"author,omitempty"`
	Popularity float64 `json:"popularity"`
}

// pp:data-source local
func newNovelSinceCmd(flags *rootFlags) *cobra.Command {
	var flagRegion string

	cmd := &cobra.Command{
		Use:   "since <duration>",
		Short: "Show hashtags and top content newly synced within a window (e.g. since 24h).",
		Long: "Shows hashtags and top content whose local snapshot is newer than the given duration " +
			"(e.g. 'since 24h', 'since 7d'). Catches what changed in the last sync without re-reading " +
			"the whole feed. Reads the local store; run 'sync' first.",
		Example:     "  tiktok-creative-center-pp-cli since 24h --region US --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			dur, err := parseDurationArg(args[0])
			if err != nil {
				return fmt.Errorf("invalid duration: %w", err)
			}
			ctx := cmd.Context()
			db, err := novelOpenStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			cutoff := time.Now().Add(-dur)
			tags, err := storeHashtagIDsSince(db, cutoff)
			if err != nil {
				return err
			}
			ads, err := loadTopAdRows(ctx, db, flagRegion)
			if err != nil {
				return err
			}

			out := sinceResult{Window: args[0]}
			for _, t := range tags {
				if flagRegion != "" && t.CountryCode != "" && !strings.EqualFold(t.CountryCode, flagRegion) {
					continue
				}
				out.NewHashtags = append(out.NewHashtags, sinceHashtag{
					Hashtag:    t.Name,
					Popularity: t.Popularity,
					SyncedAt:   t.SyncedAt,
				})
			}
			for _, a := range ads {
				out.NewTopAds = append(out.NewTopAds, sinceTopAd{
					Title:      a.Title,
					Author:     a.Author,
					Popularity: a.Popularity,
				})
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&flagRegion, "region", "", "ISO country code to filter (empty = all)")
	return cmd
}

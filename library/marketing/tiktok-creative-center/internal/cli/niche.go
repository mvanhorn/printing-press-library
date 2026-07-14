// Copyright 2026 Jon and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-implemented transcendence command for the TikTok Creative Center CLI.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

// nicheBrief is the cross-entity brief for one niche.
type nicheBrief struct {
	Niche         string         `json:"niche"`
	Region        string         `json:"region,omitempty"`
	TrendingTags  []nicheHashtag `json:"trendingHashtags"`
	TopCreators   []string       `json:"topCreators"`
	Representative []contentItem `json:"representativeVideos"`
}

type nicheHashtag struct {
	Hashtag    string  `json:"hashtag"`
	Popularity float64 `json:"popularity"`
	PublishCnt float64 `json:"publishCnt"`
}

// pp:data-source local
func newNovelNicheCmd(flags *rootFlags) *cobra.Command {
	var flagRegion string
	var flagDays string
	var flagTop string

	cmd := &cobra.Command{
		Use:   "niche <keyword>",
		Short: "One-command brief for a niche: trending hashtags, top creators, and representative videos ranked together.",
		Long: "Joins trending hashtags matching the niche with their top creators and representative " +
			"videos into one ranked brief — the cross-entity view no Creative Center page gives you. " +
			"Reads the local store; run 'sync' first.",
		Example:     "  tiktok-creative-center-pp-cli niche \"marvel rivals\" --region US --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			niche := args[0]
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
			if len(rows) == 0 {
				return fmt.Errorf("%s", syncFirstHint)
			}

			brief := buildNicheBrief(niche, flagRegion, rows)
			if len(brief.TrendingTags) == 0 {
				return fmt.Errorf("no hashtags matched niche %q in local store; try a broader keyword or re-run sync", niche)
			}
			return flags.printJSON(cmd, brief)
		},
	}
	cmd.Flags().StringVar(&flagRegion, "region", "US", "ISO country code to filter synced hashtags")
	cmd.Flags().StringVar(&flagDays, "days", "7", "Time range in days (informational; applied at sync time)")
	cmd.Flags().StringVar(&flagTop, "top", "10", "Number of hashtags/content items to return per section")
	return cmd
}

// buildNicheBrief assembles the cross-entity brief from hashtag rows. Pure for
// testability.
func buildNicheBrief(niche, region string, rows []hashtagRow) nicheBrief {
	top := 10
	matched := make([]hashtagRow, 0, len(rows))
	for _, r := range rows {
		if matchNiche(r, niche) {
			matched = append(matched, r)
		}
	}
	sort.SliceStable(matched, func(i, j int) bool { return matched[i].Popularity > matched[j].Popularity })
	if top > 0 && len(matched) > top {
		matched = matched[:top]
	}

	brief := nicheBrief{Niche: niche, Region: region}
	creatorSet := map[string]bool{}
	for _, r := range matched {
		brief.TrendingTags = append(brief.TrendingTags, nicheHashtag{
			Hashtag:    r.Name,
			Popularity: r.Popularity,
			PublishCnt: r.PublishCnt,
		})
		for _, c := range r.TopCreators {
			if !creatorSet[c] {
				creatorSet[c] = true
				brief.TopCreators = append(brief.TopCreators, c)
			}
		}
		brief.Representative = append(brief.Representative, hashtagVideoItems(r)...)
	}
	brief.Representative = rankContentByPopularity(brief.Representative, top)
	return brief
}

// Copyright 2026 Jon and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-implemented transcendence command for the TikTok Creative Center CLI.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// decisionBrief is the synthesized content + account recommendation.
type decisionBrief struct {
	Niche                 string           `json:"niche"`
	Region                string           `json:"region,omitempty"`
	TrendingHashtags      []nicheHashtag   `json:"trendingHashtags"`
	WhiteSpace            []viralResult    `json:"whiteSpace"`
	WorkingContentFormats []contentItem    `json:"workingContentFormats"`
	CompetitorPositioning []competitorAd   `json:"competitorPositioning"`
	Recommendation        recommendation   `json:"recommendation"`
}

type recommendation struct {
	HashtagsToRide   []string `json:"hashtagsToRide"`
	ContentAngles    []string `json:"contentAngles"`
	OpenAccountAngle string   `json:"openAccountAngle"`
}

// pp:data-source local
func newNovelDecideCmd(flags *rootFlags) *cobra.Command {
	var flagRegion string
	var flagDays string
	var flagTop string

	cmd := &cobra.Command{
		Use:   "decide <niche>",
		Short: "Synthesize a content + account recommendation for a niche from local trend data.",
		Long: "The actual goal: given a niche, synthesizes trending tags, viral opportunity (white space), " +
			"working content formats, and competitor positioning into a concrete recommendation — " +
			"what content to make, which hashtags to ride, and what account angle is still open. " +
			"Every other command feeds this one. Reads the local store; run 'sync' first.",
		Example:     "  tiktok-creative-center-pp-cli decide \"marvel rivals\" --region US --days 7 --agent",
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
			ads, err := loadTopAdRows(ctx, db, flagRegion)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return fmt.Errorf("%s", syncFirstHint)
			}

			brief := buildDecision(niche, flagRegion, rows, ads)
			// WhiteSpace is a global opportunity ranking independent of the niche
			// (see buildDecision), so it does not indicate the niche itself
			// matched anything. Only TrendingHashtags/WorkingContentFormats/
			// CompetitorPositioning are niche-scoped.
			if len(brief.TrendingHashtags) == 0 && len(brief.WorkingContentFormats) == 0 && len(brief.CompetitorPositioning) == 0 {
				return fmt.Errorf("no hashtags matched niche %q in local store; try a broader keyword or re-run sync", niche)
			}
			// decide always emits JSON — it is a brief, not a list.
			if !flags.asJSON && !flags.compact && flags.selectFields == "" {
				flags.asJSON = true
			}
			return flags.printJSON(cmd, brief)
		},
	}
	cmd.Flags().StringVar(&flagRegion, "region", "US", "ISO country code to filter synced data")
	cmd.Flags().StringVar(&flagDays, "days", "7", "Time range in days (informational; applied at sync time)")
	cmd.Flags().StringVar(&flagTop, "top", "10", "Number of items per section")
	return cmd
}

// buildDecision synthesizes the full recommendation. Pure for testability.
func buildDecision(niche, region string, rows []hashtagRow, ads []topAdRow) decisionBrief {
	top := 10

	// Niche-matched trending tags.
	nicheRows := make([]hashtagRow, 0, len(rows))
	for _, r := range rows {
		if matchNiche(r, niche) {
			nicheRows = append(nicheRows, r)
		}
	}

	brief := decisionBrief{Niche: niche, Region: region}

	// White space = opportunity ranking across ALL hashtags (not just niche),
	// so the recommendation can point at adjacent underserved tags too.
	brief.WhiteSpace = toViralResults(viralRank(rows, top))

	// Trending hashtags within the niche.
	brief.TrendingHashtags = toNicheHashtags(nicheRows, top)

	// Working content formats = top content in the niche.
	brief.WorkingContentFormats = rankContentByPopularity(buildContentFeed(niche, nicheRows, ads), top)

	// Competitor positioning = top ads in the niche (proxy for competitors).
	brief.CompetitorPositioning = topAdsInNiche(niche, ads, top)

	brief.Recommendation = synthesizeRecommendation(niche, brief)
	return brief
}

// toViralResults converts hashtag rows to viral result rows with ranks.
func toViralResults(rows []hashtagRow) []viralResult {
	out := make([]viralResult, 0, len(rows))
	for i, r := range rows {
		out = append(out, viralResult{
			Hashtag:          r.Name,
			PublishCnt:       r.PublishCnt,
			Popularity:       r.Popularity,
			OpportunityScore: opportunityScore(r),
			Rank:             i + 1,
		})
	}
	return out
}

// toNicheHashtags converts niche-matched rows to nicheHashtag, popularity desc.
func toNicheHashtags(rows []hashtagRow, top int) []nicheHashtag {
	sorted := append([]hashtagRow(nil), rows...)
	sortByPopularity(sorted)
	if top > 0 && len(sorted) > top {
		sorted = sorted[:top]
	}
	out := make([]nicheHashtag, 0, len(sorted))
	for _, r := range sorted {
		out = append(out, nicheHashtag{
			Hashtag:    r.Name,
			Popularity: r.Popularity,
			PublishCnt: r.PublishCnt,
		})
	}
	return out
}

// topAdsInNiche returns the top ads matching the niche keyword.
func topAdsInNiche(niche string, ads []topAdRow, top int) []competitorAd {
	nl := strings.ToLower(niche)
	out := make([]competitorAd, 0, len(ads))
	for _, a := range ads {
		if niche != "" && !adMatchesNiche(a, nl) {
			continue
		}
		out = append(out, competitorAd{Title: a.Title, Popularity: a.Popularity})
	}
	sortByPopularityAds(out)
	if top > 0 && len(out) > top {
		out = out[:top]
	}
	return out
}

// synthesizeRecommendation derives concrete suggestions from the brief data.
func synthesizeRecommendation(niche string, b decisionBrief) recommendation {
	rec := recommendation{}

	// Hashtags to ride: top white-space tags (underserved + rising) preferred;
	// fall back to niche trending tags.
	for _, w := range b.WhiteSpace {
		if w.Hashtag != "" {
			rec.HashtagsToRide = append(rec.HashtagsToRide, w.Hashtag)
		}
		if len(rec.HashtagsToRide) >= 5 {
			break
		}
	}
	for _, t := range b.TrendingHashtags {
		if contains(rec.HashtagsToRide, t.Hashtag) {
			continue
		}
		rec.HashtagsToRide = append(rec.HashtagsToRide, t.Hashtag)
		if len(rec.HashtagsToRide) >= 8 {
			break
		}
	}

	// Content angles derived from top working content titles.
	for _, c := range b.WorkingContentFormats {
		if c.Title == "" {
			continue
		}
		angle := c.Title
		if len(angle) > 80 {
			angle = angle[:77] + "..."
		}
		rec.ContentAngles = append(rec.ContentAngles, angle)
		if len(rec.ContentAngles) >= 5 {
			break
		}
	}

	rec.OpenAccountAngle = openAccountAngle(niche, b)
	return rec
}

// openAccountAngle proposes an account angle from the largest white-space gap.
func openAccountAngle(niche string, b decisionBrief) string {
	if niche == "" {
		niche = "this niche"
	}
	if len(b.WhiteSpace) == 0 {
		return fmt.Sprintf("Build an account around %s; sync more data to pinpoint the lowest-competition angle.", niche)
	}
	top := b.WhiteSpace[0]
	saturation := "low"
	if top.PublishCnt > 10000 {
		saturation = "moderate"
	}
	if top.PublishCnt > 100000 {
		saturation = "high"
	}
	return fmt.Sprintf(
		"Anchor an account around #%s in %s — high popularity (%.0f) with %s publish count (%.0f) "+
			"means white space before it saturates. Ride the top trending tags and model the working content formats above.",
		top.Hashtag, niche, top.Popularity, saturation, top.PublishCnt,
	)
}

// contains reports whether s contains v.
func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

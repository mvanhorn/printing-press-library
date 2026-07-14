// Copyright 2026 Jon and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-implemented transcendence command for the TikTok Creative Center CLI.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// competitorBrief summarizes a competitor's positioning from the local store.
type competitorBrief struct {
	Competitor      string           `json:"competitor"`
	TopContent      []competitorAd   `json:"topContent"`
	HashtagsRidden  []string         `json:"hashtagsRidden"`
	Positioning     string           `json:"positioning"`
}

type competitorAd struct {
	Title      string  `json:"title"`
	Popularity float64 `json:"popularity"`
}

// pp:data-source local
func newNovelCompetitorCmd(flags *rootFlags) *cobra.Command {
	var flagRegion string
	var flagTop string

	cmd := &cobra.Command{
		Use:   "competitor <handle-or-keyword>",
		Short: "Summarize a competitor's top-performing content and which trending hashtags they ride.",
		Long: "Searches the Top Ads library for a competitor (by author handle/name or ad keyword) and " +
			"matches their content against stored trending hashtags to summarize creative strategy and " +
			"trend positioning. Reads the local store; run 'sync' first.",
		Example:     "  tiktok-creative-center-pp-cli competitor \"myketowersmtyk\" --region US --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			query := args[0]
			ctx := cmd.Context()
			db, err := novelOpenStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			ads, err := loadTopAdRows(ctx, db, flagRegion)
			if err != nil {
				return err
			}
			rows, err := loadHashtagRows(ctx, db, flagRegion)
			if err != nil {
				return err
			}
			if len(ads) == 0 {
				return fmt.Errorf("%s", syncFirstHint)
			}

			brief := buildCompetitorBrief(query, ads, rows)
			if len(brief.TopContent) == 0 {
				return fmt.Errorf("no top ads matched competitor %q in local store; check the handle or re-run sync", query)
			}
			if top := parseIntFlag(flagTop, 10); top > 0 && len(brief.TopContent) > top {
				brief.TopContent = brief.TopContent[:top]
			}
			return flags.printJSON(cmd, brief)
		},
	}
	cmd.Flags().StringVar(&flagRegion, "region", "US", "ISO country code to filter synced data")
	cmd.Flags().StringVar(&flagTop, "top", "10", "Number of top content items to return")
	return cmd
}

// buildCompetitorBrief assembles the competitor summary. Pure for testability.
func buildCompetitorBrief(query string, ads []topAdRow, rows []hashtagRow) competitorBrief {
	q := strings.ToLower(strings.TrimSpace(query))
	brief := competitorBrief{Competitor: query}
	tagSet := map[string]bool{}
	for _, a := range ads {
		if !competitorMatches(a, q) {
			continue
		}
		brief.TopContent = append(brief.TopContent, competitorAd{
			Title:      a.Title,
			Popularity: a.Popularity,
		})
		for _, tag := range matchHashtagsFromAd(a, rows) {
			if !tagSet[tag] {
				tagSet[tag] = true
				brief.HashtagsRidden = append(brief.HashtagsRidden, tag)
			}
		}
	}
	sort.SliceStable(brief.TopContent, func(i, j int) bool {
		return brief.TopContent[i].Popularity > brief.TopContent[j].Popularity
	})
	brief.Positioning = competitorPositioning(brief)
	return brief
}

// competitorMatches reports whether a top ad belongs to the competitor query.
func competitorMatches(a topAdRow, q string) bool {
	if q == "" {
		return true
	}
	if strings.Contains(strings.ToLower(a.Author), q) {
		return true
	}
	if strings.Contains(strings.ToLower(a.Handle), q) {
		return true
	}
	if strings.Contains(strings.ToLower(a.AuthorID), q) {
		return true
	}
	if strings.Contains(strings.ToLower(a.Title), q) {
		return true
	}
	if strings.Contains(strings.ToLower(a.AdText), q) {
		return true
	}
	for _, k := range a.Keywords {
		if strings.Contains(strings.ToLower(k), q) {
			return true
		}
	}
	return false
}

// matchHashtagsFromAd returns trending hashtag names mentioned in an ad's text.
// Hashtag names are often run-together ("marvelrivals") while ad prose has
// spaces ("marvel rivals"), so both sides are compared space-stripped too.
func matchHashtagsFromAd(a topAdRow, rows []hashtagRow) []string {
	hay := strings.ToLower(a.Title + " " + a.AdText)
	hayFlat := stripSpaces(hay)
	var out []string
	for _, r := range rows {
		if r.Name == "" {
			continue
		}
		name := strings.ToLower(r.Name)
		if strings.Contains(hay, name) || strings.Contains(hayFlat, stripSpaces(name)) {
			out = append(out, r.Name)
		}
	}
	return out
}

// stripSpaces removes all whitespace from s.
func stripSpaces(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// competitorPositioning synthesizes a one-line positioning summary.
func competitorPositioning(b competitorBrief) string {
	if len(b.TopContent) == 0 {
		return "no competing content found"
	}
	var tags string
	if len(b.HashtagsRidden) > 0 {
		tags = fmt.Sprintf(", riding %d trending hashtag(s) (%s)", len(b.HashtagsRidden), joinTruncated(b.HashtagsRidden, 5))
	}
	return fmt.Sprintf("%d top-performing ad(s) in the local library%s", len(b.TopContent), tags)
}

// joinTruncated joins up to n items, appending "+N" if truncated.
func joinTruncated(items []string, n int) string {
	if len(items) <= n {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:n], ", ") + fmt.Sprintf(" +%d", len(items)-n)
}

// Copyright 2026 Jon and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-implemented transcendence command for the TikTok Creative Center CLI.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// similarResult is one co-rising hashtag related to the target.
type similarResult struct {
	Hashtag          string   `json:"hashtag"`
	SimilarityReason string   `json:"similarityReason"`
	SharedIndustries []string `json:"sharedIndustries,omitempty"`
	SharedCreators   []string `json:"sharedCreators,omitempty"`
	Score            float64  `json:"score"`
}

// pp:data-source local
func newNovelSimilarCmd(flags *rootFlags) *cobra.Command {
	var flagRegion string
	var flagTop string

	cmd := &cobra.Command{
		Use:   "similar <hashtag>",
		Short: "Surface similar or co-rising hashtags using shared industries and creators.",
		Long: "Given a hashtag name or ID, ranks other synced hashtags by shared industries and " +
			"co-occurring top creators — the joined hashtag+creator+industry graph that only exists " +
			"locally after sync. Reads the local store; run 'sync' first.",
		Example:     "  tiktok-creative-center-pp-cli similar \"marvelrivalss9\" --region US --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			ctx := cmd.Context()
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

			target, ok := findHashtag(rows, args[0])
			if !ok {
				return fmt.Errorf("hashtag %q not found in local store", args[0])
			}

			out := rankSimilar(target, rows)
			if top := parseIntFlag(flagTop, 10); top > 0 && len(out) > top {
				out = out[:top]
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&flagRegion, "region", "US", "ISO country code to filter synced hashtags")
	cmd.Flags().StringVar(&flagTop, "top", "10", "Number of similar hashtags to return")
	return cmd
}

// findHashtag locates a hashtag by ID or name (case-insensitive substring on name).
func findHashtag(rows []hashtagRow, query string) (hashtagRow, bool) {
	q := strings.TrimSpace(query)
	for _, r := range rows {
		if r.ID == q || strings.EqualFold(r.Name, q) {
			return r, true
		}
	}
	for _, r := range rows {
		if strings.Contains(strings.ToLower(r.Name), strings.ToLower(q)) {
			return r, true
		}
	}
	return hashtagRow{}, false
}

// rankSimilar scores every other hashtag against the target by shared
// industries (weight 2) and shared creators (weight 1), desc.
func rankSimilar(target hashtagRow, rows []hashtagRow) []similarResult {
	out := make([]similarResult, 0, len(rows))
	for _, r := range rows {
		if r.ID == target.ID {
			continue
		}
		ind := sharedIndustries(target, r)
		creators := sharedCreators(target, r)
		score := float64(len(ind))*2 + float64(len(creators))
		if score == 0 {
			continue
		}
		reason := "shared_industries"
		if len(creators) > 0 && len(ind) > 0 {
			reason = "shared_industries_and_creators"
		} else if len(creators) > 0 {
			reason = "shared_creators"
		}
		out = append(out, similarResult{
			Hashtag:          r.Name,
			SimilarityReason: reason,
			SharedIndustries: ind,
			SharedCreators:   creators,
			Score:            score,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

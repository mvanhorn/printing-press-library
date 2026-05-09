// Copyright 2026 actionsslave. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newMakersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "makers",
		Short: "Aggregate stats across a maker's full Product Hunt portfolio",
	}
	cmd.AddCommand(newMakersPortfolioCmd(flags))
	return cmd
}

type portfolioPost struct {
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	VotesCount    int    `json:"votesCount"`
	CommentsCount int    `json:"commentsCount"`
	FeaturedAt    string `json:"featuredAt,omitempty"`
	CreatedAt     string `json:"createdAt"`
}

type portfolioResult struct {
	Username         string          `json:"username"`
	Name             string          `json:"name"`
	Headline         string          `json:"headline,omitempty"`
	FollowersCount   int             `json:"followersCount,omitempty"`
	TotalLaunches    int             `json:"totalLaunches"`
	TotalVotes       int             `json:"totalVotes"`
	AvgVotesPerLaunch float64        `json:"avgVotesPerLaunch"`
	BestLaunch       *portfolioPost  `json:"bestLaunch,omitempty"`
	DaysSinceLastLaunch int          `json:"daysSinceLastLaunch,omitempty"`
	RecentPosts      []portfolioPost `json:"recentPosts"`
}

func newMakersPortfolioCmd(flags *rootFlags) *cobra.Command {
	var postsLimit int

	cmd := &cobra.Command{
		Use:   "portfolio <username>",
		Short: "Aggregate stats across a maker's full product history",
		Long: `Fetches a maker's recent posts and aggregates key stats:
total votes across all launches, average votes per launch, best launch ever,
and days since their last launch. Ideal for maker research or benchmarking
your own portfolio against others.`,
		Example: strings.Trim(`
  product-hunt-pp-cli makers portfolio levelsio
  product-hunt-pp-cli makers portfolio rrhoover --json
  product-hunt-pp-cli makers portfolio levelsio --posts-limit 50`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			phc, err := flags.newPHClient()
			if err != nil {
				return err
			}

			// pp:client-call — live API call via phgraphql client
			user, posts, err := phc.GetUser(cmd.Context(), args[0], postsLimit)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			result := portfolioResult{
				Username:       user.Username,
				Name:           user.Name,
				Headline:       user.Headline,
				FollowersCount: user.FollowersCount,
				TotalLaunches:  len(posts),
			}

			var best *portfolioPost
			var lastLaunchTime time.Time
			for _, p := range posts {
				result.TotalVotes += p.VotesCount

				pp := portfolioPost{
					Name:          p.Name,
					Slug:          p.Slug,
					VotesCount:    p.VotesCount,
					CommentsCount: p.CommentsCount,
					FeaturedAt:    p.FeaturedAt,
					CreatedAt:     p.CreatedAt,
				}
				result.RecentPosts = append(result.RecentPosts, pp)

				if best == nil || p.VotesCount > best.VotesCount {
					pCopy := pp
					best = &pCopy
				}

				if p.CreatedAt != "" {
					if ts, err := time.Parse(time.RFC3339, p.CreatedAt); err == nil {
						if lastLaunchTime.IsZero() || ts.After(lastLaunchTime) {
							lastLaunchTime = ts
						}
					}
				}
			}

			result.BestLaunch = best

			if len(posts) > 0 {
				result.AvgVotesPerLaunch = float64(result.TotalVotes) / float64(len(posts))
			}

			if !lastLaunchTime.IsZero() {
				result.DaysSinceLastLaunch = int(time.Since(lastLaunchTime).Hours() / 24)
			}

			// Sort recent posts by votes descending
			sort.Slice(result.RecentPosts, func(i, j int) bool {
				return result.RecentPosts[i].VotesCount > result.RecentPosts[j].VotesCount
			})

			data, err := json.Marshal(result)
			if err != nil {
				return err
			}

			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().IntVar(&postsLimit, "posts-limit", 20, "Number of recent posts to include in the portfolio")
	return cmd
}

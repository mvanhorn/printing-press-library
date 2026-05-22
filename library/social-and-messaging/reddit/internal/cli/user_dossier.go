// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/config"
)

type perSubActivity struct {
	Sub               string  `json:"subreddit"`
	SubmissionCount   int     `json:"submission_count"`
	CommentCount      int     `json:"comment_count"`
	SubmissionScore   int     `json:"submission_score_sum"`
	CommentScore      int     `json:"comment_score_sum"`
	LastActivityUTC   float64 `json:"last_activity_utc"`
	LastActivityHuman string  `json:"last_activity_human"`
}

type userDossierReport struct {
	Username      string           `json:"username"`
	IsSuspended   bool             `json:"is_suspended"`
	CreatedUTC    float64          `json:"created_utc,omitempty"`
	LinkKarma     int              `json:"link_karma"`
	CommentKarma  int              `json:"comment_karma"`
	TotalKarma    int              `json:"total_karma"`
	IsMod         bool             `json:"is_mod,omitempty"`
	IsGold        bool             `json:"is_gold,omitempty"`
	IsEmployee    bool             `json:"is_employee,omitempty"`
	PerSub        []perSubActivity `json:"per_sub"`
	OverallTotals struct {
		Submissions int `json:"submissions"`
		Comments    int `json:"comments"`
	} `json:"overall_totals"`
	RecentTopPosts []struct {
		Sub       string  `json:"subreddit"`
		Title     string  `json:"title"`
		Score     int     `json:"score"`
		Permalink string  `json:"permalink"`
		CreatedAt float64 `json:"created_utc"`
	} `json:"recent_top_posts"`
}

// newUserDossierCmd aggregates one user's submitted history, comment history,
// and karma into a per-sub dossier. Answers "is this user a karma farmer or a
// real contributor in the subs I care about?"
//
// Calls four Reddit endpoints in sequence:
//
//	/user/<u>/about (karma totals)
//	/user/<u>/submitted (up to N most-recent submissions)
//	/user/<u>/comments (up to N most-recent comments)
//
// Then aggregates locally by subreddit.
func newUserDossierCmd(flags *rootFlags) *cobra.Command {
	var (
		inSubs string
		limit  int
		topN   int
	)
	cmd := &cobra.Command{
		Use:   "dossier <username>",
		Short: "Cross-sub user dossier: aggregate activity, karma, top posts per sub",
		Long: `Aggregate a Redditor's activity across multiple subreddits.

The dossier joins four endpoints (about, submitted, comments) and groups by
subreddit. Answers the vetting question "is this user a karma farmer or a real
contributor in the subs I care about?"

Use --in to restrict the per-sub breakdown to a comma-separated list of subs.`,
		Example: `  reddit-pp-cli user dossier spez --in programming,golang
  reddit-pp-cli user dossier some-candidate --in mysub,relatedsub --agent`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			username := strings.TrimSpace(args[0])
			if username == "" {
				return usageErr(fmt.Errorf("username required"))
			}
			username = strings.TrimPrefix(username, "u/")
			username = strings.TrimPrefix(username, "/u/")

			if dryRunOK(flags) {
				return nil
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := client.New(cfg, flags.timeout, flags.rateLimit)

			report, err := buildUserDossier(cmd.Context(), c, username, splitCSV(inSubs), limit, topN)
			if err != nil {
				return apiErr(err)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			renderDossier(cmd.OutOrStdout(), report)
			return nil
		},
	}
	cmd.Flags().StringVar(&inSubs, "in", "", "Comma-separated subreddits to restrict the dossier to")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max submissions+comments to fetch per user (each)")
	cmd.Flags().IntVar(&topN, "top", 5, "Number of recent top posts to surface in the report")
	return cmd
}

func splitCSV(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func buildUserDossier(ctx context.Context, c *client.Client, username string, inSubs []string, limit, topN int) (*userDossierReport, error) {
	rpt := &userDossierReport{Username: username}

	// 1. about
	aboutPath := "/user/" + username + "/about"
	body, err := c.Get(ctx, aboutPath, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching about: %w", err)
	}
	var aboutEnv struct {
		Data struct {
			IsSuspended  bool    `json:"is_suspended"`
			CreatedUTC   float64 `json:"created_utc"`
			LinkKarma    int     `json:"link_karma"`
			CommentKarma int     `json:"comment_karma"`
			TotalKarma   int     `json:"total_karma"`
			IsMod        bool    `json:"is_mod"`
			IsGold       bool    `json:"is_gold"`
			IsEmployee   bool    `json:"is_employee"`
		} `json:"data"`
	}
	_ = json.Unmarshal(body, &aboutEnv)
	rpt.IsSuspended = aboutEnv.Data.IsSuspended
	rpt.CreatedUTC = aboutEnv.Data.CreatedUTC
	rpt.LinkKarma = aboutEnv.Data.LinkKarma
	rpt.CommentKarma = aboutEnv.Data.CommentKarma
	rpt.TotalKarma = aboutEnv.Data.TotalKarma
	rpt.IsMod = aboutEnv.Data.IsMod
	rpt.IsGold = aboutEnv.Data.IsGold
	rpt.IsEmployee = aboutEnv.Data.IsEmployee

	subFilter := map[string]bool{}
	for _, s := range inSubs {
		subFilter[strings.ToLower(s)] = true
	}

	bySub := map[string]*perSubActivity{}
	getOrInit := func(sr string) *perSubActivity {
		if a, ok := bySub[sr]; ok {
			return a
		}
		a := &perSubActivity{Sub: sr}
		bySub[sr] = a
		return a
	}

	// 2. submitted
	submittedPath := "/user/" + username + "/submitted"
	subBody, err := c.Get(ctx, submittedPath, map[string]string{
		"limit": fmt.Sprintf("%d", limit),
		"sort":  "new",
	})
	if err == nil {
		var env struct {
			Data struct {
				Children []struct {
					Data struct {
						Subreddit  string  `json:"subreddit"`
						Score      int     `json:"score"`
						CreatedUTC float64 `json:"created_utc"`
						Title      string  `json:"title"`
						Permalink  string  `json:"permalink"`
					} `json:"data"`
				} `json:"children"`
			} `json:"data"`
		}
		if err := json.Unmarshal(subBody, &env); err == nil {
			for _, ch := range env.Data.Children {
				sr := ch.Data.Subreddit
				if len(subFilter) > 0 && !subFilter[strings.ToLower(sr)] {
					continue
				}
				a := getOrInit(sr)
				a.SubmissionCount++
				a.SubmissionScore += ch.Data.Score
				if ch.Data.CreatedUTC > a.LastActivityUTC {
					a.LastActivityUTC = ch.Data.CreatedUTC
				}
				rpt.OverallTotals.Submissions++

				// Top posts pool
				rpt.RecentTopPosts = append(rpt.RecentTopPosts, struct {
					Sub       string  `json:"subreddit"`
					Title     string  `json:"title"`
					Score     int     `json:"score"`
					Permalink string  `json:"permalink"`
					CreatedAt float64 `json:"created_utc"`
				}{Sub: sr, Title: ch.Data.Title, Score: ch.Data.Score, Permalink: ch.Data.Permalink, CreatedAt: ch.Data.CreatedUTC})
			}
		}
	}

	// 3. comments
	commentsPath := "/user/" + username + "/comments"
	cmBody, err := c.Get(ctx, commentsPath, map[string]string{
		"limit": fmt.Sprintf("%d", limit),
		"sort":  "new",
	})
	if err == nil {
		var env struct {
			Data struct {
				Children []struct {
					Data struct {
						Subreddit  string  `json:"subreddit"`
						Score      int     `json:"score"`
						CreatedUTC float64 `json:"created_utc"`
					} `json:"data"`
				} `json:"children"`
			} `json:"data"`
		}
		if err := json.Unmarshal(cmBody, &env); err == nil {
			for _, ch := range env.Data.Children {
				sr := ch.Data.Subreddit
				if len(subFilter) > 0 && !subFilter[strings.ToLower(sr)] {
					continue
				}
				a := getOrInit(sr)
				a.CommentCount++
				a.CommentScore += ch.Data.Score
				if ch.Data.CreatedUTC > a.LastActivityUTC {
					a.LastActivityUTC = ch.Data.CreatedUTC
				}
				rpt.OverallTotals.Comments++
			}
		}
	}

	for _, a := range bySub {
		if a.LastActivityUTC > 0 {
			a.LastActivityHuman = time.Unix(int64(a.LastActivityUTC), 0).UTC().Format(time.RFC3339)
		}
		rpt.PerSub = append(rpt.PerSub, *a)
	}
	sort.Slice(rpt.PerSub, func(i, j int) bool {
		ti := rpt.PerSub[i].SubmissionCount + rpt.PerSub[i].CommentCount
		tj := rpt.PerSub[j].SubmissionCount + rpt.PerSub[j].CommentCount
		return ti > tj
	})

	sort.Slice(rpt.RecentTopPosts, func(i, j int) bool {
		return rpt.RecentTopPosts[i].Score > rpt.RecentTopPosts[j].Score
	})
	if len(rpt.RecentTopPosts) > topN {
		rpt.RecentTopPosts = rpt.RecentTopPosts[:topN]
	}

	return rpt, nil
}

func renderDossier(w io.Writer, r *userDossierReport) {
	if r.IsSuspended {
		fmt.Fprintf(w, "u/%s — SUSPENDED\n", r.Username)
		return
	}
	created := ""
	if r.CreatedUTC > 0 {
		created = time.Unix(int64(r.CreatedUTC), 0).UTC().Format("2006-01-02")
	}
	fmt.Fprintf(w, "u/%s — created %s — karma: link=%d comment=%d total=%d\n",
		r.Username, created, r.LinkKarma, r.CommentKarma, r.TotalKarma)
	fmt.Fprintf(w, "Activity: %d submissions, %d comments across %d sub(s)\n\n",
		r.OverallTotals.Submissions, r.OverallTotals.Comments, len(r.PerSub))
	fmt.Fprintln(w, "Subreddit            Subs   SubScore  Comms  CmtScore  LastActivity")
	for _, a := range r.PerSub {
		fmt.Fprintf(w, "%-20s %-6d %-9d %-6d %-9d %s\n",
			truncate("r/"+a.Sub, 20), a.SubmissionCount, a.SubmissionScore, a.CommentCount, a.CommentScore, a.LastActivityHuman)
	}
	if len(r.RecentTopPosts) > 0 {
		fmt.Fprintln(w, "\nTop posts:")
		for i, p := range r.RecentTopPosts {
			fmt.Fprintf(w, "  %d. [r/%s, %d pts] %s\n     https://reddit.com%s\n", i+1, p.Sub, p.Score, p.Title, p.Permalink)
		}
	}
}

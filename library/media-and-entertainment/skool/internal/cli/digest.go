// Copyright 2026 quoxientzero. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newDigestCmd is the parent for digest sub-commands.
func newDigestCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Time-windowed activity aggregates across a community",
	}
	cmd.AddCommand(newDigestSinceCmd(flags))
	return cmd
}

// newDigestSinceCmd: skool-pp-cli digest since <duration>
// Computes a fast aggregate of new posts and active members within a window.
func newDigestSinceCmd(flags *rootFlags) *cobra.Command {
	var flagCommunity string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "since <duration>",
		Short: "Aggregate new posts and active members in a community since a duration ago",
		Example: "  skool-pp-cli digest since 24h --community bewarethedefault --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			cutoff, err := parseSinceArg(args[0])
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			community := strings.TrimSpace(flagCommunity)
			if community == "" && c.Config != nil {
				community = c.Config.TemplateVars["community"]
			}
			if community == "" {
				return usageErr(fmt.Errorf("--community is required (or set template_vars.community in config)"))
			}

			path := "/_next/data/{buildId}/" + community + ".json"
			params := map[string]string{"g": community}
			raw, err := c.Get(path, params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var envelope struct {
				PageProps struct {
					Total     int `json:"total"`
					PostTrees []struct {
						Post struct {
							ID        string `json:"id"`
							Name      string `json:"name"`
							CreatedAt string `json:"createdAt"`
							UpdatedAt string `json:"updatedAt"`
							UserID    string `json:"userId"`
							Metadata  struct {
								Comments int `json:"comments"`
								Upvotes  int `json:"upvotes"`
								Content  string `json:"content"`
							} `json:"metadata"`
							User struct {
								Name     string `json:"name"`
								FirstName string `json:"firstName"`
							} `json:"user"`
						} `json:"post"`
					} `json:"postTrees"`
					LeaderboardsData struct {
						Users []struct {
							UserID string `json:"userId"`
							User   struct {
								Name     string `json:"name"`
								Metadata struct {
									LastOffline int64 `json:"lastOffline"`
								} `json:"metadata"`
							} `json:"user"`
						} `json:"users"`
					} `json:"leaderboardsData"`
				} `json:"pageProps"`
			}
			if err := json.Unmarshal(raw, &envelope); err != nil {
				return fmt.Errorf("parsing digest response: %w", err)
			}

			type postRow struct {
				ID         string `json:"id"`
				Name       string `json:"name"`
				CreatedAt  string `json:"created_at"`
				By         string `json:"by"`
				Comments   int    `json:"comments"`
				Upvotes    int    `json:"upvotes"`
				Snippet    string `json:"snippet,omitempty"`
			}
			newPosts := make([]postRow, 0, 16)
			for _, pt := range envelope.PageProps.PostTrees {
				p := pt.Post
				ts, perr := time.Parse(time.RFC3339, p.CreatedAt)
				if perr != nil {
					continue
				}
				if ts.Before(cutoff) {
					continue
				}
				snip := p.Metadata.Content
				if len(snip) > 160 {
					snip = snip[:160] + "…"
				}
				row := postRow{
					ID: p.ID, Name: p.Name, CreatedAt: p.CreatedAt,
					By: p.User.Name, Comments: p.Metadata.Comments, Upvotes: p.Metadata.Upvotes,
					Snippet: snip,
				}
				newPosts = append(newPosts, row)
				if flagLimit > 0 && len(newPosts) >= flagLimit {
					break
				}
			}

			activeMembers := 0
			cutoffNano := cutoff.UnixNano()
			for _, u := range envelope.PageProps.LeaderboardsData.Users {
				// Skool stores lastOffline as nanoseconds since epoch
				if u.User.Metadata.LastOffline > 0 && u.User.Metadata.LastOffline >= cutoffNano {
					activeMembers++
				}
			}

			out := map[string]any{
				"since":            cutoff.UTC().Format(time.RFC3339),
				"community":        community,
				"new_posts":        newPosts,
				"new_post_count":   len(newPosts),
				"active_members":   activeMembers,
				"community_total":  envelope.PageProps.Total,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagCommunity, "community", "", "Community slug (defaults to template_vars.community)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Cap new_posts to N (0 = all matching)")
	return cmd
}

// parseSinceArg accepts duration strings (24h, 7d, 30m, 2h45m) or absolute
// dates (YYYY-MM-DD, RFC3339). Returns the absolute cutoff time.
func parseSinceArg(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("since duration is required (e.g. 24h, 7d, 2026-05-01)")
	}
	// Try Go duration first (with `d` extension).
	if d, ok := parseFlexibleDuration(s); ok {
		return time.Now().Add(-d), nil
	}
	// Try RFC3339.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	// Try YYYY-MM-DD.
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("could not parse %q as duration (e.g. 24h, 7d) or date (2026-05-01)", s)
}

// parseFlexibleDuration extends time.ParseDuration with day support.
func parseFlexibleDuration(s string) (time.Duration, bool) {
	// Handle pure "Nd" first.
	if strings.HasSuffix(s, "d") {
		nStr := strings.TrimSuffix(s, "d")
		n, err := strconv.Atoi(nStr)
		if err == nil && n > 0 {
			return time.Duration(n) * 24 * time.Hour, true
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, false
	}
	return d, true
}

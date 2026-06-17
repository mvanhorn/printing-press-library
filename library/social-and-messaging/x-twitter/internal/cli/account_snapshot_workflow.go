// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/client"
	"github.com/spf13/cobra"
)

type accountSnapshotResult struct {
	Profile    *accountSnapshotProfile `json:"profile"`
	Recent     []resolvedPostRecord    `json:"recent_posts,omitempty"`
	PinnedPost *resolvedPostRecord     `json:"pinned_post,omitempty"`
	Warnings   []string                `json:"warnings,omitempty"`
}

func newNovelAccountCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "account",
		Short:       "Account-level power-user workflows",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelAccountSnapshotCmd(flags))
	cmd.AddCommand(newNovelAccountWhoamiCmd(flags))
	return cmd
}

func newNovelAccountWhoamiCmd(flags *rootFlags) *cobra.Command {
	var live bool
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Show the signed-in account identity available to this CLI",
		Long:  "Shows the X Articles cookie user id from captured browser cookies. With --live, also probes /2/users/me using OAuth2 user-context auth.",
		Example: `  x-twitter-pp-cli account whoami --agent
  x-twitter-pp-cli account whoami --live --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			result := map[string]any{
				"x_articles_cookie": map[string]any{
					"status": "missing",
				},
			}
			if cookies, err := client.LoadCookieAuth(); err == nil {
				cookieLane := map[string]any{"status": "ok"}
				if userID := cookies.ArticleUserID(); userID != "" {
					cookieLane["user_id"] = userID
				}
				result["x_articles_cookie"] = cookieLane
			} else {
				result["x_articles_cookie"].(map[string]any)["error"] = err.Error()
			}
			if live {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				body, err := c.GetNoCache(cmd.Context(), "/2/users/me", nil)
				if err != nil {
					result["oauth2_user_context"] = map[string]any{
						"status": "error",
						"error":  err.Error(),
					}
				} else if user := userSummaryFromMeProbe(body); len(user) > 0 {
					result["oauth2_user_context"] = map[string]any{
						"status": "ok",
						"user":   user,
					}
				} else {
					result["oauth2_user_context"] = map[string]any{"status": "unknown"}
				}
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}
			cookieLane, _ := result["x_articles_cookie"].(map[string]any)
			if userID, _ := cookieLane["user_id"].(string); userID != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "X Articles cookie user id: %s\n", userID)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "X Articles cookie user id: unavailable\n")
			}
			if oauthLane, _ := result["oauth2_user_context"].(map[string]any); len(oauthLane) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "OAuth2 user-context: %v\n", oauthLane["status"])
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&live, "live", false, "Also probe /2/users/me with OAuth2 user-context auth")
	return cmd
}

func newNovelAccountSnapshotCmd(flags *rootFlags) *cobra.Command {
	var dbPath, include, format string
	var recent int
	var live bool
	cmd := &cobra.Command{
		Use:   "snapshot <username-or-id>",
		Short: "Capture a profile and recent-post snapshot for an X account",
		Example: `  x-twitter-pp-cli account snapshot @username --recent 20 --agent
  x-twitter-pp-cli account snapshot 12345 --include recent,profile,metrics --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			mode := flags.dataSource
			if live {
				mode = "live"
			}
			result, err := buildAccountSnapshot(cmd, flags, args[0], dbPath, mode, include, recent)
			if err != nil {
				return err
			}
			if strings.EqualFold(format, "markdown") || strings.EqualFold(format, "md") {
				return writeAccountSnapshotMarkdown(cmd.OutOrStdout(), result)
			}
			if format != "" && !strings.EqualFold(format, "json") {
				return usageErr(fmt.Errorf("invalid --format %q: expected json or markdown", format))
			}
			return printJSONFiltered(cmd.OutOrStdout(), workflowEnvelope{
				Meta:     workflowMeta("account snapshot", result.Profile.Source),
				Results:  result,
				Warnings: result.Warnings,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local database")
	cmd.Flags().StringVar(&include, "include", "profile,recent,metrics,pinned", "Comma-separated sections: profile,recent,metrics,pinned,raw")
	cmd.Flags().IntVar(&recent, "recent", 20, "Number of recent posts to include when recent is enabled")
	cmd.Flags().BoolVar(&live, "live", false, "Bypass local lookup for account/profile data")
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json or markdown")
	return cmd
}

func buildAccountSnapshot(cmd *cobra.Command, flags *rootFlags, input, dbPath, mode, include string, recent int) (*accountSnapshotResult, error) {
	sections := parseIncludeSet(include)
	if len(sections) == 0 {
		sections = parseIncludeSet("profile,recent,metrics,pinned")
	}
	profile, err := resolveAccountProfile(cmd, flags, input, dbPath, mode, sections["raw"])
	if err != nil {
		return nil, err
	}
	result := &accountSnapshotResult{Profile: profile}
	postInclude := parseIncludeSet("author,links,refs,metrics")
	if !sections["metrics"] {
		delete(postInclude, "metrics")
		profile.PublicMetrics = nil
	}
	if dbPath == "" {
		dbPath = defaultDBPath("x-twitter-pp-cli")
	}
	if sections["recent"] && recent > 0 {
		var posts []*resolvedPostRecord
		var err error
		if mode != "live" {
			posts, err = localRecentPostsForAccount(cmd, dbPath, profile.ID, recent, postInclude)
		}
		if (err != nil || len(posts) == 0) && mode != "local" {
			posts, err = liveRecentPostsForAccount(cmd, flags, profile.ID, recent, postInclude)
		}
		if err != nil {
			result.Warnings = append(result.Warnings, "recent_posts_unavailable")
		}
		for _, rec := range posts {
			if rec != nil {
				result.Recent = append(result.Recent, *rec)
			}
		}
	}
	if sections["pinned"] && profile.PinnedTweetID != "" {
		pinned, err := resolvePost(cmd, flags, profile.PinnedTweetID, dbPath, mode, postInclude)
		if err == nil {
			result.PinnedPost = pinned
		} else {
			result.Warnings = append(result.Warnings, "pinned_post_unavailable")
		}
	}
	return result, nil
}

func writeAccountSnapshotMarkdown(w workflowWriter, result *accountSnapshotResult) error {
	if result == nil || result.Profile == nil {
		return nil
	}
	p := result.Profile
	if err := workflowFprintf(w, "# %s\n\n", p.ProfileURL); err != nil {
		return err
	}
	if p.Name != "" {
		if err := workflowFprintf(w, "- Name: %s\n", p.Name); err != nil {
			return err
		}
	}
	if p.Description != "" {
		if err := workflowFprintf(w, "- Bio: %s\n", p.Description); err != nil {
			return err
		}
	}
	if p.Location != "" {
		if err := workflowFprintf(w, "- Location: %s\n", p.Location); err != nil {
			return err
		}
	}
	if p.URL != "" {
		if err := workflowFprintf(w, "- URL: %s\n", p.URL); err != nil {
			return err
		}
	}
	if len(p.PublicMetrics) > 0 {
		if err := workflowFprintf(w, "- Metrics: %v\n", p.PublicMetrics); err != nil {
			return err
		}
	}
	if err := workflowFprintln(w); err != nil {
		return err
	}
	if result.PinnedPost != nil {
		if err := workflowFprintf(w, "## Pinned\n\n[%s](%s)\n\n%s\n\n", result.PinnedPost.TweetID, result.PinnedPost.URL, result.PinnedPost.Text); err != nil {
			return err
		}
	}
	if len(result.Recent) > 0 {
		if err := workflowFprintln(w, "## Recent Posts"); err != nil {
			return err
		}
		for _, post := range result.Recent {
			if err := workflowFprintf(w, "### %s\n\n%s\n\n", post.URL, post.Text); err != nil {
				return err
			}
		}
	}
	return nil
}

// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel-feature command (Phase 3).

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type modRules struct {
	Defaults struct {
		BanAuthorOnReject bool `yaml:"ban_author_on_reject"`
	} `yaml:"defaults"`
	Rules []struct {
		Name        string `yaml:"name"`
		Match       string `yaml:"match"`      // regex pattern on comment text
		Author      string `yaml:"author"`     // optional regex on author display name
		Action      string `yaml:"action"`     // approve | reject
		BanAuthor   bool   `yaml:"ban_author"` // override default for this rule
		Description string `yaml:"description"`
	} `yaml:"rules"`
}

type heldComment struct {
	ID          string
	Text        string
	Author      string
	PublishedAt time.Time
	VideoID     string
	ChannelID   string
	Raw         json.RawMessage
}

func newModCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mod",
		Short: "Moderate comments: queue, batch approve/reject, pattern-based auto-mod",
		Long: `Wraps the YouTube Data API's commentThreads + comments endpoints with
moderation-specific verbs. 'mod queue' surfaces the heldForReview backlog;
'mod approve|reject' applies setModerationStatus in batches; 'mod auto'
applies YAML-defined rules to incoming or held comments.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newModQueueCmd(flags))
	cmd.AddCommand(newModApproveCmd(flags))
	cmd.AddCommand(newModRejectCmd(flags))
	cmd.AddCommand(newModAutoCmd(flags))
	return cmd
}

func newModQueueCmd(flags *rootFlags) *cobra.Command {
	var since string
	var limit int
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "List comments held for review (heldForReview moderation status)",
		Long: `Pages through commentThreads.list with moderationStatus=heldForReview for
the authenticated channel. Output is a JSON array of held comment threads
with the snippet fields agents need to make a decision.

Quota cost: ~1 unit per page. For large queues use --limit to cap.`,
		Example: "  youtube-pp-cli mod queue --since 7d --limit 200 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:endpoint":   "youtube.comment-threads-list",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// commentThreads.list requires one of (allThreadsRelatedToChannelId, channelId,
			// videoId, postId, id). For the moderation queue on the authenticated user's
			// channel, we resolve the channel ID first via channels.list?mine=true.
			quotaLogCost("channels-list", 1)
			chData, err := c.Get("/youtube/v3/channels", map[string]string{
				"part": "id",
				"mine": "true",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var chResp struct {
				Items []struct {
					ID string `json:"id"`
				} `json:"items"`
			}
			_ = json.Unmarshal(chData, &chResp)
			if len(chResp.Items) == 0 {
				return apiErr(fmt.Errorf("no authenticated channel found"))
			}
			myChannelID := chResp.Items[0].ID

			params := map[string]string{
				"part":                         "snippet,replies",
				"moderationStatus":             "heldForReview",
				"allThreadsRelatedToChannelId": myChannelID,
				"maxResults":                   "100",
			}

			var items []json.RawMessage
			pageToken := ""
			pages := 0
			for {
				if pageToken != "" {
					params["pageToken"] = pageToken
				}
				quotaLogCost("comment-threads-list", 1)
				data, err := c.Get("/youtube/v3/commentThreads", params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var page struct {
					Items         []json.RawMessage `json:"items"`
					NextPageToken string            `json:"nextPageToken"`
				}
				if err := json.Unmarshal(data, &page); err != nil {
					return fmt.Errorf("decoding page: %w", err)
				}
				items = append(items, page.Items...)
				pages++
				if page.NextPageToken == "" || (limit > 0 && len(items) >= limit) {
					break
				}
				pageToken = page.NextPageToken
				if pages > 50 {
					break // safety cap
				}
			}
			if limit > 0 && len(items) > limit {
				items = items[:limit]
			}

			// Filter by since (best-effort: parse publishedAt from each item)
			if since != "" {
				cutoff, err := parseSince(since)
				if err == nil {
					filtered := items[:0]
					for _, it := range items {
						var snip struct {
							Snippet struct {
								TopLevelComment struct {
									Snippet struct {
										PublishedAt time.Time `json:"publishedAt"`
									} `json:"snippet"`
								} `json:"topLevelComment"`
							} `json:"snippet"`
						}
						_ = json.Unmarshal(it, &snip)
						if snip.Snippet.TopLevelComment.Snippet.PublishedAt.After(cutoff) {
							filtered = append(filtered, it)
						}
					}
					items = filtered
				}
			}

			return flags.printJSON(cmd, map[string]any{
				"kind":       "youtube#heldCommentList",
				"count":      len(items),
				"pages":      pages,
				"quota_cost": pages,
				"items":      items,
			})
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only include comments newer than this duration (e.g. 24h, 7d)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of comments to return (0 = no limit)")
	return cmd
}

func parseSince(s string) (time.Time, error) {
	// Accept "7d", "24h", "30m"
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		days := 0
		if _, err := fmt.Sscanf(strings.TrimSuffix(s, "d"), "%d", &days); err != nil {
			return time.Time{}, err
		}
		return time.Now().Add(-time.Duration(days) * 24 * time.Hour), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().Add(-d), nil
}

func setModerationStatus(c clientLike, ids []string, status string, banAuthor bool) (json.RawMessage, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("no comment IDs provided")
	}
	params := map[string]string{
		"id":               strings.Join(ids, ","),
		"moderationStatus": status,
	}
	if banAuthor {
		params["banAuthor"] = "true"
	}
	// comments.setModerationStatus is a POST with no body, params in query
	path := "/youtube/v3/comments/setModerationStatus?" + url.Values{
		"id":               {strings.Join(ids, ",")},
		"moderationStatus": {status},
	}.Encode()
	if banAuthor {
		path += "&banAuthor=true"
	}
	quotaLogCost("comments-set-mod", 50)
	data, _, err := c.Post(path, nil)
	return data, err
}

// clientLike is the small interface we need from client.Client. Lets tests stub.
type clientLike interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
	Post(path string, body any) (json.RawMessage, int, error)
}

func newModApproveCmd(flags *rootFlags) *cobra.Command {
	var idsFlag string
	var banAuthor bool
	cmd := &cobra.Command{
		Use:   "approve [comment-ids...]",
		Short: "Approve comments via comments.setModerationStatus=published",
		Long: `Accepts comment IDs as args, comma-separated --ids, or whitespace-separated
on stdin. Sends one batched setModerationStatus call (50 units per call,
regardless of batch size).`,
		Example: "  youtube-pp-cli mod approve abc123 def456\n" +
			"  youtube-pp-cli mod queue --json | jq -r '.items[].id' | youtube-pp-cli mod approve",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := collectIDs(args, idsFlag)
			if len(ids) == 0 && !flags.dryRun {
				return usageErr(fmt.Errorf("no comment IDs provided (pass as args, --ids, or stdin)"))
			}
			if flags.dryRun {
				return flags.printJSON(cmd, map[string]any{
					"would_set":   "published",
					"comment_ids": ids,
					"ban_author":  banAuthor,
					"quota_cost":  50,
				})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			_, err = setModerationStatus(c, ids, "published", banAuthor)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return flags.printJSON(cmd, map[string]any{
				"approved":   len(ids),
				"ban_author": banAuthor,
			})
		},
	}
	cmd.Flags().StringVar(&idsFlag, "ids", "", "Comma-separated comment IDs (alternative to args)")
	cmd.Flags().BoolVar(&banAuthor, "ban-author", false, "Also ban the comment author (banAuthor=true)")
	return cmd
}

func newModRejectCmd(flags *rootFlags) *cobra.Command {
	var idsFlag string
	var banAuthor bool
	cmd := &cobra.Command{
		Use:   "reject [comment-ids...]",
		Short: "Reject comments via comments.setModerationStatus=rejected",
		Long: `Accepts comment IDs as args, comma-separated --ids, or whitespace-separated
on stdin. The 'rejected' status is the equivalent of the deprecated
markAsSpam endpoint; YouTube no longer surfaces those comments publicly.`,
		Example:     "  youtube-pp-cli mod reject abc123 --ban-author",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := collectIDs(args, idsFlag)
			if len(ids) == 0 && !flags.dryRun {
				return usageErr(fmt.Errorf("no comment IDs provided (pass as args, --ids, or stdin)"))
			}
			if flags.dryRun {
				return flags.printJSON(cmd, map[string]any{
					"would_set":   "rejected",
					"comment_ids": ids,
					"ban_author":  banAuthor,
					"quota_cost":  50,
				})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			_, err = setModerationStatus(c, ids, "rejected", banAuthor)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return flags.printJSON(cmd, map[string]any{
				"rejected":   len(ids),
				"ban_author": banAuthor,
			})
		},
	}
	cmd.Flags().StringVar(&idsFlag, "ids", "", "Comma-separated comment IDs (alternative to args)")
	cmd.Flags().BoolVar(&banAuthor, "ban-author", false, "Also ban the comment author (banAuthor=true)")
	return cmd
}

func collectIDs(args []string, idsFlag string) []string {
	var ids []string
	if idsFlag != "" {
		for _, s := range strings.Split(idsFlag, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				ids = append(ids, s)
			}
		}
	}
	ids = append(ids, args...)
	// If no args/flag, try stdin
	if len(ids) == 0 {
		fi, _ := os.Stdin.Stat()
		if (fi.Mode() & os.ModeCharDevice) == 0 {
			data, _ := os.ReadFile("/dev/stdin")
			for _, line := range strings.Fields(string(data)) {
				ids = append(ids, line)
			}
		}
	}
	return ids
}

func newModAutoCmd(flags *rootFlags) *cobra.Command {
	var rulesPath, since string
	var apply bool
	cmd := &cobra.Command{
		Use:   "auto",
		Short: "Apply YAML-defined regex rules to held comments, with --apply to actually mutate",
		Long: `Reads a rules YAML and applies it to the heldForReview queue (or to the
recent comment stream when --since is set). Each rule has a match regex on
comment text (and optional author regex) plus an action (approve|reject).

Without --apply, the command prints a dry-run showing which rules would
fire for which comments.

Example rules.yaml:

  defaults:
    ban_author_on_reject: false
  rules:
    - name: spam-link-emoji
      match: '(?i)(check my channel|🔗.*free)'
      action: reject
      ban_author: true
    - name: thanks-from-known-fan
      author: '^(SuperFan42|LongtimeViewer)$'
      action: approve`,
		Example:     "  youtube-pp-cli mod auto --rules rules.yaml\n  youtube-pp-cli mod auto --rules rules.yaml --apply",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if rulesPath == "" {
				if flags.dryRun {
					return nil
				}
				return usageErr(fmt.Errorf("--rules is required"))
			}
			rulesData, err := os.ReadFile(rulesPath)
			if err != nil {
				return configErr(fmt.Errorf("reading rules: %w", err))
			}
			var rules modRules
			if err := yaml.Unmarshal(rulesData, &rules); err != nil {
				return configErr(fmt.Errorf("parsing rules YAML: %w", err))
			}

			// Pre-compile regexes
			type compiledRule struct {
				Name      string
				Text      *regexp.Regexp
				Author    *regexp.Regexp
				Action    string
				BanAuthor bool
			}
			var compiled []compiledRule
			for _, r := range rules.Rules {
				cr := compiledRule{
					Name:      r.Name,
					Action:    r.Action,
					BanAuthor: r.BanAuthor || rules.Defaults.BanAuthorOnReject && r.Action == "reject",
				}
				if r.Match != "" {
					re, err := regexp.Compile(r.Match)
					if err != nil {
						return configErr(fmt.Errorf("rule %q: bad match regex: %w", r.Name, err))
					}
					cr.Text = re
				}
				if r.Author != "" {
					re, err := regexp.Compile(r.Author)
					if err != nil {
						return configErr(fmt.Errorf("rule %q: bad author regex: %w", r.Name, err))
					}
					cr.Author = re
				}
				compiled = append(compiled, cr)
			}

			if flags.dryRun {
				return flags.printJSON(cmd, map[string]any{
					"would_fetch":  "comment-threads-list?moderationStatus=heldForReview",
					"rules_loaded": len(compiled),
					"would_apply":  apply,
				})
			}

			// Fetch held queue. commentThreads.list requires one of
			// (allThreadsRelatedToChannelId, channelId, videoId, postId, id) —
			// resolve the authenticated channel ID first, same as mod queue.
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			quotaLogCost("channels-list", 1)
			chData, err := c.Get("/youtube/v3/channels", map[string]string{
				"part": "id",
				"mine": "true",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var chResp struct {
				Items []struct {
					ID string `json:"id"`
				} `json:"items"`
			}
			_ = json.Unmarshal(chData, &chResp)
			if len(chResp.Items) == 0 {
				return apiErr(fmt.Errorf("no authenticated channel found"))
			}
			myChannelID := chResp.Items[0].ID

			params := map[string]string{
				"part":                         "snippet",
				"moderationStatus":             "heldForReview",
				"allThreadsRelatedToChannelId": myChannelID,
				"maxResults":                   "100",
			}

			// Paginate exactly like newModQueueCmd so backlogs >100 are
			// covered. A single-page fetch silently dropped every comment
			// past item 100 and made queue_size/applied_count understate
			// the work that should have been done.
			type queueItem struct {
				ID      string `json:"id"`
				Snippet struct {
					TopLevelComment struct {
						ID      string `json:"id"`
						Snippet struct {
							TextOriginal      string    `json:"textOriginal"`
							AuthorDisplayName string    `json:"authorDisplayName"`
							PublishedAt       time.Time `json:"publishedAt"`
						} `json:"snippet"`
					} `json:"topLevelComment"`
				} `json:"snippet"`
			}
			var allItems []queueItem
			pageToken := ""
			pages := 0
			for {
				if pageToken != "" {
					params["pageToken"] = pageToken
				}
				quotaLogCost("comment-threads-list", 1)
				data, err := c.Get("/youtube/v3/commentThreads", params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var page struct {
					Items         []queueItem `json:"items"`
					NextPageToken string      `json:"nextPageToken"`
				}
				if err := json.Unmarshal(data, &page); err != nil {
					return fmt.Errorf("decoding page: %w", err)
				}
				allItems = append(allItems, page.Items...)
				pages++
				if page.NextPageToken == "" {
					break
				}
				pageToken = page.NextPageToken
				if pages > 50 {
					break // safety cap, mirrors mod queue
				}
			}

			// Match rules
			type decision struct {
				CommentID string `json:"comment_id"`
				Author    string `json:"author"`
				Text      string `json:"text"`
				Rule      string `json:"rule"`
				Action    string `json:"action"`
				BanAuthor bool   `json:"ban_author"`
			}
			var decisions []decision
			cutoff := time.Time{}
			if since != "" {
				if t, err := parseSince(since); err == nil {
					cutoff = t
				}
			}

			for _, it := range allItems {
				snip := it.Snippet.TopLevelComment.Snippet
				if !cutoff.IsZero() && snip.PublishedAt.Before(cutoff) {
					continue
				}
				for _, r := range compiled {
					textOK := r.Text == nil || r.Text.MatchString(snip.TextOriginal)
					authorOK := r.Author == nil || r.Author.MatchString(snip.AuthorDisplayName)
					if textOK && authorOK {
						decisions = append(decisions, decision{
							CommentID: it.Snippet.TopLevelComment.ID,
							Author:    snip.AuthorDisplayName,
							Text:      truncate(snip.TextOriginal, 80),
							Rule:      r.Name,
							Action:    r.Action,
							BanAuthor: r.BanAuthor,
						})
						break // first matching rule wins
					}
				}
			}

			result := map[string]any{
				"queue_size": len(allItems),
				"decisions":  decisions,
				"applied":    apply,
			}

			if !apply {
				result["note"] = "Dry preview; pass --apply to mutate comments."
				return flags.printJSON(cmd, result)
			}

			// Apply: group decisions by (action, banAuthor) and batch
			type batchKey struct {
				Action    string
				BanAuthor bool
			}
			batches := map[batchKey][]string{}
			for _, d := range decisions {
				k := batchKey{Action: d.Action, BanAuthor: d.BanAuthor}
				batches[k] = append(batches[k], d.CommentID)
			}
			applied := 0
			for k, ids := range batches {
				_, err := setModerationStatus(c, ids, k.Action, k.BanAuthor)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				applied += len(ids)
			}
			result["applied_count"] = applied
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&rulesPath, "rules", "", "Path to rules.yaml")
	cmd.Flags().StringVar(&since, "since", "", "Only consider comments newer than this duration (e.g. 1h, 7d)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually apply mutations (default: dry preview)")
	return cmd
}

// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/client"
	"github.com/spf13/cobra"
)

// topPostsMetrics is the set of ranking metrics top-posts accepts.
var topPostsMetrics = []string{"engagement", "likes", "retweets", "replies", "quotes", "bookmarks", "impressions"}

// topPostsPageSize is the maximum posts the X timeline endpoint returns per page.
const topPostsPageSize = 100

// publicMetrics is the subset of a Tweet's public_metrics that top-posts ranks
// on. impression_count is only populated for the authenticated user's own posts
// on Basic tier and above, so it is a pointer: nil means the API omitted the
// field (free tier) rather than a genuine zero.
type publicMetrics struct {
	RetweetCount    int  `json:"retweet_count"`
	ReplyCount      int  `json:"reply_count"`
	LikeCount       int  `json:"like_count"`
	QuoteCount      int  `json:"quote_count"`
	BookmarkCount   int  `json:"bookmark_count"`
	ImpressionCount *int `json:"impression_count"`
}

// tweetItem is the subset of a /2/users/{id}/tweets entry top-posts decodes.
type tweetItem struct {
	ID            string        `json:"id"`
	Text          string        `json:"text"`
	CreatedAt     string        `json:"created_at"`
	PublicMetrics publicMetrics `json:"public_metrics"`
}

// rankedPost is one row of the ranked readout.
type rankedPost struct {
	Rank        int    `json:"rank"`
	ID          string `json:"id"`
	URL         string `json:"url"`
	CreatedAt   string `json:"created_at,omitempty"`
	Text        string `json:"text"`
	Likes       int    `json:"likes"`
	Retweets    int    `json:"retweets"`
	Replies     int    `json:"replies"`
	Quotes      int    `json:"quotes"`
	Bookmarks   int    `json:"bookmarks"`
	Impressions *int   `json:"impressions,omitempty"`
	Engagement  int    `json:"engagement"`
	Score       int    `json:"score"`
}

func newTopPostsCmd(flags *rootFlags) *cobra.Command {
	var flagUserID string
	var flagLimit int
	var flagMaxFetch int
	var flagMetric string
	var flagExclude string

	cmd := &cobra.Command{
		Use:         "top-posts",
		Short:       "Rank your recent posts by engagement (likes, reposts, replies, quotes)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: "  x-twitter-pp-cli top-posts\n" +
			"  x-twitter-pp-cli top-posts --metric likes --limit 5\n" +
			"  x-twitter-pp-cli top-posts --user-id 2244994945 --max-fetch 200 --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			metric := strings.ToLower(strings.TrimSpace(flagMetric))
			if !isValidTopPostsMetric(metric) {
				return fmt.Errorf("invalid --metric %q: must be one of %s", flagMetric, strings.Join(topPostsMetrics, ", "))
			}
			if flagLimit < 1 {
				return fmt.Errorf("--limit must be at least 1")
			}
			if flagMaxFetch < 1 {
				return fmt.Errorf("--max-fetch must be at least 1")
			}

			// Dry-run: emit nothing and touch no network (composite read-only contract).
			if flags.dryRun {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Resolve the target user and a username for building post URLs.
			userID := strings.TrimSpace(flagUserID)
			username := ""
			if userID == "" {
				meData, merr := c.Get(context.Background(), "/2/users/me", nil)
				if merr != nil {
					return classifyAPIError(merr, flags)
				}
				id, uname, derr := decodeUserEnvelope(meData)
				if derr != nil {
					return fmt.Errorf("decoding /2/users/me: %w", derr)
				}
				if id == "" {
					return fmt.Errorf("could not resolve authenticated user id from /2/users/me")
				}
				userID, username = id, uname
			} else {
				// Look up the username so post URLs are canonical; tolerate a
				// lookup failure by falling back to the id-based URL form.
				if uData, uerr := c.Get(context.Background(), "/2/users/"+userID, nil); uerr == nil {
					_, username, _ = decodeUserEnvelope(uData)
				}
			}

			items, err := fetchUserPosts(c, flags, userID, flagMaxFetch, flagExclude)
			if err != nil {
				return err
			}

			// Gracefully handle impression ranking on tiers that omit the field.
			effectiveMetric := metric
			if metric == "impressions" && !impressionsAvailable(items) {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: impression_count is unavailable on this access tier; ranking by engagement instead.")
				effectiveMetric = "engagement"
			}

			posts := rankTopPosts(items, username, effectiveMetric, flagLimit)

			// Surface a shortfall so a caller doesn't read fewer rows than asked
			// for as if it were the full leaderboard.
			if len(posts) < flagLimit {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"note: returned %d posts (fewer than --limit %d); only %d posts were available within --max-fetch %d.\n",
					len(posts), flagLimit, len(items), flagMaxFetch)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				// The SCORE column makes the ranking metric visible, so ordering is
				// explained in table mode even for metrics without their own column.
				headers := []string{"#", "SCORE(" + effectiveMetric + ")", "LIKES", "RTS", "REPLIES", "QUOTES", "BOOKMARKS", "IMPRESSIONS", "TEXT", "URL"}
				rows := make([][]string, 0, len(posts))
				for _, p := range posts {
					impr := "n/a"
					if p.Impressions != nil {
						impr = strconv.Itoa(*p.Impressions)
					}
					rows = append(rows, []string{
						strconv.Itoa(p.Rank),
						strconv.Itoa(p.Score),
						strconv.Itoa(p.Likes),
						strconv.Itoa(p.Retweets),
						strconv.Itoa(p.Replies),
						strconv.Itoa(p.Quotes),
						strconv.Itoa(p.Bookmarks),
						impr,
						flattenText(p.Text, 60),
						p.URL,
					})
				}
				return flags.printTable(cmd, headers, rows)
			}

			return flags.printJSON(cmd, posts)
		},
	}
	cmd.Flags().StringVar(&flagUserID, "user-id", "", "Rank posts for this user ID instead of the authenticated user.")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Number of top posts to return after ranking.")
	cmd.Flags().IntVar(&flagMaxFetch, "max-fetch", 100, "Maximum recent posts to fetch and rank (paginated, 100 per page).")
	cmd.Flags().StringVar(&flagMetric, "metric", "engagement", "Ranking metric: engagement, likes, retweets, replies, quotes, bookmarks, impressions.")
	cmd.Flags().StringVar(&flagExclude, "exclude", "", "Comma-separated entities to exclude from the timeline (e.g. replies,retweets).")
	return cmd
}

// isValidTopPostsMetric reports whether metric is one of the supported keys.
func isValidTopPostsMetric(metric string) bool {
	for _, m := range topPostsMetrics {
		if metric == m {
			return true
		}
	}
	return false
}

// engagementOf sums the public engagement actions on a post.
func engagementOf(pm publicMetrics) int {
	return pm.LikeCount + pm.RetweetCount + pm.ReplyCount + pm.QuoteCount
}

// metricScore returns the ranking value for a metric and whether it is
// available — false only for impressions when the API omitted impression_count.
func metricScore(pm publicMetrics, metric string) (int, bool) {
	switch metric {
	case "likes":
		return pm.LikeCount, true
	case "retweets":
		return pm.RetweetCount, true
	case "replies":
		return pm.ReplyCount, true
	case "quotes":
		return pm.QuoteCount, true
	case "bookmarks":
		return pm.BookmarkCount, true
	case "impressions":
		if pm.ImpressionCount == nil {
			return 0, false
		}
		return *pm.ImpressionCount, true
	default: // engagement
		return engagementOf(pm), true
	}
}

// impressionsAvailable reports whether any item carried impression_count, which
// distinguishes a tier that omits the field from posts with genuine zeroes.
func impressionsAvailable(items []tweetItem) bool {
	for _, it := range items {
		if it.PublicMetrics.ImpressionCount != nil {
			return true
		}
	}
	return false
}

// rankTopPosts sorts items by the chosen metric (descending), breaking ties by
// total engagement then by recency (higher id = newer), and returns the top
// `limit` as ranked rows.
func rankTopPosts(items []tweetItem, username, metric string, limit int) []rankedPost {
	ordered := make([]tweetItem, len(items))
	copy(ordered, items)
	sort.SliceStable(ordered, func(i, j int) bool {
		si, _ := metricScore(ordered[i].PublicMetrics, metric)
		sj, _ := metricScore(ordered[j].PublicMetrics, metric)
		if si != sj {
			return si > sj
		}
		ei, ej := engagementOf(ordered[i].PublicMetrics), engagementOf(ordered[j].PublicMetrics)
		if ei != ej {
			return ei > ej
		}
		return idNewer(ordered[i].ID, ordered[j].ID)
	})
	if limit > 0 && len(ordered) > limit {
		ordered = ordered[:limit]
	}
	posts := make([]rankedPost, 0, len(ordered))
	for i, it := range ordered {
		pm := it.PublicMetrics
		score, _ := metricScore(pm, metric)
		posts = append(posts, rankedPost{
			Rank:        i + 1,
			ID:          it.ID,
			URL:         postURL(username, it.ID),
			CreatedAt:   it.CreatedAt,
			Text:        it.Text,
			Likes:       pm.LikeCount,
			Retweets:    pm.RetweetCount,
			Replies:     pm.ReplyCount,
			Quotes:      pm.QuoteCount,
			Bookmarks:   pm.BookmarkCount,
			Impressions: pm.ImpressionCount,
			Engagement:  engagementOf(pm),
			Score:       score,
		})
	}
	return posts
}

// idNewer reports whether post id a is newer than b. X Snowflake IDs are
// monotonically increasing integers, so it compares them numerically; for any
// id that doesn't parse as int64 (overflow, non-numeric) it falls back to
// longer-is-larger then lexicographic so the ordering stays deterministic.
func idNewer(a, b string) bool {
	ai, aerr := strconv.ParseInt(a, 10, 64)
	bi, berr := strconv.ParseInt(b, 10, 64)
	if aerr == nil && berr == nil {
		return ai > bi
	}
	if len(a) != len(b) {
		return len(a) > len(b)
	}
	return a > b
}

// postURL builds a canonical post URL, falling back to the id-only form when the
// username could not be resolved.
func postURL(username, id string) string {
	if username == "" {
		return "https://x.com/i/web/status/" + id
	}
	return fmt.Sprintf("https://x.com/%s/status/%s", username, id)
}

// flattenText collapses internal whitespace (newlines, tabs) to single spaces so
// a multi-line post renders on one tabwriter row, then truncates on a rune
// boundary so multibyte characters are never split mid-codepoint.
func flattenText(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}

// decodeUserEnvelope extracts the id and username from a single-user response
// ({"data": {"id": ..., "username": ...}}), used for both /2/users/me and
// /2/users/{id}.
func decodeUserEnvelope(data json.RawMessage) (id, username string, err error) {
	var env struct {
		Data struct {
			ID       string `json:"id"`
			Username string `json:"username"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return "", "", err
	}
	return env.Data.ID, env.Data.Username, nil
}

// decodeTweetsPage extracts tweet items and the pagination next_token from one
// /2/users/{id}/tweets page. A page with no data (e.g. a protected or empty
// timeline) decodes to an empty slice, not an error.
func decodeTweetsPage(data json.RawMessage) (items []tweetItem, nextToken string, err error) {
	var env struct {
		Data []tweetItem `json:"data"`
		Meta struct {
			NextToken string `json:"next_token"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, "", err
	}
	return env.Data, env.Meta.NextToken, nil
}

// fetchUserPosts pages the user timeline until it has gathered maxFetch posts or
// the timeline is exhausted, reusing the CLI's existing client + error plumbing.
func fetchUserPosts(c *client.Client, flags *rootFlags, userID string, maxFetch int, exclude string) ([]tweetItem, error) {
	path := "/2/users/" + userID + "/tweets"
	collected := make([]tweetItem, 0, maxFetch)
	nextToken := ""
	for len(collected) < maxFetch {
		pageSize := maxFetch - len(collected)
		if pageSize > topPostsPageSize {
			pageSize = topPostsPageSize
		}
		if pageSize < 5 {
			pageSize = 5 // X requires max_results in [5, 100].
		}
		params := map[string]string{
			"max_results":  strconv.Itoa(pageSize),
			"tweet.fields": "public_metrics,created_at",
		}
		if exclude != "" {
			params["exclude"] = exclude
		}
		if nextToken != "" {
			params["pagination_token"] = nextToken
		}
		data, err := c.Get(context.Background(), path, params)
		if err != nil {
			return nil, classifyAPIError(err, flags)
		}
		items, token, derr := decodeTweetsPage(data)
		if derr != nil {
			return nil, fmt.Errorf("decoding posts page: %w", derr)
		}
		collected = append(collected, items...)
		if token == "" || len(items) == 0 {
			break
		}
		nextToken = token
	}
	if len(collected) > maxFetch {
		collected = collected[:maxFetch]
	}
	return collected, nil
}

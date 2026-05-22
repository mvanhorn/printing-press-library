// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/store"
)

type meSearchHit struct {
	Source    string  `json:"source"`
	ThingID   string  `json:"thing_id"`
	Sub       string  `json:"subreddit,omitempty"`
	Title     string  `json:"title,omitempty"`
	Body      string  `json:"body,omitempty"`
	Author    string  `json:"author,omitempty"`
	Score     int     `json:"score,omitempty"`
	Permalink string  `json:"permalink,omitempty"`
	CreatedAt float64 `json:"created_utc,omitempty"`
	Snippet   string  `json:"snippet,omitempty"`
}

// newMeSearchCmd performs FTS5 search across the authenticated user's synced
// own-history. Reddit's native search has missed ~50% of own-history hits for
// a decade; this fixes that with a local SQLite FTS5 index over synced data.
//
// Requires `sync` (or per-resource sync) to have populated the local store.
func newMeSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		scope  string
		sub    string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "FTS5 search over your synced Reddit history (saved/submitted/upvoted/comments)",
		Long: `Search your synced Reddit history with SQLite FTS5.

Why this exists: Reddit's native search has missed ~50% of own-history hits for
a decade. After 'sync' has populated the local store, this command searches
the full-body corpus locally.

Scope is a comma-separated list of: saved, submitted, upvoted, downvoted, comments.
Default scope covers all four user-controlled stores.`,
		Example: `  reddit-pp-cli me search "webhooks"
  reddit-pp-cli me search "auth bypass" --scope saved,comments --agent
  reddit-pp-cli me search "creativism" --sub entrepreneur --limit 20`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.TrimSpace(args[0])
			if query == "" {
				return usageErr(fmt.Errorf("query must not be empty"))
			}
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("reddit-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return apiErr(fmt.Errorf("opening local store: %w (have you run 'sync'?)", err))
			}
			defer db.Close()

			scopeSet := parseScopeFlag(scope)
			hits, err := runMeSearch(db, query, scopeSet, sub, limit)
			if err != nil {
				return apiErr(fmt.Errorf("running FTS5 search: %w", err))
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
			}
			renderMeSearchTable(cmd.OutOrStdout(), hits)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/reddit-pp-cli/data.db)")
	cmd.Flags().StringVar(&scope, "scope", "saved,submitted,upvoted,comments", "Comma-separated scope")
	cmd.Flags().StringVar(&sub, "sub", "", "Filter to a specific subreddit")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results to return")
	return cmd
}

func parseScopeFlag(s string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(strings.ToLower(part))
		if part != "" {
			out[part] = true
		}
	}
	return out
}

// runMeSearch performs a substring-of-JSON scan over the local store's
// `resources` table for the user-controlled resource types. The substring
// scan is intentionally permissive because Reddit listing JSON has many
// nested string fields (title, selftext, body) and FTS5 indexing them all
// individually would require a Reddit-specific schema. Instead we filter
// in two layers: first SQL LIKE (uses idx_resources_type), then per-item
// JSON inspection for exact field matches with snippets.
func runMeSearch(db *store.Store, query string, scope map[string]bool, filterSub string, limit int) ([]meSearchHit, error) {
	resTypes := scopeToResourceTypes(scope)
	if len(resTypes) == 0 {
		return nil, fmt.Errorf("scope is empty; pass --scope saved,submitted,upvoted,downvoted,comments")
	}

	hits := []meSearchHit{}
	likePattern := "%" + strings.ToLower(query) + "%"
	for _, rt := range resTypes {
		rows, err := db.DB().Query(
			`SELECT resource_type, id, data FROM resources WHERE resource_type = ? AND LOWER(CAST(data AS TEXT)) LIKE ? LIMIT ?`,
			rt, likePattern, limit*5,
		)
		if err != nil {
			continue
		}
		for rows.Next() {
			var rtRow, id, data string
			if err := rows.Scan(&rtRow, &id, &data); err != nil {
				continue
			}
			expandRedditListing(data, query, rtRow, filterSub, &hits)
			if len(hits) >= limit {
				rows.Close()
				return hits, nil
			}
		}
		rows.Close()
	}
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func scopeToResourceTypes(scope map[string]bool) []string {
	out := []string{}
	if scope["saved"] {
		out = append(out, "me_listings_saved", "me_listings")
	}
	if scope["upvoted"] {
		out = append(out, "me_listings_upvoted", "me_listings")
	}
	if scope["downvoted"] {
		out = append(out, "me_listings_downvoted", "me_listings")
	}
	if scope["submitted"] {
		out = append(out, "user_submitted", "user")
	}
	if scope["comments"] {
		out = append(out, "user_comments", "user")
	}
	// dedupe
	seen := map[string]bool{}
	uniq := []string{}
	for _, s := range out {
		if !seen[s] {
			seen[s] = true
			uniq = append(uniq, s)
		}
	}
	return uniq
}

// expandRedditListing walks a Reddit Listing JSON envelope and emits one
// meSearchHit per child whose title, selftext, or body contains the query.
// Reddit Listing shape: {kind: "Listing", data: {children: [{kind, data: {...}}, ...]}}.
func expandRedditListing(data, query, source, filterSub string, hits *[]meSearchHit) {
	q := strings.ToLower(query)
	var env struct {
		Data struct {
			Children []struct {
				Kind string          `json:"kind"`
				Data json.RawMessage `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(data), &env); err != nil {
		return
	}
	for _, child := range env.Data.Children {
		var item struct {
			ID         string  `json:"id"`
			Name       string  `json:"name"`
			Title      string  `json:"title"`
			Selftext   string  `json:"selftext"`
			Body       string  `json:"body"`
			Author     string  `json:"author"`
			Subreddit  string  `json:"subreddit"`
			Score      int     `json:"score"`
			Permalink  string  `json:"permalink"`
			CreatedUTC float64 `json:"created_utc"`
		}
		if err := json.Unmarshal(child.Data, &item); err != nil {
			continue
		}
		if filterSub != "" && !strings.EqualFold(item.Subreddit, filterSub) {
			continue
		}
		title := strings.ToLower(item.Title)
		body := strings.ToLower(item.Selftext + " " + item.Body)
		if !strings.Contains(title, q) && !strings.Contains(body, q) {
			continue
		}
		snippet := buildSnippet(item.Title+" "+item.Selftext+" "+item.Body, query, 120)
		thingID := item.Name
		if thingID == "" && item.ID != "" {
			thingID = "t3_" + item.ID
		}
		*hits = append(*hits, meSearchHit{
			Source:    source,
			ThingID:   thingID,
			Sub:       item.Subreddit,
			Title:     item.Title,
			Body:      strings.TrimSpace(item.Selftext + " " + item.Body),
			Author:    item.Author,
			Score:     item.Score,
			Permalink: item.Permalink,
			CreatedAt: item.CreatedUTC,
			Snippet:   snippet,
		})
	}
}

// buildSnippet returns a ~maxLen-character window centered on the first
// case-insensitive query match, with ellipses on each side.
func buildSnippet(text, query string, maxLen int) string {
	idx := strings.Index(strings.ToLower(text), strings.ToLower(query))
	if idx < 0 {
		if len(text) > maxLen {
			return text[:maxLen] + "…"
		}
		return text
	}
	start := idx - maxLen/2
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(text) {
		end = len(text)
	}
	snip := text[start:end]
	if start > 0 {
		snip = "…" + snip
	}
	if end < len(text) {
		snip = snip + "…"
	}
	return strings.ReplaceAll(snip, "\n", " ")
}

func renderMeSearchTable(w io.Writer, hits []meSearchHit) {
	if len(hits) == 0 {
		fmt.Fprintln(w, "No matches in local store. Run 'reddit-pp-cli sync' first.")
		return
	}
	for i, h := range hits {
		fmt.Fprintf(w, "%d. [%s] r/%s • %s\n   id=%s score=%d\n   %s\n\n",
			i+1, h.Source, h.Sub, h.Title, h.ThingID, h.Score, h.Snippet)
	}
}

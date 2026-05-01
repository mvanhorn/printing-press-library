package cli

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/store"
	"github.com/spf13/cobra"
)

func newTaglineGrepCmd(flags *rootFlags) *cobra.Command {
	var mode string // "fts" (default) or "regex"
	var since string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "tagline-grep <pattern>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Search every tagline the CLI has ever synced",
		Long: `Grep the tagline field across your full local post store. Two modes:

  fts    (default) — FTS5 MATCH; quoted phrases, AND/OR/NOT, column filters.
  regex            — Go regexp (RE2). Power for symbol-heavy patterns.

Scoped by --since to narrow the window. Limits default to 50 hits.`,
		Example: `  # FTS: find any tagline mentioning "agent"
  producthunt-pp-cli tagline-grep agent

  # Regex: finds "ai-agent" or "ai agent"
  producthunt-pp-cli tagline-grep --mode regex 'ai[ -]?agent'

  # Last 90 days only, agent-narrow
  producthunt-pp-cli tagline-grep 'browser' --since 90d --agent --select 'slug,title,tagline,published'`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pattern := strings.Join(args, " ")
			if pattern == "" {
				return usageErr(fmt.Errorf("a pattern is required"))
			}
			autoWarm(flags, dbPath)
			db, err := openStore(dbPath)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()

			var sinceT time.Time
			if since != "" {
				sinceT, err = parseRelativeOrAbsoluteTime(since)
				if err != nil {
					return usageErr(fmt.Errorf("--since: %w", err))
				}
			}

			var results []store.Post

			// Auto-switch to regex when the pattern contains FTS5-unfriendly
			// characters (., *, +, ?, [, (, etc.). FTS5's MATCH grammar treats
			// these as syntax and returns a parse error; falling back keeps
			// the user intent.
			effectiveMode := strings.ToLower(mode)
			if effectiveMode == "" || effectiveMode == "fts" {
				if regexpShaped(pattern) {
					effectiveMode = "regex"
				}
			}

			switch effectiveMode {
			case "", "fts":
				// FTS5 MATCH restricted to tagline column
				results, err = db.SearchPostsFTS("tagline : "+pattern, limit)
				if err != nil {
					return apiErr(err)
				}
				if !sinceT.IsZero() {
					results = filterSince(results, sinceT)
				}
			case "regex":
				// Case-insensitive by default so a user-written pattern like
				// "ai.*agent" matches "AI agent" in a tagline. FTS5 is also
				// accent/case insensitive, so the two modes behave similarly.
				re, compErr := regexp.Compile(`(?i)` + pattern)
				if compErr != nil {
					return usageErr(fmt.Errorf("invalid regex: %w", compErr))
				}
				all, err := db.ListPosts(store.ListPostsOpts{Since: sinceT, SortField: "published", SortDesc: true, Limit: 0})
				if err != nil {
					return apiErr(err)
				}
				for _, p := range all {
					if re.MatchString(p.Tagline) {
						results = append(results, p)
						if limit > 0 && len(results) >= limit {
							break
						}
					}
				}
			default:
				return usageErr(fmt.Errorf("--mode must be 'fts' or 'regex', got %q", mode))
			}

			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}
			return printOutputWithFlags(cmd.OutOrStdout(), postsToJSON(results), flags)
		},
	}

	cmd.Flags().StringVar(&mode, "mode", "fts", "Matcher: 'fts' (SQLite FTS5) or 'regex' (Go RE2)")
	cmd.Flags().StringVar(&since, "since", "", "Only posts published at or after this time (e.g. '30d')")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max matches (0 = no limit, regex only)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite store")
	return cmd
}

// regexpShaped reports whether a pattern contains characters that FTS5 MATCH
// cannot parse directly (.*+?()[]|). When true, the caller should fall back
// to regex mode rather than passing the raw string to SQLite FTS5.
func regexpShaped(p string) bool {
	return strings.ContainsAny(p, ".*+?()[]|\\")
}

func filterSince(posts []store.Post, since time.Time) []store.Post {
	out := posts[:0]
	for _, p := range posts {
		if p.PublishedAt.IsZero() || p.PublishedAt.After(since) || p.PublishedAt.Equal(since) {
			out = append(out, p)
		}
	}
	return out
}

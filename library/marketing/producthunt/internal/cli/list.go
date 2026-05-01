package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/store"
	"github.com/spf13/cobra"
)

func newListCmd(flags *rootFlags) *cobra.Command {
	var (
		limit     int
		offset    int
		author    string
		since     string
		until     string
		sortField string
		asc       bool
		dbPath    string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "List posts from the local store with filters",
		Long: `Query the local store of posts the CLI has ever synced. Filter by author
name, date range, or custom sort. Returns the stable post payload so --json
and --select work as expected.

Run 'sync' first to populate the store. An empty store returns [].`,
		Example: `  # Last 20 posts you've synced
  producthunt-pp-cli list --limit 20

  # Posts by a specific maker
  producthunt-pp-cli list --author 'Ryan Hoover' --limit 50 --json

  # Posts published in the last 7 days, oldest first
  producthunt-pp-cli list --since 7d --sort published --asc

  # Agent-friendly narrow payload, top 5 most-seen
  producthunt-pp-cli list --sort seen_count --limit 5 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			autoWarm(flags, dbPath)
			db, err := openStore(dbPath)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()

			opts := store.ListPostsOpts{
				Author:    author,
				Limit:     limit,
				Offset:    offset,
				SortField: sortField,
				SortDesc:  !asc,
			}

			if since != "" {
				t, err := parseRelativeOrAbsoluteTime(since)
				if err != nil {
					return usageErr(fmt.Errorf("--since: %w", err))
				}
				opts.Since = t
			}
			if until != "" {
				t, err := parseRelativeOrAbsoluteTime(until)
				if err != nil {
					return usageErr(fmt.Errorf("--until: %w", err))
				}
				opts.Until = t
			}

			posts, err := db.ListPosts(opts)
			if err != nil {
				return apiErr(err)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), postsToJSON(posts), flags)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Max entries to return (0 = no limit)")
	cmd.Flags().IntVar(&offset, "offset", 0, "Pagination offset")
	cmd.Flags().StringVar(&author, "author", "", "Filter by exact author name (maker or hunter)")
	cmd.Flags().StringVar(&since, "since", "", "Only posts published at or after (e.g. '7d', '24h', '2026-04-01')")
	cmd.Flags().StringVar(&until, "until", "", "Only posts published at or before")
	cmd.Flags().StringVar(&sortField, "sort", "published", "Sort column: published, updated, title, author, seen_count, first_seen")
	cmd.Flags().BoolVar(&asc, "asc", false, "Sort ascending instead of descending")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite store")
	return cmd
}

// parseRelativeOrAbsoluteTime accepts either a duration-like suffix (7d, 24h,
// 30m) or an absolute RFC3339 / date-only string. Relative times are computed
// from time.Now().
func parseRelativeOrAbsoluteTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty value")
	}
	// Duration-like: 30m, 24h, 7d, 2w, 6mo
	if d, ok := parseLooseDuration(s); ok {
		return time.Now().Add(-d), nil
	}
	// Absolute
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z07:00", "2006-01-02", "2006/01/02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time %q (try '7d', '24h', or '2026-04-01')", s)
}

// parseLooseDuration accepts '30m', '24h', '7d', '2w', '6mo', '1y' (case-insensitive).
// Returns (duration, true) when recognized, (_, false) otherwise.
func parseLooseDuration(s string) (time.Duration, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0, false
	}
	// Find the boundary between digits and unit.
	var i int
	for i = 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9') {
			break
		}
	}
	if i == 0 || i == len(s) {
		return 0, false
	}
	num := s[:i]
	unit := s[i:]

	var n int
	_, err := fmt.Sscanf(num, "%d", &n)
	if err != nil || n < 0 {
		return 0, false
	}

	switch unit {
	case "s":
		return time.Duration(n) * time.Second, true
	case "m", "min":
		return time.Duration(n) * time.Minute, true
	case "h", "hr":
		return time.Duration(n) * time.Hour, true
	case "d", "day", "days":
		return time.Duration(n) * 24 * time.Hour, true
	case "w", "wk", "week", "weeks":
		return time.Duration(n) * 7 * 24 * time.Hour, true
	case "mo", "month", "months":
		return time.Duration(n) * 30 * 24 * time.Hour, true
	case "y", "yr", "year", "years":
		return time.Duration(n) * 365 * 24 * time.Hour, true
	}
	return 0, false
}

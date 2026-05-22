// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/store"
)

// newSearchLocalCmd searches synced subreddit content via local FTS5 (same
// substring-of-JSON engine as `me search` but scoped to listings/sub data).
// Replaces the workflow that died when Pushshift shut down in 2023.
func newSearchLocalCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		sub    string
		kind   string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "search-local [query]",
		Short: "FTS5 search over synced subreddit content (sub-scoped local search)",
		Long: `Search synced subreddit content using SQLite FTS5 instead of Reddit's
broken native search. Replaces Pushshift workflows after its 2023 shutdown.

Requires 'sync' or per-resource sync to have populated the listings/submissions
tables for the target subreddit. Without sync data, returns an empty result set.`,
		Example: `  reddit-pp-cli search-local "webhook auth" --sub programming
  reddit-pp-cli search-local "release notes" --sub golang --type submissions --agent
  reddit-pp-cli search-local "rate limit"`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			q := strings.TrimSpace(args[0])
			if q == "" {
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
				return apiErr(fmt.Errorf("opening store: %w", err))
			}
			defer db.Close()

			scope := map[string]bool{"submissions": true, "comments": true}
			if kind != "" {
				scope = map[string]bool{}
				for _, k := range strings.Split(kind, ",") {
					scope[strings.TrimSpace(strings.ToLower(k))] = true
				}
			}

			resTypes := []string{}
			if scope["submissions"] {
				resTypes = append(resTypes, "listings_sub_hot", "listings_sub_new", "listings_sub_top",
					"listings_sub_rising", "listings_sub_controversial", "listings")
			}
			if scope["comments"] {
				resTypes = append(resTypes, "submissions_get", "submissions")
			}

			hits := []meSearchHit{}
			likePattern := "%" + strings.ToLower(q) + "%"
			for _, rt := range resTypes {
				rows, err := db.DB().Query(
					`SELECT id, data FROM resources WHERE resource_type = ? AND LOWER(CAST(data AS TEXT)) LIKE ? LIMIT ?`,
					rt, likePattern, limit*5,
				)
				if err != nil {
					continue
				}
				for rows.Next() {
					var id, data string
					if err := rows.Scan(&id, &data); err != nil {
						continue
					}
					expandRedditListing(data, q, rt, sub, &hits)
					if len(hits) >= limit {
						rows.Close()
						break
					}
				}
				rows.Close()
				if len(hits) >= limit {
					break
				}
			}
			if len(hits) > limit {
				hits = hits[:limit]
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
			}
			renderMeSearchTable(cmd.OutOrStdout(), hits)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&sub, "sub", "", "Filter to a specific subreddit")
	cmd.Flags().StringVar(&kind, "type", "", "Filter type: submissions,comments (default both)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results")
	return cmd
}

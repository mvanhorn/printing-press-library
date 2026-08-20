// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/cliutil"
	"github.com/spf13/cobra"
)

// pp:data-source live
// search queries Medium's own internal GraphQL endpoint (medium.com/_/graphql)
// for posts matching a free-text query — no API key, no RapidAPI, no cookies.
// It is the v2 search surface, served through the Resolver. Because this is
// Medium's unversioned internal API, a search-surface outage degrades to a
// clear, typed message (ErrSurfaceUnavailable) rather than a crash — feed/read
// keep working regardless.
func newSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search Medium for posts matching a query (no key, no cookies).",
		Long: strings.Trim(`
Search Medium for posts matching a free-text query, sourced directly from
Medium's own internal GraphQL endpoint with no API key and no cookies.

Results are paginated up to --limit (default 10). Each result is a normalized
post summary (id, title, author, url, published date).

Because search uses Medium's unversioned internal API, a search-surface outage
returns a clear "surface unavailable" message; the feed and read commands, which
do not depend on it, keep working.`, "\n"),
		Example: strings.Trim(`
  medium-reader-pp-cli search "product builder"
  medium-reader-pp-cli search "design systems" --limit 25
  medium-reader-pp-cli search ux --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if limit <= 0 {
				limit = 10
			}
			// Dogfood matrix bounds the crawl so the per-command timeout holds.
			if cliutil.IsDogfoodEnv() && limit > 10 {
				limit = 10
			}
			if dryRunOK(flags) {
				q := query
				if q == "" {
					q = "<query>"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would search Medium for %q (limit %d) via %s\n", q, limit, "medium.com/_/graphql")
				return nil
			}
			if query == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<query> is required"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			resolver := flags.newResolver()
			posts, err := resolver.Search(ctx, query, limit)
			if err != nil {
				// The graphql path is keyless: no API-key hint to emit. A
				// search-surface outage is the typed ErrSurfaceUnavailable;
				// surface it as an upstream source error with the clear message
				// (apiErr preserves the wrapped sentinel for callers).
				return apiErr(fmt.Errorf("search %q: %w", query, err))
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(posts) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "no posts found for %q\n", query)
					return nil
				}
				headers := []string{"DATE", "AUTHOR", "TITLE", "URL"}
				rows := make([][]string, 0, len(posts))
				for _, p := range posts {
					date := ""
					if !p.PublishedAt.IsZero() {
						date = p.PublishedAt.Format("2006-01-02")
					}
					rows = append(rows, []string{date, p.Author, p.Title, p.URL})
				}
				return flags.printTable(cmd, headers, rows)
			}
			return printJSONFiltered(cmd.OutOrStdout(), posts, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of results to return")
	return cmd
}

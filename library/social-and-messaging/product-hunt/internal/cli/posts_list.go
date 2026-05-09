// Copyright 2026 actionsslave. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newPostsListCmd(flags *rootFlags) *cobra.Command {
	var flagTopic string
	var flagSort string
	var flagFeatured bool
	var flagAfter string
	var flagBefore string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List posts with optional topic, sort, and date filters",
		Example: strings.Trim(`
  product-hunt-pp-cli posts list
  product-hunt-pp-cli posts list --topic developer-tools --sort RANKING --limit 20
  product-hunt-pp-cli posts list --featured --json
  product-hunt-pp-cli posts list --after 2026-01-01 --json --select name,tagline,votesCount`, "\n"),
		Annotations: map[string]string{"pp:endpoint": "posts.list", "pp:method": "GET", "pp:path": "/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			phc, err := flags.newPHClient()
			if err != nil {
				return err
			}

			order := flagSort
			if order == "" {
				order = "RANKING"
			}
			limit := flagLimit
			if limit <= 0 {
				limit = 20
			}

			conn, err := phc.GetPosts(cmd.Context(), limit, "", flagTopic, order, flagFeatured, flagAfter, flagBefore)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Extract nodes from connection edges
			posts := make([]any, len(conn.Edges))
			for i, e := range conn.Edges {
				posts[i] = e.Node
			}
			data, err := json.Marshal(posts)
			if err != nil {
				return err
			}

			prov := DataProvenance{Source: "live"}
			printProvenance(cmd, len(conn.Edges), prov)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						return err
					}
					if len(items) >= 25 {
						fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(items))
					}
					return nil
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&flagTopic, "topic", "", "Filter by topic slug (e.g. developer-tools)")
	cmd.Flags().StringVar(&flagSort, "sort", "", "Sort order: RANKING, NEWEST, VOTES, FEATURED_AT (default: RANKING)")
	cmd.Flags().BoolVar(&flagFeatured, "featured", false, "Only show featured posts")
	cmd.Flags().StringVar(&flagAfter, "after", "", "Show posts after this date (ISO 8601)")
	cmd.Flags().StringVar(&flagBefore, "before", "", "Show posts before this date (ISO 8601)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum number of posts to return (default: 20)")

	return cmd
}

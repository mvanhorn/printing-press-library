// Copyright 2026 actionsslave. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newPostsCommentsCmd(flags *rootFlags) *cobra.Command {
	var flagSort string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "comments <slug>",
		Short: "List comments for a post",
		Example: "  product-hunt-pp-cli posts comments example-slug\n  product-hunt-pp-cli posts comments my-product --sort NEWEST --json",
		Annotations: map[string]string{"pp:endpoint": "posts.comments", "pp:method": "GET", "pp:path": "/graphql/{slug}/comments", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			phc, err := flags.newPHClient()
			if err != nil {
				return err
			}

			order := flagSort
			if order == "" {
				order = "VOTES"
			}
			limit := flagLimit
			if limit <= 0 {
				limit = 20
			}

			conn, err := phc.GetPostComments(cmd.Context(), args[0], limit, "", order)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			comments := make([]any, len(conn.Edges))
			for i, e := range conn.Edges {
				comments[i] = e.Node
			}
			data, err := json.Marshal(comments)
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
	cmd.Flags().StringVar(&flagSort, "sort", "", "Sort order: NEWEST, VOTES (default: VOTES)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum comments to return (default: 20)")

	return cmd
}

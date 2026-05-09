// Copyright 2026 actionsslave. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newTopicsListCmd(flags *rootFlags) *cobra.Command {
	var flagQuery string
	var flagLimit int
	var flagSort string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List topics with optional search query",
		Example: "  product-hunt-pp-cli topics list\n  product-hunt-pp-cli topics list --query ai --limit 10 --json",
		Annotations: map[string]string{"pp:endpoint": "topics.list", "pp:method": "GET", "pp:path": "/graphql/topics", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			phc, err := flags.newPHClient()
			if err != nil {
				return err
			}

			limit := flagLimit
			if limit <= 0 {
				limit = 20
			}

			conn, err := phc.GetTopics(cmd.Context(), limit, "", flagQuery, flagSort)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			topics := make([]any, len(conn.Edges))
			for i, e := range conn.Edges {
				topics[i] = e.Node
			}
			data, err := json.Marshal(topics)
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
	cmd.Flags().StringVar(&flagQuery, "query", "", "Filter topics by name")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum topics to return (default: 20)")
	cmd.Flags().StringVar(&flagSort, "sort", "", "Sort order: NEWEST, FOLLOWERS_COUNT (default: FOLLOWERS_COUNT)")

	return cmd
}

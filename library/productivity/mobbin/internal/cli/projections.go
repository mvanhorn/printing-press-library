// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newProjectionCmd(flags *rootFlags, use, kind, short string) *cobra.Command {
	var platform string
	var limit int
	cmd := &cobra.Command{Use: use, Short: short, Annotations: map[string]string{"mcp:read-only": "true"}}
	list := &cobra.Command{
		Use:         "list",
		Short:       "List dictionary entries from the local cache (filterable by --platform).",
		Long:        short,
		Example:     "  mobbin-pp-cli " + use + " list --platform web --limit 25",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rows, err := fetchDictionary(cmd.Context(), c, kind, platform)
			if err != nil {
				return err
			}
			sortRows(rows, "slug")
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return flags.printJSON(cmd, rows)
		},
	}
	list.Flags().StringVar(&platform, "platform", "", "Filter to web, ios, or android")
	list.Flags().IntVar(&limit, "limit", 50, "Maximum rows to return")
	cmd.AddCommand(list)
	return cmd
}

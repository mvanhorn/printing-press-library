// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		clientCmd, _, err := root.Find([]string{"mcpclient"})
		if err != nil {
			return
		}
		addNovelCommandIfAbsent(clientCmd, newNovelMCPClientListCmd(flags))
	})
}

func newNovelMCPClientListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "Browse all MCP clients on MCP Market",
		Example:     "  mcpmarket-pp-cli mcpclient list --limit 25 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "mcpclient list")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			items, err := fetchItemList(ctx, c, "/client", nil)
			if err != nil {
				return apiErr(err)
			}
			persistItems(ctx, flags, "mcpclient", items)
			return printJSONFiltered(cmd.OutOrStdout(), applyLimit(items, limit), flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "maximum clients to return")
	return cmd
}

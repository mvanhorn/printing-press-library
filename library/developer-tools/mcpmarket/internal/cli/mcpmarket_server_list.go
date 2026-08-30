// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		serverCmd, _, err := root.Find([]string{"server"})
		if err != nil {
			return
		}
		addNovelCommandIfAbsent(serverCmd, newNovelServerListCmd(flags))
		addNovelCommandIfAbsent(serverCmd, newNovelServerSearchCmd(flags))
		addNovelCommandIfAbsent(serverCmd, newNovelServerCategoryCmd(flags))
		addNovelCommandIfAbsent(serverCmd, newNovelServerLeaderboardCmd(flags))
		addNovelCommandIfAbsent(serverCmd, newNovelServerDailyCmd(flags))
	})
}

func newNovelServerListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "Browse all MCP servers on MCP Market",
		Example:     "  mcpmarket-pp-cli server list --limit 25 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "server list")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			items, err := fetchItemList(ctx, c, "/server", nil)
			if err != nil {
				return apiErr(err)
			}
			persistItems(ctx, flags, "server", items)
			return printJSONFiltered(cmd.OutOrStdout(), applyLimit(items, limit), flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "maximum servers to return")
	return cmd
}

func newNovelServerSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "Search the MCP Market catalog for servers and skills",
		Example:     "  mcpmarket-pp-cli server search \"web scraping\" --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "server search")
			}
			if len(args) == 0 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("query is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			items, err := fetchSearchResults(ctx, c, args[0])
			if err != nil {
				return apiErr(err)
			}
			persistItems(ctx, flags, "server", items)
			return printJSONFiltered(cmd.OutOrStdout(), applyLimit(items, limit), flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "maximum results to return")
	return cmd
}

func newNovelServerCategoryCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "category <slug>",
		Short:       "List servers in one catalog category",
		Example:     "  mcpmarket-pp-cli server category api-development --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "server category")
			}
			if len(args) == 0 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("category slug is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			items, err := fetchItemList(ctx, c, "/categories/"+slugifyCategory(args[0]), nil)
			if err != nil {
				return apiErr(err)
			}
			persistItems(ctx, flags, "server", items)
			return printJSONFiltered(cmd.OutOrStdout(), applyLimit(items, limit), flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "maximum servers to return")
	return cmd
}

func newNovelServerLeaderboardCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "leaderboard",
		Short:       "Top 100 MCP servers of all time",
		Example:     "  mcpmarket-pp-cli server leaderboard --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "server leaderboard")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			items, err := fetchItemList(ctx, c, "/leaderboards", nil)
			if err != nil {
				return apiErr(err)
			}
			persistItems(ctx, flags, "server", items)
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}
	return cmd
}

func newNovelServerDailyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "daily",
		Short:       "Today's trending MCP servers",
		Example:     "  mcpmarket-pp-cli server daily --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "server daily")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			items, err := fetchItemList(ctx, c, "/daily", nil)
			if err != nil {
				return apiErr(err)
			}
			persistItems(ctx, flags, "server", items)
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}
	return cmd
}

// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		skillCmd, _, err := root.Find([]string{"skill"})
		if err != nil {
			return
		}
		addNovelCommandIfAbsent(skillCmd, newNovelSkillListCmd(flags))
		addNovelCommandIfAbsent(skillCmd, newNovelSkillLeaderboardCmd(flags))
		addNovelCommandIfAbsent(skillCmd, newNovelSkillDailyCmd(flags))
	})
}

func newNovelSkillListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "Browse all Agent Skills on MCP Market",
		Example:     "  mcpmarket-pp-cli skill list --limit 25 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "skill list")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			items, err := fetchItemList(ctx, c, "/tools/skills", nil)
			if err != nil {
				return apiErr(err)
			}
			persistItems(ctx, flags, "skill", items)
			return printJSONFiltered(cmd.OutOrStdout(), applyLimit(items, limit), flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "maximum skills to return")
	return cmd
}

func newNovelSkillLeaderboardCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "leaderboard",
		Short:       "Top 100 Agent Skills of all time",
		Example:     "  mcpmarket-pp-cli skill leaderboard --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "skill leaderboard")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			items, err := fetchItemList(ctx, c, "/tools/skills/leaderboard", nil)
			if err != nil {
				return apiErr(err)
			}
			persistItems(ctx, flags, "skill", items)
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}
	return cmd
}

func newNovelSkillDailyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "daily",
		Short:       "Today's trending Agent Skills",
		Example:     "  mcpmarket-pp-cli skill daily --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "skill daily")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			items, err := fetchItemList(ctx, c, "/daily/skills", nil)
			if err != nil {
				return apiErr(err)
			}
			persistItems(ctx, flags, "skill", items)
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}
	return cmd
}

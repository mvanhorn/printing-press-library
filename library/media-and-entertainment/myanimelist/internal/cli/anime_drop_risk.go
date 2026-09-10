// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newNovelAnimeDropRiskCmd predicts abandonment before someone commits dozens of
// episodes. It joins the /stats status distribution with the episode count,
// neither of which appears next to the average score on the detail page.
func newNovelAnimeDropRiskCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drop-risk <id>",
		Short: "Estimate how likely MyAnimeList users are to abandon this title",
		Long: "Use this command for how likely MyAnimeList users are to abandon a title (dropped/on-hold share from the status distribution).\n" +
			"Do NOT use this command for how divisive its scores are; use 'anime divisive' instead.\n" +
			"Do NOT use this command for raw status counts; use 'anime stats' instead.",
		Example: "  myanimelist-pp-cli anime drop-risk 21 --json",
		Annotations: map[string]string{
			"mcp:read-only":     "true",
			"pp:data-source":    "live",
			"pp:happy-args":     "id=21",
			"pp:novel-scaffold": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "anime drop-risk")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an anime id is required, e.g. myanimelist-pp-cli anime drop-risk 21"))
			}
			id, err := malIntArg(args[0], "id")
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, cerr := flags.newClient()
			if cerr != nil {
				return cerr
			}
			stats, err := malStatsWith(ctx, c, "anime", id)
			if err != nil {
				return err
			}
			episodes := 0
			if detail, derr := malDetailWith(ctx, c, "anime", id); derr == nil {
				episodes = detail.Episodes
			}
			view, err := stats.DropRisk(episodes)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return printAutoTable(cmd.OutOrStdout(), []map[string]any{{
				"title":             view.Title,
				"risk":              view.RiskBand,
				"dropped_percent":   view.DroppedShare,
				"on_hold_percent":   view.OnHoldShare,
				"completed_percent": view.CompletionShare,
				"episodes":          view.Episodes,
			}})
		},
	}
	return cmd
}

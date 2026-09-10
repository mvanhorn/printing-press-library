// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newNovelAnimeConsistencyCmd answers "did this show hold up, or fall apart?".
// It reads the per-episode poll averages and reply counts from the episode
// table, which no MyAnimeList page aggregates and no other tool surfaces.
func newNovelAnimeConsistencyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "consistency <id>",
		Short: "Chart episode-by-episode reception across a season",
		Long: "Use this command for episode-by-episode reception (poll averages, reply counts, late-season drop-off).\n" +
			"Do NOT use this command for the raw episode list; use 'anime episodes' instead.\n" +
			"Do NOT use this command for the overall score distribution; use 'anime divisive' instead.",
		Example: "  myanimelist-pp-cli anime consistency 52991 --json",
		Annotations: map[string]string{
			"mcp:read-only":     "true",
			"pp:data-source":    "live",
			"pp:happy-args":     "id=52991",
			"pp:novel-scaffold": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "anime consistency")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an anime id is required, e.g. myanimelist-pp-cli anime consistency 52991"))
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
			eps, err := malEpisodesWith(ctx, c, id)
			if err != nil {
				return err
			}
			view, err := malhtmlConsistency(id, eps)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return printAutoTable(cmd.OutOrStdout(), []map[string]any{{
				"episodes":  view.Episodes,
				"rated":     view.RatedEpisodes,
				"mean_poll": view.MeanPoll,
				"first_3":   view.First3Mean,
				"last_3":    view.Last3Mean,
				"verdict":   view.Verdict,
				"worst_ep":  view.WorstEpisode,
			}})
		},
	}
	return cmd
}

// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newNovelAnimeDivisiveCmd answers "is this show loved, or just argued about?".
// MyAnimeList renders the 1-10 vote distribution as a bar chart on /stats and
// exposes it nowhere programmatically, so the polarization reading only exists
// because this CLI parses that page.
func newNovelAnimeDivisiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "divisive <id>",
		Short: "Measure whether a title is universally loved or bitterly split",
		Long: "Use this command for the shape of a title's 1-10 score distribution (polarization, love-it/hate-it share).\n" +
			"Do NOT use this command for whether a title's score is rising or falling; use 'drift' instead.\n" +
			"Do NOT use this command for the raw vote table; use 'anime stats' instead.",
		Example: "  myanimelist-pp-cli anime divisive 5114 --json",
		Annotations: map[string]string{
			"mcp:read-only":     "true",
			"pp:data-source":    "live",
			"pp:happy-args":     "id=5114",
			"pp:novel-scaffold": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "anime divisive")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an anime id is required, e.g. myanimelist-pp-cli anime divisive 5114"))
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
			view, err := stats.Divisiveness()
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return printAutoTable(cmd.OutOrStdout(), []map[string]any{{
				"title":      view.Title,
				"score":      view.Mean,
				"verdict":    view.Verdict,
				"index":      view.Index,
				"love_share": view.LoveShare,
				"hate_share": view.HateShare,
				"votes":      view.Votes,
			}})
		},
	}
	return cmd
}

// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/yahoofantasy"
	"github.com/spf13/cobra"
)

func newNovelDraftRoiCmd(flags *rootFlags) *cobra.Command {
	var flagLeague string
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "draft-roi",
		Short: "Evaluate snake draft rounds or auction dollar costs against season-to-date fantasy points and current roster retention.",
		Long: `Join league draft results with season-to-date player points and current team rosters
to compute surplus points over round expectations, points per auction dollar, drafter
retention, and steal/value/par/bust verdicts.`,
		Example: strings.Trim(`
  # Evaluate draft steals and busts for a league
  yahoo-fantasy-pp-cli draft-roi --league 449.l.365083 --agent

  # Evaluate draft ROI from local SQLite mirror
  yahoo-fantasy-pp-cli draft-roi --league 449.l.365083 --data-source local --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "auto",
			"pp:happy-args":       "--league=449.l.365083",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "draft-roi")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if err := validateDataSourceStrategy(flags, "auto"); err != nil {
				return usageErr(err)
			}

			leagueKey := strings.TrimSpace(flagLeague)
			if leagueKey == "" && len(args) > 0 {
				leagueKey = strings.TrimSpace(args[0])
			}
			if leagueKey == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--league is required (e.g. --league 449.l.365083)"))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("yahoo-fantasy-pp-cli")
			}

			var picks []yahoofantasy.DraftPickRecord
			playersByKey := map[string]yahoofantasy.PlayerRecord{}
			ownerByPlayer := map[string]string{}
			hasDB := false

			if flags.dataSource != "live" {
				blobs, dbExists, err := loadLocalResourceBlobs(ctx, cmd, flags, dbPath, "leagues", "players", "teams", "draft")
				if err != nil {
					return err
				}
				hasDB = dbExists
				for _, b := range blobs {
					picks = append(picks, yahoofantasy.ExtractDraftPicks(b)...)
				}
				for _, p := range extractAllPlayersFromBlobs(blobs) {
					playersByKey[p.PlayerKey] = p
					if p.OwnerTeamKey != "" {
						ownerByPlayer[p.PlayerKey] = p.OwnerTeamKey
					}
				}
			}

			if len(picks) == 0 && flags.dataSource != "local" && (hasLiveAuthConfigured(flags) || cliutil.IsVerifyEnv()) {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				drPath := fmt.Sprintf("/league/%s/draftresults", leagueKey)
				if rawDR, getErr := c.Get(ctx, drPath, map[string]string{"format": "json"}); getErr == nil {
					picks = yahoofantasy.ExtractDraftPicks(rawDR)
				}
			}

			if !hasDB && len(picks) == 0 {
				if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: yahoo-fantasy-pp-cli sync --resources users --db %s\n", dbPath, dbPath)
				}
			}

			rows := yahoofantasy.ComputeDraftROI(picks, playersByKey, ownerByPlayer)
			if rows == nil {
				rows = make([]yahoofantasy.DraftROIRow, 0)
			}
			if flagLimit > 0 && len(rows) > flagLimit {
				rows = rows[:flagLimit]
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No draft picks found for league.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintf(tw, "RANK\tROUND (PICK)\tPLAYER\tDRAFTED BY\tPTS\tROUND EXP\tSURPLUS\tRETAINED\tVERDICT\n")
			for _, r := range rows {
				fmt.Fprintf(tw, "%d\tR%d (#%d)\t%s (%s)\t%s\t%.1f\t%.1f\t%+.1f\t%v\t%s\n",
					r.Rank, r.Round, r.Pick, r.PlayerName, r.PlayerKey, r.DraftedByTeamKey,
					r.SeasonPoints, r.RoundExpectedPoints, r.SurplusPoints, r.RetainedByDrafter, r.Verdict)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&flagLeague, "league", "", "Yahoo league key in {game_id}.l.{league_id} format (e.g. 449.l.365083)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Optional maximum number of ranked draft picks to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database mirror")
	return cmd
}

// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/yahoofantasy"
	"github.com/spf13/cobra"
)

func newNovelAddDropCmd(flags *rootFlags) *cobra.Command {
	var flagLeague string
	var flagTeam string
	var flagAdd string
	var flagDrop string
	var flagFaab int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "add-drop",
		Short: "Execute or dry-run free-agent adds, drops, atomic add/drops, and FAAB waiver claims.",
		Long: `Validate player availability and remaining FAAB budget, construct Yahoo's required
<fantasy_content><transaction> XML payload for single add, single drop, or atomic add/drop
with optional --faab bid, and preview (--dry-run) or submit POST /league/{league_key}/transactions.`,
		Example: strings.Trim(`
  # Preview an atomic add/drop with a $15 FAAB waiver bid
  yahoo-fantasy-pp-cli add-drop --league 449.l.365083 --team 449.l.365083.t.1 --add 449.p.31862 --drop 449.p.30123 --faab 15 --dry-run --agent

  # Execute an immediate free-agent add/drop
  yahoo-fantasy-pp-cli add-drop --league 449.l.365083 --team 449.l.365083.t.1 --add 449.p.31862 --drop 449.p.30123 --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "false",
			"pp:data-source":      "auto",
			"pp:happy-args":       "--league=449.l.365083;--team=449.l.365083.t.1;--add=449.p.31862;--drop=449.p.30123;--faab=15;--data-source=local",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			hasFaab := cmd.Flags().Changed("faab")
			previewPlan, err := yahoofantasy.PlanAddDrop(flagLeague, flagTeam, flagAdd, flagDrop, flagFaab, hasFaab, -1)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.ErrOrStderr(), "POST /league/%s/transactions\n%s\n", previewPlan.LeagueKey, previewPlan.XMLPayload)
				return writeDryRun(cmd.OutOrStdout(), flags, "add-drop")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if err := validateDataSourceStrategy(flags, "auto"); err != nil {
				return usageErr(err)
			}

			currentFAAB := -1
			var addName, dropName string
			if flags.dataSource != "live" {
				blobs, _, _ := loadLocalResourceBlobs(ctx, cmd, flags, dbPath, "teams", "players", "leagues", "users")
				for _, t := range extractAllTeamsFromBlobs(blobs) {
					if t.TeamKey == strings.TrimSpace(flagTeam) && (t.HasFaab || t.FaabBalance > 0) {
						currentFAAB = t.FaabBalance
					}
				}
				for _, p := range extractAllPlayersFromBlobs(blobs) {
					if p.PlayerKey == strings.TrimSpace(flagAdd) {
						addName = p.FullName
					}
					if p.PlayerKey == strings.TrimSpace(flagDrop) {
						dropName = p.FullName
					}
				}
			}

			plan, err := yahoofantasy.PlanAddDrop(flagLeague, flagTeam, flagAdd, flagDrop, flagFaab, hasFaab, currentFAAB)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			plan.AddPlayerName = addName
			plan.DropPlayerName = dropName

			if flags.dataSource != "local" {
				if cliutil.IsAnyHarness() && !cliutil.IsVerifyEnv() {
					return writeHarnessRefusal(cmd.OutOrStdout(), flags, "submit add/drop transaction")
				}
				if hasLiveAuthConfigured(flags) || cliutil.IsVerifyEnv() {
					c, clientErr := flags.newClient()
					if clientErr != nil {
						return clientErr
					}
					reqBody := map[string]any{
						"type":            plan.TransactionType,
						"team_key":        plan.TeamKey,
						"add_player_key":  plan.AddPlayerKey,
						"drop_player_key": plan.DropPlayerKey,
					}
					if hasFaab || plan.FaabBid > 0 {
						reqBody["faab_bid"] = plan.FaabBid
					}
					respData, status, postErr := c.PostWithParams(ctx, fmt.Sprintf("/league/%s/transactions", plan.LeagueKey), nil, reqBody)
					if postErr != nil {
						return classifyAPIError(cmd.OutOrStdout(), postErr, flags)
					}
					plan.Applied = status >= 200 && status < 300
					plan.UpstreamResponse = string(respData)
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintf(tw, "LEAGUE\tTEAM\tTYPE\tADD\tDROP\tFAAB BID\tAPPLIED\n")
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t$%d\t%v\n",
				plan.LeagueKey, plan.TeamKey, plan.TransactionType, plan.AddPlayerKey, plan.DropPlayerKey, plan.FaabBid, plan.Applied)
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&flagLeague, "league", "", "Target Yahoo league key in {game_id}.l.{league_id} format (e.g. 449.l.365083)")
	cmd.Flags().StringVar(&flagTeam, "team", "", "Destination/source Yahoo team key in {game_id}.l.{league_id}.t.{team_id} format (e.g. 449.l.365083.t.1)")
	cmd.Flags().StringVar(&flagAdd, "add", "", "Yahoo player key to add or claim off waivers (e.g. 449.p.31862)")
	cmd.Flags().StringVar(&flagDrop, "drop", "", "Yahoo player key to drop from your active roster (e.g. 449.p.30123)")
	cmd.Flags().IntVar(&flagFaab, "faab", 0, "Optional FAAB dollar bid amount for waiver claims")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database mirror")
	return cmd
}

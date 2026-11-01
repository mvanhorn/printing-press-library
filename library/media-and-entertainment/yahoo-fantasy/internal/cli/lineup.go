// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/yahoofantasy"
	"github.com/spf13/cobra"
)

func newNovelLineupCmd(flags *rootFlags) *cobra.Command {
	var flagTeam string
	var flagWeek int
	var flagDate string
	var flagSet string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "lineup",
		Short: "Audit active starting slots for OUT, IR, or BYE players, preview bench swaps, and submit Yahoo XML roster updates.",
		Long: `Audit a Yahoo Fantasy team's starting lineup for injured (OUT, IR, Doubtful) or bye-week
players, recommend healthy eligible bench replacements, and optionally apply slot assignments
(--set QB=449.p.31862,BN=449.p.30123) via PUT /team/{team_key}/roster using Yahoo's
required <fantasy_content><roster> XML format.`,
		Example: strings.Trim(`
  # Audit starting lineup for Week 9 and preview a QB swap without mutating
  yahoo-fantasy-pp-cli lineup --team 449.l.365083.t.1 --week 9 --set QB=449.p.31862,BN=449.p.30123 --dry-run --agent

  # Apply a starting lineup change for Week 9
  yahoo-fantasy-pp-cli lineup --team 449.l.365083.t.1 --week 9 --set QB=449.p.31862,BN=449.p.30123 --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "false",
			"pp:data-source":      "auto",
			"pp:happy-args":       "--team=449.l.365083.t.1;--week=9;--set=QB=449.p.31862,BN=449.p.30123;--data-source=local",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				if swaps, err := yahoofantasy.ParseSlotSwaps(flagSet); err == nil && len(swaps) > 0 {
					wk := flagWeek
					if wk <= 0 {
						wk = 1
					}
					if xmlBytes, xmlErr := yahoofantasy.BuildRosterUpdateXML(yahoofantasy.RosterUpdateRequest{
						CoverageType: "week",
						Week:         wk,
						Date:         flagDate,
						Players:      swaps,
					}); xmlErr == nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "PUT /team/%s/roster\n%s\n", flagTeam, string(xmlBytes))
					}
				}
				return writeDryRun(cmd.OutOrStdout(), flags, "lineup")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			teamKey := strings.TrimSpace(flagTeam)
			if teamKey == "" && len(args) > 0 {
				teamKey = strings.TrimSpace(args[0])
			}
			if teamKey == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--team is required (e.g. --team 449.l.365083.t.1)"))
			}

			swaps, err := yahoofantasy.ParseSlotSwaps(flagSet)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			week := flagWeek
			if week <= 0 {
				week = 1
			}

			if err := validateDataSourceStrategy(flags, "auto"); err != nil {
				return usageErr(err)
			}

			var roster []yahoofantasy.PlayerRecord
			if flags.dataSource != "live" {
				blobs, _, _ := loadLocalResourceBlobs(ctx, cmd, flags, dbPath, "teams", "players", "users")
				for _, p := range extractAllPlayersFromBlobs(blobs) {
					if p.OwnerTeamKey == "" || p.OwnerTeamKey == teamKey {
						roster = append(roster, p)
					}
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if len(roster) == 0 && flags.dataSource != "local" && (hasLiveAuthConfigured(flags) || cliutil.IsVerifyEnv()) {
				rosterPath := fmt.Sprintf("/team/%s/roster;week=%s", teamKey, strconv.Itoa(week))
				if rawRoster, getErr := c.Get(ctx, rosterPath, map[string]string{"format": "json"}); getErr == nil {
					roster = yahoofantasy.ExtractPlayers(rawRoster)
				}
			}

			plan, err := yahoofantasy.AnalyzeLineup(teamKey, week, flagDate, roster, swaps)
			if err != nil {
				return usageErr(err)
			}

			if len(plan.PlannedSwaps) > 0 && flags.dataSource != "local" {
				if cliutil.IsAnyHarness() && !cliutil.IsVerifyEnv() {
					return writeHarnessRefusal(cmd.OutOrStdout(), flags, "submit lineup update")
				}
				if hasLiveAuthConfigured(flags) || cliutil.IsVerifyEnv() {
					reqBody := map[string]any{
						"coverage_type": plan.CoverageType,
						"week":          plan.Week,
						"date":          plan.Date,
						"players":       plan.PlannedSwaps,
					}
					respData, status, putErr := c.PutWithParams(ctx, fmt.Sprintf("/team/%s/roster", teamKey), nil, reqBody)
					if putErr != nil {
						return classifyAPIError(cmd.OutOrStdout(), putErr, flags)
					}
					plan.Applied = status >= 200 && status < 300
					plan.UpstreamResponse = string(respData)
				}
			}

			if flags.dataSource == "local" && len(roster) == 0 && len(plan.PlannedSwaps) == 0 {
				if dbPath == "" {
					dbPath = defaultDBPath("yahoo-fantasy-pp-cli")
				}
				if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: yahoo-fantasy-pp-cli sync --resources users --db %s\n", dbPath, dbPath)
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintf(tw, "TEAM\tWEEK\tACTIVE STARTERS\tBENCH\tALERTS\tSWAPS\tAPPLIED\n")
			fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d\t%v\n",
				plan.TeamKey, plan.Week, plan.ActiveStarters, plan.BenchCount, len(plan.Alerts), len(plan.PlannedSwaps), plan.Applied)
			for _, a := range plan.Alerts {
				fmt.Fprintf(tw, "ALERT [%s]\t%s (%s)\t%s\tSuggested: %s (%s)\n",
					a.Slot, a.StarterName, a.StarterStatus, a.Reason, a.RecommendedBenchName, a.RecommendedBenchKey)
			}
			for _, s := range plan.PlannedSwaps {
				fmt.Fprintf(tw, "SWAP\t%s\t->\t%s\n", s.PlayerKey, s.Position)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&flagTeam, "team", "", "Target Yahoo fantasy team key in {game_id}.l.{league_id}.t.{team_id} format (e.g. 449.l.365083.t.1)")
	cmd.Flags().IntVar(&flagWeek, "week", 0, "Target NFL week number for weekly lineup changes (e.g. 9)")
	cmd.Flags().StringVar(&flagDate, "date", "", "Target date in YYYY-MM-DD format for daily sports (NBA/MLB/NHL)")
	cmd.Flags().StringVar(&flagSet, "set", "", "Comma-separated slot assignments in POSITION=PLAYER_KEY format (e.g. QB=449.p.31862,BN=449.p.30123)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database mirror")
	return cmd
}

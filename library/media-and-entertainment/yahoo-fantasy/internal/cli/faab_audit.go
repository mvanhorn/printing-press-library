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

func newNovelFaabAuditCmd(flags *rootFlags) *cobra.Command {
	var flagLeague string
	var flagDefaultBudget int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "faab-audit",
		Short: "Track every manager's remaining FAAB budget, positional spending breakdown, winning bids, and blocking bid thresholds.",
		Long: `Join league team FAAB balances with historical transaction bid logs to show remaining
budget share, total spent, positional spend breakdown, largest bid, and minimum blocking
bid thresholds over the next rival manager.`,
		Example: strings.Trim(`
  # Audit league FAAB budgets and blocking bid thresholds
  yahoo-fantasy-pp-cli faab-audit --league 449.l.365083 --agent

  # Audit FAAB budgets from local SQLite mirror
  yahoo-fantasy-pp-cli faab-audit --league 449.l.365083 --data-source local --json
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
				return writeDryRun(cmd.OutOrStdout(), flags, "faab-audit")
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

			var teams []yahoofantasy.TeamRecord
			var transactions []yahoofantasy.TransactionRecord
			hasDB := false

			if flags.dataSource != "live" {
				blobs, dbExists, err := loadLocalResourceBlobs(ctx, cmd, flags, dbPath, "teams", "transactions", "leagues", "users")
				if err != nil {
					return err
				}
				hasDB = dbExists
				for _, t := range extractAllTeamsFromBlobs(blobs) {
					if strings.HasPrefix(t.TeamKey, leagueKey+".t.") || leagueKey == "" {
						teams = append(teams, t)
					}
				}
				for _, b := range blobs {
					transactions = append(transactions, yahoofantasy.ExtractTransactions(b)...)
				}
			}

			if len(teams) == 0 && flags.dataSource != "local" && (hasLiveAuthConfigured(flags) || cliutil.IsVerifyEnv()) {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				tmPath := fmt.Sprintf("/league/%s/teams", leagueKey)
				if rawTm, getErr := c.Get(ctx, tmPath, map[string]string{"format": "json"}); getErr == nil {
					teams = yahoofantasy.ExtractTeams(rawTm)
				}
				txPath := fmt.Sprintf("/league/%s/transactions", leagueKey)
				if rawTx, txErr := c.Get(ctx, txPath, map[string]string{"format": "json"}); txErr == nil {
					transactions = yahoofantasy.ExtractTransactions(rawTx)
				}
			}

			if !hasDB && len(teams) == 0 && len(transactions) == 0 {
				if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: yahoo-fantasy-pp-cli sync --resources users --db %s\n", dbPath, dbPath)
				}
			}

			rows := yahoofantasy.ComputeFAABAudit(flagDefaultBudget, teams, transactions)
			if rows == nil {
				rows = make([]yahoofantasy.FAABAuditRow, 0)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No FAAB budget or transaction records found.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintf(tw, "RANK\tTEAM\tREMAINING\tSPENT\tSHARE%%\tPAID BIDS\tAVG BID\tMAX BID\tBLOCKING BID\n")
			for _, r := range rows {
				fmt.Fprintf(tw, "%d\t%s (%s)\t$%d\t$%d\t%.1f%%\t%d\t$%.2f\t$%d\t$%d\n",
					r.Rank, r.TeamName, r.TeamKey, r.RemainingFAAB, r.TotalSpentFAAB,
					r.BudgetSharePct, r.PaidAcquisitions, r.AvgWinningBid, r.LargestBid, r.BlockingBidOverNext)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&flagLeague, "league", "", "Yahoo league key in {game_id}.l.{league_id} format (e.g. 449.l.365083)")
	cmd.Flags().IntVar(&flagDefaultBudget, "default-budget", 100, "Starting season FAAB budget when team balance is unspecified")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database mirror")
	return cmd
}

// Copyright 2026 Charles Garrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature #4 — cost ledger / budget report. Hand-authored.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

func newBudgetCmd(flags *rootFlags) *cobra.Command {
	var since string
	var groupBy string

	cmd := &cobra.Command{
		Use:   "budget",
		Short: "Roll up Semrush API unit balance snapshots into a spend ledger by day or command.",
		Long: `budget is the cost-tracking surface for the local credit_log table.
Every novel command (and 'budget' itself) snapshots the free Semrush
units-remaining balance at the start of RunE; this command rolls those
snapshots into deltas grouped by day or by triggering command, and
projects month-end burn.

Free probe — invoking 'budget' costs zero API units.`,
		Example:     "  semrush-pp-cli budget --since 30d --group-by command",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openNovelStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			// Self-record: budget snapshots the balance even when called on
			// its own, so a regular `budget` invocation always sees fresh
			// data.
			recordBalanceSnapshotForCmd(ctx, db, flags, cmd.CommandPath(), cmd.ErrOrStderr())

			window, err := parseSince(since)
			if err != nil {
				return usageErr(err)
			}
			cutoff := time.Now().Add(-window).Unix()

			rows, err := db.DB().QueryContext(ctx,
				`SELECT ts, command, units_remaining, balance_source
				 FROM credit_log
				 WHERE ts >= ?
				 ORDER BY ts ASC`, cutoff)
			if err != nil {
				return fmt.Errorf("query credit_log: %w", err)
			}
			defer rows.Close()

			type ledgerRow struct {
				TS             int64  `json:"ts"`
				TSISO          string `json:"ts_iso"`
				Command        string `json:"command"`
				UnitsRemaining int64  `json:"units_remaining"`
				DeltaUnits     int64  `json:"delta_units"`
				BalanceSource  string `json:"balance_source"`
			}
			var ledger []ledgerRow
			var prev int64
			havePrev := false
			for rows.Next() {
				var r ledgerRow
				if err := rows.Scan(&r.TS, &r.Command, &r.UnitsRemaining, &r.BalanceSource); err != nil {
					return fmt.Errorf("scan credit_log: %w", err)
				}
				r.TSISO = time.Unix(r.TS, 0).UTC().Format(time.RFC3339)
				if havePrev {
					r.DeltaUnits = prev - r.UnitsRemaining
				}
				prev = r.UnitsRemaining
				havePrev = true
				ledger = append(ledger, r)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate credit_log: %w", err)
			}

			type group struct {
				Key        string `json:"key"`
				Calls      int    `json:"calls"`
				UnitsSpent int64  `json:"units_spent"`
				FirstTS    int64  `json:"first_ts"`
				LastTS     int64  `json:"last_ts"`
				FirstISO   string `json:"first_iso"`
				LastISO    string `json:"last_iso"`
			}
			groups := map[string]*group{}
			var totalSpent int64
			for _, r := range ledger {
				var key string
				switch groupBy {
				case "day":
					key = time.Unix(r.TS, 0).UTC().Format("2006-01-02")
				default:
					key = r.Command
				}
				g, ok := groups[key]
				if !ok {
					g = &group{Key: key, FirstTS: r.TS, FirstISO: r.TSISO}
					groups[key] = g
				}
				g.Calls++
				if r.DeltaUnits > 0 {
					g.UnitsSpent += r.DeltaUnits
					totalSpent += r.DeltaUnits
				}
				g.LastTS = r.TS
				g.LastISO = r.TSISO
			}
			var groupList []*group
			for _, g := range groups {
				groupList = append(groupList, g)
			}
			// Deterministic order: by most-recent activity desc, tiebreak by key.
			// Go map iteration is non-deterministic, so without this sort,
			// scripted diffs of two budget reports flap between runs.
			sort.Slice(groupList, func(i, j int) bool {
				if groupList[i].LastTS != groupList[j].LastTS {
					return groupList[i].LastTS > groupList[j].LastTS
				}
				return groupList[i].Key < groupList[j].Key
			})

			// Month-end burn projection
			var balanceNow int64
			var monthEndProjection int64
			if len(ledger) > 0 {
				balanceNow = ledger[len(ledger)-1].UnitsRemaining
				windowDur := window
				if windowDur <= 0 {
					windowDur = 24 * time.Hour
				}
				daysInWindow := windowDur.Hours() / 24
				if daysInWindow > 0 && totalSpent > 0 {
					perDay := float64(totalSpent) / daysInWindow
					now := time.Now()
					monthEnd := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
					daysToMonthEnd := monthEnd.Sub(now).Hours() / 24
					monthEndProjection = balanceNow - int64(perDay*daysToMonthEnd)
				}
			}

			out := map[string]any{
				"since":                since,
				"group_by":             groupBy,
				"cutoff_ts":            cutoff,
				"balance_now":          balanceNow,
				"total_units_spent":    totalSpent,
				"month_end_projection": monthEndProjection,
				"groups":               groupList,
				"ledger":               ledger,
			}
			raw, err := json.Marshal(out)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "Aggregation window (e.g. 7d, 30d, 12w)")
	cmd.Flags().StringVar(&groupBy, "group-by", "command", "Roll up by 'day' or 'command'")
	return cmd
}

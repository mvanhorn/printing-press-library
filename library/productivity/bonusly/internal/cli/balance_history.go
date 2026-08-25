// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source computed

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/bonusly/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/bonusly/internal/types"
	"github.com/spf13/cobra"
)

type balanceHistorySnapshot struct {
	RecordedAt     string `json:"recorded_at"`
	GivingBalance  int64  `json:"giving_balance"`
	EarningBalance int64  `json:"earning_balance"`
	MonthlyBudget  int64  `json:"monthly_budget"`
}

func newNovelBalanceHistoryCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "history",
		Short:       "Track your giving-allowance burn rate over time and see forfeiture coming before the monthly reset, not after.",
		Example:     "  bonusly-pp-cli balance history --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				// pp:hand-edit bonusly-endpoint-fix — was an unconditional
				// Fprintln, so --dry-run --json produced plain text.
				return writeDryRun(cmd.OutOrStdout(), flags, "track balance history")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// pp:hand-edit bonusly-endpoint-fix — /users/points_balance 404s
			// live; balance fields live directly on /users/me instead
			// (giving_balance, earning_balance, lifetime_earnings). Bonusly
			// does not expose monthly_budget/currency/exchange_rate/
			// lifetime_given/lifetime_redeemed to non-admin accounts via any
			// endpoint found so far, so types.Balance.MonthlyBudget etc. stay
			// zero-valued here rather than guessing a source for them.
			// Routed through resolveReadWithStrategyAndResponsePath (the
			// same helper users_me.go uses) rather than a raw c.Get() — see
			// resolveMyUser's doc comment in helpers.go.
			balRaw, _, err := resolveReadWithStrategyAndResponsePath(cmd.Context(), c, flags, "live", "users", false, "/users/me", nil, nil, "", cmd.ErrOrStderr())
			if err != nil {
				// pp:hand-edit bonusly-dogfood-exit-code — see the matching
				// comment in recognition_gap.go.
				return classifyAPIError(err, flags)
			}
			var balEnvelope struct {
				Result types.Balance `json:"result"`
			}
			if err := json.Unmarshal(balRaw, &balEnvelope); err != nil {
				return err
			}
			bal := balEnvelope.Result

			// open store and lazy init table
			dbPath := defaultDBPath("bonusly-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			if err := store.EnsureBonuslyBalanceHistory(cmd.Context(), db.DB()); err != nil {
				return err
			}

			// Insert current snapshot
			_, err = db.DB().ExecContext(cmd.Context(), `
				INSERT INTO balance_history (recorded_at, giving_balance, earning_balance, monthly_budget)
				VALUES (CURRENT_TIMESTAMP, ?, ?, ?)`,
				bal.GivingBalance, bal.EarningBalance, bal.MonthlyBudget)
			if err != nil {
				return err
			}

			// SELECT all ordered by recorded_at
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT recorded_at, giving_balance, earning_balance, monthly_budget
				FROM balance_history
				ORDER BY recorded_at ASC`)
			if err != nil {
				return err
			}
			defer rows.Close()

			var snapshots []balanceHistorySnapshot
			for rows.Next() {
				var s balanceHistorySnapshot
				if err := rows.Scan(&s.RecordedAt, &s.GivingBalance, &s.EarningBalance, &s.MonthlyBudget); err != nil {
					return err
				}
				snapshots = append(snapshots, s)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			var burnRatePerDay *float64
			var note string

			if len(snapshots) < 2 {
				note = "first snapshot recorded; run this again on a later day to see a trend"
			} else {
				first := snapshots[0]
				last := snapshots[len(snapshots)-1]

				// recorded_at is UTC in CURRENT_TIMESTAMP format "YYYY-MM-DD HH:MM:SS"
				firstTime, err1 := time.Parse("2006-01-02 15:04:05", first.RecordedAt)
				lastTime, err2 := time.Parse("2006-01-02 15:04:05", last.RecordedAt)
				if err1 == nil && err2 == nil {
					duration := lastTime.Sub(firstTime)
					days := duration.Hours() / 24.0
					if days > 0.0001 {
						diff := float64(first.GivingBalance - last.GivingBalance)
						rate := diff / days
						burnRatePerDay = &rate
					}
				}
			}

			if flags.asJSON || flags.agent {
				res := map[string]any{
					"snapshots": snapshots,
				}
				if note != "" {
					res["note"] = note
				}
				if burnRatePerDay != nil {
					res["burn_rate_per_day"] = *burnRatePerDay
				} else {
					res["burn_rate_per_day"] = nil
				}
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "RECORDED AT\tGIVING\tEARNING\tMONTHLY BUDGET\n")
			for _, s := range snapshots {
				fmt.Fprintf(tw, "%s\t%d\t%d\t%d\n", s.RecordedAt, s.GivingBalance, s.EarningBalance, s.MonthlyBudget)
			}
			fmt.Fprintln(tw)
			if note != "" {
				fmt.Fprintf(tw, "NOTE\t%s\n", note)
			}
			if burnRatePerDay != nil {
				fmt.Fprintf(tw, "BURN RATE PER DAY\t%.2f\n", *burnRatePerDay)
			}
			_ = tw.Flush()
			return nil
		},
	}
	return cmd
}

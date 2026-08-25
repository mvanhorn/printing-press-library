// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/parallel/internal/store"
	"github.com/mvanhorn/printing-press-library/library/ai/parallel/internal/types"
	"github.com/spf13/cobra"
)

func newNovelTasksGuardCmd(flags *rootFlags) *cobra.Command {
	var flagMinBalance int

	cmd := &cobra.Command{
		Use:   "guard",
		Short: "Refuse Task creates when prepaid balance is below a threshold.",
		Example: strings.Trim(`
  parallel-pp-cli tasks guard --min-balance 500 --dry-run --json --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{
					"ok":          true,
					"dry_run":     true,
					"min_balance": flagMinBalance,
				})
			}
			if err := validateDataSourceStrategy(flags, "auto"); err != nil {
				return err
			}

			path := "/account/service/v1/balance"
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, err := c.Get(cmd.Context(), path, nil)
			if err != nil {
				msg := err.Error()
				if strings.Contains(msg, "HTTP 401") || strings.Contains(msg, "HTTP 403") {
					return authErr(fmt.Errorf("%w\nhint: Account balance requires OAuth Bearer auth (see docs.parallel.ai/integrations/account-api), not PARALLEL_API_KEY alone", err))
				}
				return classifyAPIError(err, flags)
			}

			var balance types.BalanceResponse
			if err := json.Unmarshal(data, &balance); err != nil {
				return fmt.Errorf("parsing balance response: %w", err)
			}

			if db, openErr := store.OpenWithContext(cmd.Context(), defaultDBPath("parallel-pp-cli")); openErr == nil {
				_ = db.InsertBalanceSnapshot(
					time.Now().UTC(),
					balance.OrgId,
					balance.CreditBalanceCents,
					balance.PendingDebitBalanceCents,
					balance.WillInvoice,
					data,
				)
				_ = db.Close()
			}

			min := float64(flagMinBalance)
			blocked := balance.CreditBalanceCents < min || (!balance.WillInvoice && balance.CreditBalanceCents < min)
			out := map[string]any{
				"ok":                          !blocked,
				"credit_balance_cents":        balance.CreditBalanceCents,
				"min_balance":                 flagMinBalance,
				"will_invoice":                balance.WillInvoice,
				"pending_debit_balance_cents": balance.PendingDebitBalanceCents,
			}

			if err := flags.printJSON(cmd, out); err != nil {
				return err
			}
			if blocked {
				return fmt.Errorf("balance guard: credit_balance_cents %.0f below min_balance %d (will_invoice=%v)",
					balance.CreditBalanceCents, flagMinBalance, balance.WillInvoice)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagMinBalance, "min-balance", 0, "Minimum prepaid credit balance in cents required to proceed")
	return cmd
}

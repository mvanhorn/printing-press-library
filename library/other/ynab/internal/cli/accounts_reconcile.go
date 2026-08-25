// Copyright 2026 mikesnowbie and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live
// No local mirror exists for this CLI (no sync command was generated for
// this spec), so this command reads live from the YNAB API on every call.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
)

// ynabAccountTransactionsEnvelope mirrors YNAB's TransactionsResponse for the
// account-scoped transactions endpoint.
type ynabAccountTransactionsEnvelope struct {
	Data struct {
		Transactions []ynabTransaction `json:"transactions"`
	} `json:"data"`
}

// ynabAccountEnvelope mirrors YNAB's AccountResponse for the single-account
// endpoint: { "data": { "account": {...} } }.
type ynabAccountEnvelope struct {
	Data struct {
		Account struct {
			ClearedBalanceCurrency float64 `json:"cleared_balance_currency"`
		} `json:"account"`
	} `json:"data"`
}

type ynabTransaction struct {
	ID             string  `json:"id"`
	Date           string  `json:"date"`
	AmountCurrency float64 `json:"amount_currency"`
	Amount         int64   `json:"amount"`
	Memo           *string `json:"memo"`
	Cleared        string  `json:"cleared"` // "cleared", "uncleared", "reconciled"
	Approved       bool    `json:"approved"`
	PayeeName      *string `json:"payee_name"`
	Deleted        bool    `json:"deleted"`
}

type reconcileCandidate struct {
	ID      string  `json:"id"`
	Date    string  `json:"date"`
	Payee   string  `json:"payee,omitempty"`
	Memo    string  `json:"memo,omitempty"`
	Amount  float64 `json:"amount"`
	Cleared string  `json:"cleared"`
}

// reconcileDefaultSinceDate is passed as since_date when the user does not
// supply --since-date. YNAB's transactions endpoint defaults to a one-year
// lookback when since_date is omitted entirely (confirmed in the bundled
// OpenAPI spec), which would silently exclude the discrepancy-causing
// transaction for any account with cleared history older than a year. This
// command's whole purpose is finding that transaction, so it always requests
// the account's full history unless the caller narrows the window themselves.
const reconcileDefaultSinceDate = "2000-01-01"

type reconcileResult struct {
	AccountID         string               `json:"account_id"`
	StatementBalance  float64              `json:"statement_balance"`
	LocalClearedTotal float64              `json:"local_cleared_total"`
	Difference        float64              `json:"difference"`
	Matched           bool                 `json:"matched"`
	Candidates        []reconcileCandidate `json:"candidate_transactions,omitempty"`
	Note              string               `json:"note,omitempty"`
}

func newNovelAccountsReconcileCmd(flags *rootFlags) *cobra.Command {
	var flagStatementBalance string
	var flagSinceDate string
	var flagPlan string

	cmd := &cobra.Command{
		Use:     "reconcile <account-id>",
		Short:   "Diff the local cleared-transaction total against a bank statement balance and surface the transaction(s)",
		Example: "  ynab-pp-cli accounts reconcile a1b2c3d4 --statement-balance 1042.17 --agent",
		// Live happy-path fixture: a syntactically-valid but almost certainly
		// nonexistent account UUID, so the live dogfood happy-path check
		// exercises a real request instead of being skipped for lacking
		// required-flag/positional input. A 404 for that ID is the expected,
		// graceful outcome (typed exit 3), not a failure.
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "<account-id>=00000000-0000-0000-0000-000000000000;--statement-balance=0.00",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "accounts reconcile")
			}
			if len(args) == 0 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("missing required argument <account-id>\nUsage: %s <account-id> --statement-balance <amount>", cmd.CommandPath()))
			}
			accountID := args[0]
			if flagStatementBalance == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--statement-balance is required"))
			}
			statementBalance, err := strconv.ParseFloat(flagStatementBalance, 64)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--statement-balance %q is not a valid number", flagStatementBalance))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Ground truth for "what YNAB thinks the cleared balance is" is the
			// account's own cleared_balance_currency, which YNAB computes
			// all-time. A windowed sum over recently-listed transactions is
			// NOT equivalent to this unless the account is younger than the
			// window — summing transactions was tried first and produces a
			// wrong total for any older account, so the account endpoint is
			// the source of truth and the transaction list is used only to
			// surface candidate discrepancies below.
			acctPath := "/plans/{plan_id}/accounts/{account_id}"
			acctPath = replacePathParam(acctPath, "plan_id", flagPlan)
			acctPath = replacePathParam(acctPath, "account_id", accountID)
			acctRaw, err := c.GetWithHeadersNoCache(ctx, acctPath, nil, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var acctEnvelope ynabAccountEnvelope
			if err := json.Unmarshal(acctRaw, &acctEnvelope); err != nil {
				return fmt.Errorf("parsing account response: %w", err)
			}
			clearedTotal := acctEnvelope.Data.Account.ClearedBalanceCurrency

			diff := math.Round((statementBalance-clearedTotal)*100) / 100
			matched := math.Abs(diff) < 0.005

			result := reconcileResult{
				AccountID:         accountID,
				StatementBalance:  statementBalance,
				LocalClearedTotal: math.Round(clearedTotal*100) / 100,
				Difference:        diff,
				Matched:           matched,
			}

			candidates := make([]reconcileCandidate, 0)
			if !matched {
				// Fetch recent transactions only to surface likely culprits for
				// the discrepancy; the comparison above already used the
				// account's real cleared_balance, not this windowed list.
				txPath := "/plans/{plan_id}/accounts/{account_id}/transactions"
				txPath = replacePathParam(txPath, "plan_id", flagPlan)
				txPath = replacePathParam(txPath, "account_id", accountID)
				sinceDate := flagSinceDate
				if sinceDate == "" {
					sinceDate = reconcileDefaultSinceDate
				}
				txParams := map[string]string{"since_date": sinceDate}
				txRaw, txErr := c.GetWithHeadersNoCache(ctx, txPath, txParams, nil)
				if txErr != nil {
					return classifyAPIError(txErr, flags)
				}
				var txEnvelope ynabAccountTransactionsEnvelope
				if err := json.Unmarshal(txRaw, &txEnvelope); err != nil {
					return fmt.Errorf("parsing transactions response: %w", err)
				}

				// Surface individual cleared transactions closest in absolute
				// value to the discrepancy amount — the most likely culprits
				// for a missing/duplicate/miskeyed entry.
				sort.Slice(txEnvelope.Data.Transactions, func(i, j int) bool {
					di := math.Abs(math.Abs(txEnvelope.Data.Transactions[i].AmountCurrency) - math.Abs(diff))
					dj := math.Abs(math.Abs(txEnvelope.Data.Transactions[j].AmountCurrency) - math.Abs(diff))
					return di < dj
				})
				for _, t := range txEnvelope.Data.Transactions {
					if t.Deleted || (t.Cleared != "cleared" && t.Cleared != "reconciled") {
						continue
					}
					payee := ""
					if t.PayeeName != nil {
						payee = *t.PayeeName
					}
					memo := ""
					if t.Memo != nil {
						memo = *t.Memo
					}
					candidates = append(candidates, reconcileCandidate{
						ID: t.ID, Date: t.Date, Payee: payee, Memo: memo,
						Amount: t.AmountCurrency, Cleared: t.Cleared,
					})
					if len(candidates) >= 5 {
						break
					}
				}
				result.Candidates = candidates
				result.Note = fmt.Sprintf("statement balance and cleared balance differ by %.2f; candidate_transactions is sorted by closeness to that amount, not chronology", diff)
				if flagSinceDate != "" {
					result.Note += fmt.Sprintf(". Candidates are searched from %s onward only, per --since-date; the discrepancy itself is computed from the account's real all-time cleared balance, so a culprit transaction before that date will not appear above.", flagSinceDate)
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Statement balance:     %.2f\n", result.StatementBalance)
			fmt.Fprintf(cmd.OutOrStdout(), "Local cleared total:   %.2f\n", result.LocalClearedTotal)
			fmt.Fprintf(cmd.OutOrStdout(), "Difference:            %.2f\n", result.Difference)
			if result.Matched {
				fmt.Fprintln(cmd.OutOrStdout(), "Matched — no discrepancy found.")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nNot matched. Closest candidate transactions:")
			for _, cand := range candidates {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %-10s  %8.2f  %s %s\n", cand.Date, cand.Cleared, cand.Amount, cand.Payee, cand.Memo)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagStatementBalance, "statement-balance", "", "The bank statement's ending balance to reconcile against (required)")
	cmd.Flags().StringVar(&flagSinceDate, "since-date", "", "Only consider transactions on or after this date (YYYY-MM-DD); defaults to searching the account's full history")
	cmd.Flags().StringVar(&flagPlan, "plan", "last-used", `Plan (budget) ID, or "last-used" for the most recently used plan`)
	return cmd
}

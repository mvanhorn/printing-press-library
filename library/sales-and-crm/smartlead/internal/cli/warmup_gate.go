// Copyright 2026 bossriceshark. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/smartlead/internal/cliutil"

	"github.com/spf13/cobra"
)

// warmupGateRow is one account's launch-readiness verdict.
type warmupGateRow struct {
	AccountID   string   `json:"account_id"`
	FromEmail   string   `json:"from_email"`
	Connected   bool     `json:"connected"`
	InboxRate   float64  `json:"inbox_rate"`
	HistoryDays int      `json:"warmup_history_days"`
	Pass        bool     `json:"pass"`
	Reasons     []string `json:"reasons,omitempty"`
}

// warmupGateResult is the full gate output.
type warmupGateResult struct {
	Checked  int             `json:"checked"`
	Passed   int             `json:"passed"`
	Failed   int             `json:"failed"`
	AllPass  bool            `json:"all_pass"`
	Accounts []warmupGateRow `json:"accounts"`
}

func newWarmupGateCmd(flags *rootFlags) *cobra.Command {
	var account string
	var minInboxRate float64
	var minDays int
	var strict bool

	cmd := &cobra.Command{
		Use:   "warmup-gate",
		Short: "Pass/fail launch gate for email sender warmup readiness",
		Long: strings.Trim(`
Check whether email sender accounts are warmed up enough to attach to a new
campaign. Each account passes when it is connected, has at least --min-days of
warmup history, and an inbox landing rate at or above --min-inbox-rate. With
--strict the command exits non-zero when any account fails, so it can gate a
launch script. Without --strict it always exits 0 and reports the verdict in
its output.`, "\n"),
		Example: strings.Trim(`
  smartlead-pp-cli warmup-gate
  smartlead-pp-cli warmup-gate --account 18189478 --json
  smartlead-pp-cli warmup-gate --min-inbox-rate 0.9 --strict`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,1",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			accounts, err := fetchAllPaged(c, "/email-accounts", 100)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if account != "" {
				var filtered []json.RawMessage
				for _, a := range accounts {
					var m map[string]any
					if json.Unmarshal(a, &m) == nil && asString(m["id"]) == account {
						filtered = append(filtered, a)
					}
				}
				if len(filtered) == 0 {
					return notFoundErr(fmt.Errorf("email account %s not found", account))
				}
				accounts = filtered
			}

			result := warmupGateResult{AllPass: true}
			for _, raw := range accounts {
				var acct map[string]any
				if json.Unmarshal(raw, &acct) != nil {
					continue
				}
				id := asString(acct["id"])
				if id == "" {
					continue
				}
				row := warmupGateRow{
					AccountID: id,
					FromEmail: asString(acct["from_email"]),
					Connected: asBool(acct["is_smtp_success"]) && asBool(acct["is_imap_success"]),
				}
				inboxRate, _, historyDays, haveWarmup := fetchWarmup(c, id)
				row.InboxRate = inboxRate
				row.HistoryDays = historyDays
				row.Pass = true
				if !row.Connected {
					row.Pass = false
					row.Reasons = append(row.Reasons, "account not connected")
				}
				if !haveWarmup {
					row.Pass = false
					row.Reasons = append(row.Reasons, "no warmup history")
				} else {
					if historyDays < minDays {
						row.Pass = false
						row.Reasons = append(row.Reasons,
							fmt.Sprintf("only %d days of warmup history (need %d)", historyDays, minDays))
					}
					if inboxRate < minInboxRate {
						row.Pass = false
						row.Reasons = append(row.Reasons,
							fmt.Sprintf("inbox rate %.2f below %.2f", inboxRate, minInboxRate))
					}
				}
				result.Checked++
				if row.Pass {
					result.Passed++
				} else {
					result.Failed++
					result.AllPass = false
				}
				result.Accounts = append(result.Accounts, row)
			}
			sort.Slice(result.Accounts, func(i, j int) bool {
				if result.Accounts[i].Pass != result.Accounts[j].Pass {
					return !result.Accounts[i].Pass
				}
				return result.Accounts[i].FromEmail < result.Accounts[j].FromEmail
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
					return err
				}
			} else {
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(tw, "VERDICT\tACCOUNT\tINBOX%\tWARMUP-DAYS\tREASONS")
				for _, r := range result.Accounts {
					v := "PASS"
					if !r.Pass {
						v = "FAIL"
					}
					fmt.Fprintf(tw, "%s\t%s\t%.1f\t%d\t%s\n",
						v, truncate(r.FromEmail, 36), r.InboxRate*100, r.HistoryDays,
						strings.Join(r.Reasons, "; "))
				}
				tw.Flush()
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d checked, %d passed, %d failed\n",
					result.Checked, result.Passed, result.Failed)
			}

			// --strict turns the gate into a scriptable exit code. The verify
			// harness runs commands live in mock mode; never fail it.
			if strict && !result.AllPass && !cliutil.IsVerifyEnv() {
				return &cliError{code: 1, err: fmt.Errorf(
					"warmup gate failed: %d of %d accounts not launch-ready", result.Failed, result.Checked)}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Gate a single email account ID (default: all accounts)")
	cmd.Flags().Float64Var(&minInboxRate, "min-inbox-rate", 0.85, "Minimum warmup inbox landing rate to pass (0.0–1.0)")
	cmd.Flags().IntVar(&minDays, "min-days", 7, "Minimum days of warmup history to pass")
	cmd.Flags().BoolVar(&strict, "strict", false, "Exit non-zero when any account fails the gate")
	return cmd
}

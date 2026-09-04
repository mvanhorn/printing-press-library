// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto

package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/cliutil"
	"github.com/spf13/cobra"
)

func newNovelFairnessNudgeCmd(flags *rootFlags) *cobra.Command {
	var customMessage string
	var overrideExpenseID int
	var send bool
	var dbPath string

	cmd := &cobra.Command{
		Use:  "nudge <friend>",
		Long: "Use this command to preview a payment reminder, and pass --send to opt in to posting it as a comment. Do NOT use it to record a payment; use 'settle-up --record'.\n\nExit code 3 means the named group/friend was not found in the local store.",
		// MinimumNArgs(1), not ExactArgs(1): the MCP command-mirror whitespace-splits
		// a quoted multi-word friend name (args:"Tahoe Trip") into several positionals
		// (["Tahoe","Trip"]). ExactArgs(1) rejected those before resolution could run,
		// and even when it didn't, only the first token reached resolveFairnessFriend
		// and substring-matched the wrong friend. Accept the extra positionals and
		// rejoin them below.
		Short:   "Send a friendly payment reminder to a friend who owes you",
		Example: "  splitwise-pp-cli fairness nudge \"Alex Kim\"",
		// CLI-only write action: keep nudge off the MCP surface so an agent can't
		// auto-post reminders, and so it is never grouped under the read-only
		// fairness parent tool.
		// pp:method GET declares the DEFAULT invocation (no --record/--send) as a read for the live-dogfood runner so the
		// happy path runs without an injected --dry-run; the write path is opt-in, harness-refused, and still advertised
		// to MCP hosts as non-read-only via mcp:read-only=false. See .printing-press-patches/splitwise-print-only-live-gate.json.
		Annotations: map[string]string{"mcp:hidden": "true", "mcp:read-only": "false", "pp:happy-args": "<friend>=Example Friend", "pp:method": "GET", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) == 0 {
				return novelErr(cmd, flags, usageErr(errors.New("friend name or id is required")))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fairness nudge")
			}
			if send && cliutil.IsAnyHarness() {
				return writeHarnessRefusal(cmd.OutOrStdout(), flags, "fairness nudge --send")
			}

			db, err := openSplitwiseStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			friends, err := loadFriends(db)
			if err != nil {
				return err
			}
			expenses, err := loadExpenses(db)
			if err != nil {
				return err
			}
			youID := loadCurrentUserID(db)

			// Rejoin multi-word names split into separate positionals by the MCP
			// command-mirror. Inline join (not joinNameArgs) keeps this branch
			// self-contained; once the multiword settle/resolve PR lands joinNameArgs
			// in this package, this can be refactored to call it.
			friendQuery := strings.TrimSpace(strings.Join(args, " "))
			friend, ok, resolveErr := resolveSettleFriend(friendQuery, friends)
			if resolveErr != nil {
				return novelErr(cmd, flags, usageErr(resolveErr))
			}
			if !ok {
				return novelErr(cmd, flags, notFoundErr(fmt.Errorf("no friend matches %q; run sync first", friendQuery)))
			}
			friendID, friendName := friend.ID, friendDisplayName(friend)
			if friendName == "" {
				friendName = fmt.Sprintf("friend %d", friend.ID)
			}

			target, ok := selectNudgeExpense(expenses, friendID, youID)
			if cmd.Flags().Changed("expense-id") {
				overrideTarget, found := findExpenseByID(expenses, overrideExpenseID)
				if !found {
					return novelErr(cmd, flags, usageErr(fmt.Errorf("no expense matches --expense-id %d", overrideExpenseID)))
				}
				// Apply the same guards selectNudgeExpense uses, so a manual
				// --expense-id can't post a wrong-amount reminder (friend not on the
				// expense → message quotes the total) or a doomed comment (deleted /
				// payment row → opaque API error).
				if problem := nudgeExpenseProblem(overrideTarget, friendID); problem != "" {
					return novelErr(cmd, flags, usageErr(fmt.Errorf("--expense-id %d is not a valid nudge target: %s", overrideExpenseID, problem)))
				}
				target = overrideTarget
				ok = true
			}
			if !ok {
				return fmt.Errorf("no shared unsettled expense found to comment on")
			}

			msg := buildNudgeMessage(friendName, friendID, target, customMessage)
			result := map[string]any{
				"friend":     friendName,
				"friend_id":  friendID,
				"expense_id": target.ID,
				"message":    msg,
				"sent":       false,
			}

			if !send || flags.dryRun {
				if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
					return flags.emitStructured(cmd, result)
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "friend: %s (id %d)\n", friendName, friendID)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "target expense: %d | %s | %s %s\n", target.ID, strings.TrimSpace(target.Description), strings.TrimSpace(target.Cost), strings.TrimSpace(target.CurrencyCode))
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "message: %s\n", msg)
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "preview only — re-run with --send to post the reminder comment")
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, status, err := c.PostWithParams(
				cmd.Context(),
				"/create_comment",
				map[string]string{},
				map[string]any{"content": msg, "expense_id": fmt.Sprint(target.ID)},
			)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if status < 200 || status >= 300 {
				return fmt.Errorf("create-comment failed: status %d", status)
			}
			if envErr := splitwiseMutationError(data); envErr != nil {
				return envErr
			}

			result["sent"] = true
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.emitStructured(cmd, result)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "sent reminder to %s on expense %d\n", friendName, target.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&customMessage, "message", "", "Custom reminder message")
	cmd.Flags().IntVar(&overrideExpenseID, "expense-id", 0, "Override the target expense id")
	cmd.Flags().BoolVar(&send, "send", false, "Post the reminder comment")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}

func selectNudgeExpense(expenses []Expense, friendID, youID int) (Expense, bool) {
	var bestYouPaid Expense
	var bestYouPaidDate time.Time
	bestYouPaidOK := false
	bestYouPaidParsed := false

	var bestFallback Expense
	var bestFallbackDate time.Time
	bestFallbackOK := false
	bestFallbackParsed := false

	for _, e := range expenses {
		if e.Payment || expenseDeleted(e.DeletedAt) {
			continue
		}
		friendOwes := false
		youPaid := false
		for _, u := range e.Users {
			if u.UserID == friendID && parseAmount(u.OwedShare) > 0 {
				friendOwes = true
			}
			if u.UserID == youID && parseAmount(u.PaidShare) > 0 {
				youPaid = true
			}
		}
		if !friendOwes {
			continue
		}

		t, parsed := parseSplitwiseDate(e.Date)
		if youPaid {
			if !bestYouPaidOK || isMoreRecentCandidate(parsed, t, bestYouPaidParsed, bestYouPaidDate) {
				bestYouPaid = e
				bestYouPaidDate = t
				bestYouPaidOK = true
				bestYouPaidParsed = parsed
			}
			continue
		}
		if !bestFallbackOK || isMoreRecentCandidate(parsed, t, bestFallbackParsed, bestFallbackDate) {
			bestFallback = e
			bestFallbackDate = t
			bestFallbackOK = true
			bestFallbackParsed = parsed
		}
	}

	if bestYouPaidOK {
		return bestYouPaid, true
	}
	if bestFallbackOK {
		return bestFallback, true
	}
	return Expense{}, false
}

func isMoreRecentCandidate(parsed bool, t time.Time, bestParsed bool, best time.Time) bool {
	if parsed {
		if !bestParsed {
			return true
		}
		return t.After(best)
	}
	return !bestParsed
}

func buildNudgeMessage(friendName string, friendID int, e Expense, custom string) string {
	if custom != "" {
		return custom
	}
	desc := strings.TrimSpace(e.Description)
	if desc == "" {
		desc = fmt.Sprintf("expense %d", e.ID)
	}
	// Quote the friend's own owed_share on this expense — the amount the reminder
	// is actually about — not the expense total, which would overstate what they
	// owe on a split. Fall back to the expense cost only if no share is recorded.
	amount := strings.TrimSpace(e.Cost)
	for _, u := range e.Users {
		if u.UserID == friendID {
			if s := strings.TrimSpace(u.OwedShare); s != "" {
				amount = s
			}
			break
		}
	}
	if amount == "" {
		amount = "0"
	}
	amountStr := strings.TrimSpace(amount + " " + strings.TrimSpace(e.CurrencyCode))
	// Reachability caveat: Friend does not expose registration/email status in
	// this CLI, so v1 does not pre-gate recipient deliverability.
	return fmt.Sprintf("Hey %s, friendly reminder about your share of %q (%s) whenever you get a chance - thanks!", friendName, desc, amountStr)
}

func findExpenseByID(expenses []Expense, id int) (Expense, bool) {
	for _, e := range expenses {
		if e.ID == id {
			return e, true
		}
	}
	return Expense{}, false
}

// nudgeExpenseProblem returns a human-readable reason the expense is not a valid
// nudge target for friendID, or "" if it is. Mirrors the guards selectNudgeExpense
// applies so a manual --expense-id override can't post a wrong-amount reminder
// (friend not on the expense → message would quote the total cost) or a doomed
// comment (a deleted or payment/settlement row).
func nudgeExpenseProblem(e Expense, friendID int) string {
	if expenseDeleted(e.DeletedAt) {
		return "that expense is deleted"
	}
	if e.Payment {
		return "that expense is a payment/settlement record, not a shared charge"
	}
	for _, u := range e.Users {
		if u.UserID == friendID && parseAmount(u.OwedShare) > 0 {
			return ""
		}
	}
	return "that friend has no positive owed share on that expense"
}

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newFairnessNudgeCmd(flags *rootFlags) *cobra.Command {
	var customMessage string
	var overrideExpenseID int
	var send bool

	cmd := &cobra.Command{
		Use: "nudge <friend>",
		// MinimumNArgs(1), not ExactArgs(1): the MCP command-mirror whitespace-splits
		// a quoted multi-word friend name (args:"Tahoe Trip") into several positionals
		// (["Tahoe","Trip"]). ExactArgs(1) rejected those before resolution could run,
		// and even when it didn't, only the first token reached resolveFairnessFriend
		// and substring-matched the wrong friend. Accept the extra positionals and
		// rejoin them below.
		Args:  cobra.MinimumNArgs(1),
		Short: "Send a friendly payment reminder to a friend who owes you",
		// CLI-only write action: keep nudge off the MCP surface so an agent can't
		// auto-post reminders, and so it is never grouped under the read-only
		// fairness parent tool.
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "would send a nudge")
				return nil
			}

			db, err := openSplitwiseStore(cmd.Context())
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
			friendID, friendName, ok := resolveFairnessFriend(friendQuery, friends)
			if !ok {
				return usageErr(fmt.Errorf("no friend matches %q; run sync first", friendQuery))
			}

			target, ok := selectNudgeExpense(expenses, friendID, youID)
			if cmd.Flags().Changed("expense-id") {
				overrideTarget, found := findExpenseByID(expenses, overrideExpenseID)
				if !found {
					return usageErr(fmt.Errorf("no expense matches --expense-id %d", overrideExpenseID))
				}
				// Apply the same guards selectNudgeExpense uses, so a manual
				// --expense-id can't post a wrong-amount reminder (friend not on the
				// expense → message quotes the total) or a doomed comment (deleted /
				// payment row → opaque API error).
				if problem := nudgeExpenseProblem(overrideTarget, friendID); problem != "" {
					return usageErr(fmt.Errorf("--expense-id %d is not a valid nudge target: %s", overrideExpenseID, problem))
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
					return flags.printJSON(cmd, result)
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
				return classifyAPIError(err, flags)
			}
			if status < 200 || status >= 300 {
				return fmt.Errorf("create-comment failed: status %d", status)
			}
			if envErr := splitwiseMutationError(data); envErr != nil {
				return envErr
			}

			result["sent"] = true
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, result)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "sent reminder to %s on expense %d\n", friendName, target.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&customMessage, "message", "", "Custom reminder message")
	cmd.Flags().IntVar(&overrideExpenseID, "expense-id", 0, "Override the target expense id")
	cmd.Flags().BoolVar(&send, "send", false, "Post the reminder comment")
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

// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local

package cli

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/store"
	"github.com/spf13/cobra"
)

func newNovelLedgerCmd(flags *rootFlags) *cobra.Command {
	var friendRef string
	var dbPath string
	cmd := &cobra.Command{
		Use:   "ledger <group>",
		Short: "Replay a group's expenses in date order with a running balance per member, or one friend across all groups (--friend)",
		Long:  "Use this command to see how balances got to where they are, expense by expense — '<group>' for one group's members, '--friend' for one person across every group. Do NOT use it for the current snapshot; use 'balances --by-group'. Do NOT use it to compute transfers; use 'settle-up'. Do NOT use it for spend totals; use 'spend'.\n\nExit code 3 means the named group/friend was not found in the local store.",
		Example: "  splitwise-pp-cli ledger \"Tahoe Trip\" --agent\n" +
			"  splitwise-pp-cli ledger --friend \"Alex Kim\" --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<group>=Example Group", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "ledger")
			}
			if len(args) > 0 && strings.TrimSpace(friendRef) != "" {
				return novelErr(cmd, flags, usageErr(errors.New("use either a group or --friend, not both")))
			}
			if len(args) == 0 && strings.TrimSpace(friendRef) == "" {
				return novelErr(cmd, flags, usageErr(errors.New("provide a group name or --friend")))
			}
			if strings.TrimSpace(friendRef) == "" {
				return runGroupLedger(cmd, flags, joinNameArgs(args), dbPath)
			}

			db, err := openSplitwiseStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			return runFriendLedger(cmd, flags, db, friendRef)
		},
	}
	cmd.Flags().StringVar(&friendRef, "friend", "", "Replay balances with this friend across all groups")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}

func runGroupLedger(cmd *cobra.Command, flags *rootFlags, groupRef, dbPath string) error {
	groupCmd := newLedgerCmd(flags)
	if dbPath != "" {
		_ = groupCmd.Flags().Set("db", dbPath)
	}
	return groupCmd.RunE(cmd, []string{groupRef})
}

type friendLedgerRow struct {
	Date           string  `json:"date"`
	Description    string  `json:"description"`
	Group          string  `json:"group"`
	Currency       string  `json:"currency"`
	Delta          float64 `json:"delta"`
	RunningBalance float64 `json:"running_balance"`
	Payment        bool    `json:"payment"`
}

func runFriendLedger(cmd *cobra.Command, flags *rootFlags, db *store.Store, friendRef string) error {
	// one "not synced" hint per invocation: the generated helper returns true once it has printed
	_ = hintIfUnsynced(cmd, db, "get-friends") ||
		hintIfUnsynced(cmd, db, "get-expenses")
	hintIfStale(cmd, db, "get-friends", flags.maxAge)
	hintIfStale(cmd, db, "get-expenses", flags.maxAge)
	friends, err := loadFriends(db)
	if err != nil {
		return err
	}
	friend, ok, err := resolveSettleFriend(friendRef, friends)
	if err != nil {
		return novelErr(cmd, flags, usageErr(err))
	}
	if !ok {
		return novelErr(cmd, flags, notFoundErr(fmt.Errorf("no friend matches %q; run sync first", strings.TrimSpace(friendRef))))
	}
	youID := loadCurrentUserID(db)
	if youID == 0 {
		return novelErr(cmd, flags, usageErr(errors.New("current user is not synced; run: splitwise-pp-cli sync --resources get-current-user")))
	}
	expenses, err := loadExpenses(db)
	if err != nil {
		return err
	}
	groups, err := loadGroups(db)
	if err != nil {
		return err
	}
	groupNames := map[int]string{0: "no group"}
	for _, group := range groups {
		groupNames[group.ID] = strings.TrimSpace(group.Name)
	}

	type pending struct {
		friendLedgerRow
		delta float64
	}
	pendingRows := make([]pending, 0)
	for _, expense := range expenses {
		if expenseDeleted(expense.DeletedAt) {
			continue
		}
		var youNet, friendNet float64
		var hasYou, hasFriend bool
		for _, user := range expense.Users {
			net := parseAmount(user.PaidShare) - parseAmount(user.OwedShare)
			switch user.UserID {
			case youID:
				youNet, hasYou = net, true
			case friend.ID:
				friendNet, hasFriend = net, true
			}
		}
		if !hasYou || !hasFriend {
			continue
		}
		delta := pairwiseDelta(youNet, friendNet)
		groupName := strings.TrimSpace(groupNames[expense.GroupID])
		if groupName == "" {
			groupName = fmt.Sprintf("Group %d", expense.GroupID)
		}
		pendingRows = append(pendingRows, pending{friendLedgerRow: friendLedgerRow{
			Date: strings.TrimSpace(expense.Date), Description: strings.TrimSpace(expense.Description),
			Group: groupName, Currency: strings.TrimSpace(expense.CurrencyCode), Payment: expense.Payment,
		}, delta: delta})
	}
	sort.SliceStable(pendingRows, func(i, j int) bool { return pendingRows[i].Date < pendingRows[j].Date })
	running := make(map[string]float64)
	rows := make([]friendLedgerRow, 0, len(pendingRows))
	for _, item := range pendingRows {
		running[item.Currency] += item.delta
		item.Delta = round2(item.delta)
		item.RunningBalance = round2(running[item.Currency])
		rows = append(rows, item.friendLedgerRow)
	}
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return flags.emitStructured(cmd, rows)
	}
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "DATE\tDESCRIPTION\tGROUP\tCURRENCY\tDELTA\tRUNNING BALANCE\tPAYMENT")
	for _, row := range rows {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%.2f\t%.2f\t%t\n", row.Date, row.Description, row.Group, row.Currency, row.Delta, row.RunningBalance, row.Payment)
	}
	return tw.Flush()
}

func pairwiseDelta(youNet, friendNet float64) float64 {
	if youNet > 0 && friendNet < 0 {
		return min(youNet, -friendNet)
	}
	if youNet < 0 && friendNet > 0 {
		return -min(-youNet, friendNet)
	}
	return 0
}

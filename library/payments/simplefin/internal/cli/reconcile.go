// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: content-based duplicate detection. SimpleFIN transaction IDs
// are unstable and mirrored across accounts, so identity-based dedup misses
// real duplicates. We hash amount + posted-day + normalized description to
// catch the mirrored charges that break every importer.
//
// pp:data-source local

package cli

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/simplefin"
	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/store"
)

type dupMember struct {
	ResourceID  string `json:"resource_id"`
	Account     string `json:"account"`
	Posted      string `json:"posted"`
	Amount      string `json:"amount"`
	Description string `json:"description"`
}

type dupGroup struct {
	Hash         string      `json:"hash"`
	CrossAccount bool        `json:"cross_account"`
	Members      []dupMember `json:"members"`
}

func newNovelReconcileCmd(flags *rootFlags) *cobra.Command {
	var flagFix bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Finds and merges duplicate transactions created by SimpleFIN's unstable, mirrored IDs using content hashing.",
		Long: "Detect duplicate transactions by content (amount + posted day + normalized description)\n" +
			"rather than the unstable IDs SimpleFIN reuses and mirrors across accounts. By default this\n" +
			"is read-only and reports duplicate groups.\n\n" +
			"--fix removes only SAME-ACCOUNT repeats (unambiguous duplicates). Cross-account collisions\n" +
			"are flagged as possible mirrors but never auto-deleted — they may be legitimate transfers\n" +
			"between your own accounts. Note: --fix edits the local store only; a later 'sync' can\n" +
			"re-introduce removed rows from the API, so treat it as a one-time view cleanup.\n\n" +
			"Use this command to find duplicate transactions from mirrored IDs. For stale-connection\n" +
			"(balance-date age) problems use 'stale'.",
		Example: "  simplefin-pp-cli reconcile --agent",
		// No mcp:read-only hint: with --fix this command deletes rows from the
		// local store, so an MCP host must treat it as state-mutating and
		// prompt for confirmation rather than assume it is safe.
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dryRunOK(flags) {
				return nil
			}
			db, ok, err := resolveSimplefinDB(ctx, cmd, flags, dbPath)
			if err != nil || !ok {
				return err
			}
			defer db.Close()
			hintIfUnsynced(cmd, db, "transactions")

			txns, err := loadTransactions(ctx, db, "", 0, 0, true)
			if err != nil {
				return err
			}
			byHash := map[string][]storedTxn{}
			for _, t := range txns {
				h := simplefin.ContentHash(simplefin.Transaction{
					Amount: t.Amount, Posted: t.Posted, Payee: t.Payee, Description: t.Description,
				})
				byHash[h] = append(byHash[h], t)
			}
			groups := make([]dupGroup, 0)
			removable := make([]string, 0)
			for h, members := range byHash {
				if len(members) < 2 {
					continue
				}
				sort.Slice(members, func(i, j int) bool { return members[i].ResourceID < members[j].ResourceID })
				g := dupGroup{Hash: h}
				// Only same-account repeats are unambiguous duplicates safe to
				// remove. Cross-account collisions may be legitimate transfers
				// between your own accounts, so report them but never auto-delete.
				accountSeen := map[string]bool{}
				for _, m := range members {
					g.Members = append(g.Members, dupMember{
						ResourceID: m.ResourceID, Account: m.AccountName,
						Posted: time.Unix(m.Posted, 0).UTC().Format("2006-01-02"),
						Amount: m.Amount, Description: m.Description,
					})
					if accountSeen[m.AccountID] {
						removable = append(removable, m.ResourceID)
					}
					accountSeen[m.AccountID] = true
				}
				g.CrossAccount = len(accountSeen) > 1
				groups = append(groups, g)
			}
			sort.Slice(groups, func(i, j int) bool { return len(groups[i].Members) > len(groups[j].Members) })

			if flagFix && len(removable) > 0 {
				if err := deleteTransactions(ctx, db, removable); err != nil {
					return err
				}
			}

			result := map[string]any{
				"duplicate_groups": groups,
				"duplicate_count":  len(removable),
				"fixed":            flagFix,
			}
			if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, result)
			}
			if len(groups) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no duplicate transactions detected")
				return nil
			}
			for _, g := range groups {
				tag := ""
				if g.CrossAccount {
					tag = "  [cross-account: possible transfer — review, not auto-removed]"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "duplicate (%d): %s %s%s\n", len(g.Members), g.Members[0].Amount, truncate(g.Members[0].Description, 40), tag)
				for _, m := range g.Members {
					fmt.Fprintf(cmd.OutOrStdout(), "    %s  %s  %s\n", m.Posted, truncate(m.Account, 24), m.ResourceID)
				}
			}
			if flagFix {
				fmt.Fprintf(cmd.OutOrStdout(), "\nremoved %d same-account duplicate transactions (a later 'sync' may re-introduce them)\n", len(removable))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d same-account duplicates across %d groups. Re-run with --fix to remove them.\n", len(removable), len(groups))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagFix, "fix", false, "Remove detected duplicates (keeps the first member of each group)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}

func deleteTransactions(ctx context.Context, db *store.Store, resourceIDs []string) error {
	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range resourceIDs {
		if _, err := tx.ExecContext(ctx, `DELETE FROM resources WHERE resource_type = 'transactions' AND id = ?`, id); err != nil {
			return err
		}
		// Pair every resources delete with its resources_fts row (keyed by the
		// same ftsRowID the store uses on upsert), or `search` returns ghost
		// hits for transactions reconcile just removed.
		if _, err := tx.ExecContext(ctx, `DELETE FROM resources_fts WHERE rowid = ?`, store.FTSRowID("transactions", id)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM transaction_categories WHERE txn_id = ?`, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

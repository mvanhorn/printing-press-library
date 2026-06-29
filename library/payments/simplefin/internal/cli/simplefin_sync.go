// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored SimpleFIN sync. Replaces the generic framework sync because the
// SimpleFIN /accounts endpoint returns accounts with nested transactions and
// holdings in a single call — the generic resource-walker cannot split those out.
//
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/simplefin"
	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/store"
)

type syncStats struct {
	AccountsSynced int      `json:"accounts_synced"`
	TxnsInserted   int      `json:"txns_inserted"`
	TxnsUpdated    int      `json:"txns_updated"`
	HoldingsSynced int      `json:"holdings_synced"`
	Connections    int      `json:"connections"`
	Categorized    int      `json:"categorized"`
	SnapshotAt     int64    `json:"snapshot_at"`
	Window         string   `json:"window"`
	APIMessages    []string `json:"api_messages,omitempty"`
	Errors         []string `json:"errors,omitempty"`
}

func newSimplefinSyncCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var since string
	var startDate string
	var endDate string
	var pending bool
	var balancesOnly bool
	var account string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Pull all accounts, transactions, balances, and holdings into the local SQLite store",
		Long: "Fetch the full SimpleFIN Account Set (every institution in one call) and upsert it into the\n" +
			"local store. Transactions and balances are preserved across syncs, a balance snapshot is\n" +
			"recorded for net-worth trajectory, and categorization rules are applied to new transactions.\n\n" +
			"SimpleFIN allows roughly 24 requests/day and disables tokens on abuse — one sync pulls\n" +
			"everything, so avoid scripting tight loops. A single call is capped at 90 days; the local\n" +
			"store accumulates history across syncs.",
		Example:     "  simplefin-pp-cli sync --since 90d",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			now := time.Now().UTC()
			// Build the date window. Default to 90d (the protocol's max span).
			params := map[string]string{"version": "2"}
			windowDesc := "default (since yesterday)"
			if balancesOnly {
				params["balances-only"] = "1"
			}
			if pending {
				params["pending"] = "1"
			}
			if account != "" {
				params["account"] = account
			}
			if startDate != "" {
				t, err := simplefin.ParseDate(startDate, now)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--start-date: %w", err))
				}
				params["start-date"] = simplefin.EpochParam(t)
				windowDesc = "from " + t.Format("2006-01-02")
			} else if since != "" {
				t, err := simplefin.ParseDate(since, now)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--since: %w", err))
				}
				params["start-date"] = simplefin.EpochParam(t)
				windowDesc = "since " + t.Format("2006-01-02")
			}
			if endDate != "" {
				t, err := simplefin.ParseDate(endDate, now)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--end-date: %w", err))
				}
				params["end-date"] = simplefin.EpochParam(t)
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would GET /accounts %v (%s)\n", params, windowDesc)
				return nil
			}

			// Under live dogfood, narrow the window to keep the matrix fast.
			if cliutil.IsDogfoodEnv() {
				if _, ok := params["start-date"]; !ok {
					params["start-date"] = simplefin.EpochParam(now.Add(-7 * 24 * time.Hour))
					windowDesc = "since " + now.Add(-7*24*time.Hour).Format("2006-01-02") + " (dogfood-curtailed)"
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Fail fast with onboarding guidance when no Access URL is set,
			// rather than sending an unauthenticated request and trying to
			// JSON-parse the HTML error page the bridge returns.
			if c.Config.AuthHeader() == "" {
				return authErr(errNoSimplefinCredentials())
			}
			raw, err := c.GetNoCache(ctx, "/accounts", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			set, err := simplefin.ParseAccountSet(raw)
			if err != nil {
				return err
			}

			if dbPath == "" {
				dbPath = defaultDBPath("simplefin-pp-cli")
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureSimpleFINSchema(ctx); err != nil {
				return fmt.Errorf("ensuring schema: %w", err)
			}

			stats, err := applyAccountSet(ctx, db, set, now)
			if err != nil {
				return err
			}
			stats.Window = windowDesc

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, stats)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d accounts (%s)\n", stats.AccountsSynced, stats.Window)
			fmt.Fprintf(cmd.OutOrStdout(), "  transactions: %d new, %d updated\n", stats.TxnsInserted, stats.TxnsUpdated)
			if stats.HoldingsSynced > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  holdings:     %d\n", stats.HoldingsSynced)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  categorized:  %d new transactions\n", stats.Categorized)
			fmt.Fprintf(cmd.OutOrStdout(), "  balance snapshot recorded at %s\n", time.Unix(stats.SnapshotAt, 0).Format(time.RFC3339))
			for _, m := range stats.APIMessages {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: %s\n", m)
			}
			for _, e := range stats.Errors {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", e)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().StringVar(&since, "since", "90d", "Pull transactions since this date (unix epoch, relative like 90d/4w, or YYYY-MM-DD). Max 90-day span.")
	cmd.Flags().StringVar(&startDate, "start-date", "", "Explicit window start (overrides --since)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "Window end (unix epoch, relative, or YYYY-MM-DD)")
	cmd.Flags().BoolVar(&pending, "pending", true, "Include pending transactions")
	cmd.Flags().BoolVar(&balancesOnly, "balances-only", false, "Fetch balances only, skip transactions (lighter on the rate limit)")
	cmd.Flags().StringVar(&account, "account", "", "Restrict the sync to a single account id")
	return cmd
}

// TxnResourceID namespaces a transaction by account, since SimpleFIN ids are
// only unique within an account ("may reuse transaction ids for different
// accounts"). The generic resources PK is (resource_type, id), so a bare txn id
// would collide across accounts.
func TxnResourceID(accountID, txnID string) string {
	return accountID + "|" + txnID
}

func applyAccountSet(ctx context.Context, db *store.Store, set *simplefin.AccountSet, now time.Time) (*syncStats, error) {
	stats := &syncStats{SnapshotAt: now.Unix()}
	for _, e := range set.Errlist {
		stats.Errors = append(stats.Errors, fmt.Sprintf("%s: %s", e.Code, e.Msg))
	}
	stats.APIMessages = set.XAPIMessage

	for _, conn := range set.Connections {
		if data, err := json.Marshal(conn); err == nil {
			if err := db.Upsert("connections", conn.ConnID, data); err == nil {
				stats.Connections++
			}
		}
	}

	dbh := db.DB()
	for _, acct := range set.Accounts {
		// Account row (without nested arrays) into resources.
		acctData := map[string]any{
			"id":                acct.ID,
			"name":              acct.Name,
			"conn_id":           acct.ConnID,
			"currency":          acct.Currency,
			"balance":           acct.Balance,
			"available_balance": acct.AvailableBalance,
			"balance_date":      acct.BalanceDate,
			"account_type":      inferAccountType(acct),
		}
		if data, err := json.Marshal(acctData); err == nil {
			if err := db.Upsert("accounts", acct.ID, data); err != nil {
				stats.Errors = append(stats.Errors, fmt.Sprintf("account %s: %v", acct.ID, err))
				continue
			}
		}
		stats.AccountsSynced++

		// Balance snapshot (time series the protocol never returns).
		if bal, ok := simplefin.ParseAmount(acct.Balance); ok {
			var avail any
			if a, ok := simplefin.ParseAmount(acct.AvailableBalance); ok {
				avail = a
			}
			if _, err := dbh.ExecContext(ctx,
				`INSERT OR REPLACE INTO balance_snapshots (account_id, snapshot_at, balance, available_balance) VALUES (?, ?, ?, ?)`,
				acct.ID, now.Unix(), bal, avail); err != nil {
				stats.Errors = append(stats.Errors, fmt.Sprintf("snapshot %s: %v", acct.ID, err))
			}
		}

		// Transactions.
		for _, txn := range acct.Transactions {
			rid := TxnResourceID(acct.ID, txn.ID)
			existed := resourceExists(ctx, dbh, "transactions", rid)
			txnData := map[string]any{
				"id":            txn.ID,
				"account_id":    acct.ID,
				"account_name":  acct.Name,
				"posted":        txn.Posted,
				"amount":        txn.Amount,
				"description":   cliutil.CleanText(txn.Description),
				"payee":         cliutil.CleanText(txn.Payee),
				"memo":          cliutil.CleanText(txn.Memo),
				"pending":       txn.Pending,
				"transacted_at": txn.TransactedAt,
			}
			data, err := json.Marshal(txnData)
			if err != nil {
				continue
			}
			if err := db.Upsert("transactions", rid, data); err != nil {
				stats.Errors = append(stats.Errors, fmt.Sprintf("txn %s: %v", rid, err))
				continue
			}
			if existed {
				stats.TxnsUpdated++
			} else {
				stats.TxnsInserted++
			}
		}

		// Holdings.
		for i, h := range acct.Holdings {
			hid := h.ID
			if hid == "" {
				hid = fmt.Sprintf("idx%d", i)
			}
			rid := acct.ID + "|" + hid
			hdata := map[string]any{
				"id":             h.ID,
				"account_id":     acct.ID,
				"account_name":   acct.Name,
				"symbol":         h.Symbol,
				"description":    cliutil.CleanText(h.Description),
				"shares":         h.Shares,
				"cost_basis":     h.CostBasis,
				"purchase_price": h.PurchasePrice,
				"market_value":   h.MarketValue,
				"currency":       h.Currency,
			}
			if data, err := json.Marshal(hdata); err == nil {
				if err := db.Upsert("holdings", rid, data); err == nil {
					stats.HoldingsSynced++
				}
			}
		}
	}

	// Apply categorization rules to newly-uncategorized transactions.
	n, err := applyCategorizationRules(ctx, db)
	if err != nil {
		stats.Errors = append(stats.Errors, fmt.Sprintf("categorize: %v", err))
	}
	stats.Categorized = n

	// Record sync state so freshness hints (hintIfUnsynced/hintIfStale) and
	// 'doctor' know the store has been populated and how recently.
	_ = db.SaveSyncState("accounts", "", stats.AccountsSynced)
	_ = db.SaveSyncState("transactions", "", stats.TxnsInserted+stats.TxnsUpdated)
	if stats.HoldingsSynced > 0 {
		_ = db.SaveSyncState("holdings", "", stats.HoldingsSynced)
	}
	return stats, nil
}

// inferAccountType maps an account to a coarse type so net worth can treat
// credit cards and loans as liabilities. SimpleFIN exposes no type field, so we
// infer from the name (best-effort).
func inferAccountType(acct simplefin.Account) string {
	// Any account that reports investment holdings is an investment account,
	// regardless of name — brokerage naming ("Stock Plan", "JTWROS",
	// "Individual") is too varied to catch by keyword.
	if len(acct.Holdings) > 0 {
		return "investment"
	}
	n := strings.ToLower(acct.Name)
	switch {
	case containsAny(n, "credit", "card", "visa", "mastercard", "amex"):
		return "credit-card"
	case containsAny(n, "mortgage", "loan", "heloc"):
		return "loan"
	case containsAny(n, "401k", "401(k)", "ira", "roth", "brokerage", "invest", "hsa"):
		return "investment"
	case containsAny(n, "saving"):
		return "savings"
	case containsAny(n, "checking", "chequing"):
		return "checking"
	default:
		return "depository"
	}
}

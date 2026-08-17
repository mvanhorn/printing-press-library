// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared helpers for the hand-authored SimpleFIN commands: store access, the
// missing-mirror guard, and typed loaders over the generic resources table.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/store"
)

func lowerName(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// humanMoney formats an amount with thousands separators and 2 decimals for
// human-readable tables ONLY. JSON/CSV paths keep raw float values so machine
// consumers stay parseable.
func humanMoney(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	s := strconv.FormatFloat(v, 'f', 2, 64)
	intPart, frac, _ := strings.Cut(s, ".")
	n := len(intPart)
	var b strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 && (n-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(intPart[i])
	}
	out := b.String() + "." + frac
	if neg {
		out = "-" + out
	}
	return out
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func resourceExists(ctx context.Context, dbh *sql.DB, resourceType, id string) bool {
	var one int
	err := dbh.QueryRowContext(ctx,
		`SELECT 1 FROM resources WHERE resource_type = ? AND id = ?`, resourceType, id).Scan(&one)
	return err == nil
}

// resolveSimplefinDB resolves the db path, checks the mirror exists, and opens
// the store with the SimpleFIN schema ensured. Returns (db, ok). When the
// mirror is missing it prints the sync hint, emits empty JSON for machine
// output, and returns ok=false so the caller returns nil cleanly — a missing
// local mirror is an empty-cache state, not an error.
func resolveSimplefinDB(ctx context.Context, cmd *cobra.Command, flags *rootFlags, dbPath string) (*store.Store, bool, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("simplefin-pp-cli")
	}
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local data at %s\nrun: simplefin-pp-cli sync --since 90d\n", dbPath)
		if flags.asJSON || flags.agent {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return nil, false, nil
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, false, fmt.Errorf("opening database: %w", err)
	}
	if err := db.EnsureSimpleFINSchema(ctx); err != nil {
		_ = db.Close()
		return nil, false, fmt.Errorf("ensuring schema: %w", err)
	}
	return db, true, nil
}

// storedAccount is the decoded shape of a resources row of type "accounts".
type storedAccount struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	ConnID           string `json:"conn_id"`
	Currency         string `json:"currency"`
	Balance          string `json:"balance"`
	AvailableBalance string `json:"available_balance"`
	BalanceDate      int64  `json:"balance_date"`
	AccountType      string `json:"account_type"`
}

// storedTxn is the decoded shape of a resources row of type "transactions".
type storedTxn struct {
	ID           string `json:"id"`
	AccountID    string `json:"account_id"`
	AccountName  string `json:"account_name"`
	Posted       int64  `json:"posted"`
	Amount       string `json:"amount"`
	Description  string `json:"description"`
	Payee        string `json:"payee"`
	Memo         string `json:"memo"`
	Pending      bool   `json:"pending"`
	TransactedAt int64  `json:"transacted_at"`
	// ResourceID is the namespaced (account|txn) primary key; populated by loaders.
	ResourceID string `json:"-"`
}

// storedHolding is the decoded shape of a resources row of type "holdings".
type storedHolding struct {
	ID            string `json:"id"`
	AccountID     string `json:"account_id"`
	AccountName   string `json:"account_name"`
	Symbol        string `json:"symbol"`
	Description   string `json:"description"`
	Shares        string `json:"shares"`
	CostBasis     string `json:"cost_basis"`
	PurchasePrice string `json:"purchase_price"`
	MarketValue   string `json:"market_value"`
	Currency      string `json:"currency"`
}

// loadAccounts returns every stored account (drain-first; closes rows before
// returning so callers may run follow-up queries).
func loadAccounts(ctx context.Context, db *store.Store) ([]storedAccount, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'accounts' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	out := make([]storedAccount, 0)
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			_ = rows.Close()
			return nil, err
		}
		var a storedAccount
		if json.Unmarshal([]byte(data), &a) == nil {
			out = append(out, a)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// loadTransactions returns transactions, optionally filtered. accountFilter
// matches account id OR a case-insensitive substring of the account name; an
// empty filter returns all. minPosted/maxPosted (unix seconds, 0 = unbounded)
// bound the window. includePending controls whether pending rows are returned.
func loadTransactions(ctx context.Context, db *store.Store, accountFilter string, minPosted, maxPosted int64, includePending bool) ([]storedTxn, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT id, data FROM resources WHERE resource_type = 'transactions'`)
	if err != nil {
		return nil, err
	}
	af := strings.ToLower(strings.TrimSpace(accountFilter))
	out := make([]storedTxn, 0)
	for rows.Next() {
		var rid, data string
		if err := rows.Scan(&rid, &data); err != nil {
			_ = rows.Close()
			return nil, err
		}
		var t storedTxn
		if json.Unmarshal([]byte(data), &t) != nil {
			continue
		}
		t.ResourceID = rid
		if !includePending && t.Pending {
			continue
		}
		if minPosted > 0 && t.Posted < minPosted {
			continue
		}
		if maxPosted > 0 && t.Posted >= maxPosted {
			continue
		}
		if af != "" {
			if strings.ToLower(t.AccountID) != af && !strings.Contains(strings.ToLower(t.AccountName), af) {
				continue
			}
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// loadHoldings returns every stored holding.
func loadHoldings(ctx context.Context, db *store.Store) ([]storedHolding, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'holdings' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	out := make([]storedHolding, 0)
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			_ = rows.Close()
			return nil, err
		}
		var h storedHolding
		if json.Unmarshal([]byte(data), &h) == nil {
			out = append(out, h)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// loadCategoryMap returns txn-resource-id -> category for all categorized txns.
func loadCategoryMap(ctx context.Context, db *store.Store) (map[string]string, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT txn_id, category FROM transaction_categories`)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for rows.Next() {
		var id, cat string
		if err := rows.Scan(&id, &cat); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out[id] = cat
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

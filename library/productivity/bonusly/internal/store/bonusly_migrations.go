package store

import (
	"context"
	"database/sql"
)

// EnsureBonuslyBalanceHistory creates the balance_history table used by
// `balance history` if it does not already exist. Called lazily from the
// command, not from the store's own migration list.
func EnsureBonuslyBalanceHistory(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS balance_history (
		recorded_at DATETIME NOT NULL,
		giving_balance INTEGER,
		earning_balance INTEGER,
		monthly_budget INTEGER
	)`)
	return err
}

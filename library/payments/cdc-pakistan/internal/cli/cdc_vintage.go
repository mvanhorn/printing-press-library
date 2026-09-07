// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"

	"github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/internal/store"
)

// newestVintage returns the most recent stored vintage date, or "" when none.
//
// WHY THIS EXISTS. A cross-sectional read that scans EVERY stored vintage
// silently blends them: with 2024-01-31 and 2025-11-30 both loaded, a
// `--min-pct 25` cross-section returned 24 rows for 16 issuers -- each issuer
// once per vintage -- while stamping a single vintage label on the result and
// inflating its own count from 12 to 24. Two vintages of the same security are
// not two securities. Vintage-scoped reads default to the newest one and say so.
func newestVintage(ctx context.Context, db *store.Store) (string, error) {
	var v sql.NullString
	err := db.DB().QueryRowContext(ctx,
		`SELECT MAX(vintage_date) FROM cdc_penetration_rows`).Scan(&v)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}
	if !v.Valid {
		return "", nil
	}
	return v.String, nil
}

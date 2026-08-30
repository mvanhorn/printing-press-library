// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel support for the Phase 3 commands (regen-safe).
// pp:data-source auto
// Supported strategies: auto, local, live, or computed.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/openai-ads/internal/store"
)

// microsPerUnit is the number of micros in one unit of account currency.
// Every monetary field in the OpenAI Ads API is MICROS: 1_000_000 micros = 1
// unit (e.g. 1 MXN). See the Phase 3 brief's MONEY RULE.
const microsPerUnit int64 = 1_000_000

// defaultCurrency is used when the store has no ad-account snapshot to read a
// real currency from (fresh / empty databases, unit tests). The test account
// reports MXN; MXN is a safe neutral default for money rendering.
const defaultCurrency = "MXN"

// renderMicros formats a micros integer as a human amount with the account
// currency suffix, e.g. 15000000 micros => "15.00 MXN". A 0/empty amount
// still renders "0.00 <currency>" so callers never emit a raw integer.
func renderMicros(micros int64, currency string) string {
	if strings.TrimSpace(currency) == "" {
		currency = defaultCurrency
	}
	amount := float64(micros) / float64(microsPerUnit)
	return fmt.Sprintf("%.2f %s", amount, strings.TrimSpace(currency))
}

// renderNullableMicros renders a possibly-NULL micros value. Returns "" when
// the source column was NULL so callers can drop the column cleanly.
func renderNullableMicros(micros sql.NullInt64, currency string) string {
	if !micros.Valid {
		return ""
	}
	return renderMicros(micros.Int64, currency)
}

// accountCurrency returns the account currency cached in the store during
// sync (ad_account.currency_code). When no ad-account snapshot exists it
// falls back to defaultCurrency so money rendering stays deterministic.
func accountCurrency(db *store.Store) string {
	if db == nil {
		return defaultCurrency
	}
	var cur sql.NullString
	err := db.DB().QueryRow(`SELECT currency_code FROM ad_account LIMIT 1`).Scan(&cur)
	if err == nil && cur.Valid && strings.TrimSpace(cur.String) != "" {
		return strings.TrimSpace(cur.String)
	}
	return defaultCurrency
}

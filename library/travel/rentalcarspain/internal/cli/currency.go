// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.
// Indicative EUR→currency display, backed by cached ECB reference rates.

package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/fx"
	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/store"
)

// fxCacheTTL is how long ECB rates are trusted before a refetch. ECB publishes
// once per working day, so half a day keeps quotes current without hammering.
const fxCacheTTL = 12 * time.Hour

// moneyFmt renders EUR amounts, optionally adding an indicative conversion.
type moneyFmt struct {
	currency string   // "" or "EUR" => EUR only
	rates    fx.Rates // loaded when a non-EUR currency is requested
}

// newMoneyFmt builds a formatter for the requested --currency. It loads ECB
// rates (cached, refetched when stale) only when a non-EUR currency is asked
// for. On any failure it silently falls back to EUR-only so pricing still
// works offline; the (possibly nil) warning explains the fallback.
func newMoneyFmt(ctx context.Context, flags *rootFlags) (moneyFmt, error) {
	code := strings.ToUpper(strings.TrimSpace(flags.currency))
	if code == "" || code == "EUR" {
		return moneyFmt{currency: "EUR"}, nil
	}
	rates, err := cachedRates(ctx, flags)
	if err != nil {
		return moneyFmt{currency: "EUR"}, fmt.Errorf("showing EUR only: %w", err)
	}
	if !rates.Supported(code) {
		return moneyFmt{currency: "EUR"}, fmt.Errorf("showing EUR only: unknown currency %q", code)
	}
	return moneyFmt{currency: code, rates: rates}, nil
}

// active reports whether an indicative conversion will be shown.
func (m moneyFmt) active() bool { return m.currency != "" && m.currency != "EUR" }

// format renders an EUR amount. EUR-only: "289.00 EUR". With a target currency:
// "≈ 329.60 USD (289.00 EUR)" — the EUR figure is the amount actually billed.
func (m moneyFmt) format(amountEUR float64) string {
	if !m.active() {
		return fmt.Sprintf("%.2f EUR", amountEUR)
	}
	conv, ok := m.rates.Convert(amountEUR, m.currency)
	if !ok {
		return fmt.Sprintf("%.2f EUR", amountEUR)
	}
	return fmt.Sprintf("≈ %.2f %s (%.2f EUR)", conv, m.currency, amountEUR)
}

// cachedRates returns ECB rates from the local cache, refetching from the ECB
// when the cache is empty or older than fxCacheTTL. A fetch failure falls back
// to whatever is cached (even if stale) so conversions still work offline.
func cachedRates(ctx context.Context, flags *rootFlags) (fx.Rates, error) {
	db, err := store.OpenWithContext(ctx, defaultDBPath("rentalcarspain-pp-cli"))
	if err == nil {
		defer db.Close()
		cached, cerr := db.GetFXRates(ctx)
		fresh := cerr == nil && cached.Rates != nil && time.Since(cached.UpdatedAt) < fxCacheTTL
		if fresh && !flags.noCache {
			return fx.Rates{Date: cached.Date, Rate: cached.Rates}, nil
		}
		// Stale or empty: try a refetch, else fall back to whatever we have.
		if live, ferr := fx.FetchECB(ctx, carHTTPClient(flags)); ferr == nil {
			_ = db.UpsertFXRates(ctx, live.Date, live.Rate)
			return live, nil
		} else if cached.Rates != nil {
			return fx.Rates{Date: cached.Date, Rate: cached.Rates}, nil
		} else {
			return fx.Rates{}, ferr
		}
	}
	// No store: fetch directly.
	return fx.FetchECB(ctx, carHTTPClient(flags))
}

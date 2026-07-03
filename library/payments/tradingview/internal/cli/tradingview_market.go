// Copyright 2026 jbriaux and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored TradingView market helpers shared by the lookup, quote, and
// convert commands. Not generated; safe from regen overwrite.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"tradingview-pp-cli/internal/client"
	"tradingview-pp-cli/internal/cliutil"
)

const (
	tvSymbolSearchURL = "https://symbol-search.tradingview.com/symbol_search/v3/"
	tvScannerSymbol   = "/symbol" // relative to scanner.tradingview.com base
)

// tvSymbolMatch is one candidate from the symbol-search endpoint.
type tvSymbolMatch struct {
	Symbol      string `json:"symbol"`      // fully-qualified EXCHANGE:TICKER
	Ticker      string `json:"ticker"`      // raw ticker
	Description string `json:"description"` // human name, e.g. "Apple Inc."
	Type        string `json:"type"`        // stock, spot, forex, index, ...
	Exchange    string `json:"exchange"`    // NASDAQ, BINANCE, ...
	Currency    string `json:"currency"`    // quote currency, e.g. USD
	Country     string `json:"country"`     // ISO country code
	Primary     bool   `json:"primary"`     // is_primary_listing
}

// tvRawQuote is the flat object returned by scanner /symbol.
type tvRawQuote struct {
	Close       float64 `json:"close"`
	Currency    string  `json:"currency"`
	Change      float64 `json:"change"`
	Description string  `json:"description"`
	Type        string  `json:"type"`
}

// stripEm removes TradingView's <em>...</em> match-highlight markup and
// decodes any HTML entities.
func stripEm(s string) string {
	s = strings.ReplaceAll(s, "<em>", "")
	s = strings.ReplaceAll(s, "</em>", "")
	return cliutil.CleanText(s)
}

// searchSymbols queries the symbol-search endpoint and returns cleaned
// candidates. typeFilter and exchange are optional server-side filters.
func searchSymbols(ctx context.Context, c *client.Client, query, typeFilter, exchange string, limit int) ([]tvSymbolMatch, error) {
	params := map[string]string{
		"text":   query,
		"hl":     "0",
		"lang":   "en",
		"domain": "production",
	}
	if typeFilter != "" {
		params["search_type"] = typeFilter
	}
	if exchange != "" {
		params["exchange"] = exchange
	}
	data, err := c.Get(ctx, tvSymbolSearchURL, params)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Symbols []struct {
			Symbol       string `json:"symbol"`
			Description  string `json:"description"`
			Type         string `json:"type"`
			Exchange     string `json:"exchange"`
			Prefix       string `json:"prefix"`
			CurrencyCode string `json:"currency_code"`
			Country      string `json:"country"`
			IsPrimary    bool   `json:"is_primary_listing"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing symbol search response: %w", err)
	}
	matches := make([]tvSymbolMatch, 0, len(raw.Symbols))
	for _, s := range raw.Symbols {
		ticker := stripEm(s.Symbol)
		if ticker == "" {
			continue
		}
		exch := s.Exchange
		if s.Prefix != "" {
			exch = s.Prefix
		}
		fq := ticker
		if exch != "" {
			fq = exch + ":" + ticker
		}
		matches = append(matches, tvSymbolMatch{
			Symbol:      fq,
			Ticker:      ticker,
			Description: stripEm(s.Description),
			Type:        s.Type,
			Exchange:    s.Exchange,
			Currency:    s.CurrencyCode,
			Country:     s.Country,
			Primary:     s.IsPrimary,
		})
		if limit > 0 && len(matches) >= limit {
			break
		}
	}
	return matches, nil
}

// resolveSymbol turns a user-supplied query into a fully-qualified
// EXCHANGE:TICKER. If the query already contains a colon it is returned
// as-is (uppercased). Otherwise it searches and returns the best candidate,
// preferring the primary listing.
func resolveSymbol(ctx context.Context, c *client.Client, query, typeFilter string) (string, []tvSymbolMatch, error) {
	if strings.Contains(query, ":") {
		return strings.ToUpper(query), nil, nil
	}
	matches, err := searchSymbols(ctx, c, query, typeFilter, "", 10)
	if err != nil {
		return "", nil, err
	}
	if len(matches) == 0 {
		return "", nil, fmt.Errorf("no symbol found for %q", query)
	}
	// Prefer a primary listing; otherwise take the first match.
	best := matches[0]
	for _, m := range matches {
		if m.Primary {
			best = m
			break
		}
	}
	return best.Symbol, matches, nil
}

// fetchRawQuote fetches the flat price object for a fully-qualified ticker.
// Returns (quote, found, error). found is false when the symbol resolves but
// carries no price (empty response from no_404).
func fetchRawQuote(ctx context.Context, c *client.Client, fqSymbol string) (tvRawQuote, bool, error) {
	params := map[string]string{
		"symbol": fqSymbol,
		"fields": "close,currency,change,description,type",
		"no_404": "true",
	}
	// Use the no-cache path: prices are the whole point of this CLI, and the
	// generated client's 5-minute GET cache would otherwise serve a stale
	// "last price". GetNoCache still refreshes the cache on success.
	data, err := c.GetNoCache(ctx, tvScannerSymbol, params)
	if err != nil {
		return tvRawQuote{}, false, err
	}
	var q tvRawQuote
	if err := json.Unmarshal(data, &q); err != nil {
		return tvRawQuote{}, false, fmt.Errorf("parsing quote response: %w", err)
	}
	if q.Close == 0 && q.Currency == "" {
		return tvRawQuote{}, false, nil
	}
	return q, true, nil
}

// isUSDLike reports whether a currency should be treated as 1:1 with USD.
// TradingView prices most crypto in USDT/USDC; both are USD-pegged stablecoins.
func isUSDLike(cur string) bool {
	switch strings.ToUpper(cur) {
	case "USD", "USDT", "USDC", "DAI", "BUSD", "TUSD":
		return true
	}
	return false
}

// fetchForexRate returns the number of `to` units per 1 `from` unit using
// TradingView's FX_IDC forex symbols. Returns (rate, via, ok, error) where
// via names the source symbol used (for transparency).
func fetchForexRate(ctx context.Context, c *client.Client, from, to string) (float64, string, bool, error) {
	from = strings.ToUpper(from)
	to = strings.ToUpper(to)
	if from == to {
		return 1, "identity", true, nil
	}
	// USD-pegged stablecoins convert 1:1 with USD for the peg side.
	if isUSDLike(from) && isUSDLike(to) {
		return 1, "usd-peg", true, nil
	}
	fromU := from
	if isUSDLike(from) {
		fromU = "USD"
	}
	toU := to
	if isUSDLike(to) {
		toU = "USD"
	}
	if fromU == toU {
		return 1, "usd-peg", true, nil
	}
	// Direct pair: FX_IDC:<FROM><TO> close = TO per 1 FROM.
	direct := "FX_IDC:" + fromU + toU
	if q, found, err := fetchRawQuote(ctx, c, direct); err != nil {
		return 0, "", false, err
	} else if found && q.Close > 0 {
		return q.Close, direct, true, nil
	}
	// Inverse pair: FX_IDC:<TO><FROM> close = FROM per 1 TO -> invert.
	inverse := "FX_IDC:" + toU + fromU
	if q, found, err := fetchRawQuote(ctx, c, inverse); err != nil {
		return 0, "", false, err
	} else if found && q.Close > 0 {
		return 1 / q.Close, inverse + " (inverted)", true, nil
	}
	// Cross via USD: FROM->USD then USD->TO.
	if fromU != "USD" && toU != "USD" {
		fu, via1, ok1, err := fetchForexRate(ctx, c, fromU, "USD")
		if err != nil {
			return 0, "", false, err
		}
		ut, via2, ok2, err := fetchForexRate(ctx, c, "USD", toU)
		if err != nil {
			return 0, "", false, err
		}
		if ok1 && ok2 {
			return fu * ut, via1 + " x " + via2, true, nil
		}
	}
	return 0, "", false, nil
}

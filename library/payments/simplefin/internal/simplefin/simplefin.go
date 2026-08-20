// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// SimpleFIN protocol domain types + claim flow. Spec: https://www.simplefin.org/protocol.html

// Package simplefin holds the SimpleFIN v2 protocol data model, the setup-token
// claim flow, and shared parsing helpers used by the hand-authored commands.
package simplefin

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/cliutil"
)

// AccountSet is the JSON returned by GET /accounts — the entire financial
// picture across every connected institution in one response.
type AccountSet struct {
	Errlist     []APIError   `json:"errlist"`
	Connections []Connection `json:"connections"`
	Accounts    []Account    `json:"accounts"`
	XAPIMessage []string     `json:"x-api-message,omitempty"`
}

// APIError is a structured error from the errlist array.
type APIError struct {
	Code      string `json:"code"`
	Msg       string `json:"msg"`
	ConnID    string `json:"conn_id,omitempty"`
	AccountID string `json:"account_id,omitempty"`
}

// Connection is a single login to an institution (v2 flattened Organization).
type Connection struct {
	ConnID  string `json:"conn_id"`
	Name    string `json:"name"`
	OrgID   string `json:"org_id"`
	OrgName string `json:"org_name,omitempty"`
	OrgURL  string `json:"org_url,omitempty"`
	SfinURL string `json:"sfin_url,omitempty"`
}

// Account is a single account with its balance, transactions, and holdings.
type Account struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	ConnID           string         `json:"conn_id,omitempty"`
	Currency         string         `json:"currency"`
	Balance          string         `json:"balance"`
	AvailableBalance string         `json:"available-balance,omitempty"`
	BalanceDate      int64          `json:"balance-date"`
	Transactions     []Transaction  `json:"transactions,omitempty"`
	Holdings         []Holding      `json:"holdings,omitempty"`
	Extra            map[string]any `json:"extra,omitempty"`
	// Org is the deprecated v1 organization object, retained as a fallback for
	// older SimpleFIN servers that have not adopted the v2 connections list.
	Org map[string]any `json:"org,omitempty"`
}

// Transaction is one posted or pending transaction. payee/memo are bridge
// extensions beyond the bare protocol.
type Transaction struct {
	ID           string         `json:"id"`
	Posted       int64          `json:"posted"`
	Amount       string         `json:"amount"`
	Description  string         `json:"description"`
	Payee        string         `json:"payee,omitempty"`
	Memo         string         `json:"memo,omitempty"`
	TransactedAt int64          `json:"transacted_at,omitempty"`
	Pending      bool           `json:"pending,omitempty"`
	Extra        map[string]any `json:"extra,omitempty"`
}

// Holding is an investment position (SimpleFIN Bridge extension).
type Holding struct {
	ID            string `json:"id"`
	Created       int64  `json:"created,omitempty"`
	CostBasis     string `json:"cost_basis,omitempty"`
	Currency      string `json:"currency,omitempty"`
	Description   string `json:"description,omitempty"`
	MarketValue   string `json:"market_value,omitempty"`
	PurchasePrice string `json:"purchase_price,omitempty"`
	Shares        string `json:"shares,omitempty"`
	Symbol        string `json:"symbol,omitempty"`
}

// ParseAccountSet decodes a raw /accounts response.
func ParseAccountSet(data []byte) (*AccountSet, error) {
	var set AccountSet
	if err := json.Unmarshal(data, &set); err != nil {
		return nil, fmt.Errorf("parsing SimpleFIN account set: %w", err)
	}
	return &set, nil
}

// ClaimSetupToken decodes a base64 SimpleFIN setup token to a claim URL, POSTs
// to it, and returns the resulting Access URL (with embedded Basic-auth creds).
// Per the protocol this is one-time-use: a 403 means the token was already
// claimed or is invalid, which may indicate it was compromised.
func ClaimSetupToken(ctx context.Context, setupToken string, timeout time.Duration) (string, error) {
	setupToken = strings.TrimSpace(setupToken)
	if setupToken == "" {
		return "", fmt.Errorf("empty setup token")
	}
	claimURL, err := decodeSetupToken(setupToken)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(claimURL)
	if err != nil || u.Scheme != "https" {
		return "", fmt.Errorf("decoded claim URL must be https, got %q", claimURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, claimURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Length", "0")
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return "", fmt.Errorf("claiming setup token: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("claim failed (403): this setup token was already claimed or is invalid. If you did not just claim it, the token may be compromised — generate a new one")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("claim failed: HTTP %d", resp.StatusCode)
	}
	accessURL := strings.TrimSpace(string(body))
	if !strings.HasPrefix(accessURL, "https://") {
		return "", fmt.Errorf("claim response was not an https Access URL")
	}
	return accessURL, nil
}

func decodeSetupToken(setupToken string) (string, error) {
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if decoded, err := enc.DecodeString(setupToken); err == nil {
			s := strings.TrimSpace(string(decoded))
			if strings.HasPrefix(s, "https://") {
				return s, nil
			}
		}
	}
	return "", fmt.Errorf("setup token is not a valid base64-encoded https URL")
}

// ParseAmount parses a SimpleFIN numeric string (e.g. "-33293.43") to a float.
func ParseAmount(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

var (
	digitRun   = regexp.MustCompile(`\d+`)
	nonWord    = regexp.MustCompile(`[^a-z0-9 ]+`)
	whitespace = regexp.MustCompile(`\s+`)
)

// NormalizePayee produces a stable merchant key from a transaction: prefers the
// payee, falls back to the description, lowercases, strips digit runs (order
// numbers, store numbers) and punctuation. Used for recurring detection and
// merchant grouping.
func NormalizePayee(t Transaction) string {
	base := t.Payee
	if strings.TrimSpace(base) == "" {
		base = t.Description
	}
	base = cliutil.CleanText(base)
	base = strings.ToLower(base)
	base = digitRun.ReplaceAllString(base, "")
	base = nonWord.ReplaceAllString(base, " ")
	base = whitespace.ReplaceAllString(base, " ")
	return strings.TrimSpace(base)
}

// ContentHash returns a content-based dedup key (amount + posted day +
// normalized description). SimpleFIN transaction IDs are unstable and mirrored
// across accounts, so identity-based dedup misses real duplicates; content
// hashing catches them. Account id is intentionally excluded so the same charge
// mirrored into multiple accounts collides.
func ContentHash(t Transaction) string {
	day := time.Unix(t.Posted, 0).UTC().Format("2006-01-02")
	desc := NormalizePayee(t)
	h := sha256.Sum256([]byte(t.Amount + "|" + day + "|" + desc))
	return fmt.Sprintf("%x", h[:8])
}

// ParseDate accepts a unix-epoch integer, a relative duration (30d, 1w, 24h),
// or a YYYY-MM-DD date and returns the corresponding time. now is used as the
// anchor for relative durations.
func ParseDate(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	// Unix epoch seconds.
	if epoch, err := strconv.ParseInt(s, 10, 64); err == nil && len(s) >= 6 {
		return time.Unix(epoch, 0).UTC(), nil
	}
	// Relative duration (supports d/w via ParseDurationLoose).
	if d, err := cliutil.ParseDurationLoose(s); err == nil {
		return now.Add(-d), nil
	}
	// Absolute date.
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("could not parse date %q (use unix epoch, relative like 30d/1w, or YYYY-MM-DD)", s)
}

// EpochParam formats a time as the unix-second string SimpleFIN expects.
func EpochParam(t time.Time) string {
	return strconv.FormatInt(t.Unix(), 10)
}

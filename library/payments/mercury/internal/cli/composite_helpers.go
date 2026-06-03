// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/mercury/internal/client"
)

// flexFloat decodes a JSON value that the Mercury API may serialize as either a
// number or a string (transaction amounts have appeared in both shapes across
// endpoints). It accepts either so the composites never panic on a shape change.
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("flexFloat: cannot parse %q", s)
	}
	*f = flexFloat(v)
	return nil
}

// mercuryAccount is the subset of an /accounts row the composites read.
type mercuryAccount struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Nickname       string  `json:"nickname"`
	CurrentBalance float64 `json:"currentBalance"`
	Status         string  `json:"status"`
	Kind           string  `json:"kind"`
}

// mercuryTxn is the subset of a transaction the composites read. Amount is
// signed: negative is money leaving the account (outflow), positive is inflow.
type mercuryTxn struct {
	ID               string    `json:"id"`
	Amount           flexFloat `json:"amount"`
	CreatedAt        string    `json:"createdAt"`
	PostedAt         string    `json:"postedAt"`
	Status           string    `json:"status"`
	Kind             string    `json:"kind"`
	CounterpartyName string    `json:"counterpartyName"`
	MercuryCategory  string    `json:"mercuryCategory"`
	Note             string    `json:"note"`
}

// fetchMercuryList runs a paginated read through the CLI's existing read
// plumbing and decodes the unwrapped array into T. Read-only; adds no new HTTP.
func fetchMercuryList[T any](ctx context.Context, c *client.Client, flags *rootFlags, resource, path string, params map[string]string, cursorParam string) ([]T, error) {
	data, _, err := resolvePaginatedRead(ctx, c, flags, resource, path, params, nil, true, cursorParam, "", "")
	if err != nil {
		return nil, err
	}
	data = extractResponseData(data)
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", resource, err)
	}
	return items, nil
}

// fetchAccounts lists accounts, optionally narrowed to a single account ID.
func fetchAccounts(ctx context.Context, c *client.Client, flags *rootFlags, accountID string) ([]mercuryAccount, error) {
	accounts, err := fetchMercuryList[mercuryAccount](ctx, c, flags, "accounts", "/accounts", map[string]string{
		"limit": "500",
		"order": "asc",
	}, "")
	if err != nil {
		return nil, err
	}
	if accountID == "" {
		return accounts, nil
	}
	for _, a := range accounts {
		if a.ID == accountID {
			return []mercuryAccount{a}, nil
		}
	}
	return nil, fmt.Errorf("account %q not found", accountID)
}

// fetchAccountTxns lists every transaction for an account within [start, end]
// (date strings, empty to omit), paging through all results.
func fetchAccountTxns(ctx context.Context, c *client.Client, flags *rootFlags, accountID, start, end string) ([]mercuryTxn, error) {
	path := replacePathParam("/account/{accountId}/transactions", "accountId", accountID)
	params := map[string]string{
		"limit": "500",
		"order": "desc",
	}
	if start != "" {
		params["start"] = start
	}
	if end != "" {
		params["end"] = end
	}
	return fetchMercuryList[mercuryTxn](ctx, c, flags, "transactions", path, params, "offset")
}

// parseTxnTime parses a transaction timestamp, preferring postedAt and falling
// back to createdAt. Returns false when neither parses.
func parseTxnTime(t mercuryTxn) (time.Time, bool) {
	for _, s := range []string{t.PostedAt, t.CreatedAt} {
		if s == "" {
			continue
		}
		if parsed, err := time.Parse(time.RFC3339, s); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

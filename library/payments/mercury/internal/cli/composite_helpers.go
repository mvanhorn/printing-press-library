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

// mercuryPageSize is the per-page limit the composites request. Mercury caps a
// single accounts/transactions page at 500, so a larger limit is truncated
// server-side — paginate instead.
const mercuryPageSize = 500

// decodeMercuryPage decodes one page of a Mercury list response into T. It
// accepts the live envelope (`{"<field>":[...], "page":{"nextPage":...}}`) and
// the bare-array shape the local store returns, so the composites work under any
// --data-source. nextCursor is the page.nextPage value (empty when the response
// carries no further page).
func decodeMercuryPage[T any](data json.RawMessage, field string) (items []T, nextCursor string, err error) {
	// Bare array (local store, or a top-level-array endpoint).
	var bare []T
	if json.Unmarshal(data, &bare) == nil {
		return bare, "", nil
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, "", fmt.Errorf("decoding %s page: %w", field, err)
	}
	if raw, ok := env[field]; ok && len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, "", fmt.Errorf("decoding %s: %w", field, err)
		}
	}
	if pageRaw, ok := env["page"]; ok {
		var p struct {
			NextPage string `json:"nextPage"`
		}
		_ = json.Unmarshal(pageRaw, &p)
		nextCursor = p.NextPage
	}
	return items, nextCursor, nil
}

// pageMercuryList drives the pagination loop against an injected page getter so
// the control flow is unit-testable without a live client. Mercury exposes two
// pagination styles: a `page.nextPage` cursor (accounts, threaded through
// cursorParam="start_after") and offset/limit (transactions, cursorParam=
// "offset"). When the response carries a cursor it is followed; otherwise the
// reader advances by offset until a short page signals the end. singlePage stops
// after the first page (the local store returns everything at once).
func pageMercuryList[T any](field, cursorParam string, pageSize int, params map[string]string, singlePage bool, get func(map[string]string) (json.RawMessage, error)) ([]T, error) {
	page := map[string]string{}
	for k, v := range params {
		page[k] = v
	}
	if _, ok := page["limit"]; !ok {
		page["limit"] = strconv.Itoa(pageSize)
	}

	var out []T
	offset := 0
	lastCursor := ""
	for {
		data, err := get(page)
		if err != nil {
			return nil, err
		}
		items, nextCursor, derr := decodeMercuryPage[T](data, field)
		if derr != nil {
			return nil, derr
		}
		out = append(out, items...)

		if singlePage || len(items) == 0 {
			return out, nil
		}
		switch {
		case nextCursor != "" && cursorParam != "":
			if nextCursor == lastCursor {
				return out, nil // cursor not advancing — stop rather than loop forever
			}
			lastCursor = nextCursor
			page[cursorParam] = nextCursor
		case cursorParam == "offset":
			if len(items) < pageSize {
				return out, nil // short page — last one
			}
			offset += pageSize
			page["offset"] = strconv.Itoa(offset)
		default:
			return out, nil // no cursor available — single page only
		}
	}
}

// fetchMercuryList pages every item from a Mercury list endpoint whose array is
// nested under `field`, routing each page through resolveRead so --data-source
// still applies. Read-only; adds no new HTTP surface.
func fetchMercuryList[T any](ctx context.Context, c *client.Client, flags *rootFlags, resource, path, field string, params map[string]string, cursorParam string) ([]T, error) {
	get := func(p map[string]string) (json.RawMessage, error) {
		data, _, err := resolveRead(ctx, c, flags, resource, true, path, p, nil)
		return data, err
	}
	return pageMercuryList[T](field, cursorParam, mercuryPageSize, params, flags.dataSource == "local", get)
}

// fetchAccounts lists every account, optionally narrowed to a single account ID.
func fetchAccounts(ctx context.Context, c *client.Client, flags *rootFlags, accountID string) ([]mercuryAccount, error) {
	accounts, err := fetchMercuryList[mercuryAccount](ctx, c, flags, "accounts", "/accounts", "accounts", map[string]string{
		"order": "asc",
	}, "start_after")
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
		"order": "desc",
	}
	if start != "" {
		params["start"] = start
	}
	if end != "" {
		params["end"] = end
	}
	return fetchMercuryList[mercuryTxn](ctx, c, flags, "transactions", path, "transactions", params, "offset")
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

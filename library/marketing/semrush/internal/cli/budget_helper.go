// Copyright 2026 charles-garrison. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel feature support. NOT generator-emitted.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/semrush/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/semrush/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/semrush/internal/store"
)

// balanceProbePath is the free Semrush API-units balance endpoint. The
// generated client treats this absolute URL path verbatim — see
// internal/client/client.go where target URL composition skips the
// BaseURL concat for URLs that already start with https://.
const balanceProbePath = "https://www.semrush.com/users/countapiunits.html"

// recordBalanceSnapshot calls the free balance endpoint and inserts a
// (ts, command, units_remaining) row into credit_log. Best-effort: the
// caller should treat any error as a soft warning, not a hard failure.
//
// Skipped automatically when running under verify or dogfood, so test
// pipelines don't make extra API calls.
//
// The endpoint returns the unit count as a plain integer body (no JSON
// envelope), but the generated client always parses through JSON. So we
// accept both shapes: a bare integer body and an envelope/array.
func recordBalanceSnapshot(ctx context.Context, db *store.Store, c *client.Client, commandPath string) error {
	if db == nil || c == nil {
		return nil
	}
	if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
		return nil
	}
	if err := db.EnsureNovelTables(ctx); err != nil {
		return fmt.Errorf("ensure novel tables: %w", err)
	}
	data, err := c.Get(ctx, balanceProbePath, nil)
	if err != nil {
		return fmt.Errorf("balance probe: %w", err)
	}
	units, ok := parseBalanceUnits(data)
	if !ok {
		return fmt.Errorf("balance probe: could not parse units from response: %s", strings.TrimSpace(string(data)))
	}
	_, err = db.DB().ExecContext(ctx,
		`INSERT INTO credit_log (command, units_remaining, balance_source) VALUES (?, ?, 'api')`,
		commandPath, units,
	)
	if err != nil {
		return fmt.Errorf("insert credit_log: %w", err)
	}
	return nil
}

// parseBalanceUnits extracts the integer units-remaining count from a
// balance-endpoint response. The endpoint returns a bare integer body
// in production, but the generated client always wraps responses in
// JSON envelopes. We handle: bare integer, JSON number, single-element
// JSON array of numbers, JSON envelope with a "units" or "data" key.
func parseBalanceUnits(data json.RawMessage) (int64, bool) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return 0, false
	}
	if n, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return n, true
	}
	// Quoted JSON string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
			return n, true
		}
	}
	// JSON array — first element
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err == nil && len(arr) > 0 {
		return parseBalanceUnits(arr[0])
	}
	// JSON object — look for a units / data / value key
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err == nil {
		for _, k := range []string{"units", "units_remaining", "balance", "data", "value"} {
			if raw, ok := obj[k]; ok {
				if n, parsed := parseBalanceUnits(raw); parsed {
					return n, true
				}
			}
		}
	}
	return 0, false
}

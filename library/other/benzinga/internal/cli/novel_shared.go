// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared helpers for the hand-built cross-entity novel commands (watch, why,
// catalysts, analyst-accuracy, earnings-season, insider-cluster). These all read
// the local SQLite mirror and join across resource types the REST API cannot
// combine in one call.
//
// pp:data-source local

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/benzinga/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/benzinga/internal/store"

	"github.com/spf13/cobra"
)

// novelStore opens the local store for a novel command. When the DB file does
// not exist yet, it prints a sync hint (and an empty machine result) and returns
// ok=false so the caller can `return nil` cleanly — a missing mirror is an empty
// local-cache state, not a usage or API failure.
func novelStore(cmd *cobra.Command, flags *rootFlags, dbPath, syncHint string) (*store.Store, bool, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("benzinga-pp-cli")
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: %s\n", dbPath, syncHint)
		if wantsMachineOutput(flags) {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return nil, false, nil
	}
	// The novel commands are all read-only (mcp:read-only) and only SELECT from
	// the mirror, so open read-only: this enforces the contract at the driver
	// level, skips the schema-migration write path on a pure read, and avoids
	// holding the WAL write lock that a concurrent `sync` would block on.
	db, err := store.OpenReadOnlyContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, false, fmt.Errorf("opening local database: %w", err)
	}
	return db, true, nil
}

// novelRows returns parsed JSON data rows for the given resource types using the
// drain-first pattern (scan + close before any follow-up query).
func novelRows(ctx context.Context, db *store.Store, resourceTypes ...string) ([]map[string]any, error) {
	if len(resourceTypes) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(resourceTypes))
	args := make([]any, len(resourceTypes))
	for i, rt := range resourceTypes {
		placeholders[i] = "?"
		args[i] = rt
	}
	query := "SELECT data FROM resources WHERE resource_type IN (" + strings.Join(placeholders, ",") + ")"
	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	raws := make([][]byte, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			_ = rows.Close()
			return nil, err
		}
		raws = append(raws, append([]byte(nil), data...))
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(raws))
	for _, b := range raws {
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

// novelStr coerces a JSON value to a trimmed string.
func novelStr(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// novelNested extracts a string from a nested object field, e.g. security.ticker.
func novelNested(m map[string]any, parent, child string) string {
	if obj, ok := m[parent].(map[string]any); ok {
		return novelStr(obj, child)
	}
	return ""
}

// novelFloat parses a possibly-string numeric field; ok=false on empty/unparseable.
func novelFloat(m map[string]any, key string) (float64, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return t, true
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

// normTicker strips a leading $ and upper-cases.
func normTicker(s string) string {
	return strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(s), "$"))
}

// novelTickerSet builds an upper-cased ticker set from positional args plus a
// comma-separated flag. An empty result means "no ticker filter".
func novelTickerSet(args []string, flag string) map[string]bool {
	set := map[string]bool{}
	add := func(csv string) {
		for _, t := range strings.Split(csv, ",") {
			if t = normTicker(t); t != "" {
				set[t] = true
			}
		}
	}
	for _, a := range args {
		add(a)
	}
	if flag != "" {
		add(flag)
	}
	return set
}

// novelDur parses a loose duration ("7d", "24h", "1w"); empty returns def.
func novelDur(s string, def time.Duration) (time.Duration, error) {
	if strings.TrimSpace(s) == "" {
		return def, nil
	}
	return cliutil.ParseDurationLoose(s)
}

// novelNewsTickers extracts tickers from a news row's stocks[] array.
func novelNewsTickers(m map[string]any) []string {
	out := []string{}
	if arr, ok := m["stocks"].([]any); ok {
		for _, it := range arr {
			if mm, ok := it.(map[string]any); ok {
				if name, _ := mm["name"].(string); name != "" {
					out = append(out, normTicker(name))
				}
			}
		}
	}
	return out
}

var novelTimeLayouts = []string{
	"Mon, 02 Jan 2006 15:04:05 -0700", // news created/updated (RFC1123Z)
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// novelEventTime derives a best-effort timestamp for a row across the resource
// shapes: numeric `updated` (unix), `created`/`updated` RFC822 strings, or
// `date`(+`time`). Returns the zero time if nothing parses.
func novelEventTime(m map[string]any) time.Time {
	switch t := m["updated"].(type) {
	case float64:
		if t > 1e12 {
			return time.UnixMilli(int64(t)).UTC()
		}
		if t > 1e9 {
			return time.Unix(int64(t), 0).UTC()
		}
	case string:
		for _, layout := range novelTimeLayouts {
			if ts, err := time.Parse(layout, t); err == nil {
				return ts.UTC()
			}
		}
		if n, err := strconv.ParseInt(t, 10, 64); err == nil && n > 1e9 {
			return time.Unix(n, 0).UTC()
		}
	}
	if s := novelStr(m, "created"); s != "" {
		if ts, err := time.Parse(novelTimeLayouts[0], s); err == nil {
			return ts.UTC()
		}
	}
	d := novelStr(m, "date")
	if d == "" {
		d = novelStr(m, "transaction_date")
	}
	if d != "" {
		if tm := novelStr(m, "time"); tm != "" {
			if ts, err := time.Parse("2006-01-02 15:04:05", d+" "+tm); err == nil {
				return ts
			}
		}
		if ts, err := time.Parse("2006-01-02", d); err == nil {
			return ts
		}
	}
	return time.Time{}
}

// novelEmit writes results as filtered JSON for machine output, or runs the
// provided human renderer otherwise. results must be a slice for the empty-state
// handling to read naturally.
func novelEmit(cmd *cobra.Command, flags *rootFlags, results any, human func()) error {
	if wantsMachineOutput(flags) {
		return printJSONFiltered(cmd.OutOrStdout(), results, flags)
	}
	human()
	return nil
}

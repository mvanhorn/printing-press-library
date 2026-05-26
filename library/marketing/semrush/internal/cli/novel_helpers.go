// Copyright 2026 charles-garrison. Licensed under Apache-2.0. See LICENSE.
//
// Shared helpers for the hand-authored novel feature commands
// (drift / snapshot / budget / keyword-gap / backlink-gap / audit-triage /
//  tracking-drift / domain-regions / backlink-new / serp-features /
//  cannibalization / audit-regression).
//
// NOT generator-emitted — survives regen because it lives in a hand-named
// file with no DO NOT EDIT header.

package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/semrush/internal/store"
)

// openNovelStore opens the local store and ensures the novel-feature
// tables (snapshot_labels, credit_log) exist. Returns a store handle the
// caller must Close.
func openNovelStore(ctx context.Context) (*store.Store, error) {
	db, err := store.OpenWithContext(ctx, defaultDBPath("semrush-pp-cli"))
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w", err)
	}
	if err := db.EnsureNovelTables(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ensuring novel tables: %w", err)
	}
	return db, nil
}

// parseSince parses a duration string accepting the same shorthand the
// Cobra --since flags use (1h, 30m, 7d, 30d, 12w). Go's time.ParseDuration
// stops at "h", so we extend it to days and weeks.
func parseSince(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	// Handle "Nd" and "Nw" suffixes Go's standard parser rejects.
	if l := len(s); l > 1 {
		last := s[l-1]
		if last == 'd' || last == 'w' {
			n, err := strconv.Atoi(s[:l-1])
			if err != nil {
				return 0, fmt.Errorf("invalid duration %q: %w", s, err)
			}
			unit := 24 * time.Hour
			if last == 'w' {
				unit = 7 * 24 * time.Hour
			}
			return time.Duration(n) * unit, nil
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	return d, nil
}

// recordBalanceSnapshotForCmd is the per-command wrapper around
// recordBalanceSnapshot that obeys the verify/dogfood/no-client gates and
// logs failures to stderr without blocking the command. Call this at the
// START of every novel command's RunE.
func recordBalanceSnapshotForCmd(ctx context.Context, db *store.Store, flags *rootFlags, commandPath string, stderr interface{ Write(p []byte) (int, error) }) {
	c, err := flags.newClient()
	if err != nil {
		// no client (e.g. missing config) — skip silently
		return
	}
	if err := recordBalanceSnapshot(ctx, db, c, commandPath); err != nil {
		fmt.Fprintf(stderr, "warning: balance snapshot failed: %v\n", err)
	}
}

// parseSemrushCSV parses a v3 Analytics-style semicolon-delimited CSV
// response into a slice of column-keyed rows. The first line is the
// header. Empty input or a header-only response returns nil. Numeric
// values are returned as JSON numbers when they parse cleanly; otherwise
// values are returned as strings (preserving original Semrush text).
//
// Lives here because v3 Analytics responses are CSV — the generated
// `resolveRead` helper wraps these in a per-endpoint envelope, but novel
// commands that call `c.Get` directly need to parse the response
// themselves.
func parseSemrushCSV(raw string) []map[string]any {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.Trim(raw, "\n ")
	if raw == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	if len(lines) < 2 {
		return nil
	}
	headers := strings.Split(lines[0], ";")
	for i, h := range headers {
		headers[i] = strings.TrimSpace(h)
	}
	out := make([]map[string]any, 0, len(lines)-1)
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, ";")
		row := make(map[string]any, len(headers))
		for i, h := range headers {
			if i >= len(fields) {
				row[h] = ""
				continue
			}
			v := strings.TrimSpace(fields[i])
			if v == "" {
				row[h] = ""
				continue
			}
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				row[h] = n
			} else if f, err := strconv.ParseFloat(v, 64); err == nil {
				row[h] = f
			} else {
				row[h] = v
			}
		}
		out = append(out, row)
	}
	return out
}

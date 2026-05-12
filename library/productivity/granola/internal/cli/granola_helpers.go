// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/store"
)

// openGranolaCache loads the local cache file. Returns a typed error if
// the file is missing so commands can surface a helpful message.
//
// PATCH(encrypted-cache): pass an empty path to LoadCache so the resolver
// picks the encrypted sibling (cache-v6.json.enc) when it exists. The
// previous version pinned to DefaultCachePath() which is the plaintext
// path - on modern Granola installs the plaintext copy is a stale stub
// and the real data lives only in the .enc file. ResolveCachePath()
// returns the path used so the error message stays informative.
func openGranolaCache() (*granola.Cache, error) {
	path, _ := granola.ResolveCachePath()
	c, err := granola.LoadCache("")
	if err != nil {
		return nil, fmt.Errorf("loading Granola cache at %s: %w", path, err)
	}
	return c, nil
}

// openGranolaStore opens (or creates) the SQLite store and ensures the
// granola-specific schema is in place.
func openGranolaStore(ctx context.Context) (*store.Store, error) {
	dbPath := defaultDBPath("granola-pp-cli")
	if err := os.MkdirAll(strings.TrimSuffix(dbPath, "/data.db"), 0o755); err != nil {
		return nil, fmt.Errorf("creating data dir: %w", err)
	}
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local store: %w", err)
	}
	if err := granola.EnsureSchema(ctx, s.DB()); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// openGranolaStoreRead opens the store for reading; returns (nil, nil)
// if the database hasn't been created yet so the caller can emit a
// helpful "run sync first" message.
func openGranolaStoreRead(ctx context.Context) (*store.Store, error) {
	dbPath := defaultDBPath("granola-pp-cli")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, nil
	}
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	if err := granola.EnsureSchema(ctx, s.DB()); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// emitJSON writes v to cmd's stdout as JSON, honoring --compact and
// --select via printJSONFiltered.
func emitJSON(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

// emitNDJSON writes each item on its own line.
func emitNDJSON(cmd *cobra.Command, items []any) error {
	w := cmd.OutOrStdout()
	for _, it := range items {
		b, err := json.Marshal(it)
		if err != nil {
			return err
		}
		if _, err := w.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	return nil
}

// emitNDJSONLine writes one ndjson line.
func emitNDJSONLine(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

// parseTimeWindow translates --last 7d / --since DATE / --until DATE
// into an absolute [from, to] pair. Either end may be zero-valued.
// --since accepts both absolute dates ("2026-05-01") and relative
// durations ("7d", "24h") — relative durations are subtracted from now.
func parseTimeWindow(last, since, until string) (from, to time.Time, err error) {
	now := time.Now()
	if last != "" {
		d, perr := parseDurationLoose(last)
		if perr != nil {
			err = fmt.Errorf("invalid --last %q: %w", last, perr)
			return
		}
		from = now.Add(-d)
		to = now
		return
	}
	if since != "" {
		from, err = parseSinceOrDate(since, now)
		if err != nil {
			err = fmt.Errorf("invalid --since %q: %w", since, err)
			return
		}
	}
	if until != "" {
		to, err = parseSinceOrDate(until, now)
		if err != nil {
			err = fmt.Errorf("invalid --until %q: %w", until, err)
			return
		}
	}
	return
}

// parseSinceOrDate tries a relative duration first (suffixes d/w/h/m/s),
// then falls back to an absolute date.
func parseSinceOrDate(s string, now time.Time) (time.Time, error) {
	if d, err := parseDurationLoose(s); err == nil {
		return now.Add(-d), nil
	}
	return parseAnyDate(s)
}

// timeNow is a wall-clock indirection used by commands so tests can swap
// the clock. Defaults to time.Now.
var timeNow = time.Now

// parseDurationLoose accepts "7d", "30d", "3h", and standard Go durations.
func parseDurationLoose(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	// "Nd" -> N*24h
	if strings.HasSuffix(s, "d") {
		var n int
		if _, err := fmt.Sscanf(s, "%dd", &n); err == nil {
			return time.Duration(n) * 24 * time.Hour, nil
		}
	}
	// "Nw" -> N*7d
	if strings.HasSuffix(s, "w") {
		var n int
		if _, err := fmt.Sscanf(s, "%dw", &n); err == nil {
			return time.Duration(n) * 7 * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(s)
}

func parseAnyDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02 15:04", "2006-01-02", "01/02/2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	if d, err := parseDurationLoose(s); err == nil {
		return time.Now().Add(-d), nil
	}
	return time.Time{}, fmt.Errorf("unrecognized date %q", s)
}

// withinWindow returns true if t is inside [from, to] when those are set.
// Zero from/to are unbounded.
func withinWindow(t time.Time, from, to time.Time) bool {
	if t.IsZero() {
		return false
	}
	if !from.IsZero() && t.Before(from) {
		return false
	}
	if !to.IsZero() && t.After(to) {
		return false
	}
	return true
}

// stderr writes a one-line user-visible note to stderr (warnings only).
func stderr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Ensure sql import is referenced even when no .go file under cli/ uses
// it directly; we re-export from this package.
var _ = sql.Open

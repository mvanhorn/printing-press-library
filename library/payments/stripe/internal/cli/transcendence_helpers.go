// Copyright 2026 chris-rodriguez. Licensed under Apache-2.0. See LICENSE.

// Hand-written helpers shared by the eight "transcendence" commands
// (sql, health, dunning-queue, payout reconcile, customer 360, subs at-risk,
// events since, metadata grep). NOT generated — safe to edit.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// parseHumanDuration accepts Go-standard durations ("30m", "2h") plus
// agent-natural day/week shortcuts ("30d", "2w"). Empty or whitespace
// inputs return (0, nil) — callers decide whether that means "no filter"
// or an error.
func parseHumanDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	// d/w shortcuts: split numeric prefix from suffix
	if m := humanDurRE.FindStringSubmatch(s); m != nil {
		n, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return 0, fmt.Errorf("parse duration %q: %w", s, err)
		}
		switch m[2] {
		case "d":
			return time.Duration(n * float64(24*time.Hour)), nil
		case "w":
			return time.Duration(n * float64(7*24*time.Hour)), nil
		}
	}
	return time.ParseDuration(s)
}

var humanDurRE = regexp.MustCompile(`^(\d+(?:\.\d+)?)([dw])$`)

// transcendenceDBPath returns the configured DB path or the default one.
// Centralizes the "user passed --db" / "fall back to default" pattern.
func transcendenceDBPath(override string) string {
	if override != "" {
		return override
	}
	return defaultDBPath("stripe-pp-cli")
}

// jsonGet pulls a top-level value from a JSON RawMessage, returning ("", false)
// when the key is absent or the value is null. Strings are unquoted; numbers and
// booleans are returned via their canonical Go fmt representation.
func jsonGet(raw json.RawMessage, key string) (string, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", false
	}
	v, ok := obj[key]
	if !ok || string(v) == "null" {
		return "", false
	}
	var s string
	if err := json.Unmarshal(v, &s); err == nil {
		return s, true
	}
	return strings.Trim(string(v), `"`), true
}

// jsonGetInt extracts a numeric field. Stripe returns most amounts as int64
// (cents), but JSON unmarshals to float64 by default — we widen to int64.
func jsonGetInt(raw json.RawMessage, key string) (int64, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return 0, false
	}
	v, ok := obj[key]
	if !ok || string(v) == "null" {
		return 0, false
	}
	var n int64
	if err := json.Unmarshal(v, &n); err == nil {
		return n, true
	}
	var f float64
	if err := json.Unmarshal(v, &f); err == nil {
		return int64(f), true
	}
	return 0, false
}

// cursorPath returns the on-disk file where the events-since cursor is
// persisted for the given profile namespace.
func cursorPath(profile string) (string, error) {
	if profile == "" {
		profile = "default"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "stripe-pp-cli", "cursors")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, profile+".json"), nil
}

// readCursor returns the saved cursor value or "" if none.
func readCursor(profile string) (string, error) {
	p, err := cursorPath(profile)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	var rec struct {
		Cursor string `json:"cursor"`
	}
	if err := json.Unmarshal(b, &rec); err != nil {
		return "", fmt.Errorf("parse cursor file %s: %w", p, err)
	}
	return rec.Cursor, nil
}

// writeCursor persists the new cursor value for the given profile.
func writeCursor(profile, cursor string) error {
	p, err := cursorPath(profile)
	if err != nil {
		return err
	}
	rec := struct {
		Cursor string `json:"cursor"`
	}{Cursor: cursor}
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

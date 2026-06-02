// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored support for the Email Bison novel (transcendence) commands.

package cli

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/email-bison/internal/store"
)

// openNovelStore opens the local SQLite store using the standard default path,
// returning a clear hint to run sync when the store cannot be opened.
func openNovelStore(ctx context.Context) (*store.Store, error) {
	dbPath := defaultDBPath("email-bison-pp-cli")
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w\nRun 'email-bison-pp-cli sync' first.", err)
	}
	return db, nil
}

// parseSince converts a --since value into a cutoff time. It accepts a Go
// duration ("24h", "90m"), a day count ("7d"), or an explicit RFC3339 / date
// ("2026-06-01") value. An empty string defaults to 24h ago.
func parseSince(s string) (time.Time, error) {
	now := time.Now().UTC()
	s = strings.TrimSpace(s)
	if s == "" {
		return now.Add(-24 * time.Hour), nil
	}
	if strings.HasSuffix(s, "d") {
		if n, err := strconv.Atoi(strings.TrimSuffix(s, "d")); err == nil {
			return now.AddDate(0, 0, -n), nil
		}
	}
	if d, err := time.ParseDuration(s); err == nil {
		return now.Add(-d), nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid --since value %q (use 24h, 7d, or 2006-01-02)", s)
}

// mergeTagPattern matches Email Bison merge tags like {FIRST_NAME} in subject
// and body text. Tags are uppercase tokens wrapped in single curly braces.
var mergeTagPattern = regexp.MustCompile(`\{([A-Z0-9_]+)\}`)

// extractMergeTags returns the distinct merge-tag names found in the given text.
func extractMergeTags(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range mergeTagPattern.FindAllStringSubmatch(text, -1) {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

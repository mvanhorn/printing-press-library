// Shared helpers for the hand-written Craigslist commands. Lives alongside the
// generated cli package so unexported root flags + helpers stay in scope.
//
// Centralizes two concerns the novel commands keep needing:
//   - Opening the local store with cl tables ensured (one path; one error
//     message; honors the data-source flag for read-only fallbacks).
//   - Parsing user-facing duration tokens like "24h", "3d", "30d" — Go's
//     stdlib only knows "24h", not "3d", and every novel command uses
//     --since with day-friendly suffixes.
package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/craigslist/internal/store"
)

// openCLStore opens (or creates) the local SQLite store and ensures the
// Craigslist-specific tables exist. Callers are responsible for Close().
func openCLStore(ctx context.Context) (*store.Store, error) {
	db, err := store.OpenWithContext(ctx, defaultDBPath("craigslist-pp-cli"))
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	if err := db.EnsureCLTables(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ensure cl tables: %w", err)
	}
	return db, nil
}

// quoteFTS wraps a user-supplied query string for SQLite FTS5 MATCH so it
// is interpreted as a literal phrase. FTS5 reserves single-quote, double-
// quote, parentheses, and operator keywords (AND/OR/NOT/NEAR); the safe
// shape for end-user input is a double-quoted phrase with embedded
// double-quotes doubled. Empty input returns "" so callers can short-
// circuit before adding the MATCH clause.
func quoteFTS(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// parseDuration accepts Go's stdlib formats plus "<N>d" (days). Empty input
// returns 0 with no error so callers can treat "no filter" as the default.
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, nil
	}
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, fmt.Errorf("parse duration %q: %w", s, err)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", s, err)
	}
	return d, nil
}

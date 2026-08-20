package localstore

import "strings"

// ftsQuote wraps a user-supplied search query in FTS5-safe phrase quoting so
// characters like `-`, `:`, `(`, `)`, `*`, `^`, and quote marks are treated as
// literal text rather than FTS5 operators. Without this, "nothing-matches"
// parses as `nothing NOT matches` and SQLite returns "no such column: matches".
//
// Inside FTS5 phrase syntax (`"..."`) any embedded double quote must be doubled.
func ftsQuote(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return q
	}
	escaped := strings.ReplaceAll(q, `"`, `""`)
	return `"` + escaped + `"`
}

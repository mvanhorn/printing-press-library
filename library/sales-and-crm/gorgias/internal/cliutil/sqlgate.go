// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cliutil

import (
	"fmt"
	"strings"
)

// ValidateReadOnlySQL gates ad-hoc SQL queries against the local SQLite
// mirror. The gate is an allowlist (SELECT or WITH only) applied AFTER
// stripping leading whitespace, line comments, block comments, and
// semicolons that SQLite itself ignores before parsing. A naive HasPrefix
// check on a keyword blocklist is bypassable by prefixing the dangerous
// statement with "/* x */" or "-- x\n" — strings.TrimSpace strips outer
// whitespace but does not understand SQL comment syntax.
//
// Combined with the empirical fact that modernc.org/sqlite's mode=ro does
// NOT block VACUUM INTO (writes a snapshot to a new file) or ATTACH
// DATABASE (opens a separate writable handle), such a bypass produces
// silent exfiltration to an attacker-chosen path.
//
// SELECT and WITH are the only allowed leading keywords. WITH supports
// SELECT-form CTEs; CTE-wrapped writes ("WITH x AS (...) INSERT ...") are
// caught by OpenReadOnly's mode=ro one layer down. PRAGMA, ATTACH, VACUUM,
// and every other DDL/DML keyword fail at this gate before reaching SQLite.
//
// Both the CLI `sql` command and the MCP `sql` tool call this so the gate
// is identical at both surfaces — divergence here is a real exploit
// vector, not a stylistic complaint.
func ValidateReadOnlySQL(query string) error {
	stripped := StripLeadingSQLNoise(query)
	if stripped == "" {
		return fmt.Errorf("empty query")
	}
	upper := strings.ToUpper(stripped)
	switch {
	case strings.HasPrefix(upper, "SELECT"):
		return nil
	case strings.HasPrefix(upper, "WITH"):
		return nil
	}
	// Surface the first keyword the user actually wrote so the error
	// is debuggable from the message alone.
	first := firstSQLWord(upper)
	return fmt.Errorf("only SELECT or WITH queries are allowed (got: %s)", first)
}

// StripLeadingSQLNoise removes leading whitespace, SQL line comments
// (-- to end of line), block comments (/* ... */), and statement
// separators (;) from query. SQLite skips these before parsing the first
// keyword, so a security gate that does not strip them mismatches what
// the driver actually executes.
func StripLeadingSQLNoise(query string) string {
	for {
		query = strings.TrimLeft(query, " \t\r\n;")
		switch {
		case strings.HasPrefix(query, "--"):
			if idx := strings.IndexByte(query, '\n'); idx >= 0 {
				query = query[idx+1:]
				continue
			}
			return ""
		case strings.HasPrefix(query, "/*"):
			if idx := strings.Index(query[2:], "*/"); idx >= 0 {
				query = query[2+idx+2:]
				continue
			}
			return ""
		default:
			return query
		}
	}
}

// firstSQLWord returns the leading run of letters/underscores in s. Used
// solely to make the rejection error informative ("got: PRAGMA") — not a
// part of the security gate itself.
func firstSQLWord(s string) string {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'A' && c <= 'Z') && c != '_' {
			return s[:i]
		}
	}
	return s
}

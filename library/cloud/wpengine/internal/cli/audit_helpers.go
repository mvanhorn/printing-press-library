// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared helpers for the hand-written fleet-audit commands (audit *, whois).

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/wpengine/internal/store"
)

// drainRows applies the drain-first discipline uniformly: iterate, scan via
// the closure, then surface rows.Err() and Close() errors — closing the rows
// on every error path. SQLite runs on a single connection, so callers must
// fully drain before issuing follow-up queries.
func drainRows(rows *sql.Rows, what string, scan func(*sql.Rows) error) error {
	for rows.Next() {
		if err := scan(rows); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scanning %s row: %w", what, err)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterating %s rows: %w", what, err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("closing %s rows: %w", what, err)
	}
	return nil
}

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// openAuditStore resolves the db path, applies the missing-mirror guard, and
// opens the local store. When the mirror file does not exist it prints a sync
// hint to stderr, emits emptyJSON on stdout for machine callers, and returns
// handled=true so the caller can exit 0 (missing mirror is an empty local
// cache state, not a failure).
func openAuditStore(cmd *cobra.Command, flags *rootFlags, dbPath, hintResource, emptyJSON string) (*store.Store, bool, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("wpengine-pp-cli")
	}
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: wpengine-pp-cli sync --db %s\n", dbPath, dbPath)
		if flags.asJSON || flags.agent {
			fmt.Fprintln(cmd.OutOrStdout(), emptyJSON)
		}
		return nil, true, nil
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, false, fmt.Errorf("opening local database: %w", err)
	}
	if !hintIfUnsynced(cmd, db, hintResource) {
		hintIfStale(cmd, db, hintResource, flags.maxAge)
	}
	return db, false, nil
}

// rejectLiveDataSource enforces pp:data-source local: these commands read the
// synced mirror only and have no live equivalent.
func rejectLiveDataSource(flags *rootFlags) error {
	if flags.dataSource == "live" {
		return usageErr(fmt.Errorf("this command reads the local mirror only (no live equivalent); run 'sync' to refresh, then retry without --data-source live"))
	}
	return nil
}

// rejectLocalDataSource enforces pp:data-source live: these commands call the
// remote API only and have no local data source.
func rejectLocalDataSource(flags *rootFlags) error {
	if flags.dataSource == "local" {
		return usageErr(fmt.Errorf("this command calls the live API only (no local data source); retry without --data-source local"))
	}
	return nil
}

// parseAPITime parses the timestamp shapes the WP Engine API emits
// (RFC3339 date-times and bare dates). Zero time + false on miss.
func parseAPITime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// versionBelow reports whether version a sorts strictly below threshold b,
// comparing dot-separated numeric segments ("8.1.2" < "8.2"). Non-numeric
// segments compare as strings; missing segments compare as zero.
func versionBelow(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		var sa, sb string
		if i < len(as) {
			sa = as[i]
		}
		if i < len(bs) {
			sb = bs[i]
		}
		// Missing segments compare as zero ("8.2" == "8.2.0"), not as the
		// empty string — otherwise "" < "0" flags exact-threshold versions.
		if sa == "" {
			sa = "0"
		}
		if sb == "" {
			sb = "0"
		}
		na, ea := strconv.Atoi(strings.TrimSpace(sa))
		nb, eb := strconv.Atoi(strings.TrimSpace(sb))
		switch {
		case ea == nil && eb == nil:
			if na != nb {
				return na < nb
			}
		default:
			if sa != sb {
				return sa < sb
			}
		}
	}
	return false
}

// certNameCovers reports whether a certificate name (common name or SAN
// entry, possibly a wildcard like *.example.com) covers the given domain.
func certNameCovers(certName, domain string) bool {
	certName = strings.ToLower(strings.TrimSpace(certName))
	domain = strings.ToLower(strings.TrimSpace(domain))
	if certName == "" || domain == "" {
		return false
	}
	if certName == domain {
		return true
	}
	if base, ok := strings.CutPrefix(certName, "*."); ok {
		// RFC 6125 semantics: a wildcard covers exactly one additional left
		// label and does NOT cover the apex (example.com is not matched by
		// *.example.com). WP Engine certs list the apex as a separate SAN
		// entry, which is matched by the exact-equality branch above.
		if rest, ok2 := strings.CutSuffix(domain, "."+base); ok2 && rest != "" && !strings.Contains(rest, ".") {
			return true
		}
	}
	return false
}

// decodeRedirectTargets extracts redirect target names from a Domain's
// redirects_to array, tolerating both object entries ({"name": ...}) and
// bare strings.
func decodeRedirectTargets(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var objs []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &objs); err == nil {
		out := make([]string, 0, len(objs))
		for _, o := range objs {
			var name string
			if v, ok := o["name"]; ok {
				_ = json.Unmarshal(v, &name)
			}
			if name != "" {
				out = append(out, name)
			}
		}
		return out
	}
	var strs []string
	if err := json.Unmarshal(raw, &strs); err == nil {
		return strs
	}
	return nil
}

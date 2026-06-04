// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cliutil

import (
	"strings"
	"testing"
)

// TestValidateReadOnlySQL_AllowsSelectAndWITH pins the contract: the gate
// accepts SELECT and WITH-prefix queries, including CTEs, mixed case,
// leading whitespace, leading SQL comments, and leading statement
// separators. SELECT-form CTEs ("WITH x AS (SELECT ...) SELECT") must work
// because novel CLI sql commands in the public library accept them as
// legitimate read-only queries; the MCP surface keeps parity.
func TestValidateReadOnlySQL_AllowsSelectAndWITH(t *testing.T) {
	allowed := []string{
		"SELECT 1",
		"select * from resources",
		"  SELECT 1",
		"\tSELECT 1",
		"\nSELECT 1",
		";SELECT 1",
		"-- comment\nSELECT 1",
		"/* comment */ SELECT 1",
		"/* comment */SELECT 1",
		"/**/SELECT 1",
		"-- one\n-- two\nSELECT 1",
		"/* a *//* b */ SELECT 1",
		"WITH r AS (SELECT 1) SELECT * FROM r",
		"with r as (select 1) select * from r",
	}
	for _, q := range allowed {
		if err := ValidateReadOnlySQL(q); err != nil {
			t.Errorf("ValidateReadOnlySQL(%q) = %v, want nil", q, err)
		}
	}
}

// TestValidateReadOnlySQL_RejectsBypassVectors covers the comment-prefix
// bypass class that defeated the earlier prefix-blocklist gate. mode=ro on
// modernc.org/sqlite does not block VACUUM INTO (writes a fresh file) or
// ATTACH DATABASE (opens a separate writable handle), so the gate is the
// only defense against those vectors. A successful bypass at this layer
// would let an MCP-trusting agent silently exfiltrate the local database.
func TestValidateReadOnlySQL_RejectsBypassVectors(t *testing.T) {
	rejected := []string{
		"VACUUM INTO '/tmp/x.db'",
		"ATTACH DATABASE 'file:/tmp/x.db?mode=rwc' AS evil",
		"INSERT INTO resources VALUES ('x', 'y', '{}')",
		"UPDATE resources SET resource_type = 'evil'",
		"DELETE FROM resources",
		"REPLACE INTO resources VALUES ('seed', 'evil', '{}')",
		"DROP TABLE resources",
		"PRAGMA writable_schema = ON",
		"REINDEX",
		"DETACH DATABASE x",
		"/* x */ VACUUM INTO '/tmp/exfil.db'",
		"/* x */VACUUM INTO '/tmp/exfil.db'",
		"-- x\nVACUUM INTO '/tmp/exfil.db'",
		"/**/VACUUM INTO '/tmp/exfil.db'",
		"/* x */ ATTACH DATABASE 'file:/tmp/x.db?mode=rwc' AS evil",
		"-- x\nATTACH DATABASE '/tmp/x.db' AS evil",
		";VACUUM INTO '/tmp/x.db'",
		"; ; VACUUM INTO '/tmp/x.db'",
		"/* a */ /* b */ INSERT INTO t VALUES (1)",
		"/* outer /* not nested */ */ SELECT 1", // SQLite doesn't nest comments — trailing "*/" closes the outer; whatever follows is rejected if not SELECT/WITH.
		"-- only a comment",
		"/* only a comment */",
		"",
		"   ",
		";",
	}
	for _, q := range rejected {
		if err := ValidateReadOnlySQL(q); err == nil {
			t.Errorf("ValidateReadOnlySQL(%q) = nil, want error", q)
		}
	}
}

// TestStripLeadingSQLNoise checks the helper directly so a regression in the
// stripping logic (off-by-one on /* */ length, missing newline handling on
// --) surfaces close to the source rather than only via the integration
// behavior of ValidateReadOnlySQL.
func TestStripLeadingSQLNoise(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"SELECT 1", "SELECT 1"},
		{"  SELECT 1", "SELECT 1"},
		{"\t\nSELECT 1", "SELECT 1"},
		{";SELECT 1", "SELECT 1"},
		{";; ;SELECT 1", "SELECT 1"},
		{"-- x\nSELECT 1", "SELECT 1"},
		{"-- x\n-- y\nSELECT 1", "SELECT 1"},
		{"/* x */SELECT 1", "SELECT 1"},
		{"/**/SELECT 1", "SELECT 1"},
		{"/* x */ /* y */ SELECT 1", "SELECT 1"},
		{"-- only", ""},
		{"/* only", ""},
		{"", ""},
	}
	for _, c := range cases {
		got := StripLeadingSQLNoise(c.in)
		if !strings.EqualFold(got, c.want) {
			t.Errorf("StripLeadingSQLNoise(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

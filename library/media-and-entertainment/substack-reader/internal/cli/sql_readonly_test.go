// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// Security gate tests for the CLI sql command's read-only validator. Mirrors
// the MCP sql tool's contract: only a single SELECT/WITH statement, resistant
// to comment- and multi-statement-injection bypasses.

package cli

import "testing"

func TestValidateReadOnlySQL_Allows(t *testing.T) {
	ok := []string{
		"SELECT 1",
		"select id from resources",
		"  SELECT * FROM resources WHERE resource_type='posts'",
		"WITH x AS (SELECT 1) SELECT * FROM x",
		"/* comment */ SELECT 1",
		"-- lead comment\nSELECT 1",
		"SELECT 1;", // a single trailing semicolon is fine
	}
	for _, q := range ok {
		if err := validateReadOnlySQL(q); err != nil {
			t.Errorf("expected %q to be allowed, got error: %v", q, err)
		}
	}
}

func TestValidateReadOnlySQL_Rejects(t *testing.T) {
	bad := []string{
		"INSERT INTO resources VALUES (1)",
		"UPDATE resources SET data='x'",
		"DELETE FROM resources",
		"DROP TABLE resources",
		"ATTACH DATABASE '/tmp/x.db' AS evil",
		"PRAGMA user_version",
		"VACUUM INTO '/tmp/leak.db'",
		"SELECT 1; ATTACH DATABASE '/tmp/x.db' AS evil", // multi-statement injection
		"SELECT 1; DROP TABLE resources",
		"/* SELECT */ DROP TABLE resources", // comment does not make it a SELECT
		"",
	}
	for _, q := range bad {
		if err := validateReadOnlySQL(q); err == nil {
			t.Errorf("expected %q to be rejected, but it passed", q)
		}
	}
}

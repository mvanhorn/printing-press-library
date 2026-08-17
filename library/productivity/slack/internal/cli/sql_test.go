// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestSQLStatementIsReadOnly(t *testing.T) {
	tests := []struct {
		name   string
		stmt   string
		wantOK bool
	}{
		{"plain select", "SELECT 1", true},
		{"lowercase select", "select id from resources", true},
		{"leading whitespace", "   SELECT id FROM resources", true},
		{"trailing semicolon", "SELECT id FROM resources;", true},
		{"cte", "WITH x AS (SELECT 1) SELECT * FROM x", true},
		{"explain", "EXPLAIN QUERY PLAN SELECT 1", true},
		{"pragma", "PRAGMA table_info(resources)", true},
		{"newline after verb", "SELECT\nid FROM resources", true},

		{"empty", "", false},
		{"whitespace only", "   ", false},
		{"update", "UPDATE resources SET data = '{}'", false},
		{"delete", "DELETE FROM resources", false},
		{"insert", "INSERT INTO resources VALUES (1)", false},
		{"drop", "DROP TABLE resources", false},
		{"stacked statements", "SELECT 1; DROP TABLE resources", false},
		{"stacked with trailing semicolon", "SELECT 1; DELETE FROM resources;", false},
		{"selectish prefix but not select", "SELECTED FROM resources", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOK, why := sqlStatementIsReadOnly(tt.stmt)
			if gotOK != tt.wantOK {
				t.Fatalf("sqlStatementIsReadOnly(%q) = %v (%s), want %v", tt.stmt, gotOK, why, tt.wantOK)
			}
			if !gotOK && why == "" {
				t.Fatalf("rejection of %q must carry a reason", tt.stmt)
			}
		})
	}
}

func TestSQLStatementRejectsStackedDestructiveTail(t *testing.T) {
	// Guards the specific bypass of checking only the leading verb.
	ok, why := sqlStatementIsReadOnly("SELECT 1; DROP TABLE resources")
	if ok {
		t.Fatal("stacked statement with destructive tail must be refused")
	}
	if why == "" {
		t.Fatal("expected a reason explaining the refusal")
	}
}

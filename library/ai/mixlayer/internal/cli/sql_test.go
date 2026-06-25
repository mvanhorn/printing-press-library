// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestValidateReadOnlySQLRejectsCTEWrappedDML(t *testing.T) {
	queries := []string{
		"WITH x AS (DELETE FROM runs WHERE 1=1) SELECT 1",
		"SELECT * FROM pragma_table_info('vault')",
	}
	for _, query := range queries {
		if err := validateReadOnlySQL(query); err == nil {
			t.Fatalf("validateReadOnlySQL accepted %q", query)
		}
	}
}

func TestValidateReadOnlySQLAllowsReadQueries(t *testing.T) {
	for _, query := range []string{
		"select model, count(*) from runs group by model",
		"with recent as (select * from runs) select * from recent",
		"select 'delete is text, not a keyword' as note",
	} {
		t.Run(query, func(t *testing.T) {
			if err := validateReadOnlySQL(query); err != nil {
				t.Fatalf("validateReadOnlySQL(%q) = %v, want nil", query, err)
			}
		})
	}
}

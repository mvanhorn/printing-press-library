// PATCH(amend-2026-05-25): tests for expand/contract decomposition.
package dbchange

import (
	"strings"
	"testing"
)

func decomposeOne(t *testing.T, sql string) DecompResult {
	t.Helper()
	m := Parse(sql, "t")
	if len(m.Statements) != 1 {
		t.Fatalf("%q: expected 1 statement, got %d", sql, len(m.Statements))
	}
	return Decompose(m.Statements[0])
}

func TestDecomposeDropColumn(t *testing.T) {
	r := decomposeOne(t, "alter table public.t drop column legacy;")
	if !r.Decomposed {
		t.Fatalf("DROP COLUMN should decompose, got refusal: %s", r.RefusedReason)
	}
	if len(r.Steps) != 2 {
		t.Fatalf("expected 2 steps (tombstone-rename + deferred drop), got %d", len(r.Steps))
	}
	if !r.Steps[0].AutoApply {
		t.Error("tombstone-rename step should be auto-applicable")
	}
	if !strings.Contains(r.Steps[0].SQL, "RENAME COLUMN legacy TO legacy_pp_dropped") {
		t.Errorf("step 0 should tombstone-rename, got %q", r.Steps[0].SQL)
	}
	if r.Steps[1].AutoApply {
		t.Error("the hard drop step must NOT be auto-applicable")
	}
}

func TestDecomposeTypeChange(t *testing.T) {
	r := decomposeOne(t, "alter table public.t alter column amount type bigint;")
	if !r.Decomposed {
		t.Fatalf("ALTER TYPE should decompose, got refusal: %s", r.RefusedReason)
	}
	if len(r.Steps) != 3 {
		t.Fatalf("expected 3 steps (expand/backfill/contract), got %d", len(r.Steps))
	}
	if r.Steps[0].Phase != PhaseExpand || !r.Steps[0].AutoApply {
		t.Errorf("step 0 should be an auto-applicable expand, got %+v", r.Steps[0])
	}
	if !strings.Contains(r.Steps[0].SQL, "ADD COLUMN IF NOT EXISTS amount_pp_new bigint") {
		t.Errorf("expand step should add the new typed column, got %q", r.Steps[0].SQL)
	}
	if r.Steps[1].Phase != PhaseBackfill || r.Steps[1].AutoApply {
		t.Errorf("step 1 should be a non-auto backfill, got %+v", r.Steps[1])
	}
	if r.Steps[2].Phase != PhaseContract || r.Steps[2].AutoApply {
		t.Errorf("step 2 should be a non-auto contract, got %+v", r.Steps[2])
	}
}

func TestDecomposeRefusals(t *testing.T) {
	cases := []struct {
		sql      string
		wantRule string
	}{
		{"truncate public.t;", "truncate_irreducible"},
		{"delete from public.t;", "unbounded_delete"},
		{"delete from public.t where id = 1;", "delete_irreducible"},
		{"drop table public.t;", "drop_table_dependents"},
		{"drop schema public;", "drop_schema_dependents"},
	}
	for _, tc := range cases {
		r := decomposeOne(t, tc.sql)
		if r.Decomposed {
			t.Errorf("%q: expected refusal, got decomposition", tc.sql)
			continue
		}
		if r.RefusedRule != tc.wantRule {
			t.Errorf("%q: refused_rule = %q, want %q", tc.sql, r.RefusedRule, tc.wantRule)
		}
		if r.RefusedReason == "" {
			t.Errorf("%q: refusal must carry a human reason", tc.sql)
		}
	}
}

func TestDecomposeAdditivePassthrough(t *testing.T) {
	r := decomposeOne(t, "create table public.t (id int);")
	if !r.Decomposed || len(r.Steps) != 1 {
		t.Fatalf("additive statement should pass through as 1 step, got %+v", r)
	}
	if !r.Steps[0].AutoApply {
		t.Error("a reversible additive statement should be auto-applicable on passthrough")
	}
}

func TestDecomposeManifestRollup(t *testing.T) {
	m := Parse("create table public.t (id int); truncate public.t;", "t")
	results, anyRefused := DecomposeManifest(m)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !anyRefused {
		t.Error("a manifest containing TRUNCATE must report anyRefused=true")
	}
}

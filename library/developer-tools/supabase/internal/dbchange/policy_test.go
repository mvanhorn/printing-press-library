// PATCH(amend-2026-05-25): tests for the policy approver, est-rows gate, and
// expand/contract decomposition.
package dbchange

import "testing"

func TestEstRowsAffectedClassification(t *testing.T) {
	cases := []struct {
		sql  string
		want int // per-statement EstRowsAffected
	}{
		{"create table public.t (id int);", 0},
		{"grant select on public.t to anon;", 0},
		{`create policy "p" on public.t for select using (true);`, 0},
		{"alter table public.t add column z int;", 0},                                   // nullable add
		{"alter table public.t add column z int default 42;", 0},                        // constant default
		{"alter table public.t add column z timestamptz default now();", -1},            // volatile default rewrites rows
		{"alter table public.t add column z uuid default gen_random_uuid();", -1},       // volatile default
		{"alter table public.t alter column z type bigint;", -1},                        // type change rewrites
		{"delete from public.t where id = 1;", -1},
		{"truncate public.t;", -1},
		{"insert into public.t (id) values (1);", -1},
		{"update public.t set x = 1;", -1},
	}
	for _, tc := range cases {
		m := Parse(tc.sql, "t")
		if len(m.Statements) != 1 {
			t.Fatalf("%q: expected 1 statement, got %d", tc.sql, len(m.Statements))
		}
		if got := m.Statements[0].EstRowsAffected; got != tc.want {
			t.Errorf("%q: est_rows_affected = %d, want %d", tc.sql, got, tc.want)
		}
	}
}

func TestMaxEstRowsRollup(t *testing.T) {
	// A purely additive plan rolls up to 0.
	add := Parse("create table public.t (id int); grant select on public.t to anon;", "t")
	if add.MaxEstRowsAffected != 0 {
		t.Errorf("additive plan max_est_rows = %d, want 0", add.MaxEstRowsAffected)
	}
	// Any unknown statement makes the whole plan -1.
	mixed := Parse("create table public.t (id int); delete from public.t where id = 1;", "t")
	if mixed.MaxEstRowsAffected != -1 {
		t.Errorf("mixed plan max_est_rows = %d, want -1", mixed.MaxEstRowsAffected)
	}
}

func TestSafeDefaultPolicyAuthorizesAdditive(t *testing.T) {
	sql := `create table public.t (id int);
grant select on public.t to anon;
create policy "p" on public.t for select using ((select auth.uid()) = id);
alter table public.t enable row level security;`
	m := Parse(sql, "t")
	v := SafeDefaultPolicy().Evaluate(m, 0)
	if !v.Authorized {
		t.Fatalf("safe-default should authorize additive+reversible plan; reasons: %+v", v.Reasons)
	}
}

func TestSafeDefaultPolicyRefusesDestructive(t *testing.T) {
	m := Parse("drop table public.t;", "t")
	v := SafeDefaultPolicy().Evaluate(m, 0)
	if v.Authorized {
		t.Fatal("safe-default must not authorize DROP TABLE")
	}
	if !hasRule(v.Reasons, "destructive_not_allowed") {
		t.Errorf("expected destructive_not_allowed reason, got %+v", v.Reasons)
	}
}

func TestSafeDefaultPolicyRefusesDataWrite(t *testing.T) {
	m := Parse("update public.t set x = 1;", "t")
	v := SafeDefaultPolicy().Evaluate(m, 0)
	if v.Authorized {
		t.Fatal("safe-default must not authorize UPDATE")
	}
	if !hasRule(v.Reasons, "touches_data_rows") && !hasRule(v.Reasons, "est_rows_exceeds_cap") {
		t.Errorf("expected a row-impact refusal, got %+v", v.Reasons)
	}
}

func TestPolicyRefusesOnLintError(t *testing.T) {
	// An otherwise-clean additive plan is refused when the caller reports an
	// ERROR-level lint finding.
	m := Parse("create table public.t (id int);", "t")
	v := SafeDefaultPolicy().Evaluate(m, 1)
	if v.Authorized {
		t.Fatal("policy must refuse a plan with ERROR-level lint findings")
	}
	if !hasRule(v.Reasons, "lint_not_clean") {
		t.Errorf("expected lint_not_clean reason, got %+v", v.Reasons)
	}
}

func TestPolicyRefusesVolatileDefaultAddColumn(t *testing.T) {
	// ADD COLUMN with now() is OpDDL + reversible but rewrites rows -> est-rows gate.
	m := Parse("alter table public.t add column created timestamptz default now();", "t")
	v := SafeDefaultPolicy().Evaluate(m, 0)
	if v.Authorized {
		t.Fatal("policy must refuse a volatile-default ADD COLUMN (full-table rewrite)")
	}
}

func TestPolicyRefusesEmptyPlan(t *testing.T) {
	m := Parse("", "t")
	v := SafeDefaultPolicy().Evaluate(m, 0)
	if v.Authorized {
		t.Fatal("policy must refuse an empty plan")
	}
	if !hasRule(v.Reasons, "empty_plan") {
		t.Errorf("expected empty_plan reason, got %+v", v.Reasons)
	}
}

func hasRule(rs []PolicyReason, rule string) bool {
	for _, r := range rs {
		if r.Rule == rule {
			return true
		}
	}
	return false
}

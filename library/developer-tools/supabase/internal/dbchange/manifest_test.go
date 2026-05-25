// PATCH: tests for the novel safe-mutation change manifest.
package dbchange

import "testing"

func TestParseDeterministicHash(t *testing.T) {
	sql := "grant execute on function public.f() to anon;"
	a := Parse(sql, "x")
	b := Parse(sql, "y") // different label must not change the hash
	if a.PlanHash != b.PlanHash {
		t.Fatalf("hash not deterministic across labels: %s vs %s", a.PlanHash, b.PlanHash)
	}
	// Cosmetic whitespace must not drift the hash.
	c := Parse("grant   execute on function public.f()   to anon;", "x")
	if a.PlanHash != c.PlanHash {
		t.Fatalf("whitespace changed hash: %s vs %s", a.PlanHash, c.PlanHash)
	}
	// A real semantic change must drift it.
	d := Parse("grant execute on function public.g() to anon;", "x")
	if a.PlanHash == d.PlanHash {
		t.Fatal("different SQL produced same hash")
	}
}

func TestClassifyOpClasses(t *testing.T) {
	cases := []struct {
		sql  string
		want OpClass
		dest bool
	}{
		{"create table public.t (id int);", OpDDL, false},
		{"grant select on public.t to anon;", OpGrant, false},
		{"revoke execute on function public.f() from public;", OpGrant, false},
		{"create policy \"p\" on public.t for select using (true);", OpRLS, false},
		{"alter table public.t enable row level security;", OpRLS, false},
		{"drop table public.t;", OpDestructive, true},
		{"truncate public.t;", OpDestructive, true},
		{"delete from public.t;", OpDestructive, true},
		{"insert into public.t (id) values (1);", OpDataWrite, false},
		{"update public.t set x = 1 where id = 2;", OpDataWrite, false},
	}
	for _, tc := range cases {
		m := Parse(tc.sql, "t")
		if len(m.Statements) != 1 {
			t.Fatalf("%q: expected 1 statement, got %d", tc.sql, len(m.Statements))
		}
		s := m.Statements[0]
		if s.OpClass != tc.want {
			t.Errorf("%q: op_class = %s, want %s", tc.sql, s.OpClass, tc.want)
		}
		if s.Destructive != tc.dest {
			t.Errorf("%q: destructive = %v, want %v", tc.sql, s.Destructive, tc.dest)
		}
	}
}

func TestMechanicalInverse(t *testing.T) {
	cases := []struct {
		sql         string
		wantInverse string
	}{
		{"grant select on public.t to anon;", "REVOKE select on public.t FROM anon;"},
		{"revoke select on public.t from anon;", "GRANT select on public.t TO anon;"},
		{"create table public.t (id int);", "DROP TABLE IF EXISTS public.t;"},
		{"create index idx_t on public.t (id);", "DROP INDEX IF EXISTS idx_t;"},
		{"alter table public.t enable row level security;", "alter table public.t DISABLE ROW LEVEL SECURITY;"},
	}
	for _, tc := range cases {
		m := Parse(tc.sql, "t")
		got := m.Statements[0].Inverse
		if got != tc.wantInverse {
			t.Errorf("%q: inverse = %q, want %q", tc.sql, got, tc.wantInverse)
		}
	}
}

func TestReversibilityVerdicts(t *testing.T) {
	cases := []struct {
		sql  string
		want Reversibility
	}{
		{"grant select on public.t to anon;", RevAuto},
		{"create table public.t (id int);", RevAuto},
		{"create or replace function public.f() returns int language sql as $$ select 1 $$;", RevSnapshot},
		{"alter table public.t add column z int;", RevAuto},
		{"delete from public.t;", RevNone},
		{"truncate public.t;", RevNone},
		{"update public.t set x = 1;", RevNone},
	}
	for _, tc := range cases {
		m := Parse(tc.sql, "t")
		if got := m.Statements[0].Reversibility; got != tc.want {
			t.Errorf("%q: reversibility = %s, want %s", tc.sql, got, tc.want)
		}
	}
}

func TestPolicyObjectIsTable(t *testing.T) {
	m := Parse(`create policy "users read own" on public.profiles for select using (auth.uid() = user_id);`, "t")
	objs := m.Statements[0].Objects
	if len(objs) != 1 || objs[0] != "public.profiles" {
		t.Fatalf("policy object = %v, want [public.profiles]", objs)
	}
}

func TestDollarQuotedBodyNotSplit(t *testing.T) {
	// A function body containing semicolons inside $$ must remain one statement.
	sql := `create function public.f() returns void language plpgsql as $$ begin perform 1; perform 2; end; $$;`
	m := Parse(sql, "t")
	if m.StatementN != 1 {
		t.Fatalf("dollar-quoted body split into %d statements, want 1", m.StatementN)
	}
}

func TestNamedDollarTagBlockNotSplit(t *testing.T) {
	// A `DO $ttl$ ... $ttl$` block (migration 008's TTL cleanup) contains a
	// semicolon-terminated DELETE inside its body. A bare-$$ splitter would
	// break it into multiple statements; the tag-aware splitter must keep it
	// whole. See spec build-correctness note #1.
	sql := `do $ttl$
begin
  delete from public.sessions where created_at < now() - interval '30 days';
end
$ttl$;`
	m := Parse(sql, "t")
	if m.StatementN != 1 {
		t.Fatalf("named-tag DO block split into %d statements, want 1", m.StatementN)
	}
}

func TestMixedTagsAndPositionalParamsSplitCorrectly(t *testing.T) {
	// A `$tag$` body must not be closed by a non-matching `$$` inside it, and a
	// `$1` positional parameter must not be mistaken for a dollar-quote tag.
	sql := `create function public.f(a int) returns int language plpgsql as $body$
begin
  return a + $1;
end
$body$;
grant execute on function public.f(int) to anon;`
	m := Parse(sql, "t")
	if m.StatementN != 2 {
		t.Fatalf("mixed tags/params produced %d statements, want 2", m.StatementN)
	}
}

func TestDestructiveAndIrreversibleFlags(t *testing.T) {
	m := Parse("delete from public.t;", "t")
	if !m.HasDestructive {
		t.Error("expected HasDestructive=true for DELETE")
	}
	if !m.HasIrreversible {
		t.Error("expected HasIrreversible=true for DELETE")
	}
}

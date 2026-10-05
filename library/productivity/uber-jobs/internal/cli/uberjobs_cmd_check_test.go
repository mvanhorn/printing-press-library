// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// check: id parsing, liveness statuses, and the local-first source rule.

package cli

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

// TestUJCParsePostingIDs pins the id grammar: commas and any whitespace
// separate, duplicates keep their first position, and one bad token fails
// the whole call (a typo must not silently drop an id).
func TestUJCParsePostingIDs(t *testing.T) {
	got, err := parsePostingIDs([]string{"303232,302016", "303232", " 155579 "}, strings.NewReader(""))
	if err != nil || strings.Join(got, ",") != "303232,302016,155579" {
		t.Fatalf("args: %v %v, want [303232 302016 155579]", got, err)
	}
	got, err = parsePostingIDs([]string{"-"}, strings.NewReader("303232, 302016\n\t155579\r\n303232,,\n"))
	if err != nil || strings.Join(got, ",") != "303232,302016,155579" {
		t.Fatalf("stdin: %v %v, want [303232 302016 155579]", got, err)
	}
	for _, bad := range [][]string{{"303232", "abc"}, {"30323x"}, {"https://jobs.uber.com/en/jobs/303232/"}, {"-12"}, {"1234567890123"}} {
		if _, err := parsePostingIDs(bad, strings.NewReader("")); ujcCode(err) != 2 {
			t.Fatalf("parsePostingIDs(%q): err = %v, want a usage error", bad, err)
		}
	}
	if _, err := parsePostingIDs([]string{"-"}, strings.NewReader("303232 nope")); ujcCode(err) != 2 {
		t.Fatalf("bad stdin token: err = %v, want a usage error", err)
	}
	// "-" is stdin only when it is the sole argument.
	if _, err := parsePostingIDs([]string{"303232", "-"}, strings.NewReader("302016")); ujcCode(err) != 2 {
		t.Fatalf("'-' among ids: err = %v, want a usage error", err)
	}
}

// ujcCheckRows indexes check rows by id.
func ujcCheckRows(rows []checkRow) map[string]checkRow {
	out := map[string]checkRow{}
	for _, r := range rows {
		out[r.ID] = r
	}
	return out
}

// TestUJCCheckLiveStatuses: a complete live read answers open, closed (with
// the store's closed_on for a posting it saw close), and never_seen; an
// incomplete read answers unknown for anything unlisted, never closed.
func TestUJCCheckLiveStatuses(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	start := time.Now().Add(-2 * time.Hour)
	closedAt := start.Add(time.Hour)
	ujcWithStore(t, ujcDefaultDB(), func(db *sql.DB) {
		ujcApply(t, db, site.Postings(t), "all", true, start)
		site.Drop(ujcIDMidNLD)
		ujcApply(t, db, site.Postings(t), "all", true, closedAt)
	})
	// A posting the store saw but never saw close: dropped only on the site.
	site.Drop(ujcIDOldNLD)
	ids := []string{ujcIDNewestNLD, ujcIDMidNLD, ujcIDOldNLD, "999999999"}
	ctx := context.Background()
	now := time.Now().UTC()

	env, rows, err := runCheck(ctx, ujcFlags(), "live", "live", "", ids, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if env.Meta.Source != uberjobs.SourceSite || !env.Meta.Complete || env.Meta.Requests != 2 || env.Scanned != 54 {
		t.Fatalf("meta = %+v scanned=%d, want a complete two-request site read of 54", env.Meta, env.Scanned)
	}
	byID := ujcCheckRows(rows)
	if len(rows) != 4 || rows[0].ID != ujcIDNewestNLD || rows[3].ID != "999999999" {
		t.Fatalf("rows out of input order: %v", rows)
	}
	if r := byID[ujcIDNewestNLD]; r.Status != "open" || r.Title == nil || r.URL == nil || r.FirstSeen == nil || r.ClosedOn != nil {
		t.Fatalf("open row = %+v, want open with title, url, and history", r)
	}
	wantClosed := closedAt.UTC().Format(time.RFC3339)
	if r := byID[ujcIDMidNLD]; r.Status != "closed" || r.ClosedOn == nil || *r.ClosedOn != wantClosed || r.Title == nil {
		t.Fatalf("closed row = %+v, want closed on %s with its stored title", r, wantClosed)
	}
	if r := byID[ujcIDOldNLD]; r.Status != "closed" || r.ClosedOn != nil {
		t.Fatalf("unlisted known row = %+v, want closed with no recorded day", r)
	}
	if r := byID["999999999"]; r.Status != "never_seen" || r.Title != nil || r.FirstSeen != nil {
		t.Fatalf("unknown id row = %+v, want never_seen with no history", r)
	}
	for _, r := range rows {
		if r.AsOf == nil || !strings.HasPrefix(r.Basis, "live read of the whole corpus") {
			t.Fatalf("row %s basis = %q as_of = %v", r.ID, r.Basis, r.AsOf)
		}
	}

	// Incomplete read: nothing unlisted may be called closed or never_seen.
	site.SetTotalBump(5)
	env, rows, err = runCheck(ctx, ujcFlags(), "live", "live", "", ids, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if env.Meta.Complete || !strings.Contains(env.Meta.Note, "unknown, not closed") {
		t.Fatalf("incomplete meta = %+v, want complete=false and a note", env.Meta)
	}
	byID = ujcCheckRows(rows)
	if byID[ujcIDNewestNLD].Status != "open" {
		t.Fatalf("listed id in a partial read = %+v, want open", byID[ujcIDNewestNLD])
	}
	for _, id := range []string{ujcIDMidNLD, ujcIDOldNLD, "999999999"} {
		if r := byID[id]; r.Status != "unknown" || r.ClosedOn != nil || !strings.HasPrefix(r.Basis, "partial live read") {
			t.Fatalf("partial read row %s = %+v, want unknown", id, r)
		}
	}
}

// TestUJCCheckCommandSources runs check through RootCmd: ids from stdin,
// auto answering from a fresh sync with no request, --max-age forcing a
// live read, and a usage error for a bad token.
func TestUJCCheckCommandSources(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	ujcSeedFullSync(t, ujcDefaultDB(), site, time.Now().Add(-90*time.Minute))

	r := ujcRun(t, "303232, 302016\n303232 155579\n", "check", "-", "--json")
	ujcWantCode(t, r, 0)
	env := ujcEnvelope(t, r.Stdout)
	if got := strings.Join(ujcRowIDs(ujcRows(env)), ","); got != "303232,302016,155579" {
		t.Fatalf("stdin ids = %s, want 303232,302016,155579 (deduped, in order)", got)
	}
	if m := ujcMeta(env); m["source"] != uberjobs.SourceLocal || m["requests"] != float64(0) {
		t.Fatalf("auto with a 90-minute-old sync: meta = %v, want local with no requests", m)
	}
	for _, row := range ujcRows(env) {
		if row["status"] != "open" || !strings.HasPrefix(row["basis"].(string), "local store, last complete sync") {
			t.Fatalf("local row = %v, want open from the store", row)
		}
	}
	if n := site.Count(""); n != 0 {
		t.Fatalf("auto with a fresh sync sent %d requests, want 0", n)
	}

	ujcWantCode(t, ujcRun(t, "", "check", "303232", "x1", "--json"), 2)
	ujcWantCode(t, ujcRun(t, "303232 nope", "check", "-", "--json"), 2)
	ujcWantCode(t, ujcRun(t, "", "check", "303232", "--max-age", "soon", "--json"), 2)
	if n := site.Count(""); n != 0 {
		t.Fatalf("usage errors sent %d requests, want 0", n)
	}

	// --max-age 1h makes the 90-minute-old sync too stale, so auto reads live.
	r = ujcRun(t, "", "check", ujcIDNewestNLD, "999999999", "--max-age", "1h", "--json")
	ujcWantCode(t, r, 0)
	env = ujcEnvelope(t, r.Stdout)
	if m := ujcMeta(env); m["source"] != uberjobs.SourceSite || m["complete"] != true {
		t.Fatalf("--max-age 1h meta = %v, want a complete live read", m)
	}
	rows := ujcRows(env)
	if rows[0]["status"] != "open" || rows[0]["first_seen"] == nil || rows[1]["status"] != "never_seen" {
		t.Fatalf("--max-age 1h rows = %v, want open (with stored history) and never_seen", rows)
	}
	if n := site.Count("/api/jobs/search/"); n != 2 {
		t.Fatalf("live check sent %d search requests, want 2", n)
	}
}

// TestUJCCheckBothHostsRefuse: with --data-source live and both the site
// and the Oracle fallback refusing, check exits 7 and prints nothing.
func TestUJCCheckBothHostsRefuse(t *testing.T) {
	site, oracle := ujcNewFake(t), ujcNewFake(t)
	site.SetMode("challenge")
	oracle.SetOracleMode("challenge")
	ujcIsolate(t, site.URL, oracle.URL)
	r := ujcRun(t, "", "check", ujcIDNewestNLD, "--data-source", "live", "--json")
	ujcWantCode(t, r, 7)
	if r.Stdout != "" {
		t.Fatalf("a refused check printed output: %s", r.Stdout)
	}
	if !strings.Contains(r.Err.Error(), "Oracle fallback also failed") {
		t.Fatalf("error = %v, want both refusals named", r.Err)
	}
	if site.Count("") != 1 || oracle.Count("") != 1 {
		t.Fatalf("site saw %d and Oracle %d requests, want exactly 1 each (never retried)", site.Count(""), oracle.Count(""))
	}
}

// TestUJCCheckLocalWithoutSync: --data-source local with history but no
// complete sync cannot prove anything, so every id is unknown.
func TestUJCCheckLocalWithoutSync(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, "", "")
	ujcWithStore(t, ujcDefaultDB(), func(db *sql.DB) {
		// A keyword-scoped read stores rows but is not a full sync.
		ujcApply(t, db, site.Postings(t), "query:ops", true, time.Now())
	})
	r := ujcRun(t, "", "check", ujcIDNewestNLD, "999999999", "--data-source", "local", "--json")
	ujcWantCode(t, r, 0)
	env := ujcEnvelope(t, r.Stdout)
	for _, row := range ujcRows(env) {
		if row["status"] != "unknown" {
			t.Fatalf("row %v status = %v, want unknown without a complete sync", row["id"], row["status"])
		}
	}
	if ujcMeta(env)["complete"] != false {
		t.Fatalf("meta.complete = %v, want false", ujcMeta(env)["complete"])
	}
}

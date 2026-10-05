// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// save, searches, and new: append-only membership diffs (B7).

package cli

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

// ujcNewRig drives runNewSearch directly against a held-open store with a
// gate-free client, so multi-step membership histories run instantly.
type ujcNewRig struct {
	t    *testing.T
	site *ujcFake
	db   *sql.DB
}

func ujcNewNewRig(t *testing.T) *ujcNewRig {
	t.Helper()
	return &ujcNewRig{t: t, site: ujcNewFake(t), db: ujcOpenStore(t, filepath.Join(t.TempDir(), "new.db"))}
}

func (r *ujcNewRig) save(name string, f uberjobs.Filters) {
	r.t.Helper()
	if _, _, err := uberjobs.SaveSearch(context.Background(), r.db, name, f, time.Now()); err != nil {
		r.t.Fatal(err)
	}
}

func (r *ujcNewRig) search(name string) uberjobs.SavedSearch {
	r.t.Helper()
	sv, err := uberjobs.GetSearch(context.Background(), r.db, name)
	if err != nil || sv == nil {
		r.t.Fatalf("saved search %q: %v %v", name, sv, err)
	}
	return *sv
}

// run diffs one saved search; c nil uses a fresh client for the site fake.
func (r *ujcNewRig) run(name, ds string, limit int, c *uberjobs.Client) newSearchRow {
	r.t.Helper()
	if c == nil {
		c = ujcClient(r.site, nil)
	}
	row, _, err := runNewSearch(context.Background(), ujcFlags(), &c, r.db, ds, r.search(name), limit, time.Now().UTC())
	if err != nil {
		r.t.Fatalf("runNewSearch(%s, %s): %v", name, ds, err)
	}
	return row
}

func ujcAddedIDs(row newSearchRow) []string {
	out := make([]string, 0, len(row.Added))
	for _, a := range row.Added {
		out = append(out, a.ID)
	}
	return out
}

func ujcRemovedIDs(row newSearchRow) []string {
	out := make([]string, 0, len(row.Removed))
	for _, rm := range row.Removed {
		out = append(out, rm.ID)
	}
	return out
}

// TestUJCNewMembershipLifecycle walks one filtered search through baseline,
// removal (store never saw the close), reopen, and a close the store did
// record.
func TestUJCNewMembershipLifecycle(t *testing.T) {
	rig := ujcNewNewRig(t)
	rig.save("gbr", uberjobs.Filters{Countries: []string{"GBR"}})

	row := rig.run("gbr", "live", 50, nil)
	if !row.BaselineEstablished || !row.BaselineAdvanced || row.CurrentCount != 7 || len(row.Added) != 0 || row.AddedCount != 0 {
		t.Fatalf("first run = %+v, want a 7-posting baseline with nothing listed", row)
	}
	if got := rig.search("gbr"); got.BaselineSize != 7 || got.BaselineAt == nil || got.LastAdvancedAt == nil {
		t.Fatalf("after baseline: %+v, want baseline_size 7 with stamps", got)
	}

	// Two members leave and none join. The store holds no posting history,
	// so it cannot say they closed: status is removed.
	rig.site.Drop(ujcIDGBRAccount, "300506")
	row = rig.run("gbr", "live", 50, nil)
	if row.BaselineEstablished || !row.BaselineAdvanced || row.RemovedCount != 2 || row.AddedCount != 0 || row.CurrentCount != 5 {
		t.Fatalf("after two closures = %+v, want 2 removed, 0 added, advanced", row)
	}
	if got := strings.Join(ujcRemovedIDs(row), ","); got != "300126,300506" {
		t.Fatalf("removed ids = %s, want sorted 300126,300506", got)
	}
	for _, rm := range row.Removed {
		if rm.Status != "removed" || rm.ClosedOn != nil || rm.Title == nil || rm.FirstSeen == "" {
			t.Fatalf("removed row = %+v, want status removed, no closed_on, title and first_seen kept", rm)
		}
	}
	if got := rig.search("gbr").BaselineSize; got != 5 {
		t.Fatalf("baseline_size after removal = %d, want 5", got)
	}

	// One comes back: added with reopened, and the one still gone is not
	// reported again.
	rig.site.Undrop(ujcIDGBRAccount)
	row = rig.run("gbr", "live", 50, nil)
	if got := ujcAddedIDs(row); len(got) != 1 || got[0] != ujcIDGBRAccount || !row.Added[0].Reopened || row.AddedCount != 1 {
		t.Fatalf("reopen run added = %v (%+v), want %s reopened", got, row.Added, ujcIDGBRAccount)
	}
	if row.RemovedCount != 0 {
		t.Fatalf("reopen run removed = %v, want none (300506 was already reported)", ujcRemovedIDs(row))
	}

	// A sync records two closures; new then reports them as closed with
	// the store's closed_on.
	t1 := time.Now().Add(-time.Hour)
	ujcApply(t, rig.db, rig.site.Postings(t), "all", true, t1)
	rig.site.Drop("302718", "300921")
	t2 := t1.Add(30 * time.Minute)
	ujcApply(t, rig.db, rig.site.Postings(t), "all", true, t2)
	row = rig.run("gbr", "live", 50, nil)
	if got := strings.Join(ujcRemovedIDs(row), ","); got != "300921,302718" {
		t.Fatalf("removed after sync = %s, want 300921,302718", got)
	}
	want := t2.UTC().Format(time.RFC3339)
	for _, rm := range row.Removed {
		if rm.Status != "closed" || rm.ClosedOn == nil || *rm.ClosedOn != want {
			t.Fatalf("removed row = %+v, want status closed with the store's closed_on %s", rm, want)
		}
	}
}

// TestUJCNewWholeCorpusRemovalIsClosed: a search with no filters read the
// whole corpus, so a member that vanished closed, even with no history.
func TestUJCNewWholeCorpusRemovalIsClosed(t *testing.T) {
	rig := ujcNewNewRig(t)
	rig.save("everything", uberjobs.Filters{})
	if row := rig.run("everything", "live", 50, nil); !row.BaselineEstablished || row.CurrentCount != 56 {
		t.Fatalf("baseline = %+v, want 56 postings", row)
	}
	rig.site.Drop(ujcIDNewestNLD)
	row := rig.run("everything", "live", 50, nil)
	today := time.Now().UTC().Format("2006-01-02")
	if len(row.Removed) != 1 || row.Removed[0].ID != ujcIDNewestNLD || row.Removed[0].Status != "closed" ||
		row.Removed[0].ClosedOn == nil || !strings.HasPrefix(*row.Removed[0].ClosedOn, today) {
		t.Fatalf("whole-corpus removal = %+v, want %s closed today", row.Removed, ujcIDNewestNLD)
	}
}

// TestUJCNewIncompleteReadDoesNotAdvance: an incomplete read lists
// additions but never removals, never advances, and the next complete read
// reports the same additions again.
func TestUJCNewIncompleteReadDoesNotAdvance(t *testing.T) {
	rig := ujcNewNewRig(t)
	rig.save("nld", uberjobs.Filters{Countries: []string{"NLD"}})
	rig.site.Drop(ujcIDNewestNLD)
	if row := rig.run("nld", "live", 50, nil); !row.BaselineEstablished || row.CurrentCount != 4 {
		t.Fatalf("baseline = %+v, want 4 postings", row)
	}
	advanced := *rig.search("nld").LastAdvancedAt

	rig.site.Undrop(ujcIDNewestNLD)
	rig.site.Drop("302409")
	rig.site.SetTotalBump(3)
	row := rig.run("nld", "live", 50, nil)
	if row.Complete || row.BaselineAdvanced || row.RemovedCount != 0 || len(row.Removed) != 0 {
		t.Fatalf("incomplete run = %+v, want no removals and no advance", row)
	}
	if got := ujcAddedIDs(row); len(got) != 1 || got[0] != ujcIDNewestNLD || row.AddedCount != 1 {
		t.Fatalf("incomplete run added = %v, want [%s]", got, ujcIDNewestNLD)
	}
	if !strings.Contains(row.Note, "removals were not computed") {
		t.Fatalf("incomplete run note = %q", row.Note)
	}
	sv := rig.search("nld")
	if sv.BaselineSize != 4 || *sv.LastAdvancedAt != advanced || sv.LastCheckedAt == nil {
		t.Fatalf("after incomplete run: %+v, want membership untouched and the check recorded", sv)
	}

	rig.site.SetTotalBump(0)
	row = rig.run("nld", "live", 50, nil)
	if got := ujcAddedIDs(row); len(got) != 1 || got[0] != ujcIDNewestNLD {
		t.Fatalf("next complete run added = %v, want the same addition %s", got, ujcIDNewestNLD)
	}
	if got := ujcRemovedIDs(row); len(got) != 1 || got[0] != "302409" || !row.BaselineAdvanced {
		t.Fatalf("next complete run removed = %v advanced=%v, want [302409] and an advance", got, row.BaselineAdvanced)
	}
}

// TestUJCNewPostedWithinNeverRemovesAgingMembers: the saved window limits
// which additions show; a listed member aging past it is never removed.
func TestUJCNewPostedWithinNeverRemovesAgingMembers(t *testing.T) {
	rig := ujcNewNewRig(t)
	rig.save("nld7", uberjobs.Filters{Countries: []string{"NLD"}, PostedWithin: "7d"})
	rig.site.Drop(ujcIDNewestNLD, ujcIDMidNLD)
	row := rig.run("nld7", "live", 50, nil)
	// 151342 is about 82 days old, far outside 7d, yet a member.
	if !row.BaselineEstablished || row.CurrentCount != 3 || rig.search("nld7").BaselineSize != 3 {
		t.Fatalf("baseline = %+v, want 3 members including the old %s", row, ujcIDOldNLD)
	}

	row = rig.run("nld7", "live", 50, nil)
	if row.RemovedCount != 0 || row.AddedCount != 0 || !row.BaselineAdvanced || rig.search("nld7").BaselineSize != 3 {
		t.Fatalf("unchanged run = %+v, want nothing removed while %s is still listed", row, ujcIDOldNLD)
	}

	// Both return: only the in-window one is shown, both become members.
	rig.site.Undrop(ujcIDNewestNLD, ujcIDMidNLD)
	row = rig.run("nld7", "live", 50, nil)
	if got := ujcAddedIDs(row); len(got) != 1 || got[0] != ujcIDNewestNLD || row.AddedCount != 1 {
		t.Fatalf("windowed additions = %v (count %d), want only %s", got, row.AddedCount, ujcIDNewestNLD)
	}
	if got := rig.search("nld7").BaselineSize; got != 5 {
		t.Fatalf("baseline_size = %d, want 5 (the out-of-window posting is a member too)", got)
	}
	if row = rig.run("nld7", "live", 50, nil); row.AddedCount != 0 || row.RemovedCount != 0 {
		t.Fatalf("follow-up run = %+v, want no changes", row)
	}
}

// TestUJCNewAddedCountBeforeLimit: added_count counts every addition, and
// --limit only truncates the listed rows (newest first).
func TestUJCNewAddedCountBeforeLimit(t *testing.T) {
	rig := ujcNewNewRig(t)
	rig.save("usa", uberjobs.Filters{Countries: []string{"USA"}})
	rig.site.Drop("302500", "302906", "300629")
	rig.run("usa", "live", 1, nil)
	rig.site.Undrop("302500", "302906", "300629")
	row := rig.run("usa", "live", 1, nil)
	if row.AddedCount != 3 || len(row.Added) != 1 || !row.AddedTruncated {
		t.Fatalf("limit 1: added_count=%d listed=%d truncated=%v, want 3/1/true", row.AddedCount, len(row.Added), row.AddedTruncated)
	}
	if row.Added[0].ID != "302500" {
		t.Fatalf("listed addition = %s, want the newest (302500)", row.Added[0].ID)
	}
	if !strings.Contains(row.Note, "listing 1 of 3") || !row.BaselineAdvanced {
		t.Fatalf("note = %q advanced=%v", row.Note, row.BaselineAdvanced)
	}
}

// TestUJCNewOracleFallback: on a site refusal, a keyword search is not
// diffed (Oracle's keyword engine differs) while a country-only search is.
func TestUJCNewOracleFallback(t *testing.T) {
	rig := ujcNewNewRig(t)
	oracle := ujcNewFake(t)
	rig.save("kw", uberjobs.Filters{Query: "Security"})
	rig.save("gbr", uberjobs.Filters{Countries: []string{"GBR"}})
	rig.run("kw", "live", 50, nil)
	rig.run("gbr", "live", 50, nil)
	kwAdvanced := *rig.search("kw").LastAdvancedAt

	rig.site.SetMode("challenge")
	oracle.Drop(ujcIDGBRAccount)
	c := ujcClient(rig.site, oracle)

	row := rig.run("kw", "live", 50, c)
	if row.Source != uberjobs.SourceOracle || row.BaselineAdvanced || len(row.Added) != 0 || len(row.Removed) != 0 {
		t.Fatalf("keyword search on fallback = %+v, want not diffed", row)
	}
	if !strings.Contains(row.Note, "keyword search is a different engine") {
		t.Fatalf("keyword fallback note = %q", row.Note)
	}
	if sv := rig.search("kw"); *sv.LastAdvancedAt != kwAdvanced || sv.BaselineSize == 0 {
		t.Fatalf("keyword search advanced on a fallback: %+v", sv)
	}

	row = rig.run("gbr", "live", 50, c)
	if row.Source != uberjobs.SourceOracle || !row.Complete || !row.BaselineAdvanced {
		t.Fatalf("country search on fallback = %+v, want a complete oracle-ce diff", row)
	}
	if got := ujcRemovedIDs(row); len(got) != 1 || got[0] != ujcIDGBRAccount || row.Removed[0].Status != "removed" || row.AddedCount != 0 {
		t.Fatalf("oracle diff removed = %v added = %v, want [%s] removed only", got, ujcAddedIDs(row), ujcIDGBRAccount)
	}
	if n := rig.site.Count("/api/jobs/search/"); n != 5 {
		t.Fatalf("site saw %d search requests, want 4 for the baselines plus 1 refusal", n)
	}
}

// TestUJCNewLocalSource covers --data-source local: no sync means no diff,
// a snapshot older than the last advance is refused, keyword searches are
// refused offline, and a newer snapshot diffs with the store's closures.
func TestUJCNewLocalSource(t *testing.T) {
	rig := ujcNewNewRig(t)
	rig.save("gbr", uberjobs.Filters{Countries: []string{"GBR"}})
	rig.save("kw", uberjobs.Filters{Query: "Security", Countries: []string{"USA"}})

	row := rig.run("gbr", "local", 50, nil)
	if row.BaselineEstablished || row.BaselineAdvanced || !strings.Contains(row.Note, "no baseline taken") || !strings.Contains(row.Note, "no complete sync") {
		t.Fatalf("local with no sync = %+v, want no baseline and a note", row)
	}

	// Baseline from a sync an hour old, then a live advance now: the old
	// snapshot predates the advance and must not be diffed.
	ujcApply(t, rig.db, rig.site.Postings(t), "all", true, time.Now().Add(-time.Hour))
	if row = rig.run("gbr", "live", 50, nil); !row.BaselineEstablished {
		t.Fatalf("live baseline = %+v", row)
	}
	row = rig.run("gbr", "local", 50, nil)
	if row.BaselineAdvanced || !strings.Contains(row.Note, "older than this search's last advance") {
		t.Fatalf("stale local snapshot = %+v, want not diffed", row)
	}

	if row = rig.run("kw", "local", 50, nil); row.BaselineEstablished || !strings.Contains(row.Note, "cannot be applied exactly offline") {
		t.Fatalf("keyword search offline = %+v, want refused", row)
	}

	// A fresh search baselined from the store, then a newer sync that
	// closed one member: the local diff reports it closed.
	rig.save("nld", uberjobs.Filters{Countries: []string{"NLD"}})
	if row = rig.run("nld", "local", 50, nil); !row.BaselineEstablished || row.Source != uberjobs.SourceLocal || row.CurrentCount != 5 {
		t.Fatalf("local baseline = %+v, want 5 NLD postings from the store", row)
	}
	rig.site.Drop(ujcIDMidNLD)
	ujcApply(t, rig.db, rig.site.Postings(t), "all", true, time.Now().Add(-30*time.Minute))
	row = rig.run("nld", "local", 50, nil)
	if got := ujcRemovedIDs(row); len(got) != 1 || got[0] != ujcIDMidNLD || row.Removed[0].Status != "closed" || !row.BaselineAdvanced {
		t.Fatalf("local diff = %+v, want %s closed and an advance", row, ujcIDMidNLD)
	}
}

// TestUJCNewCommandUsage covers the RootCmd-only paths of new.
func TestUJCNewCommandUsage(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")

	ujcWantCode(t, ujcRun(t, "", "new", "--json"), 2)
	ujcWantCode(t, ujcRun(t, "", "new", "gbr", "--all", "--json"), 2)
	ujcWantCode(t, ujcRun(t, "", "new", "a", "b", "--json"), 2)
	ujcWantCode(t, ujcRun(t, "", "new", "--all", "--limit", "-1", "--json"), 2)
	ujcWantCode(t, ujcRun(t, "", "new", "--all", "--data-source", "cache", "--json"), 2)

	r := ujcRun(t, "", "new", "nope", "--json", "--data-source", "live")
	ujcWantCode(t, r, 3)
	if !strings.Contains(r.Err.Error(), `no saved search named "nope"`) {
		t.Fatalf("unknown search error = %v", r.Err)
	}

	r = ujcRun(t, "", "new", "--all", "--json", "--data-source", "live")
	ujcWantCode(t, r, 0)
	env := ujcEnvelope(t, r.Stdout)
	if note, _ := ujcMeta(env)["note"].(string); len(ujcRows(env)) != 0 || !strings.Contains(note, "no saved searches") {
		t.Fatalf("empty --all = %v, want empty results and a note", env)
	}
	if n := site.Count(""); n != 0 {
		t.Fatalf("new usage paths sent %d requests, want 0", n)
	}
}

// TestUJCSaveAndSearches runs save and searches through RootCmd.
func TestUJCSaveAndSearches(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")

	r := ujcRun(t, "", "save", "gbr", "", "--country", "gbr", "--sort", "recent", "--json")
	ujcWantCode(t, r, 0)
	env := ujcEnvelope(t, r.Stdout)
	rows := ujcRows(env)
	filters, _ := rows[0]["filters"].(map[string]any)
	if len(rows) != 1 || rows[0]["name"] != "gbr" || rows[0]["reset"] != false || rows[0]["baseline_at"] != nil || rows[0]["baseline_size"] != float64(0) {
		t.Fatalf("save row = %v", rows)
	}
	if c, _ := filters["countries"].([]any); len(filters) != 1 || len(c) != 1 || c[0] != "GBR" {
		t.Fatalf("stored filters = %v, want only countries [GBR] (no query, no sort)", filters)
	}
	if note, _ := ujcMeta(env)["note"].(string); !strings.Contains(note, "no baseline yet") {
		t.Fatalf("save note = %q", note)
	}

	// Same filters (sort still ignored) keep the search; changed filters reset it.
	r = ujcRun(t, "", "save", "gbr", "", "--country", "GBR", "--sort", "relevant", "--json")
	ujcWantCode(t, r, 0)
	if rows := ujcRows(ujcEnvelope(t, r.Stdout)); rows[0]["reset"] != false {
		t.Fatalf("re-save with only --sort changed reset the baseline: %v", rows[0])
	}

	// Take a baseline so searches has a size to show.
	ujcWithStore(t, ujcDefaultDB(), func(db *sql.DB) {
		sv, err := uberjobs.GetSearch(context.Background(), db, "gbr")
		if err != nil || sv == nil {
			t.Fatalf("saved search: %v %v", sv, err)
		}
		c := ujcClient(site, nil)
		if row, _, err := runNewSearch(context.Background(), ujcFlags(), &c, db, "live", *sv, 50, time.Now().UTC()); err != nil || !row.BaselineEstablished {
			t.Fatalf("baseline: %+v %v", row, err)
		}
	})
	r = ujcRun(t, "", "searches", "--json")
	ujcWantCode(t, r, 0)
	rows = ujcRows(ujcEnvelope(t, r.Stdout))
	if len(rows) != 1 || rows[0]["baseline_size"] != float64(7) || rows[0]["last_advanced_at"] == nil {
		t.Fatalf("searches rows = %v, want gbr with baseline_size 7", rows)
	}

	r = ujcRun(t, "", "save", "gbr", "", "--country", "NLD", "--json")
	ujcWantCode(t, r, 0)
	env = ujcEnvelope(t, r.Stdout)
	if rows := ujcRows(env); rows[0]["reset"] != true || rows[0]["baseline_size"] != float64(0) || rows[0]["baseline_at"] != nil {
		t.Fatalf("changed filters row = %v, want reset with the baseline dropped", rows[0])
	}
	if note, _ := ujcMeta(env)["note"].(string); !strings.Contains(note, "filters changed") {
		t.Fatalf("reset note = %q", note)
	}

	for _, args := range [][]string{
		{"save", "bad name", "--country", "GBR", "--json"},
		{"save", ".hidden", "--country", "GBR", "--json"},
		{"save", strings.Repeat("a", 65), "--country", "GBR", "--json"},
		{"save", "--country", "GBR", "--json"},
		{"save", "x", "strategy", "--base-query", "ops", "--json"},
		{"save", "x", "--country", "Atlantis", "--json"},
		{"searches", "extra", "--json"},
		{"searches", "--delete", " ", "--json"},
	} {
		ujcWantCode(t, ujcRun(t, "", args...), 2)
	}

	ujcWantCode(t, ujcRun(t, "", "searches", "--delete", "nope", "--json"), 3)
	r = ujcRun(t, "", "searches", "--delete", "gbr", "--json")
	ujcWantCode(t, r, 0)
	if rows := ujcRows(ujcEnvelope(t, r.Stdout)); len(rows) != 1 || rows[0]["id"] != "gbr" || rows[0]["deleted"] != true {
		t.Fatalf("delete rows = %v", rows)
	}
	r = ujcRun(t, "", "searches", "--json")
	ujcWantCode(t, r, 0)
	if rows := ujcRows(ujcEnvelope(t, r.Stdout)); len(rows) != 0 {
		t.Fatalf("searches after delete = %v, want none", rows)
	}
	ujcWantCode(t, ujcRun(t, "", "searches", "--delete", "gbr", "--json"), 3)
	if n := site.Count(""); n != 2 {
		t.Fatalf("fake saw %d requests, want only the 2 for the baseline", n)
	}
}

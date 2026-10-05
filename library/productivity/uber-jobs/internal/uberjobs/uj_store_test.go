// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// ujPosting builds a minimal site posting with one German location.
func ujPosting(id, title string) Posting {
	return Posting{
		ID: id, Title: title, Employer: "uber", Source: SourceSite,
		JobCategory: ujStrp("Engineer"), SubTeam: ujStrp("Software Engineering"),
		CountryCode: ujStrp("DEU"), Country: ujStrp("Germany"),
		Locations:    []Location{{City: ujStrp("Berlin"), Country: ujStrp("Germany"), CountryCode: ujStrp("DEU")}},
		Description:  ujStrp("Full description of " + title),
		SalaryRanges: []SalaryRange{},
		JobPath:      "/en/jobs/" + id + "/", URL: DefaultBaseURL + "/en/jobs/" + id + "/",
	}
}

func ujRun(scope string, complete, capped bool, at time.Time) SyncRun {
	return SyncRun{StartedAt: ujStamp(at), Source: SourceSite, Scope: scope, Complete: complete, ScanCapHit: capped}
}

func ujLoad(t *testing.T, db *sql.DB, openOnly bool) map[string]StoredPosting {
	t.Helper()
	rows, err := LoadPostings(context.Background(), db, openOnly)
	if err != nil {
		t.Fatalf("LoadPostings: %v", err)
	}
	out := map[string]StoredPosting{}
	for _, r := range rows {
		out[r.ID] = r
	}
	return out
}

func ujApply(t *testing.T, db *sql.DB, ps []Posting, run SyncRun, at time.Time, opts ApplyOptions) (SyncRun, ApplyStats) {
	t.Helper()
	got, stats, err := ApplyRead(context.Background(), db, ps, run, at, opts)
	if err != nil {
		t.Fatalf("ApplyRead(%s): %v", run.Scope, err)
	}
	return got, stats
}

// TestEnsureSchemaIdempotent: the uj_ tables are created once and a second
// call on an existing store is a no-op.
func TestEnsureSchemaIdempotent(t *testing.T) {
	db := ujOpenDB(t)
	if err := EnsureSchema(context.Background(), db); err != nil {
		t.Fatalf("second EnsureSchema: %v", err)
	}
	for _, table := range []string{"uj_postings", "uj_posting_locations", "uj_sync_runs", "uj_saved_searches", "uj_saved_search_members", "uj_facets"} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&n); err != nil || n != 1 {
			t.Errorf("table %s: count=%d err=%v", table, n, err)
		}
	}
}

// TestEmptyStore: an empty store reads as no rows and no runs, never as an
// error, and LoadPostings returns a non-nil slice.
func TestEmptyStore(t *testing.T) {
	db := ujOpenDB(t)
	ctx := context.Background()
	rows, err := LoadPostings(ctx, db, false)
	if err != nil || rows == nil || len(rows) != 0 {
		t.Errorf("LoadPostings = %v, %v; want empty non-nil", rows, err)
	}
	if run, err := LastFullSync(ctx, db); run != nil || err != nil {
		t.Errorf("LastFullSync = %v, %v; want nil, nil", run, err)
	}
	if _, ok := FirstSyncAt(ctx, db); ok {
		t.Errorf("FirstSyncAt reported a sync on an empty store")
	}
	if n, err := CountPostings(ctx, db); n != 0 || err != nil {
		t.Errorf("CountPostings = %d, %v", n, err)
	}
	if f, at, err := LatestFacets(ctx, db); f != nil || at != "" || err != nil {
		t.Errorf("LatestFacets = %v %q %v; want nil", f, at, err)
	}
	if s, err := GetSearch(ctx, db, "nope"); s != nil || err != nil {
		t.Errorf("GetSearch(unknown) = %v, %v; want nil, nil", s, err)
	}
	if list, err := ListSearches(ctx, db); err != nil || len(list) != 0 {
		t.Errorf("ListSearches = %v, %v", list, err)
	}
}

// TestApplyReadLifecycle walks insert, close on a complete full read, and
// reopen when a posting returns; first_seen never moves.
func TestApplyReadLifecycle(t *testing.T) {
	db := ujOpenDB(t)
	t0, t1, t2 := ujT0, ujT0.Add(24*time.Hour), ujT0.Add(48*time.Hour)
	a, b, c := ujPosting("100", "Alpha"), ujPosting("200", "Bravo"), ujPosting("300", "Charlie")
	c.Locations = append(c.Locations, Location{City: ujStrp("Riyadh"), Country: ujStrp("Saudi Arabia"), CountryCode: ujStrp("SAU")})

	run, stats := ujApply(t, db, []Posting{a, b, c}, ujRun("all", true, false, t0), t0, ApplyOptions{})
	if stats.Inserted != 3 || stats.Updated != 0 || run.ID == 0 || run.ClosedMarked != 0 || run.FinishedAt != ujStamp(t0) {
		t.Errorf("first apply: stats=%+v run=%+v", stats, run)
	}
	got := ujLoad(t, db, false)
	if len(got) != 3 || got["100"].FirstSeen != ujStamp(t0) || got["100"].LastSeen != ujStamp(t0) || got["100"].ClosedOn != nil {
		t.Fatalf("after insert: %+v", got)
	}
	if ujS(got["300"].Description) != "Full description of Charlie" || len(got["300"].Locations) != 2 {
		t.Errorf("posting data did not round-trip: %+v", got["300"].Posting)
	}
	var locs int
	if err := db.QueryRow(`SELECT COUNT(*) FROM uj_posting_locations WHERE posting_id = '300'`).Scan(&locs); err != nil || locs != 2 {
		t.Errorf("uj_posting_locations for 300 = %d (%v), want 2", locs, err)
	}
	var team, iso string
	if err := db.QueryRow(`SELECT team, country_iso3 FROM uj_postings WHERE id = '100'`).Scan(&team, &iso); err != nil || team != "Engineer" || iso != "DEU" {
		t.Errorf("indexed columns = %q %q (%v)", team, iso, err)
	}

	run, stats = ujApply(t, db, []Posting{a, b}, ujRun("all", true, false, t1), t1, ApplyOptions{})
	if run.ClosedMarked != 1 || stats.Updated != 2 || stats.Inserted != 0 {
		t.Errorf("second apply: closed=%d stats=%+v, want 1 closed, 2 updated", run.ClosedMarked, stats)
	}
	got = ujLoad(t, db, false)
	if ujS(got["300"].ClosedOn) != ujStamp(t1) || got["300"].LastSeen != ujStamp(t0) {
		t.Errorf("absent posting: closed_on=%s last_seen=%s, want closed at t1 and last seen t0", ujS(got["300"].ClosedOn), got["300"].LastSeen)
	}
	if got["100"].FirstSeen != ujStamp(t0) || got["100"].LastSeen != ujStamp(t1) {
		t.Errorf("present posting: first=%s last=%s", got["100"].FirstSeen, got["100"].LastSeen)
	}
	if open := ujLoad(t, db, true); len(open) != 2 || open["300"].ID != "" {
		t.Errorf("openOnly returned %d rows incl. closed: %v", len(open), open["300"].ID)
	}

	run, _ = ujApply(t, db, []Posting{a, b, c}, ujRun("all", true, false, t2), t2, ApplyOptions{})
	got = ujLoad(t, db, false)
	if got["300"].ClosedOn != nil || got["300"].FirstSeen != ujStamp(t0) || got["300"].LastSeen != ujStamp(t2) || run.ClosedMarked != 0 {
		t.Errorf("returning posting: closed_on=%s first=%s last=%s closed=%d", ujS(got["300"].ClosedOn), got["300"].FirstSeen, got["300"].LastSeen, run.ClosedMarked)
	}
	if n, _ := CountPostings(context.Background(), db); n != 3 {
		t.Errorf("CountPostings = %d, want 3 (rows are never deleted)", n)
	}
}

// TestApplyReadNeverClosesOnPartialReads: a scoped, Oracle, incomplete, or
// capped read cannot prove absence, so nothing is closed.
func TestApplyReadNeverClosesOnPartialReads(t *testing.T) {
	cases := []SyncRun{
		ujRun("query:engineer", true, false, ujT0),
		ujRun("all-oracle", true, false, ujT0), // sync's scope for a fallback read
		ujRun("all", false, false, ujT0),
		ujRun("all", true, true, ujT0),
		ujRun("all", false, true, ujT0),
	}
	for _, partial := range cases {
		db := ujOpenDB(t)
		a, b := ujPosting("100", "Alpha"), ujPosting("200", "Bravo")
		ujApply(t, db, []Posting{a, b}, ujRun("all", true, false, ujT0), ujT0, ApplyOptions{})
		run, _ := ujApply(t, db, []Posting{a}, partial, ujT0.Add(time.Hour), ApplyOptions{})
		if run.ClosedMarked != 0 {
			t.Errorf("scope=%q complete=%v capped=%v closed %d postings", partial.Scope, partial.Complete, partial.ScanCapHit, run.ClosedMarked)
		}
		if open := ujLoad(t, db, true); len(open) != 2 {
			t.Errorf("scope=%q complete=%v capped=%v: %d open, want 2", partial.Scope, partial.Complete, partial.ScanCapHit, len(open))
		}
	}
}

// TestApplyReadEmptyCompleteReadClosesNothing: a full read with zero rows
// never closes anything, even when it claims complete. The site always lists
// postings, so an empty full read is a bad read (SearchAll and
// OracleSearchAll also refuse one upstream); this is the backstop.
func TestApplyReadEmptyCompleteReadClosesNothing(t *testing.T) {
	db := ujOpenDB(t)
	ujApply(t, db, []Posting{ujPosting("100", "A"), ujPosting("200", "B")}, ujRun("all", true, false, ujT0), ujT0, ApplyOptions{})
	run, _ := ujApply(t, db, []Posting{}, ujRun("all", true, false, ujT0.Add(time.Hour)), ujT0.Add(time.Hour), ApplyOptions{})
	if run.ClosedMarked != 0 || len(ujLoad(t, db, true)) != 2 {
		t.Errorf("closed=%d open=%d, want 0 and 2", run.ClosedMarked, len(ujLoad(t, db, true)))
	}
}

// TestApplyReadPreserveExisting: an Oracle fallback read (no descriptions)
// refreshes last_seen and reopens, but never overwrites stored site data.
func TestApplyReadPreserveExisting(t *testing.T) {
	db := ujOpenDB(t)
	t0, t1, t2 := ujT0, ujT0.Add(time.Hour), ujT0.Add(2*time.Hour)
	a, b := ujPosting("100", "Alpha"), ujPosting("200", "Bravo")
	ujApply(t, db, []Posting{a, b}, ujRun("all", true, false, t0), t0, ApplyOptions{})
	ujApply(t, db, []Posting{a}, ujRun("all", true, false, t1), t1, ApplyOptions{}) // closes 200

	oracleA := Posting{ID: "100", Title: "Alpha (oracle title)", Source: SourceOracle, SalaryRanges: []SalaryRange{}, Locations: []Location{}}
	oracleB := Posting{ID: "200", Title: "Bravo (oracle)", Source: SourceOracle, SalaryRanges: []SalaryRange{}, Locations: []Location{}}
	oracleNew := Posting{ID: "300", Title: "New from oracle", Source: SourceOracle, SalaryRanges: []SalaryRange{}, Locations: []Location{}}
	_, stats := ujApply(t, db, []Posting{oracleA, oracleB, oracleNew}, SyncRun{StartedAt: ujStamp(t2), Source: SourceOracle, Scope: "all", Complete: false}, t2, ApplyOptions{PreserveExisting: true})
	if stats.Updated != 2 || stats.Inserted != 1 {
		t.Errorf("stats = %+v, want 2 updated, 1 inserted", stats)
	}
	got := ujLoad(t, db, false)
	if got["100"].Title != "Alpha" || ujS(got["100"].Description) != "Full description of Alpha" || got["100"].Source != SourceSite {
		t.Errorf("stored data degraded: title=%q desc=%s source=%q", got["100"].Title, ujS(got["100"].Description), got["100"].Source)
	}
	if got["100"].LastSeen != ujStamp(t2) || got["100"].FirstSeen != ujStamp(t0) {
		t.Errorf("last_seen=%s first_seen=%s, want t2 and t0", got["100"].LastSeen, got["100"].FirstSeen)
	}
	if got["200"].ClosedOn != nil || got["200"].Title != "Bravo" {
		t.Errorf("closed posting seen by Oracle: closed_on=%s title=%q, want reopened with site data", ujS(got["200"].ClosedOn), got["200"].Title)
	}
	if got["300"].Source != SourceOracle || got["300"].Title != "New from oracle" {
		t.Errorf("new Oracle posting = %+v", got["300"].Posting)
	}
}

// TestLastFullSyncAndFirstSyncAt: only complete, uncapped, whole-corpus
// runs count as a full sync; auto data-source relies on this.
func TestLastFullSyncAndFirstSyncAt(t *testing.T) {
	db := ujOpenDB(t)
	ctx := context.Background()
	p := []Posting{ujPosting("100", "A")}
	ujApply(t, db, p, ujRun("query:x", true, false, ujT0), ujT0, ApplyOptions{})
	ujApply(t, db, p, ujRun("all", false, false, ujT0.Add(time.Minute)), ujT0.Add(time.Minute), ApplyOptions{})
	ujApply(t, db, p, ujRun("all", false, true, ujT0.Add(2*time.Minute)), ujT0.Add(2*time.Minute), ApplyOptions{})
	ujApply(t, db, p, ujRun("all-oracle", true, false, ujT0.Add(3*time.Minute)), ujT0.Add(3*time.Minute), ApplyOptions{})
	if run, err := LastFullSync(ctx, db); run != nil || err != nil {
		t.Errorf("LastFullSync after partial runs = %+v, %v; want nil", run, err)
	}
	if _, ok := FirstSyncAt(ctx, db); ok {
		t.Errorf("FirstSyncAt counted a scoped, incomplete, capped, or Oracle run")
	}

	// Defensive: a run flagged both complete and capped is not a full sync.
	// Readers never produce it (capped implies incomplete), so FirstSyncAt
	// is not asserted on it.
	odd := ujOpenDB(t)
	ujApply(t, odd, p, ujRun("all", true, true, ujT0), ujT0, ApplyOptions{})
	if run, _ := LastFullSync(ctx, odd); run != nil {
		t.Errorf("LastFullSync accepted a capped run: %+v", run)
	}

	t1 := ujT0.Add(time.Hour)
	full := ujRun("all", true, false, t1)
	full.Total, full.UniqueCount, full.Note = 584, 584, "ok"
	stored, _ := ujApply(t, db, p, full, t1, ApplyOptions{})
	ujApply(t, db, p, ujRun("query:y", true, false, t1.Add(time.Hour)), t1.Add(time.Hour), ApplyOptions{})
	got, err := LastFullSync(ctx, db)
	if err != nil || got == nil {
		t.Fatalf("LastFullSync = %v, %v", got, err)
	}
	if got.ID != stored.ID || got.Total != 584 || got.UniqueCount != 584 || !got.Complete || got.ScanCapHit || got.Scope != "all" || got.Note != "ok" || got.StartedAt != ujStamp(t1) {
		t.Errorf("LastFullSync = %+v, want the t1 full run", got)
	}

	t2 := ujT0.Add(3 * time.Hour)
	later, _ := ujApply(t, db, p, ujRun("all", true, false, t2), t2, ApplyOptions{})
	if got, _ := LastFullSync(ctx, db); got == nil || got.ID != later.ID {
		t.Errorf("LastFullSync did not move to the newest full run")
	}
	first, ok := FirstSyncAt(ctx, db)
	if !ok || !first.Equal(t1) {
		t.Errorf("FirstSyncAt = %v %v, want %v (earliest complete full run)", first, ok, t1)
	}
}

// TestLoadPostingsSortedByID keeps output order stable across runs.
func TestLoadPostingsSortedByID(t *testing.T) {
	db := ujOpenDB(t)
	ujApply(t, db, []Posting{ujPosting("300", "C"), ujPosting("100", "A"), ujPosting("200", "B")}, ujRun("all", true, false, ujT0), ujT0, ApplyOptions{})
	rows, err := LoadPostings(context.Background(), db, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].ID != "100" || rows[1].ID != "200" || rows[2].ID != "300" {
		t.Errorf("order = %v", []string{rows[0].ID, rows[1].ID, rows[2].ID})
	}
}

// TestSameTitlePostingsStayTwoRows (B2): postings are keyed by id, never by
// title, through SearchAll and ApplyRead.
func TestSameTitlePostingsStayTwoRows(t *testing.T) {
	rows := ujCorpusRows(t)
	srv := ujSearchServer(t, rows, len(rows))
	c := ujClient(t, srv.URL)
	res, err := c.SearchAll(context.Background(), Query{})
	if err != nil {
		t.Fatal(err)
	}
	postings := make([]Posting, 0, len(res.Rows))
	for _, r := range res.Rows {
		postings = append(postings, Normalize(r, ""))
	}
	db := ujOpenDB(t)
	run, stats := ujApply(t, db, postings, ujRun("all", res.Complete, res.ScanCapHit, ujT0), ujT0, ApplyOptions{})
	if stats.Inserted != 56 || run.ClosedMarked != 0 {
		t.Errorf("stats=%+v closed=%d, want 56 inserted", stats, run.ClosedMarked)
	}
	got := ujLoad(t, db, true)
	pairs := [][2]string{{"302433", "302466"}, {"302906", "300629"}, {"302278", "301296"}, {"302898", "302965"}}
	for _, pair := range pairs {
		a, b := got[pair[0]], got[pair[1]]
		if a.ID == "" || b.ID == "" {
			t.Errorf("same-title pair %v collapsed: %q %q", pair, a.ID, b.ID)
			continue
		}
		if a.Title != b.Title {
			t.Errorf("fixture pair %v no longer shares a title: %q vs %q", pair, a.Title, b.Title)
		}
	}
	if len(got) != 56 {
		t.Errorf("stored %d postings, want 56", len(got))
	}
}

// TestSaveFacetsLatest round-trips facet snapshots and returns the newest.
func TestSaveFacetsLatest(t *testing.T) {
	db := ujOpenDB(t)
	ctx := context.Background()
	f, err := ParseFacets(ujReadTestdata(t, "facets.html"))
	if err != nil {
		t.Fatal(err)
	}
	old := &Facets{Countries: []string{"Germany"}, Teams: []string{"Sales"}, TeamSubTeams: map[string][]string{}, UnmappedCountries: []string{}}
	if err := SaveFacets(ctx, db, old, ujT0); err != nil {
		t.Fatal(err)
	}
	if err := SaveFacets(ctx, db, f, ujT0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, at, err := LatestFacets(ctx, db)
	if err != nil || got == nil {
		t.Fatalf("LatestFacets = %v, %v", got, err)
	}
	if at != ujStamp(ujT0.Add(time.Hour)) || len(got.Countries) != 38 || len(got.Teams) != 18 || got.TotalJobs == nil || *got.TotalJobs != 584 {
		t.Errorf("latest = at %q countries %d teams %d total %v", at, len(got.Countries), len(got.Teams), got.TotalJobs)
	}
	if len(got.TeamSubTeams["Engineer"]) != len(f.TeamSubTeams["Engineer"]) {
		t.Errorf("mapping did not round-trip")
	}
	// Same timestamp replaces rather than failing on the primary key.
	if err := SaveFacets(ctx, db, old, ujT0.Add(time.Hour)); err != nil {
		t.Errorf("re-save at the same instant: %v", err)
	}
	if got, _, _ := LatestFacets(ctx, db); got == nil || len(got.Countries) != 1 {
		t.Errorf("re-save did not replace the snapshot")
	}
}

// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestServerQueryMapsCountries: ISO3 (and other accepted forms) become the
// site's exact country names; the rest of the filter passes through.
func TestServerQueryMapsCountries(t *testing.T) {
	f := Filters{Query: "data", Countries: []string{"GBR", "deu", "Netherlands", "sa"}, Team: "Engineer", SubTeam: "Software Engineering", ContractType: "Full time", WorkPattern: "Regular"}
	q, unmapped, err := f.ServerQuery()
	if err != nil || unmapped != nil {
		t.Fatalf("ServerQuery: unmapped=%v err=%v", unmapped, err)
	}
	want := Query{Search: "data", Countries: []string{"United Kingdom", "Germany", "Netherlands", "Saudi Arabia"}, Team: "Engineer", SubTeam: "Software Engineering", ContractType: "Full time"}
	if !reflect.DeepEqual(q, want) {
		t.Errorf("query = %+v, want %+v", q, want)
	}
	if v := q.Values(); v.Has("workPattern") || len(v["countries"]) != 4 {
		t.Errorf("values = %v: work pattern is client-side and countries repeat", v)
	}
}

// TestServerQueryRejectsUnknownCountry: an unknown code is a usage error
// before any request, not a silent unfiltered search.
func TestServerQueryRejectsUnknownCountry(t *testing.T) {
	f := Filters{Countries: []string{"GBR", "XXX", "Atlantis"}}
	q, unmapped, err := f.ServerQuery()
	if err == nil {
		t.Fatalf("ServerQuery accepted unknown countries: %+v", q)
	}
	if !reflect.DeepEqual(unmapped, []string{"XXX", "Atlantis"}) {
		t.Errorf("unmapped = %q, want [XXX Atlantis]", unmapped)
	}
	if !strings.Contains(err.Error(), "XXX, Atlantis") || !strings.Contains(err.Error(), "ISO3") {
		t.Errorf("error = %q, want it to name both and suggest ISO3", err.Error())
	}
}

func ujMatchPosting() Posting {
	p := ujPosting("100", "Senior Data Engineer")
	p.WorkPattern = ujStrp("Intern")
	p.ContractType = ujStrp("Full time")
	p.Description = ujStrp("You have experience with Python and Spark. Visa Sponsorship is available.")
	p.PostedRaw = ujStrp("2026-10-04T10:00:00Z")
	p.PostedOn, p.PostedDate, p.PostedDateIsFloor = postingDates(*p.PostedRaw)
	return p
}

// TestMatchClient: each client-side filter has a positive and a negative
// case; phrase checks are case-insensitive over the stripped description.
func TestMatchClient(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	p := ujMatchPosting()
	cases := []struct {
		name   string
		f      Filters
		window time.Duration
		want   bool
	}{
		{"no filters", Filters{}, 0, true},
		{"work pattern match, case-insensitive", Filters{WorkPattern: "intern"}, 0, true},
		{"work pattern mismatch", Filters{WorkPattern: "Regular"}, 0, false},
		{"inside window", Filters{}, 48 * time.Hour, true},
		// 1h ending 2026-10-05T12:00Z opens on 2026-10-05, floored to 00:00
		// that day; the posting is from 2026-10-04.
		{"outside window", Filters{}, time.Hour, false},
		{"contains, case-insensitive", Filters{DescriptionContains: []string{"PYTHON"}}, 0, true},
		{"contains, all required", Filters{DescriptionContains: []string{"python", "golang"}}, 0, false},
		{"contains, phrase trimmed", Filters{DescriptionContains: []string{"  spark  "}}, 0, true},
		{"excludes absent", Filters{DescriptionExcludes: []string{"clearance"}}, 0, true},
		{"excludes present, case-insensitive", Filters{DescriptionExcludes: []string{"visa sponsorship"}}, 0, false},
		{"excludes any", Filters{DescriptionExcludes: []string{"clearance", "SPARK"}}, 0, false},
	}
	for _, tc := range cases {
		if got := tc.f.MatchClient(p, tc.window, now); got != tc.want {
			t.Errorf("%s: MatchClient = %v, want %v", tc.name, got, tc.want)
		}
	}

	bare := Posting{ID: "1"}
	if (Filters{WorkPattern: "Intern"}).MatchClient(bare, 0, now) {
		t.Errorf("null work_pattern matched a work-pattern filter")
	}
	if (Filters{DescriptionContains: []string{"python"}}).MatchClient(bare, 0, now) {
		t.Errorf("null description matched a contains filter")
	}
	if !(Filters{DescriptionExcludes: []string{"python"}}).MatchClient(bare, 0, now) {
		t.Errorf("null description failed an excludes filter")
	}
	floor := ujMatchPosting()
	floor.PostedRaw = ujStrp(DateFloor)
	floor.PostedOn, floor.PostedDate, floor.PostedDateIsFloor = postingDates(DateFloor)
	if (Filters{}).MatchClient(floor, 400*24*time.Hour, now) {
		t.Errorf("floor-dated posting matched a posted-within window")
	}
}

// TestMatchLocal: offline filters over stored postings. Keyword covers
// title, team, sub-team, and description; country matches any location.
func TestMatchLocal(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	by := ujCorpusByID(t)
	multi := by["302016"] // UAE + Saudi Arabia, Sales / Sales, Fixed Term
	vendor := by["302433"]
	// isolated keeps each keyword field distinct so only one can match.
	isolated := Posting{ID: "1", Title: "Alpha Role", JobCategory: ujStrp("Zeta Team"), SubTeam: ujStrp("Omega Group"), Description: ujStrp("Plain words only.")}
	cases := []struct {
		name string
		f    Filters
		p    Posting
		want bool
	}{
		{"keyword in title", Filters{Query: "VENDOR"}, vendor, true},
		{"keyword in title (isolated)", Filters{Query: "alpha role"}, isolated, true},
		{"keyword in team", Filters{Query: "zeta team"}, isolated, true},
		{"keyword in sub-team", Filters{Query: "OMEGA group"}, isolated, true},
		{"keyword in description", Filters{Query: "plain words"}, isolated, true},
		{"keyword in real description", Filters{Query: "salesforce crm"}, multi, true},
		{"keyword absent", Filters{Query: "blockchain"}, multi, false},
		{"second location country (ISO3)", Filters{Countries: []string{"SAU"}}, multi, true},
		{"first location country", Filters{Countries: []string{"ARE"}}, multi, true},
		{"country by ISO2 and name", Filters{Countries: []string{"sa", "Germany"}}, multi, true},
		{"country absent", Filters{Countries: []string{"GBR"}}, multi, false},
		{"only unknown countries", Filters{Countries: []string{"XXX"}}, multi, false},
		{"team exact, case-insensitive", Filters{Team: "sales"}, multi, true},
		{"team substring is not a match", Filters{Team: "Sale"}, multi, false},
		{"team mismatch", Filters{Team: "Engineer"}, multi, false},
		{"sub-team exact", Filters{SubTeam: "Customer Support"}, vendor, true},
		{"sub-team mismatch", Filters{SubTeam: "Customer Experience"}, vendor, false},
		{"contract exact", Filters{ContractType: "full time"}, multi, true},
		{"contract mismatch", Filters{ContractType: "Part time"}, multi, false},
		{"contract on null field", Filters{ContractType: "Full time"}, by["300235"], false},
		{"team on null field", Filters{Team: "Sales"}, by["303232"], false},
		{"falls through to client filters", Filters{Team: "Sales", WorkPattern: "Regular"}, multi, false},
		{"all filters agree", Filters{Query: "enterprise", Countries: []string{"SAU"}, Team: "Sales", SubTeam: "Sales", ContractType: "Full time", WorkPattern: "Fixed Term"}, multi, true},
	}
	for _, tc := range cases {
		if got := tc.f.MatchLocal(tc.p, 0, now); got != tc.want {
			t.Errorf("%s: MatchLocal(%s) = %v, want %v", tc.name, tc.p.ID, got, tc.want)
		}
	}
	if !(Filters{}).MatchLocal(multi, 24*time.Hour, now) {
		t.Errorf("302016 (posted 2026-10-05) should be inside a 24h window")
	}
	if (Filters{}).MatchLocal(by["149574"], 365*24*time.Hour, now) {
		t.Errorf("floor row 149574 matched a window")
	}
}

func ujSave(t *testing.T, db *sql.DB, name string, f Filters, at time.Time) (*SavedSearch, bool) {
	t.Helper()
	s, reset, err := SaveSearch(context.Background(), db, name, f, at)
	if err != nil {
		t.Fatalf("SaveSearch(%s): %v", name, err)
	}
	return s, reset
}

func ujMembers(t *testing.T, db *sql.DB, name string) map[string]Member {
	t.Helper()
	m, err := Members(context.Background(), db, name)
	if err != nil {
		t.Fatalf("Members(%s): %v", name, err)
	}
	return m
}

func ujAdvance(t *testing.T, db *sql.DB, name string, ids []string, first bool, at time.Time) int {
	t.Helper()
	ps := make([]Posting, 0, len(ids))
	for _, id := range ids {
		ps = append(ps, ujPosting(id, "Title "+id))
	}
	n, err := AdvanceMembership(context.Background(), db, name, ps, first, at)
	if err != nil {
		t.Fatalf("AdvanceMembership(%s): %v", name, err)
	}
	return n
}

// TestSaveSearchLifecycle: create, re-save with the same filters (no
// reset), and re-save with changed filters (reset drops membership so new
// never diffs across different filters).
func TestSaveSearchLifecycle(t *testing.T) {
	db := ujOpenDB(t)
	ctx := context.Background()
	f := Filters{Query: "data", Countries: []string{"GBR"}}

	s, reset := ujSave(t, db, "uk-data", f, ujT0)
	if reset || s.Name != "uk-data" || !reflect.DeepEqual(s.Filters, f) || s.CreatedAt != ujStamp(ujT0) || s.BaselineAt != nil || s.BaselineSize != 0 {
		t.Fatalf("create = %+v reset=%v", s, reset)
	}
	ujAdvance(t, db, "uk-data", []string{"1", "2"}, true, ujT0.Add(time.Hour))

	s, reset = ujSave(t, db, "uk-data", f, ujT0.Add(2*time.Hour))
	if reset || s.BaselineSize != 2 || ujS(s.BaselineAt) != ujStamp(ujT0.Add(time.Hour)) || s.CreatedAt != ujStamp(ujT0) {
		t.Errorf("same-filter re-save = %+v reset=%v, want membership and baseline kept", s, reset)
	}

	changed := Filters{Query: "data", Countries: []string{"GBR", "DEU"}}
	s, reset = ujSave(t, db, "uk-data", changed, ujT0.Add(3*time.Hour))
	if !reset || s.BaselineSize != 0 || s.BaselineAt != nil || s.LastAdvancedAt != nil || !reflect.DeepEqual(s.Filters, changed) {
		t.Errorf("changed-filter re-save = %+v reset=%v, want reset with no baseline", s, reset)
	}
	if s.CreatedAt != ujStamp(ujT0) {
		t.Errorf("created_at moved to %s on re-save", s.CreatedAt)
	}
	if m := ujMembers(t, db, "uk-data"); len(m) != 0 {
		t.Errorf("members after reset = %d, want 0", len(m))
	}

	ujSave(t, db, "a-first", Filters{Team: "Sales"}, ujT0)
	list, err := ListSearches(ctx, db)
	if err != nil || len(list) != 2 || list[0].Name != "a-first" || list[1].Name != "uk-data" {
		t.Errorf("ListSearches = %+v, %v; want sorted by name", list, err)
	}

	if got, err := GetSearch(ctx, db, "missing"); got != nil || err != nil {
		t.Errorf("GetSearch(missing) = %v, %v", got, err)
	}
	ujAdvance(t, db, "uk-data", []string{"9"}, true, ujT0.Add(4*time.Hour))
	ok, err := DeleteSearch(ctx, db, "uk-data")
	if err != nil || !ok {
		t.Fatalf("DeleteSearch = %v, %v", ok, err)
	}
	if got, _ := GetSearch(ctx, db, "uk-data"); got != nil {
		t.Errorf("search still present after delete")
	}
	if m := ujMembers(t, db, "uk-data"); len(m) != 0 {
		t.Errorf("members survived delete: %d", len(m))
	}
	if ok, err := DeleteSearch(ctx, db, "uk-data"); ok || err != nil {
		t.Errorf("second DeleteSearch = %v, %v; want false, nil", ok, err)
	}
}

// TestDiffMembership: added (new and reopened, in read order), removed
// (present members missing, sorted), and duplicates in the read count once.
func TestDiffMembership(t *testing.T) {
	removedAt := ujStamp(ujT0)
	members := map[string]Member{
		"a": {PostingID: "a"},
		"z": {PostingID: "z"},
		"m": {PostingID: "m"},
		"c": {PostingID: "c", RemovedOn: &removedAt},
		"g": {PostingID: "g", RemovedOn: &removedAt},
	}
	current := []Posting{{ID: "d"}, {ID: "a"}, {ID: "c"}, {ID: "a"}, {ID: "d"}, {ID: "e"}}
	d := DiffMembership(members, current)
	var added []string
	for _, p := range d.Added {
		added = append(added, p.ID)
	}
	if !reflect.DeepEqual(added, []string{"d", "c", "e"}) {
		t.Errorf("added = %q, want [d c e] (read order, duplicates once)", added)
	}
	if !reflect.DeepEqual(d.Reopened, map[string]bool{"c": true}) {
		t.Errorf("reopened = %v, want only c", d.Reopened)
	}
	var removed []string
	for _, m := range d.Removed {
		removed = append(removed, m.PostingID)
	}
	if !reflect.DeepEqual(removed, []string{"m", "z"}) {
		t.Errorf("removed = %q, want [m z] sorted; already-removed g is not re-reported", removed)
	}

	empty := DiffMembership(map[string]Member{}, nil)
	if empty.Added == nil || empty.Removed == nil || empty.Reopened == nil {
		t.Errorf("empty diff has nil fields: %+v", empty)
	}
	first := DiffMembership(nil, []Posting{{ID: "x"}})
	if len(first.Added) != 1 || len(first.Removed) != 0 {
		t.Errorf("first diff = %+v", first)
	}
}

// TestAdvanceMembership: append-only membership. first stamps the
// baseline; later advances keep first_seen, mark absent members removed,
// clear removed_on when they return, and never touch other searches.
func TestAdvanceMembership(t *testing.T) {
	db := ujOpenDB(t)
	ctx := context.Background()
	t0, t1, t2, t3 := ujT0, ujT0.Add(time.Hour), ujT0.Add(2*time.Hour), ujT0.Add(3*time.Hour)
	ujSave(t, db, "s1", Filters{Team: "Engineer"}, t0)
	ujSave(t, db, "s2", Filters{Team: "Sales"}, t0)
	ujAdvance(t, db, "s2", []string{"a", "x"}, true, t0)

	if n := ujAdvance(t, db, "s1", []string{"a", "b", "c"}, true, t0); n != 0 {
		t.Errorf("first advance removed %d", n)
	}
	s, _ := GetSearch(ctx, db, "s1")
	if ujS(s.BaselineAt) != ujStamp(t0) || ujS(s.LastAdvancedAt) != ujStamp(t0) || ujS(s.LastCheckedAt) != ujStamp(t0) || s.BaselineSize != 3 {
		t.Errorf("after first advance: %+v", s)
	}
	m := ujMembers(t, db, "s1")
	if m["a"].FirstSeen != ujStamp(t0) || ujS(m["a"].Title) != "Title a" || m["a"].RemovedOn != nil {
		t.Errorf("member a = %+v", m["a"])
	}

	if n := ujAdvance(t, db, "s1", []string{"a", "b", "d"}, false, t1); n != 1 {
		t.Errorf("second advance removed %d, want 1 (c)", n)
	}
	m = ujMembers(t, db, "s1")
	if ujS(m["c"].RemovedOn) != ujStamp(t1) || m["c"].LastSeen != ujStamp(t0) {
		t.Errorf("absent member c = %+v, want removed_on t1 and last_seen t0", m["c"])
	}
	if m["a"].FirstSeen != ujStamp(t0) || m["a"].LastSeen != ujStamp(t1) || m["d"].FirstSeen != ujStamp(t1) {
		t.Errorf("a=%+v d=%+v", m["a"], m["d"])
	}
	s, _ = GetSearch(ctx, db, "s1")
	if ujS(s.BaselineAt) != ujStamp(t0) || ujS(s.LastAdvancedAt) != ujStamp(t1) || s.BaselineSize != 3 {
		t.Errorf("after second advance: baseline=%s advanced=%s size=%d, want t0/t1/3", ujS(s.BaselineAt), ujS(s.LastAdvancedAt), s.BaselineSize)
	}

	if n := ujAdvance(t, db, "s1", []string{"a", "b", "c", "d"}, false, t2); n != 0 {
		t.Errorf("third advance removed %d", n)
	}
	m = ujMembers(t, db, "s1")
	if m["c"].RemovedOn != nil || m["c"].FirstSeen != ujStamp(t0) || m["c"].LastSeen != ujStamp(t2) {
		t.Errorf("returning member c = %+v, want removed_on cleared and first_seen kept", m["c"])
	}
	if s, _ := GetSearch(ctx, db, "s1"); s.BaselineSize != 4 {
		t.Errorf("baseline size = %d, want 4", s.BaselineSize)
	}

	if n := ujAdvance(t, db, "s1", nil, false, t3); n != 4 {
		t.Errorf("empty advance removed %d, want 4", n)
	}
	if s, _ := GetSearch(ctx, db, "s1"); s.BaselineSize != 0 {
		t.Errorf("baseline size = %d, want 0 (removed members do not count)", s.BaselineSize)
	}
	if m := ujMembers(t, db, "s1"); len(m) != 4 {
		t.Errorf("members = %d, want 4 rows kept (append-only)", len(m))
	}

	other := ujMembers(t, db, "s2")
	if len(other) != 2 || other["a"].RemovedOn != nil || other["x"].RemovedOn != nil {
		t.Errorf("advancing s1 touched s2: %+v", other)
	}
}

// TestMarkChecked records a check without moving the baseline or advance
// stamps.
func TestMarkChecked(t *testing.T) {
	db := ujOpenDB(t)
	ctx := context.Background()
	ujSave(t, db, "s", Filters{}, ujT0)
	ujAdvance(t, db, "s", []string{"a"}, true, ujT0)
	if err := MarkChecked(ctx, db, "s", ujT0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	s, _ := GetSearch(ctx, db, "s")
	if ujS(s.LastCheckedAt) != ujStamp(ujT0.Add(time.Hour)) || ujS(s.LastAdvancedAt) != ujStamp(ujT0) || ujS(s.BaselineAt) != ujStamp(ujT0) || s.BaselineSize != 1 {
		t.Errorf("after MarkChecked: %+v", s)
	}
	if err := MarkChecked(ctx, db, "missing", ujT0); err != nil {
		t.Errorf("MarkChecked on a missing search: %v", err)
	}
}

// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// get (exact id match through search, B11) and facets.

package cli

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

// TestUJCGetLiveExactIDMatch guards B11: get goes through search, so it must
// return the row whose id equals the request, never the first row.
func TestUJCGetLiveExactIDMatch(t *testing.T) {
	site := ujcNewFake(t)
	ctx := context.Background()

	env := uberjobs.NewEnvelope("get", "", []uberjobs.Posting{}, 0)
	p, err := getLive(ctx, ujcClient(site, nil), ujcIDNewestNLD, &env)
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != ujcIDNewestNLD || p.Title != "People Operations Employee Data Specialist" {
		t.Fatalf("get %s returned %s %q", ujcIDNewestNLD, p.ID, p.Title)
	}
	if env.Meta.Source != uberjobs.SourceSite || !env.Meta.Complete || env.Scanned != 56 {
		t.Fatalf("meta source=%s complete=%v scanned=%d, want a complete 56-row site read", env.Meta.Source, env.Meta.Complete, env.Scanned)
	}

	// A corpus that does not hold the id: its first (only) row is another
	// posting, and a complete read proves absence.
	site.Only(ujcIDFirstRow)
	env = uberjobs.NewEnvelope("get", "", []uberjobs.Posting{}, 0)
	p, err = getLive(ctx, ujcClient(site, nil), ujcIDNewestNLD, &env)
	var nf *uberjobs.NotFoundError
	if !errors.As(err, &nf) || nf.ID != ujcIDNewestNLD {
		t.Fatalf("missing id: err = %v, want NotFoundError for %s", err, ujcIDNewestNLD)
	}
	if p != nil {
		t.Fatalf("missing id returned posting %s, want none", p.ID)
	}
}

// TestUJCGetLivePartialReadIsNotNotFound: absence from an incomplete read
// proves nothing, so it is a content error, not not-found.
func TestUJCGetLivePartialReadIsNotNotFound(t *testing.T) {
	site := ujcNewFake(t)
	site.Drop(ujcIDNewestNLD)
	site.SetTotalBump(3)
	env := uberjobs.NewEnvelope("get", "", []uberjobs.Posting{}, 0)
	_, err := getLive(context.Background(), ujcClient(site, nil), ujcIDNewestNLD, &env)
	var ce *uberjobs.ContentError
	if !errors.As(err, &ce) || !strings.Contains(ce.Reason, "partial read") {
		t.Fatalf("partial read: err = %v, want a ContentError about a partial read", err)
	}
	if ujcCode(uberErr(err)) != 5 {
		t.Fatalf("partial read maps to exit %d, want 5", ujcCode(uberErr(err)))
	}
	// The id present in a partial read is still returned.
	env = uberjobs.NewEnvelope("get", "", []uberjobs.Posting{}, 0)
	p, err := getLive(context.Background(), ujcClient(site, nil), ujcIDFirstRow, &env)
	if err != nil || p.ID != ujcIDFirstRow || env.Meta.Complete {
		t.Fatalf("listed id in a partial read: p=%v err=%v complete=%v, want the row with complete=false", p, err, env.Meta.Complete)
	}
}

// TestUJCGetLiveOracleDetailFallback: when the site refuses, get reads the
// Oracle detail record (the real fixture for 302906) in one request.
func TestUJCGetLiveOracleDetailFallback(t *testing.T) {
	site, oracle := ujcNewFake(t), ujcNewFake(t)
	site.SetMode("challenge")
	c := ujcClient(site, oracle)
	env := uberjobs.NewEnvelope("get", "", []uberjobs.Posting{}, 0)
	p, err := getLive(context.Background(), c, ujcOracleDetailID, &env)
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != ujcOracleDetailID || p.Source != uberjobs.SourceOracle || p.Title != "Senior - FRM Advisory" {
		t.Fatalf("fallback posting = %s %q source %s", p.ID, p.Title, p.Source)
	}
	if p.Description == nil || !strings.Contains(*p.Description, "About the role") || strings.Contains(*p.Description, "<p>") {
		t.Fatalf("fallback description = %v, want the stripped Oracle description", p.Description)
	}
	if p.JobCategory != nil {
		t.Fatalf("job_category = %q, want null (Oracle's Category is another taxonomy)", *p.JobCategory)
	}
	if p.CountryCode == nil || *p.CountryCode != "USA" {
		t.Fatalf("country_code = %v, want USA from Oracle's US", p.CountryCode)
	}
	if env.Meta.Source != uberjobs.SourceOracle || !env.Meta.Fallback || !strings.Contains(env.Meta.FallbackFrom, "refused") || env.Meta.Requests != 2 {
		t.Fatalf("meta = %+v, want an oracle-ce fallback after one refused request (2 requests)", env.Meta)
	}
	if n := site.Count("/api/jobs/search/"); n != 1 {
		t.Fatalf("refusing site saw %d requests, want 1", n)
	}

	// An id Oracle does not list is not-found there too.
	env = uberjobs.NewEnvelope("get", "", []uberjobs.Posting{}, 0)
	_, err = getLive(context.Background(), c, "999999999", &env)
	var nf *uberjobs.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("unknown id via Oracle: err = %v, want NotFoundError", err)
	}
}

// TestUJCGetExitCodes runs get through RootCmd for the typed exits.
func TestUJCGetExitCodes(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")

	for _, args := range [][]string{
		{"get", "abc", "--data-source", "live", "--json"},
		{"get", "30323x", "--data-source", "live", "--json"},
		{"get", ujcIDNewestNLD, ujcIDFirstRow, "--data-source", "live", "--json"},
		{"get", ujcIDNewestNLD, "--data-source", "remote", "--json"},
	} {
		ujcWantCode(t, ujcRun(t, "", args...), 2)
	}
	if n := site.Count(""); n != 0 {
		t.Fatalf("usage errors sent %d requests, want 0", n)
	}

	r := ujcRun(t, "", "get", "999999999", "--data-source", "live", "--json")
	ujcWantCode(t, r, 3)
	if r.Stdout != "" {
		t.Fatalf("not-found printed a result: %s", r.Stdout)
	}

	site.SetTotalBump(4)
	site.Drop(ujcIDNewestNLD)
	r = ujcRun(t, "", "get", ujcIDNewestNLD, "--data-source", "live", "--json", "--no-cache")
	ujcWantCode(t, r, 5)
	if !strings.Contains(r.Err.Error(), "partial read") {
		t.Fatalf("partial-read error = %v, want it to say why absence proves nothing", r.Err)
	}
}

// TestUJCGetLocalClosedIsNotFound: the local store answers get, and a
// posting the last complete sync marked closed exits 3.
func TestUJCGetLocalClosedIsNotFound(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, "", "")
	start := time.Now().Add(-2 * time.Hour)
	ujcWithStore(t, ujcDefaultDB(), func(db *sql.DB) {
		ujcApply(t, db, site.Postings(t), "all", true, start)
		site.Drop(ujcIDNewestNLD)
		ujcApply(t, db, site.Postings(t), "all", true, start.Add(time.Hour))
	})

	r := ujcRun(t, "", "get", ujcIDNewestNLD, "--data-source", "local", "--json")
	ujcWantCode(t, r, 3)
	if !strings.Contains(r.Err.Error(), "closed on") {
		t.Fatalf("closed posting error = %v, want it to name the close", r.Err)
	}

	r = ujcRun(t, "", "get", ujcIDFirstRow, "--data-source", "local", "--json")
	ujcWantCode(t, r, 0)
	env := ujcEnvelope(t, r.Stdout)
	rows := ujcRows(env)
	if len(rows) != 1 || rows[0]["id"] != ujcIDFirstRow || rows[0]["closed_on"] != nil || rows[0]["first_seen"] == nil {
		t.Fatalf("local get rows = %v, want %s open with its history", rows, ujcIDFirstRow)
	}
	if ujcMeta(env)["source"] != uberjobs.SourceLocal || ujcMeta(env)["complete"] != true {
		t.Fatalf("local get meta = %v", ujcMeta(env))
	}
	ujcWantCode(t, ujcRun(t, "", "get", "999999999", "--data-source", "local", "--json"), 3)
}

// ujcFixtureFacets parses the real /en/jobs/ page.
func ujcFixtureFacets(t *testing.T) *uberjobs.Facets {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(ujcTestdata, "facets.html"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := uberjobs.ParseFacets(raw)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// TestUJCFacetRows checks the row shape the facets command prints: an id of
// facet:value, ISO3 on countries, and parent teams on sub-teams.
func TestUJCFacetRows(t *testing.T) {
	f := ujcFixtureFacets(t)
	rows := facetRows(f)
	want := len(f.Countries) + len(f.Teams) + len(f.SubTeams) + len(f.ContractTypes) + len(f.WorkPatterns)
	if len(rows) != want || len(f.Countries) != 38 {
		t.Fatalf("facetRows gave %d rows (%d countries), want %d (38)", len(rows), len(f.Countries), want)
	}
	byID := map[string]facetRow{}
	for _, r := range rows {
		if r.ID != r.Facet+":"+r.Value {
			t.Fatalf("row id %q, want %q", r.ID, r.Facet+":"+r.Value)
		}
		if _, dup := byID[r.ID]; dup {
			t.Fatalf("duplicate facet row id %q", r.ID)
		}
		byID[r.ID] = r
		switch r.Facet {
		case "country":
			if r.ISO3 == nil || len(*r.ISO3) != 3 {
				t.Fatalf("country row %q has iso3 %v", r.Value, r.ISO3)
			}
		case "sub_team":
			if r.Teams == nil {
				t.Fatalf("sub_team row %q has nil teams", r.Value)
			}
		default:
			if r.ISO3 != nil || r.Teams != nil {
				t.Fatalf("%s row %q carries iso3/teams", r.Facet, r.Value)
			}
		}
	}
	for value, iso := range map[string]string{"Saudi Arabia": "SAU", "United Kingdom": "GBR", "Netherlands": "NLD"} {
		r, ok := byID["country:"+value]
		if !ok || *r.ISO3 != iso {
			t.Fatalf("country:%s = %+v, want iso3 %s", value, r, iso)
		}
	}
	if got := byID["sub_team:Program Management"].Teams; strings.Join(got, ",") != "Customer Support,People,Places" {
		t.Fatalf("Program Management teams = %v, want sorted [Customer Support People Places]", got)
	}
	if got := byID["sub_team:Business Operations"].Teams; strings.Join(got, ",") != "Operations" {
		t.Fatalf("Business Operations teams = %v, want [Operations]", got)
	}
	if _, ok := byID["work_pattern:Direct NCG Hire"]; !ok {
		t.Fatalf("work_pattern rows miss Direct NCG Hire")
	}
}

// TestUJCFacetsCommand runs facets through RootCmd: the --facet filter (with
// an alias), usage errors before any request, the empty local store, the
// local snapshot, and auto's fallback to it when the site refuses.
func TestUJCFacetsCommand(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")

	ujcWantCode(t, ujcRun(t, "", "facets", "--facet", "salary", "--json", "--data-source", "live"), 2)
	ujcWantCode(t, ujcRun(t, "", "facets", "extra", "--json", "--data-source", "live"), 2)
	if n := site.Count(""); n != 0 {
		t.Fatalf("usage errors sent %d requests, want 0", n)
	}

	r := ujcRun(t, "", "facets", "--data-source", "local", "--json")
	ujcWantCode(t, r, 0)
	env := ujcEnvelope(t, r.Stdout)
	if note, _ := ujcMeta(env)["note"].(string); len(ujcRows(env)) != 0 || !strings.Contains(note, "no facets snapshot") {
		t.Fatalf("local facets with no snapshot = %v, want empty results and a note", env)
	}

	r = ujcRun(t, "", "facets", "--facet", "country", "--json", "--data-source", "live")
	ujcWantCode(t, r, 0)
	env = ujcEnvelope(t, r.Stdout)
	rows := ujcRows(env)
	if len(rows) != 38 || ujcInt(t, env["hits"]) != 38 || ujcInt(t, env["scanned"]) <= 38 {
		t.Fatalf("--facet country: rows=%d hits=%v scanned=%v, want 38 of a larger list", len(rows), env["hits"], env["scanned"])
	}
	for _, row := range rows {
		if row["facet"] != "country" || row["iso3"] == nil {
			t.Fatalf("--facet country kept %v", row)
		}
	}
	if extra, _ := ujcMeta(env)["extra"].(map[string]any); extra["total_jobs"] == nil {
		t.Fatalf("meta.extra = %v, want total_jobs from the page", ujcMeta(env)["extra"])
	}

	r = ujcRun(t, "", "facets", "--facet", "subteams", "--json", "--data-source", "live")
	ujcWantCode(t, r, 0)
	for _, row := range ujcRows(ujcEnvelope(t, r.Stdout)) {
		if row["facet"] != "sub_team" {
			t.Fatalf("--facet subteams kept %v", row)
		}
	}
	if n := site.Count("/en/jobs/"); n != 1 {
		t.Fatalf("facet page fetched %d times, want 1 (the second read is cached)", n)
	}

	// A snapshot answers local reads, and auto falls back to it on refusal.
	ujcWithStore(t, ujcDefaultDB(), func(db *sql.DB) {
		if err := uberjobs.SaveFacets(context.Background(), db, ujcFixtureFacets(t), time.Now()); err != nil {
			t.Fatal(err)
		}
	})
	r = ujcRun(t, "", "facets", "--facet", "team", "--data-source", "local", "--json")
	ujcWantCode(t, r, 0)
	env = ujcEnvelope(t, r.Stdout)
	teams := []string{}
	for _, row := range ujcRows(env) {
		teams = append(teams, row["value"].(string))
	}
	sort.Strings(teams)
	if n := len(ujcFixtureFacets(t).Teams); len(teams) != n || n != 18 || !ujcContains(teams, "Operations") || ujcMeta(env)["source"] != uberjobs.SourceLocal {
		t.Fatalf("local --facet team = %v (source %v), want the 18 snapshot teams", teams, ujcMeta(env)["source"])
	}

	site.SetMode("challenge")
	r = ujcRun(t, "", "facets", "--facet", "country", "--json", "--no-cache")
	ujcWantCode(t, r, 0)
	env = ujcEnvelope(t, r.Stdout)
	if m := ujcMeta(env); m["source"] != uberjobs.SourceLocal || m["fallback"] != true || len(ujcRows(env)) != 38 {
		t.Fatalf("refused auto facets meta = %v rows=%d, want a local fallback with 38 countries", m, len(ujcRows(env)))
	}
	r = ujcRun(t, "", "facets", "--json", "--no-cache", "--data-source", "live")
	ujcWantCode(t, r, 7)
}

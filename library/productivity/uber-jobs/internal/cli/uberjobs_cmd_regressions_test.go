// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// Regression tests for the bugs the Phase 3 test pass found and fixed
// (2026-10-05), plus the test that kills mutation M20 (an Oracle fallback on
// any site error). Each one failed before its fix.

package cli

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

func TestUJCGuardRefusalSurvivesGeneratedRetry(t *testing.T) {
	ujcIsolate(t, "", "")
	ujcResetGuardRefusal(t)
	rt := &ujcRT{status: map[string]int{"jobs.uber.com": http.StatusTooManyRequests}}
	c := client.New(&config.Config{BaseURL: "https://jobs.uber.com"}, 20*time.Second, 0)
	c.NoCache = true
	c.HTTPClient.Transport = rt
	if err := installUberGuard(c); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("the generated client panicked on a guarded 429: %v", r)
		}
	}()
	_, err := c.Get(context.Background(), "/api/jobs/search/", map[string]string{"page": "1", "pagesize": "1"})
	if err == nil {
		t.Fatalf("a 429 returned no error")
	}
	if rt.Calls() != 1 {
		t.Fatalf("base saw %d calls, want exactly 1", rt.Calls())
	}
}

func TestUJCAgentPrintsOneEnvelope(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	ujcSeedFullSync(t, ujcDefaultDB(), site, time.Now())
	r := ujcRun(t, "", "postings", "--country", "GBR", "--limit", "3", "--data-source", "local", "--agent")
	ujcWantCode(t, r, 0)
	env := ujcEnvelope(t, r.Stdout)
	if ujcMeta(env)["source"] != uberjobs.SourceLocal || len(ujcRows(env)) != 3 {
		t.Fatalf("--agent envelope meta=%v rows=%d, want source local and 3 rows", ujcMeta(env), len(ujcRows(env)))
	}
}

func TestUJCDoctorContentCheckIsNeverCached(t *testing.T) {
	ujcResetGuardRefusal(t)
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	ujcWantCode(t, ujcRun(t, "", "doctor"), 0)
	site.SetMode("challenge")
	before := site.Count("")
	r := ujcRun(t, "", "doctor")
	if site.Count("") == before {
		t.Fatalf("doctor sent no health request; it answered %q from the response cache", strings.TrimSpace(r.Stdout))
	}
	ujcWantCode(t, r, 7)
	if !strings.Contains(r.Stdout, "API: blocked") {
		t.Fatalf("doctor stdout = %q, want API: blocked", r.Stdout)
	}
}

func TestUJCCareersSearchChallengeExitsRefused(t *testing.T) {
	site := ujcNewFake(t)
	site.SetMode("challenge")
	ujcIsolate(t, site.URL, "")
	r := ujcRun(t, "", "careers", "search", "--countries", "Germany", "--json")
	ujcWantCode(t, r, 7)
}

func TestUJCScreenEvidenceStopsAtLineBreaks(t *testing.T) {
	p := ujcDescPosting("1", "Requirements\nFluent Dutch\nNice to have\nPython")
	row := screenPosting(p, nil, []string{"fluent dutch"})
	if got := *row.Evidence[0].Sentence; got != "Fluent Dutch" {
		t.Fatalf("evidence sentence = %q, want only the matching line %q", got, "Fluent Dutch")
	}
}

func TestUJCNewClosedOnHasOneFormat(t *testing.T) {
	rig := ujcNewNewRig(t)
	rig.save("everything", uberjobs.Filters{})
	rig.run("everything", "live", 50, nil)
	ujcApply(t, rig.db, rig.site.Postings(t), "all", true, time.Now().Add(-time.Hour))
	rig.site.Drop(ujcIDMidNLD)
	ujcApply(t, rig.db, rig.site.Postings(t), "all", true, time.Now().Add(-30*time.Minute))
	rig.site.Drop(ujcIDNewestNLD)
	row := rig.run("everything", "live", 50, nil)
	got := map[string]string{}
	for _, rm := range row.Removed {
		got[rm.ID] = *rm.ClosedOn
	}
	if len(got) != 2 || len(got[ujcIDMidNLD]) != len(got[ujcIDNewestNLD]) {
		t.Fatalf("closed_on values = %v, want one layout for every closed row", got)
	}
}

func TestUJCDoctorIgnoresStaleGuardRefusal(t *testing.T) {
	ujcResetGuardRefusal(t)
	ujcIsolate(t, "", "")
	dir := t.TempDir()
	g := &uberGuard{base: &ujcRT{status: map[string]int{"jobs.uber.com": http.StatusForbidden}}, gate: uberjobs.NewGate(dir), stateDir: dir}
	if _, err := g.RoundTrip(ujcGet(t, "https://jobs.uber.com/api/jobs/search/")); err != nil {
		t.Fatal(err)
	}
	r := ujcRun(t, "", "doctor")
	ujcWantCode(t, r, 6)
	if !strings.Contains(r.Stdout, "API: unreachable") {
		t.Fatalf("doctor stdout = %q, want API: unreachable for a dead port", r.Stdout)
	}
}

// TestUJCReadLiveFallsBackOnlyOnRefusal: only a refusal may switch to the
// Oracle engine. A site 500, an HTML 200, or a dead port must surface as its
// own typed error with zero Oracle requests; otherwise a broken site silently
// changes engines and spends Oracle traffic the owner did not approve.
func TestUJCReadLiveFallsBackOnlyOnRefusal(t *testing.T) {
	gbr := uberjobs.Filters{Countries: []string{"GBR"}}
	for _, mode := range []string{"500", "html"} {
		t.Run(mode, func(t *testing.T) {
			site, oracle := ujcNewFake(t), ujcNewFake(t)
			site.SetMode(mode)
			read, err := readLive(context.Background(), ujcClient(site, oracle), gbr)
			if err == nil {
				t.Fatalf("site mode %s: read %d rows from %s, want the site error", mode, len(read.Postings), read.Source)
			}
			var se *uberjobs.StatusError
			var ce *uberjobs.ContentError
			if (mode == "500" && !errors.As(err, &se)) || (mode == "html" && !errors.As(err, &ce)) {
				t.Fatalf("site mode %s: err = %T %v, want the typed site error", mode, err, err)
			}
			if n := oracle.Count(""); n != 0 {
				t.Fatalf("site mode %s: Oracle saw %d requests, want 0: %v", mode, n, oracle.Requests())
			}
			if n := site.Count(""); n != 1 {
				t.Fatalf("site mode %s: site saw %d requests, want 1 (never retried)", mode, n)
			}
		})
	}
	t.Run("transport", func(t *testing.T) {
		oracle := ujcNewFake(t)
		c := uberjobs.NewClient(ujcClosedURL(t), 0, "", "")
		c.Limiter, c.RequestLog, c.OracleBase = nil, "", oracle.URL
		_, err := readLive(context.Background(), c, gbr)
		if !uberjobs.IsTransport(err) {
			t.Fatalf("dead site: err = %T %v, want a TransportError", err, err)
		}
		if n := oracle.Count(""); n != 0 {
			t.Fatalf("dead site: Oracle saw %d requests, want 0: %v", n, oracle.Requests())
		}
	})
}

// TestUJCBareInvocationPrintsHelpAndSendsNothing: verify's required-flag
// probe runs every command bare, outside its mock server, so a command that
// does network work with no arguments reaches the real site (shipcheck on
// 2026-10-05 saw facets, stats, sync and careers search try). Bare must mean
// help, with zero requests, for every command that would otherwise read.
func TestUJCBareInvocationPrintsHelpAndSendsNothing(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	for _, path := range [][]string{
		{"facets"}, {"stats"}, {"searches"}, {"sync"}, {"postings"}, {"get"}, {"check"}, {"screen"}, {"new"}, {"save"},
		{"careers", "search"}, {"careers", "lookup"},
	} {
		r := ujcRun(t, "", path...)
		ujcWantCode(t, r, 0)
		if !strings.Contains(r.Stdout, "Usage:") {
			t.Fatalf("%v run bare printed no help:\n%s", path, r.Stdout)
		}
	}
	if n := site.Count(""); n != 0 {
		t.Fatalf("bare invocations sent %d requests, want 0", n)
	}
}

// TestUJCReadLiveRefusesFallbackForUnservableFilters: Oracle rows carry no
// work pattern or description, so after a site refusal these filters would
// silently return zero rows (or, for --description-not-contains, every row)
// with exit 0. The fallback must be refused with the site's refusal instead,
// and Oracle must see no request.
func TestUJCReadLiveRefusesFallbackForUnservableFilters(t *testing.T) {
	for name, f := range map[string]uberjobs.Filters{
		"work pattern":             {Countries: []string{"GBR"}, WorkPattern: "Intern"},
		"description contains":     {Countries: []string{"GBR"}, DescriptionContains: []string{"python"}},
		"description not contains": {Countries: []string{"GBR"}, DescriptionExcludes: []string{"sponsorship"}},
	} {
		t.Run(name, func(t *testing.T) {
			site, oracle := ujcNewFake(t), ujcNewFake(t)
			site.SetMode("challenge")
			read, err := readLive(context.Background(), ujcClient(site, oracle), f)
			if err == nil {
				t.Fatalf("%s: read %d rows from %s, want the refusal", name, len(read.Postings), read.Source)
			}
			if !uberjobs.IsRefusal(err) || !strings.Contains(err.Error(), "no fallback was attempted") {
				t.Fatalf("%s: err = %v, want the site refusal with no fallback", name, err)
			}
			if n := oracle.Count(""); n != 0 {
				t.Fatalf("%s: Oracle saw %d requests, want 0", name, n)
			}
		})
	}
	// A country-only read still falls back.
	site, oracle := ujcNewFake(t), ujcNewFake(t)
	site.SetMode("challenge")
	read, err := readLive(context.Background(), ujcClient(site, oracle), uberjobs.Filters{Countries: []string{"GBR"}})
	if err != nil || read.Source != uberjobs.SourceOracle || len(read.Postings) == 0 {
		t.Fatalf("country-only read after a refusal: err=%v source=%v rows=%d, want Oracle rows", err, read != nil && read.Source != "", 0)
	}
}

// TestUJCSecondaryCountryNote: --country matches any location, so a row can
// match through a secondary location while country_code names its primary one
// (live NLD output, 2026-10-05); postings and screen must say so.
func TestUJCSecondaryCountryNote(t *testing.T) {
	gbr, nld := "GBR", "NLD"
	rows := []uberjobs.Posting{
		{ID: "1", CountryCode: &nld},
		{ID: "2", CountryCode: &gbr, Locations: []uberjobs.Location{{CountryCode: &gbr}, {CountryCode: &nld}}},
	}
	if note := secondaryCountryNote(rows, []string{"NLD"}); !strings.Contains(note, "1 of 2 postings match --country through a secondary location") {
		t.Fatalf("note = %q, want the 1-of-2 secondary-location note", note)
	}
	if note := secondaryCountryNote(rows[:1], []string{"NLD"}); note != "" {
		t.Fatalf("all rows primary-matched, note = %q, want none", note)
	}
	if note := secondaryCountryNote(rows, nil); note != "" {
		t.Fatalf("no --country, note = %q, want none", note)
	}
}

// TestUJCNewBaselineSizeAfterEstablish: the first run reported baseline_size 0
// while its note said the baseline held 17 (live, 2026-10-05); baseline_size
// is the baseline after the run, matching searches.
func TestUJCNewBaselineSizeAfterEstablish(t *testing.T) {
	rig := ujcNewNewRig(t)
	rig.save("gbr", uberjobs.Filters{Countries: []string{"GBR"}})
	row := rig.run("gbr", "live", 50, nil)
	if !row.BaselineEstablished || row.CurrentCount == 0 || row.BaselineSize != row.CurrentCount {
		t.Fatalf("first run: established=%v current=%d baseline_size=%d, want baseline_size == current_count > 0", row.BaselineEstablished, row.CurrentCount, row.BaselineSize)
	}
	sv, err := uberjobs.GetSearch(context.Background(), rig.db, "gbr")
	if err != nil || sv.BaselineSize != row.BaselineSize {
		t.Fatalf("searches baseline_size = %v (err %v), want %d", sv, err, row.BaselineSize)
	}
}

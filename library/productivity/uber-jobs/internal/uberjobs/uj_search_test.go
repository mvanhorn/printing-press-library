// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// TestSearchAllProbeThenOnePage pins the read shape: one pagesize=1 probe,
// then exactly one page sized total+50. Multi-page walks reorder between
// page sizes, so a second page request would be a regression.
func TestSearchAllProbeThenOnePage(t *testing.T) {
	rows := ujCorpusRows(t)
	srv := ujSearchServer(t, rows, len(rows))
	c := ujClient(t, srv.URL)

	res, err := c.SearchAll(context.Background(), Query{})
	if err != nil {
		t.Fatalf("SearchAll: %v", err)
	}
	hits := srv.Hits()
	if len(hits) != 2 {
		t.Fatalf("server saw %d requests, want 2 (probe + one page)", len(hits))
	}
	for i, h := range hits {
		if h.Method != http.MethodGet || h.Path != searchPath {
			t.Errorf("request %d = %s %s, want GET %s", i, h.Method, h.Path, searchPath)
		}
		if got := h.Header.Get("User-Agent"); got != UserAgent {
			t.Errorf("request %d User-Agent = %q, want %q", i, got, UserAgent)
		}
	}
	probe, _ := url.ParseQuery(hits[0].RawQuery)
	if probe.Get("page") != "1" || probe.Get("pagesize") != "1" {
		t.Errorf("probe query = %q, want page=1&pagesize=1", hits[0].RawQuery)
	}
	page, _ := url.ParseQuery(hits[1].RawQuery)
	if page.Get("page") != "1" || page.Get("pagesize") != "106" {
		t.Errorf("page query = %q, want page=1&pagesize=106 (56 + 50 headroom)", hits[1].RawQuery)
	}
	if !res.Complete || res.ScanCapHit || res.Note != "" {
		t.Errorf("Complete=%v ScanCapHit=%v Note=%q, want a complete uncapped read with no note", res.Complete, res.ScanCapHit, res.Note)
	}
	if res.Total != 56 || len(res.Rows) != 56 || res.Requests != 2 || c.Requests() != 2 {
		t.Errorf("Total=%d rows=%d Requests=%d client=%d, want 56/56/2/2", res.Total, len(res.Rows), res.Requests, c.Requests())
	}
	ids := map[string]bool{}
	for _, r := range res.Rows {
		ids[string(r.ID)] = true
	}
	if !ids["303232"] || !ids["302016"] {
		t.Errorf("newest ids 303232 and 302016 missing from the read")
	}
}

// TestSearchAllDedupesDuplicateIDs guards against a page that repeats an
// id: the duplicate is dropped and the read is marked incomplete, never
// silently reported as the full corpus.
func TestSearchAllDedupesDuplicateIDs(t *testing.T) {
	rows := ujCorpusRows(t)[:3]
	page := []json.RawMessage{rows[0], rows[1], rows[0], rows[2]}
	srv := ujSearchServer(t, page, 4)
	c := ujClient(t, srv.URL)

	res, err := c.SearchAll(context.Background(), Query{})
	if err != nil {
		t.Fatalf("SearchAll: %v", err)
	}
	if len(res.Rows) != 3 {
		t.Fatalf("rows = %d, want 3 unique", len(res.Rows))
	}
	if res.Complete {
		t.Errorf("Complete = true, want false when the site's total (4) exceeds unique rows (3)")
	}
	if res.ScanCapHit {
		t.Errorf("ScanCapHit = true, want false")
	}
	if !strings.Contains(res.Note, "1 duplicates") || !strings.Contains(res.Note, "returned 3 unique rows") {
		t.Errorf("Note = %q, want it to name the duplicate count and unique rows", res.Note)
	}
}

// TestSearchAllPageCap guards the 5000-row page cap: a larger total marks
// the read capped and incomplete so nothing downstream treats it as whole.
func TestSearchAllPageCap(t *testing.T) {
	rows := ujCorpusRows(t)[:5]
	srv := ujSearchServer(t, rows, 6000)
	c := ujClient(t, srv.URL)

	res, err := c.SearchAll(context.Background(), Query{})
	if err != nil {
		t.Fatalf("SearchAll: %v", err)
	}
	hits := srv.Hits()
	if len(hits) != 2 {
		t.Fatalf("requests = %d, want 2", len(hits))
	}
	if q, _ := url.ParseQuery(hits[1].RawQuery); q.Get("pagesize") != "5000" {
		t.Errorf("page size = %q, want the 5000 cap", q.Get("pagesize"))
	}
	if !res.ScanCapHit || res.Complete {
		t.Errorf("ScanCapHit=%v Complete=%v, want true/false", res.ScanCapHit, res.Complete)
	}
	if res.Total != 6000 || !strings.Contains(res.Note, "page cap") {
		t.Errorf("Total=%d Note=%q, want 6000 and a page-cap note", res.Total, res.Note)
	}
}

// TestSearchAllZeroTotal checks that an empty query result is complete,
// has a non-nil empty row slice, and costs only the probe.
func TestSearchAllZeroTotal(t *testing.T) {
	srv := ujSearchServer(t, nil, 0)
	c := ujClient(t, srv.URL)

	res, err := c.SearchAll(context.Background(), Query{Search: "zzzz-nothing"})
	if err != nil {
		t.Fatalf("SearchAll: %v", err)
	}
	if srv.Count() != 1 {
		t.Errorf("requests = %d, want only the probe", srv.Count())
	}
	if !res.Complete || res.Rows == nil || len(res.Rows) != 0 || res.Requests != 1 {
		t.Errorf("Complete=%v rows=%v Requests=%d, want complete, empty non-nil rows, 1 request", res.Complete, res.Rows, res.Requests)
	}
}

// TestSearchAllRowWithoutID guards the dedupe key: a row with no Id cannot
// be tracked, so the whole read fails as a content error.
func TestSearchAllRowWithoutID(t *testing.T) {
	rows := []json.RawMessage{ujCorpusRows(t)[0], json.RawMessage(`{"Title":"No id here","Reference":"123"}`)}
	srv := ujSearchServer(t, rows, 2)
	c := ujClient(t, srv.URL)

	res, err := c.SearchAll(context.Background(), Query{})
	var ce *ContentError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v (%T), want *ContentError", err, err)
	}
	if !strings.Contains(ce.Reason, "row 1 has no Id") {
		t.Errorf("Reason = %q, want it to name row 1", ce.Reason)
	}
	if res != nil {
		t.Errorf("result = %+v, want nil on a content error", res)
	}
}

// TestSearchContentChecks: every 2xx reply that fails the positive content
// check is a *ContentError, never an empty result that would read as "no
// postings" (and, in a full sync, close every stored posting).
func TestSearchContentChecks(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		reason string
	}{
		{"html page", "<!DOCTYPE html><html><head><title>Uber Careers</title></head><body>home</body></html>", "not a JSON object"},
		{"plain text", "Service temporarily unavailable", "not a JSON object"},
		{"empty body", "", "not a JSON object"},
		{"json array", `[{"Id":"1"}]`, "not a JSON object"},
		{"truncated json", `{"jobs":[{"Id":"1"}`, "invalid JSON"},
		{"missing jobs", `{"totalJobs":5}`, "missing jobs array or totalJobs"},
		{"missing totalJobs", `{"jobs":[]}`, "missing jobs array or totalJobs"},
		{"null jobs", `{"jobs":null,"totalJobs":0}`, "missing jobs array or totalJobs"},
		{"error field", `{"error":"Failed to search jobs","jobs":[],"totalJobs":0}`, "site reported error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			})
			c := ujClient(t, srv.URL)
			res, err := c.SearchAll(context.Background(), Query{})
			var ce *ContentError
			if !errors.As(err, &ce) {
				t.Fatalf("err = %v (%T), want *ContentError", err, err)
			}
			if !strings.Contains(ce.Reason, tc.reason) {
				t.Errorf("Reason = %q, want it to contain %q", ce.Reason, tc.reason)
			}
			if IsRefusal(err) || IsTransport(err) {
				t.Errorf("content error misclassified as refusal/transport: %v", err)
			}
			if res != nil {
				t.Errorf("result = %+v, want nil", res)
			}
			if srv.Count() != 1 {
				t.Errorf("requests = %d, want 1 (the probe fails, no page request)", srv.Count())
			}
		})
	}
}

// TestSearchContentCheckOnPage covers a good probe followed by a bad page:
// the second reply is checked too.
func TestSearchContentCheckOnPage(t *testing.T) {
	rows := ujCorpusRows(t)[:2]
	srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("pagesize") == "1" {
			_, _ = w.Write(ujSearchJSON(rows[:1], 2))
			return
		}
		_, _ = w.Write([]byte("<html><body>maintenance</body></html>"))
	})
	c := ujClient(t, srv.URL)
	_, err := c.SearchAll(context.Background(), Query{})
	var ce *ContentError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v, want *ContentError from the page reply", err)
	}
	if !strings.Contains(ce.URL, "pagesize=52") {
		t.Errorf("ContentError URL = %q, want the page URL", ce.URL)
	}
	if srv.Count() != 2 {
		t.Errorf("requests = %d, want 2", srv.Count())
	}
}

// TestSearchStatusError: a 5xx is a *StatusError (exit 5), not a refusal
// and not an empty result.
func TestSearchStatusError(t *testing.T) {
	srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"Failed to search jobs","jobs":[],"totalJobs":0}`))
	})
	c := ujClient(t, srv.URL)
	_, err := c.SearchAll(context.Background(), Query{})
	var se *StatusError
	if !errors.As(err, &se) || se.Status != http.StatusInternalServerError {
		t.Fatalf("err = %v (%T), want *StatusError 500", err, err)
	}
	if IsRefusal(err) {
		t.Errorf("500 must not be a refusal")
	}
	if srv.Count() != 1 {
		t.Errorf("requests = %d, want 1 (no retry)", srv.Count())
	}
}

// TestQueryValuesRepeatedCountries guards the countries key shape: it
// repeats, and the bracket form countries[] (silently ignored by the site)
// never appears, on the wire or in the encoded query.
func TestQueryValuesRepeatedCountries(t *testing.T) {
	lat, lng := 51.5, -0.12
	q := Query{
		Search:       " data engineer ",
		Countries:    []string{"United Kingdom", " ", "Germany"},
		Team:         "Engineer",
		SubTeam:      "Software Engineering",
		ContractType: "Full time",
		Location:     "London",
		Lat:          &lat,
		Lng:          &lng,
		Radius:       25,
	}
	v := q.Values()
	if got := v["countries"]; len(got) != 2 || got[0] != "United Kingdom" || got[1] != "Germany" {
		t.Errorf("countries = %q, want [United Kingdom Germany] (blank dropped)", got)
	}
	want := map[string]string{"search": "data engineer", "team": "Engineer", "subTeam": "Software Engineering", "contractTypes": "Full time", "location": "London", "lat": "51.5", "lng": "-0.12", "radius": "25"}
	for k, w := range want {
		if v.Get(k) != w {
			t.Errorf("%s = %q, want %q", k, v.Get(k), w)
		}
	}
	enc := v.Encode()
	if strings.Contains(enc, "countries[]") || strings.Contains(enc, "countries%5B%5D") {
		t.Errorf("encoded query uses the bracket form: %s", enc)
	}
	if !strings.Contains(enc, "countries=United+Kingdom") || !strings.Contains(enc, "countries=Germany") {
		t.Errorf("encoded query = %s, want repeated countries=", enc)
	}

	srv := ujSearchServer(t, nil, 0)
	c := ujClient(t, srv.URL)
	if _, err := c.SearchAll(context.Background(), Query{Countries: []string{"United Kingdom", "Germany"}}); err != nil {
		t.Fatalf("SearchAll: %v", err)
	}
	raw := srv.Hits()[0].RawQuery
	if strings.Contains(raw, "%5B") || strings.Contains(raw, "[") {
		t.Errorf("wire query uses brackets: %s", raw)
	}
	if got, _ := url.ParseQuery(raw); len(got["countries"]) != 2 {
		t.Errorf("wire countries = %q, want 2 repeated values", got["countries"])
	}
}

// TestQueryValuesGeoNeedsBoth: lat without lng (or radius without a point)
// is dropped rather than sending a half-specified geo filter.
func TestQueryValuesGeoNeedsBoth(t *testing.T) {
	lat := 10.0
	v := Query{Lat: &lat, Radius: 5}.Values()
	for _, k := range []string{"lat", "lng", "radius"} {
		if v.Has(k) {
			t.Errorf("%s set without both coordinates: %v", k, v)
		}
	}
	if !(Query{}).IsUnfiltered() || !(Query{Search: "  "}).IsUnfiltered() {
		t.Errorf("empty and blank queries should be unfiltered")
	}
	if (Query{Team: "Sales"}).IsUnfiltered() {
		t.Errorf("a team query is filtered")
	}
}

// TestProbeTotal reads the total from a one-row probe.
func TestProbeTotal(t *testing.T) {
	srv := ujSearchServer(t, ujCorpusRows(t), 584)
	c := ujClient(t, srv.URL)
	total, resp, err := c.ProbeTotal(context.Background(), Query{Team: "Sales"})
	if err != nil {
		t.Fatalf("ProbeTotal: %v", err)
	}
	if total != 584 || resp == nil || resp.CacheHit {
		t.Errorf("total=%d resp=%v, want 584 from the network", total, resp)
	}
	q, _ := url.ParseQuery(srv.Hits()[0].RawQuery)
	if q.Get("team") != "Sales" || q.Get("pagesize") != "1" {
		t.Errorf("probe query = %v, want team=Sales and pagesize=1", q)
	}
}

// TestLookup checks the batch lookup POST body and lenient decode of a
// numeric id; the site drops unknown ids, so only listed ids come back.
func TestLookup(t *testing.T) {
	srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != lookupPath {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"jobs":[{"id":303232,"title":"People Operations Employee Data Specialist","url":"/en/jobs/303232/","locations":[{"Country":"Netherlands"}],"teams":[]}]}`))
	})
	c := ujClient(t, srv.URL)
	rows, err := c.Lookup(context.Background(), []string{"303232", "999999"})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "303232" || rows[0].Title != "People Operations Employee Data Specialist" {
		t.Fatalf("rows = %+v, want one row for 303232", rows)
	}
	if len(rows[0].Locations) != 1 || rows[0].Locations[0].Country != "Netherlands" {
		t.Errorf("locations = %+v", rows[0].Locations)
	}
	h := srv.Hits()[0]
	if ct := h.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body struct {
		JobIDs []string `json:"jobIds"`
		Locale string   `json:"locale"`
	}
	if err := json.Unmarshal(h.Body, &body); err != nil {
		t.Fatalf("request body %q: %v", h.Body, err)
	}
	if len(body.JobIDs) != 2 || body.JobIDs[0] != "303232" || body.Locale != "en" {
		t.Errorf("request body = %+v, want both ids and locale en", body)
	}
}

// TestLookupContentChecks: a lookup reply without a jobs array is a content
// error, so get never reports a listed posting as "not listed".
func TestLookupContentChecks(t *testing.T) {
	for _, body := range []string{`{"items":[]}`, `{"jobs":null}`, "<html>login</html>", ""} {
		srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) })
		c := ujClient(t, srv.URL)
		_, err := c.Lookup(context.Background(), []string{"1"})
		var ce *ContentError
		if !errors.As(err, &ce) {
			t.Errorf("body %q: err = %v, want *ContentError", body, err)
		}
	}
}

// TestLookupIsNeverCached: POSTs bypass the response cache, so a lookup
// always reflects the site.
func TestLookupIsNeverCached(t *testing.T) {
	srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"jobs":[]}`)) })
	c := ujClient(t, srv.URL)
	c.CacheDir = t.TempDir()
	for i := 0; i < 2; i++ {
		if _, err := c.Lookup(context.Background(), []string{"1"}); err != nil {
			t.Fatalf("Lookup %d: %v", i, err)
		}
	}
	if srv.Count() != 2 {
		t.Errorf("requests = %d, want 2 (no cache for POST)", srv.Count())
	}
}

// TestFetchFacets serves the captured /en/jobs/ page and a page without
// the embedded lists.
func TestFetchFacets(t *testing.T) {
	page := ujReadTestdata(t, "facets.html")
	srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != facetsPath {
			_, _ = w.Write([]byte("<html><body>no facets</body></html>"))
			return
		}
		_, _ = w.Write(page)
	})
	c := ujClient(t, srv.URL)
	f, err := c.FetchFacets(context.Background())
	if err != nil {
		t.Fatalf("FetchFacets: %v", err)
	}
	if len(f.Countries) != 38 || len(f.Teams) != 18 {
		t.Errorf("countries=%d teams=%d, want 38/18", len(f.Countries), len(f.Teams))
	}
	if got := srv.Hits()[0].Header.Get("Accept"); got != "text/html" {
		t.Errorf("Accept = %q, want text/html", got)
	}

	c.BaseURL = srv.URL + "/other"
	_, err = c.FetchFacets(context.Background())
	var ce *ContentError
	if !errors.As(err, &ce) {
		t.Fatalf("page without facets: err = %v, want *ContentError", err)
	}
}

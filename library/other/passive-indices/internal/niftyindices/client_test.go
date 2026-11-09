// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.

package niftyindices

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/passive-indices/internal/cliutil"
)

func TestSlugify(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"upper with space", "NIFTY 50", "nifty50"},
		{"already lower", "nifty next 50", "niftynext50"},
		{"mixed case multi space", "NIFTY Bank", "niftybank"},
		{"no spaces", "SENSEX", "sensex"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Slugify(tc.in); got != tc.want {
				t.Errorf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"under limit", "hello", 10, "hello"},
		{"over limit", "hello", 3, "hel..."},
		{"empty", "", 5, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := truncate(tc.in, tc.max); got != tc.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
		})
	}
}

// TestCinfoBody asserts the /BackPage/ request body shape: the outer
// object's "cinfo" value is itself a JSON-encoded string (not a nested
// object), matching the BackPage PageMethods calling convention. Getting
// this double-encoding wrong is exactly the kind of bug that produces a
// silent HTTP 200 with an unparseable body rather than a clean error.
func TestCinfoBody(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)

	body, err := cinfoBody("NIFTY 50", from, to)
	if err != nil {
		t.Fatalf("cinfoBody returned error: %v", err)
	}

	var outer map[string]string
	if err := json.Unmarshal(body, &outer); err != nil {
		t.Fatalf("unmarshaling outer body: %v", err)
	}
	cinfo, ok := outer["cinfo"]
	if !ok {
		t.Fatalf("outer body missing %q key: %s", "cinfo", body)
	}

	var inner map[string]string
	if err := json.Unmarshal([]byte(cinfo), &inner); err != nil {
		t.Fatalf("cinfo value is not JSON-encoded: %v (value=%q)", err, cinfo)
	}
	want := map[string]string{
		"name":      "NIFTY 50",
		"indexName": "NIFTY 50",
		"startDate": "01-Jan-2026",
		"endDate":   "30-Jun-2026",
	}
	for k, wantV := range want {
		if inner[k] != wantV {
			t.Errorf("cinfo[%q] = %q, want %q", k, inner[k], wantV)
		}
	}
}

func TestParseSectorWeightLabel(t *testing.T) {
	cases := []struct {
		name       string
		label      string
		wantName   string
		wantWeight float64
	}{
		{"constituent label", "ABCAPITAL 2.25%", "ABCAPITAL", 2.25},
		{"sector label", "Financial Services 33.16%", "Financial Services", 33.16},
		{"integer-valued weight", "KARURVYSYA 2%", "KARURVYSYA", 2},
		{"no percent suffix falls back to zero weight", "All", "All", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name, weight := parseSectorWeightLabel(tc.label)
			if name != tc.wantName || weight != tc.wantWeight {
				t.Errorf("parseSectorWeightLabel(%q) = (%q, %v), want (%q, %v)", tc.label, name, weight, tc.wantName, tc.wantWeight)
			}
		})
	}
}

// TestParseSectorWeightsJSONP covers the two real quirks of this feed found
// during live investigation: the modelDataAvailable(...) callback carries a
// SECOND, unrelated metadata argument after the data object (must be
// ignored, not fed to json.Unmarshal), and every array/object literal in
// the first argument uses JS-style trailing commas invalid in strict JSON.
func TestParseSectorWeightsJSONP(t *testing.T) {
	body := []byte(`modelDataAvailable({"groups":[{"label":"Financial Services 33.16%","colour":"","weight":33.16,"groups":[{"label":"ABCAPITAL 2.25%","weight":2.25,"id":"1","date":"30-06-2026"},],"id":"0"},]},{ label:  "NIFTY ALPHA 50 ",file: "SectorialIndexDataNIFTY ALPHA 50.jsonp "});`)

	wire, err := parseSectorWeightsJSONP(body)
	if err != nil {
		t.Fatalf("parseSectorWeightsJSONP returned error: %v", err)
	}
	if len(wire.Groups) != 1 {
		t.Fatalf("got %d top-level groups, want 1", len(wire.Groups))
	}
	g := wire.Groups[0]
	if g.Label != "Financial Services 33.16%" {
		t.Errorf("group label = %q, want %q", g.Label, "Financial Services 33.16%")
	}
	if len(g.Groups) != 1 || g.Groups[0].Label != "ABCAPITAL 2.25%" {
		t.Errorf("nested groups = %+v, want one entry labeled %q", g.Groups, "ABCAPITAL 2.25%")
	}
}

func TestParseSectorWeightsJSONPRejectsNonCallback(t *testing.T) {
	if _, err := parseSectorWeightsJSONP([]byte(`{"not":"a callback"}`)); err == nil {
		t.Error("expected an error for a body with no modelDataAvailable(...) wrapper")
	}
}

const testCSV = "Company Name,Industry,Symbol,Series,ISIN Code\nInfosys Ltd.,Information Technology,INFY,EQ,INE009A01021\n"

func fastRetries(t *testing.T) {
	t.Helper()
	base, max := cliutil.UpstreamHTMLBackoffBase, cliutil.UpstreamHTMLBackoffMax
	cliutil.UpstreamHTMLBackoffBase, cliutil.UpstreamHTMLBackoffMax = time.Millisecond, 4*time.Millisecond
	t.Cleanup(func() { cliutil.UpstreamHTMLBackoffBase, cliutil.UpstreamHTMLBackoffMax = base, max })
}

// htmlThenServer answers the first k requests with an HTTP 200 HTML page and
// every later one with body.
func htmlThenServer(t *testing.T, k int, contentType, body string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if int(hits.Add(1)) <= k {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<!DOCTYPE html><html><body>Please try again</body></html>"))
			return
		}
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestConstituentsRetriesHTMLThenSucceeds(t *testing.T) {
	fastRetries(t)
	for _, k := range []int{0, 1, 4} {
		srv, hits := htmlThenServer(t, k, "application/octet-stream", testCSV)
		c := New(5*time.Second, 0)
		c.base = srv.URL
		rows, err := c.Constituents(context.Background(), "niftyit")
		if err != nil {
			t.Fatalf("k=%d: %v", k, err)
		}
		if len(rows) != 1 || rows[0].Symbol != "INFY" {
			t.Fatalf("k=%d: rows = %+v", k, rows)
		}
		if int(hits.Load()) != k+1 {
			t.Errorf("k=%d: hits = %d, want %d", k, hits.Load(), k+1)
		}
	}
}

func TestConstituentsExhaustsRetries(t *testing.T) {
	fastRetries(t)
	srv, hits := htmlThenServer(t, 1000, "application/octet-stream", testCSV)
	c := New(5*time.Second, 0)
	c.base = srv.URL
	_, err := c.Constituents(context.Background(), "nosuchindex")
	var he *cliutil.UpstreamHTMLError
	if !errors.As(err, &he) {
		t.Fatalf("err = %v, want *UpstreamHTMLError", err)
	}
	if he.Attempts != cliutil.UpstreamHTMLMaxAttempts || int(hits.Load()) != cliutil.UpstreamHTMLMaxAttempts {
		t.Errorf("attempts = %d, hits = %d, want %d", he.Attempts, hits.Load(), cliutil.UpstreamHTMLMaxAttempts)
	}
	if !strings.Contains(err.Error(), "HTML page instead of the CSV after 5 attempts") {
		t.Errorf("message = %q", err)
	}
}

func TestConstituentsDoesNotRetryHTTPErrors(t *testing.T) {
	fastRetries(t)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := New(5*time.Second, 0)
	c.base = srv.URL
	_, err := c.Constituents(context.Background(), "nifty50")
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") || hits.Load() != 1 {
		t.Fatalf("err = %v, hits = %d", err, hits.Load())
	}
}

func TestConstituentsHTMLBodyWithCSVContentType(t *testing.T) {
	fastRetries(t)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("<html>blocked</html>"))
	}))
	defer srv.Close()
	c := New(5*time.Second, 0)
	c.base = srv.URL
	var he *cliutil.UpstreamHTMLError
	if _, err := c.Constituents(context.Background(), "nifty50"); !errors.As(err, &he) {
		t.Fatalf("err = %v, want *UpstreamHTMLError", err)
	}
}

func TestPostBackPageAndSectorWeightsRetryHTML(t *testing.T) {
	fastRetries(t)
	srv, hits := htmlThenServer(t, 2, "application/json", `[{"INDEX_NAME":"NIFTY 50","HistoricalDate":"01 Jan 2026","OPEN":"1","HIGH":"2","LOW":"0","CLOSE":"1"}]`)
	c := New(5*time.Second, 0)
	c.base = srv.URL
	rows, err := c.History(context.Background(), "NIFTY 50", time.Now().AddDate(0, 0, -7), time.Now())
	if err != nil || len(rows) != 1 || hits.Load() != 3 {
		t.Fatalf("History: rows=%v err=%v hits=%d", rows, err, hits.Load())
	}

	sector, shits := htmlThenServer(t, 1, "application/javascript", `modelDataAvailable({"groups":[{"label":"Financial Services 33.16%","groups":[{"label":"HDFCBANK 12.00%"}]}]},{})`)
	c.sectorBase = sector.URL
	groups, err := c.SectorWeights(context.Background(), "NIFTY 50")
	if err != nil || len(groups) != 1 || shits.Load() != 2 {
		t.Fatalf("SectorWeights: groups=%v err=%v hits=%d", groups, err, shits.Load())
	}
}

func TestConstituents429IsNotRetriedAsHTML(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	c := New(5*time.Second, 0)
	c.base = srv.URL
	var rl *cliutil.RateLimitError
	if _, err := c.Constituents(context.Background(), "nifty50"); !errors.As(err, &rl) || hits.Load() != 1 {
		t.Fatalf("err = %v, hits = %d", err, hits.Load())
	}
}

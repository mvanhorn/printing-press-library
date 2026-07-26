// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package subjects

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/overpass/internal/cliutil"
)

func TestLooksLikeHTML(t *testing.T) {
	if !looksLikeHTML([]byte("  <?xml version=\"1.0\"?><html>")) {
		t.Error("an XML/HTML error page should be detected")
	}
	if !looksLikeHTML([]byte("   ")) {
		t.Error("an empty body should not count as success")
	}
	if looksLikeHTML([]byte(`{"elements":[]}`)) {
		t.Error("JSON should not be flagged as HTML")
	}
}

func TestExtractOverpassError(t *testing.T) {
	page := `<html><body><p>The data included...</p>
	<p><strong style="color:#FF0000">Error</strong>: runtime error: open64: 0 Success /osm3s_osm_base Dispatcher_Client::request_read_and_idx::timeout. The server is probably too busy to handle your request. </p></body></html>`
	got := extractOverpassError([]byte(page))
	if !strings.Contains(got, "too busy") {
		t.Errorf("did not extract the reason: %q", got)
	}
	if strings.Contains(got, "<") || strings.Contains(got, "strong") {
		t.Errorf("markup leaked into the message: %q", got)
	}
	if got := extractOverpassError([]byte("<html>nothing useful</html>")); got == "" {
		t.Error("should always return something explanatory")
	}
}

// Overpass signals overload with HTTP 200 and an HTML body. A runner that
// checks only the status code treats that as success and hands back garbage.
func TestRunFailsOverFromA200HTMLPage(t *testing.T) {
	overloaded := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><p><strong>Error</strong>: runtime error: too busy</p></html>`))
	}))
	defer overloaded.Close()

	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request sent without a User-Agent; Overpass and Nominatim both require one")
		}
		if err := r.ParseForm(); err != nil || r.Form.Get("data") == "" {
			t.Errorf("query not sent in the data field: %v", r.Form)
		}
		_, _ = w.Write([]byte(`{"elements":[{"type":"node","id":1,"lat":1,"lon":2,"tags":{}}]}`))
	}))
	defer healthy.Close()

	r := NewRunner(5 * time.Second)
	r.Mirrors = []string{overloaded.URL, healthy.URL}

	body, attempts, err := r.Run(context.Background(), "[out:json];out;")
	if err != nil {
		t.Fatalf("failover did not recover: %v", err)
	}
	if !strings.Contains(string(body), "elements") {
		t.Errorf("wrong body returned: %s", body)
	}
	if len(attempts) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(attempts))
	}
	if attempts[0].Err == "" {
		t.Error("the overloaded mirror should be recorded as a failure")
	}
	if !strings.Contains(attempts[0].Err, "too busy") {
		t.Errorf("failure reason lost: %q", attempts[0].Err)
	}
	if attempts[1].Err != "" {
		t.Errorf("the healthy mirror should be recorded as a success: %+v", attempts[1])
	}
}

func TestRunFailsOverFromNon200(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"elements":[]}`))
	}))
	defer good.Close()

	r := NewRunner(5 * time.Second)
	r.Mirrors = []string{bad.URL, good.URL}
	if _, attempts, err := r.Run(context.Background(), "q"); err != nil {
		t.Fatalf("expected failover to succeed: %v", err)
	} else if attempts[0].Status != http.StatusTooManyRequests {
		t.Errorf("status not recorded: %+v", attempts[0])
	}
}

// When every mirror is down the error must say so and name the remedy, not
// surface one host's error as if it were the whole story.
func TestRunAllMirrorsDown(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer down.Close()

	r := NewRunner(5 * time.Second)
	r.Mirrors = []string{down.URL, down.URL}
	_, attempts, err := r.Run(context.Background(), "q")
	if err == nil {
		t.Fatal("expected an error when every mirror fails")
	}
	if !strings.Contains(err.Error(), "every Overpass mirror") {
		t.Errorf("error should name the situation: %v", err)
	}
	if len(attempts) != 2 {
		t.Errorf("every attempt should be recorded, got %d", len(attempts))
	}
}

func TestParseStatus(t *testing.T) {
	body := `Connected as: 798220406
Current time: 2026-07-26T15:59:44Z
Announced endpoint: gall.openstreetmap.de/
Rate limit: 2
2 slots available now.
Currently running queries (pid, space limit, time limit, start time):
`
	rate, slots, detail := parseStatus(body)
	if rate != 2 {
		t.Errorf("rate limit = %d, want 2", rate)
	}
	if slots != 2 {
		t.Errorf("slots free = %d, want 2", slots)
	}
	if detail == "" {
		t.Error("expected a detail line")
	}
}

func TestCheckMirrors(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/status") {
			t.Errorf("status check hit %q, expected the /status path", r.URL.Path)
		}
		_, _ = w.Write([]byte("Rate limit: 6\n4 slots available now.\n"))
	}))
	defer ok.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotAcceptable)
	}))
	defer bad.Close()

	r := NewRunner(5 * time.Second)
	r.Mirrors = []string{ok.URL + "/interpreter", bad.URL + "/interpreter"}
	got := r.CheckMirrors(context.Background())
	if len(got) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(got))
	}
	if !got[0].Healthy || got[0].SlotsFree != 4 || got[0].RateLimit != 6 {
		t.Errorf("healthy mirror parsed wrong: %+v", got[0])
	}
	if got[1].Healthy {
		t.Errorf("a 406 mirror should not be healthy: %+v", got[1])
	}
}

// Throttling and "this mirror knows of nothing here" both arrive as an empty
// result set. A 429 has to reach the caller as a typed error or a rate-limited
// search is indistinguishable from a search of an empty area.
func TestRunSurfacesTypedRateLimitErrorWhenEveryMirrorThrottles(t *testing.T) {
	throttling := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer throttling.Close()

	r := NewRunner(5 * time.Second)
	r.Mirrors = []string{throttling.URL}

	_, attempts, err := r.Run(context.Background(), "q")
	if err == nil {
		t.Fatal("expected an error when the only mirror throttles")
	}
	var rl *cliutil.RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("429 did not survive as a typed cliutil.RateLimitError: %v", err)
	}
	if rl.RetryAfter != 7*time.Second {
		t.Errorf("Retry-After lost: got %s, want 7s", rl.RetryAfter)
	}
	if len(attempts) != 1 || attempts[0].Status != http.StatusTooManyRequests {
		t.Errorf("the throttled attempt was not recorded: %+v", attempts)
	}
}

// Overpass answers a 429 with a full XHTML page. Echoing that body into the
// error put a doctype and inline style markup on the error line, twice.
func TestRateLimitErrorDoesNotEchoTheHTMLPage(t *testing.T) {
	page := `<?xml version="1.0"?><!DOCTYPE html><html><body>
	<p><strong style="color:#FF0000">Error</strong>: runtime error: open64: 0 Success /osm3s_osm_base Dispatcher_Client::request_read_and_idx::timeout. The server is probably too busy to handle your request. </p></body></html>`
	throttling := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(page))
	}))
	defer throttling.Close()

	r := NewRunner(5 * time.Second)
	r.Mirrors = []string{throttling.URL}

	_, attempts, err := r.Run(context.Background(), "q")
	if err == nil {
		t.Fatal("expected an error")
	}
	// Both surfaces the user sees: the per-attempt record and the wrapped
	// final error.
	for name, got := range map[string]string{"attempt": attempts[0].Err, "error": err.Error()} {
		if strings.Contains(got, "<") || strings.Contains(got, "DOCTYPE") || strings.Contains(got, "color:#FF0000") {
			t.Errorf("%s leaked markup: %q", name, got)
		}
		if !strings.Contains(got, "too busy") {
			t.Errorf("%s lost the actual reason: %q", name, got)
		}
	}
}

// `mirrors` exists to say which mirror is worth using. A sequential probe let
// the slowest mirror set the command's runtime, so the answer to "which one is
// fast" arrived at the speed of the slow one. Probes are concurrent and each
// is bounded, so total time tracks the slowest *bounded* probe, not the sum.
func TestCheckMirrorsIsConcurrentAndBounded(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer slow.Close()
	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Rate limit: 2\n2 slots available now.\n"))
	}))
	defer fast.Close()

	r := NewRunner(30 * time.Second)
	r.ProbeTimeout = 300 * time.Millisecond
	// Three slow mirrors and one fast one: sequential probing would cost at
	// least 3 x ProbeTimeout.
	r.Mirrors = []string{
		slow.URL + "/interpreter",
		slow.URL + "/interpreter",
		slow.URL + "/interpreter",
		fast.URL + "/interpreter",
	}

	start := time.Now()
	got := r.CheckMirrors(context.Background())
	elapsed := time.Since(start)

	if len(got) != 4 {
		t.Fatalf("expected 4 statuses, got %d", len(got))
	}
	if elapsed > 2*r.ProbeTimeout {
		t.Errorf("probes did not run concurrently: took %s for 4 mirrors at a %s bound", elapsed, r.ProbeTimeout)
	}
	// Order must follow Mirrors, not completion: the list doubles as the
	// failover preference order.
	for i := 0; i < 3; i++ {
		if got[i].Healthy {
			t.Errorf("mirror %d hung past the probe bound and should not be healthy: %+v", i, got[i])
		}
		if got[i].Mirror != slow.URL+"/interpreter" {
			t.Errorf("result %d out of order: %s", i, got[i].Mirror)
		}
	}
	if !got[3].Healthy || got[3].SlotsFree != 2 {
		t.Errorf("the fast mirror was not reported healthy: %+v", got[3])
	}
}

func TestStripTags(t *testing.T) {
	if got := stripTags("<b>bold</b> text"); got != "bold text" {
		t.Errorf("stripTags = %q", got)
	}
}

// Every default mirror must serve PLANET data. A regional extract answers
// quickly and successfully with zero results for anywhere outside its region:
// silently wrong rather than visibly broken, which is worse than a dead host.
// overpass.osm.ch was removed for exactly this reason.
//
// This asserts an explicit allowlist rather than a denylist of known-regional
// hosts. A denylist passes any new regional mirror someone adds, which is the
// failure it exists to prevent. Adding an entry here is a deliberate act that
// requires verifying planet coverage live first — query a bbox on at least two
// continents and confirm both return data.
func TestDefaultMirrorsAreOnTheVettedPlanetAllowlist(t *testing.T) {
	// Verified planet-wide on 2026-07-26 against California, Zurich, and
	// Sydney bounding boxes.
	vetted := map[string]bool{
		"https://overpass-api.de/api/interpreter":         true,
		"https://overpass.kumi.systems/api/interpreter":   true,
		"https://overpass.private.coffee/api/interpreter": true,
	}
	for _, m := range Mirrors {
		if !vetted[m] {
			t.Errorf("mirror %q is not on the vetted planet-coverage allowlist; verify it serves planet data on at least two continents and add it here deliberately", m)
		}
	}
	if len(Mirrors) < 2 {
		t.Error("failover needs at least two mirrors")
	}
	if strings.Contains(strings.Join(Mirrors, " "), "overpass.osm.ch") {
		t.Error("overpass.osm.ch is a Swiss regional extract and must never be a default")
	}
}

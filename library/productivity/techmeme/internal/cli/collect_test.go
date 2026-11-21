// Copyright 2026 Dave Morin and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// The fixtures under testdata/ are trimmed copies of Techmeme's /river,
// /feed.xml and /261120/h2000, saved on 2026-11-21.

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// techmemeStub serves fixtures by path and records every request. The RSS
// feed answers 304 when If-None-Match carries its ETag.
type techmemeStub struct {
	mu    sync.Mutex
	pages map[string][]byte
	reqs  []*http.Request
}

const stubETag = `"5603-660cde99fd08e"`

func (s *techmemeStub) serve(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.reqs = append(s.reqs, r.Clone(r.Context()))
		s.mu.Unlock()
		body, ok := s.pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/feed.xml" {
			if r.Header.Get("If-None-Match") == stubETag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", stubETag)
			w.Header().Set("Last-Modified", "Sat, 21 Nov 2026 18:56:04 GMT")
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	prev := collectBaseURLOverride
	collectBaseURLOverride = srv.URL
	t.Cleanup(func() { collectBaseURLOverride = prev })
}

func (s *techmemeStub) requests() []*http.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*http.Request(nil), s.reqs...)
}

func runTechmemeCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var flags rootFlags
	root := newRootCmd(&flags)
	root.SetArgs(args)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	err := root.Execute()
	return stdout.String(), err
}

func decodeRows(t *testing.T, stdout string) []map[string]any {
	t.Helper()
	var rows []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("not a JSON line: %q", line)
		}
		rows = append(rows, m)
	}
	return rows
}

// assertPublisherURL fails on any Techmeme link, redirect or not.
func assertPublisherURL(t *testing.T, row map[string]any) {
	t.Helper()
	raw, _ := row["url"].(string)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		t.Fatalf("row %v: url %q is not an absolute link", row["id"], raw)
	}
	if strings.Contains(u.Host, "techmeme.com") || strings.HasPrefix(u.Path, "/goto/") || strings.HasPrefix(u.Path, "/r/") {
		t.Fatalf("row %v: url %q is a Techmeme link, not the publisher's", row["id"], raw)
	}
	for _, key := range []string{"id", "title", "publication", "techmeme_permalink", "date"} {
		if s, _ := row[key].(string); s == "" {
			t.Fatalf("row %v: %s is empty", row["id"], key)
		}
	}
	if _, ok := row["published_at"]; !ok {
		t.Fatalf("row %v: published_at key missing", row["id"])
	}
}

func TestCollectRiverFixtureHasPublisherURLs(t *testing.T) {
	stub := &techmemeStub{pages: map[string][]byte{"/river": readFixture(t, "collect_river.html")}}
	stub.serve(t)
	out, err := runTechmemeCmd(t, "collect", "--source", "river")
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	rows := decodeRows(t, out)
	if len(rows) != 12 {
		t.Fatalf("want 12 river rows, got %d", len(rows))
	}
	for _, r := range rows {
		assertPublisherURL(t, r)
		if r["published_at"] == nil {
			t.Fatalf("river row %v has no published_at", r["id"])
		}
	}
	first := rows[0]
	if first["id"] != "261121p9" || first["publication"] != "Financial Times" ||
		first["url"] != "https://www.ft.com/content/f3ad8a81-3890-4a8a-8853-ff0a94b08ba9" ||
		first["techmeme_permalink"] != "https://www.techmeme.com/261121/p9#a261121p9" ||
		first["published_at"] != "2026-11-21T18:55:00Z" || first["date"] != "2026-11-21" {
		t.Fatalf("unexpected first river row: %v", first)
	}
	if last := rows[len(rows)-1]; last["date"] != "2026-11-20" {
		t.Fatalf("rows under the second date header should carry it: %v", last)
	}
}

func TestCollectRSSStripsSuffixAndHonors304(t *testing.T) {
	stub := &techmemeStub{pages: map[string][]byte{"/feed.xml": readFixture(t, "collect_feed.xml")}}
	stub.serve(t)
	state := filepath.Join(t.TempDir(), "rss.json")

	out, err := runTechmemeCmd(t, "collect", "--source", "rss", "--state", state)
	if err != nil {
		t.Fatalf("first collect: %v", err)
	}
	rows := decodeRows(t, out)
	if len(rows) != 6 {
		t.Fatalf("want 6 RSS rows, got %d", len(rows))
	}
	byID := map[string]map[string]any{}
	for _, r := range rows {
		assertPublisherURL(t, r)
		if title := r["title"].(string); strings.HasSuffix(title, ")") {
			t.Fatalf("title kept its (Publication) suffix: %q", title)
		}
		byID[r["id"].(string)] = r
	}
	if r := byID["261121p9"]; r == nil || r["publication"] != "Financial Times" ||
		r["published_at"] != "2026-11-21T18:55:01Z" ||
		!strings.HasPrefix(r["title"].(string), "Investigation: Alice Huang") {
		t.Fatalf("unexpected FT row: %v", r)
	}
	if r := byID["261120p43"]; r == nil || r["publication"] != "@oracle" || !strings.HasPrefix(r["url"].(string), "https://x.com/oracle/status/") {
		t.Fatalf("an X post should keep its handle and post URL: %v", r)
	}
	saved, err := readValidators(state)
	if err != nil || saved.ETag != stubETag || saved.LastModified == "" {
		t.Fatalf("--state should hold the 200's validators: %+v (%v)", saved, err)
	}

	out, err = runTechmemeCmd(t, "collect", "--source", "rss", "--state", state)
	if err != nil {
		t.Fatalf("a 304 must exit 0: %v", err)
	}
	if out != "" {
		t.Fatalf("a 304 must print nothing, got %q", out)
	}
	reqs := stub.requests()
	if len(reqs) != 2 || reqs[1].Header.Get("If-None-Match") != stubETag || reqs[1].Header.Get("If-Modified-Since") == "" {
		t.Fatalf("second request should be conditional: %v", reqs[len(reqs)-1].Header)
	}
}

func TestCollectRSSValidatorFlags(t *testing.T) {
	stub := &techmemeStub{pages: map[string][]byte{"/feed.xml": readFixture(t, "collect_feed.xml")}}
	stub.serve(t)
	out, err := runTechmemeCmd(t, "collect", "--source", "rss", "--etag", stubETag)
	if err != nil || out != "" {
		t.Fatalf("--etag should make a 304 that prints nothing: out=%q err=%v", out, err)
	}
}

func TestCollectDayFixtureCarriesThatDay(t *testing.T) {
	stub := &techmemeStub{pages: map[string][]byte{"/261120/h2000": readFixture(t, "collect_day_261120.html")}}
	stub.serve(t)
	out, err := runTechmemeCmd(t, "collect", "--source", "day", "--date", "261120")
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	rows := decodeRows(t, out)
	if len(rows) != 6 {
		t.Fatalf("want 6 dated-page rows, got %d", len(rows))
	}
	for _, r := range rows {
		assertPublisherURL(t, r)
		if r["published_at"] != nil {
			t.Fatalf("a dated page has no times; published_at should be null: %v", r)
		}
		id := r["id"].(string)
		want := "2026-11-20"
		if strings.HasPrefix(id, "261119") {
			// Carried over from the day before; its own id says so.
			want = "2026-11-19"
		}
		if r["date"] != want {
			t.Fatalf("row %s: date %v, want %s", id, r["date"], want)
		}
		if strings.Contains(r["url"].(string), "reddit.com") {
			t.Fatalf("row %s took a discussion link instead of the headline: %v", id, r["url"])
		}
	}
	if rows[0]["id"] != "261120p17" || rows[0]["publication"] != "Scientific American" {
		t.Fatalf("unexpected first dated row: %v", rows[0])
	}
}

func TestCollectNeverRequestsSearchAndRejectsBadFlags(t *testing.T) {
	stub := &techmemeStub{pages: map[string][]byte{}}
	stub.serve(t)
	for _, args := range [][]string{
		{"collect"},
		{"collect", "--source", "search"},
		{"collect", "--source", "day"},
		{"collect", "--source", "day", "--date", "2026-11-20"},
		{"collect", "--source", "day", "--date", "261340"},
	} {
		_, err := runTechmemeCmd(t, args...)
		if code := ExitCode(err); err == nil || code != 2 {
			t.Errorf("%v: want usage exit 2, got %d (%v)", args, code, err)
		}
	}
	if _, _, _, err := collectFetch(t.Context(), collectBaseURLOverride, "/search/query?q=x", rssValidators{}, 0); err == nil {
		t.Fatal("collectFetch must refuse /search/")
	}
	if n := len(stub.requests()); n != 0 {
		t.Fatalf("bad flags and /search/ must send no request: %d sent", n)
	}
}

func TestPublisherURLRejectsTechmemeLinks(t *testing.T) {
	for in, want := range map[string]string{
		"https://www.techmeme.com/goto/ft.com/abc":       "",
		"https://techmeme.com/r/abc":                     "",
		"https://www.techmeme.com/261121/p9#a261121p9":   "",
		"/goto/www.ft.com/x":                             "",
		"https://www.ft.com/content/x?a=1&amp;b=2":       "https://www.ft.com/content/x?a=1&b=2",
		"https://www.reddit.com/r/mathematics/comments/": "https://www.reddit.com/r/mathematics/comments/",
	} {
		if got := publisherURL(in); got != want {
			t.Errorf("publisherURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCollect429ExitsWithRateLimitCode(t *testing.T) {
	prev := collectBaseURLOverride
	t.Cleanup(func() { collectBaseURLOverride = prev })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "90")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)
	collectBaseURLOverride = srv.URL
	_, err := runTechmemeCmd(t, "collect", "--source", "river")
	if code := ExitCode(err); err == nil || code != 7 {
		t.Fatalf("a 429 must exit with the rate-limit code 7, got %d (%v)", code, err)
	}
	if !strings.Contains(err.Error(), "Retry-After 90") {
		t.Fatalf("error should carry Retry-After: %v", err)
	}
}

func TestCollectUsesConfiguredBaseURL(t *testing.T) {
	stub := &techmemeStub{pages: map[string][]byte{"/river": readFixture(t, "collect_river.html")}}
	stub.serve(t)
	mirror := collectBaseURLOverride
	collectBaseURLOverride = ""
	t.Setenv("TECHMEME_BASE_URL", mirror)
	t.Setenv("HOME", t.TempDir())
	out, err := runTechmemeCmd(t, "collect", "--source", "river")
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if n := len(stub.requests()); n != 1 || len(decodeRows(t, out)) != 12 {
		t.Fatalf("collect should read TECHMEME_BASE_URL: %d requests", n)
	}
}

func TestCollectFailedDeliveryKeepsOldValidators(t *testing.T) {
	stub := &techmemeStub{pages: map[string][]byte{"/feed.xml": readFixture(t, "collect_feed.xml")}}
	stub.serve(t)
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(hook.Close)
	state := filepath.Join(t.TempDir(), "rss.json")
	if _, err := runTechmemeCmd(t, "collect", "--source", "rss", "--state", state, "--deliver", "webhook:"+hook.URL); err == nil {
		t.Fatal("a failed delivery must fail the command")
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatalf("validators must not be saved when delivery fails: %v", err)
	}
	out, err := runTechmemeCmd(t, "collect", "--source", "rss", "--state", state)
	if err != nil || len(decodeRows(t, out)) != 6 {
		t.Fatalf("the retry must fetch the feed again, not get a 304: %v", err)
	}
}

func TestCollectWebhookDeliveryIsNDJSON(t *testing.T) {
	stub := &techmemeStub{pages: map[string][]byte{"/river": readFixture(t, "collect_river.html")}}
	stub.serve(t)
	var mu sync.Mutex
	var contentType string
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		contentType = r.Header.Get("Content-Type")
		mu.Unlock()
	}))
	t.Cleanup(hook.Close)
	if _, err := runTechmemeCmd(t, "collect", "--source", "river", "--deliver", "webhook:"+hook.URL); err != nil {
		t.Fatalf("collect: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if contentType != "application/x-ndjson" {
		t.Fatalf("want application/x-ndjson, got %q", contentType)
	}
}

func TestRiverTimesFollowEasternDaylightTime(t *testing.T) {
	page := []byte(`<H2>July 1, 2026</H2>
<tr class="ritem"><td>1:00 PM &nbsp;&bull;</td><td><div class="rshr" pml="260701p1"></div><cite><A HREF="https://example.com/">Example</A>:</cite>&nbsp; <a href="https://example.com/a">Summer story</a></td></tr>`)
	rows := parseRiver(page)
	if len(rows) != 1 || rows[0].PublishedAt == nil || *rows[0].PublishedAt != "2026-07-01T17:00:00Z" {
		t.Fatalf("1:00 PM EDT is 17:00 UTC; got %+v", rows)
	}
}

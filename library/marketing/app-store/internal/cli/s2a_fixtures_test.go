// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Test harness for the slice-2a commands: a fake Apple that answers iTunes
// Lookup from fixture rows per storefront and serves the slice-1 product and
// developer page fixtures, a fixture-backed client swapped in through
// the shared appStoreClientHook seam, and a runner that executes the real Cobra tree. Reviewer
// data never appears here: the page fixtures carry no reviews and the
// synthetic Lookup rows are public catalogue fields only.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

const s2aTestdata = "../appstore/testdata"

// s2aFixedNow is the clock the harness pins (an event in the Candy Crush
// fixture runs 2026-10-12 to 2026-11-07).
var s2aFixedNow = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

type s2aRewrite struct{ target *url.URL }

func (rt s2aRewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("X-Test-Orig-Host", req.URL.Host)
	r.URL.Scheme = rt.target.Scheme
	r.URL.Host = rt.target.Host
	r.Host = rt.target.Host
	return http.DefaultTransport.RoundTrip(r)
}

// s2aFake is the fake Apple.
type s2aFake struct {
	t        *testing.T
	client   *http.Client
	cacheDir string

	mu       sync.Mutex
	requests []string
	// fail answers a request with a status instead of the fixture (0 = serve).
	fail func(host, path string, q url.Values) int
	// rows are the Lookup rows per storefront; artists the developer rows.
	rows    map[string][]map[string]any
	artists map[string]map[string]any
	// pages maps an app id to a page fixture file; pageBodies overrides it.
	pages      map[string]string
	pageBodies map[string][]byte
	// lookupCacheControl overrides the Lookup answers' Cache-Control
	// (default max-age=3600: a CDN copy 23 h into Apple's 24 h budget).
	lookupCacheControl string
}

func newS2AFake(t *testing.T) *s2aFake {
	t.Helper()
	testenv.Isolate(t)
	f := &s2aFake{
		t:          t,
		cacheDir:   t.TempDir(),
		rows:       map[string][]map[string]any{},
		artists:    map[string]map[string]any{},
		pageBodies: map[string][]byte{},
		pages: map[string]string{
			"324684580": "product_spotify_us.html",
			"553834731": "product_candycrush_us.html",
			"361309726": "product_pages_us.html",
			"850417475": "product_soda_us.html",
		},
	}
	f.loadRows()
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)
	f.client = &http.Client{Transport: s2aRewrite{target: target}}

	useFakeAppStore(t, fakeAppStore{Transport: f.client.Transport, CacheDir: f.cacheDir, Cached: true})
	prevNow := s2aNow
	s2aNow = func() time.Time { return s2aFixedNow }
	t.Cleanup(func() { s2aNow = prevNow })
	return f
}

func s2aReadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(s2aTestdata, name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func s2aFixtureRows(t *testing.T, name string) []map[string]any {
	t.Helper()
	var doc struct {
		Results []map[string]any `json:"results"`
	}
	dec := json.NewDecoder(bytes.NewReader(s2aReadFixture(t, name)))
	dec.UseNumber() // keep ids as exact digits (float64 would print 3.2468458e+08)
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return doc.Results
}

// s2aRow builds a synthetic Lookup row from public catalogue fields.
func s2aRow(id, name, bundle, artistID, artist, slug, cc, currency string, count int64, avg float64) map[string]any {
	return map[string]any{
		"wrapperType": "software", "kind": "software",
		"trackId": json.Number(id), "trackName": name, "bundleId": bundle,
		"artistId": json.Number(artistID), "artistName": artist, "sellerName": artist,
		"artistViewUrl": fmt.Sprintf("https://apps.apple.com/%s/developer/%s/id%s?uo=4", cc, strings.ToLower(strings.Fields(artist)[0]), artistID),
		"trackViewUrl":  fmt.Sprintf("https://apps.apple.com/%s/app/%s/id%s?uo=4", cc, slug, id),
		"price":         0, "formattedPrice": "Free", "currency": currency,
		"userRatingCount": count, "averageUserRating": avg,
		"userRatingCountForCurrentVersion": count / 10, "averageUserRatingForCurrentVersion": avg,
		"version": "1.0", "currentVersionReleaseDate": "2026-10-20T07:00:00Z", "releaseDate": "2012-11-14T08:00:00Z",
		"primaryGenreName": "Games", "primaryGenreId": 6014, "genreIds": []string{"6014", "7003"}, "genres": []string{"Games", "Casual"},
		"fileSizeBytes": "412345678", "minimumOsVersion": "15.0", "trackContentRating": "4+",
		"languageCodesISO2A": []string{"EN", "DE", "FR"}, "supportedDevices": []string{"iPhone15-iPhone15"},
		"screenshotUrls": []string{"https://example.invalid/shot1.jpg"}, "ipadScreenshotUrls": []string{}, "appletvScreenshotUrls": []string{},
		"description": "Synthetic description.",
	}
}

func s2aStorefrontCopy(row map[string]any, cc, currency string, count int64) map[string]any {
	out := make(map[string]any, len(row))
	for k, v := range row {
		out[k] = v
	}
	out["currency"] = currency
	out["userRatingCount"] = count
	if u, ok := row["trackViewUrl"].(string); ok {
		out["trackViewUrl"] = strings.Replace(u, "/us/", "/"+cc+"/", 1)
	}
	return out
}

func (f *s2aFake) loadRows() {
	t := f.t
	us := s2aFixtureRows(t, "lookup_batch_us.json") // 30 apps incl. Spotify 324684580, Instagram 389801252
	dev := s2aFixtureRows(t, "lookup_developer_spotify_us.json")
	f.artists["324684583"] = dev[0]
	have := map[string]bool{}
	for _, r := range us {
		have[fmt.Sprint(r["trackId"])] = true
	}
	for _, r := range dev[1:] {
		if !have[fmt.Sprint(r["trackId"])] {
			us = append(us, r)
		}
	}
	candy := s2aRow("553834731", "Candy Crush Saga", "com.midasplayer.apps.candycrushsaga", "526656015", "King", "candy-crush-saga", "us", "USD", 3456789, 4.71)
	soda := s2aRow("850417475", "Candy Crush Soda Saga", "com.king.candycrushsodasaga", "526656015", "King", "candy-crush-soda-saga", "us", "USD", 1234567, 4.64)
	pages := s2aRow("361309726", "Pages: Create Documents", "com.apple.Pages", "284417353", "Apple", "pages", "us", "USD", 456789, 4.41)
	us = append(us, candy, soda, pages)
	f.rows["us"] = us

	var spotify, instagram map[string]any
	for _, r := range us {
		switch fmt.Sprint(r["trackId"]) {
		case "324684580":
			spotify = r
		case "389801252":
			instagram = r
		}
	}
	f.rows["gb"] = []map[string]any{
		s2aStorefrontCopy(spotify, "gb", "GBP", 1876543),
		s2aStorefrontCopy(instagram, "gb", "GBP", 2345678),
		s2aStorefrontCopy(candy, "gb", "GBP", 765432),
		s2aStorefrontCopy(pages, "gb", "GBP", 54321),
	}
	// cn: Spotify, Instagram and Candy Crush are not sold there.
	f.rows["cn"] = []map[string]any{s2aStorefrontCopy(pages, "cn", "CNY", 12345)}
}

func (f *s2aFake) serve(w http.ResponseWriter, r *http.Request) {
	host := r.Header.Get("X-Test-Orig-Host")
	q := r.URL.Query()
	f.mu.Lock()
	f.requests = append(f.requests, host+" "+r.URL.Path+"?"+r.URL.RawQuery)
	fail := f.fail
	f.mu.Unlock()
	if fail != nil {
		if status := fail(host, r.URL.Path, q); status != 0 {
			w.WriteHeader(status)
			_, _ = fmt.Fprintf(w, `{"errorMessage":"synthetic %d"}`, status)
			return
		}
	}
	switch {
	case host == "itunes.apple.com" && r.URL.Path == "/lookup":
		f.serveLookup(w, q)
	case host == "apps.apple.com":
		f.servePage(w, r.URL.Path)
	default:
		http.NotFound(w, r)
	}
}

func (f *s2aFake) serveLookup(w http.ResponseWriter, q url.Values) {
	cc := q.Get("country")
	pool := f.rows[cc]
	results := make([]map[string]any, 0)
	if b := q.Get("bundleId"); b != "" {
		for _, r := range pool {
			if r["bundleId"] == b {
				results = append(results, r)
			}
		}
	} else {
		for _, id := range strings.Split(q.Get("id"), ",") {
			if artist, ok := f.artists[id]; ok && cc == "us" {
				results = append(results, artist)
				for _, r := range pool {
					if fmt.Sprint(r["artistId"]) == id && q.Get("entity") != "macSoftware" {
						results = append(results, r)
					}
				}
				continue
			}
			for _, r := range pool {
				if fmt.Sprint(r["trackId"]) == id {
					results = append(results, r)
				}
			}
		}
	}
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	cacheControl := "max-age=3600"
	f.mu.Lock()
	if f.lookupCacheControl != "" {
		cacheControl = f.lookupCacheControl
	}
	f.mu.Unlock()
	w.Header().Set("Cache-Control", cacheControl)
	_ = json.NewEncoder(w).Encode(map[string]any{"resultCount": len(results), "results": results})
}

var (
	s2aAppPathRe = regexp.MustCompile(`^/([a-z]{2})/app/(?:[^/]+/)?id(\d+)$`)
	s2aDevPathRe = regexp.MustCompile(`^/([a-z]{2})/developer/(?:[^/]+/)?id(\d+)$`)
)

func (f *s2aFake) servePage(w http.ResponseWriter, path string) {
	var body []byte
	if m := s2aAppPathRe.FindStringSubmatch(path); m != nil {
		f.mu.Lock()
		override := f.pageBodies[m[2]]
		f.mu.Unlock()
		switch {
		case override != nil:
			body = override
		case f.pages[m[2]] != "":
			body = s2aReadFixture(f.t, f.pages[m[2]])
		}
	} else if m := s2aDevPathRe.FindStringSubmatch(path); m != nil && m[2] == "324684583" {
		body = s2aReadFixture(f.t, "developer_spotify_us.html")
	}
	if body == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(body)
}

// count is the number of requests the fake answered.
func (f *s2aFake) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func (f *s2aFake) requestLog() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requests...)
}

// setPage overrides one app's page body (for absence tests).
func (f *s2aFake) setPage(appID string, body []byte) {
	f.mu.Lock()
	f.pageBodies[appID] = body
	f.mu.Unlock()
}

type s2aResult struct {
	stdout, stderr string
	err            error
	code           int
	requests       int
}

// run executes the CLI with args against the fake and reports the network
// requests the run sent (cache hits are not requests).
func (f *s2aFake) run(args ...string) s2aResult {
	f.t.Helper()
	before := f.count()
	root := RootCmd()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(args)
	err := root.Execute()
	code := 0
	if err != nil {
		code = ExitCode(err)
	}
	return s2aResult{stdout: out.String(), stderr: errb.String(), err: err, code: code, requests: f.count() - before}
}

// mustOK runs args and fails the test unless the command exited 0.
func (f *s2aFake) mustOK(args ...string) s2aResult {
	f.t.Helper()
	res := f.run(args...)
	if res.code != 0 {
		f.t.Fatalf("%v: exit %d (%v)\nstdout:\n%s\nstderr:\n%s", args, res.code, res.err, res.stdout, res.stderr)
	}
	return res
}

// openStore opens the store the commands wrote (defaultDBPath under the
// isolated home).
func (f *s2aFake) openStore() *store.Store {
	f.t.Helper()
	st, err := store.OpenWithContext(context.Background(), defaultDBPath(appStoreCLIName))
	if err != nil {
		f.t.Fatalf("open store: %v", err)
	}
	f.t.Cleanup(func() { _ = st.Close() })
	return st
}

func s2aJSONObject(t *testing.T, s string) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("stdout is not a JSON object: %v\n%s", err, s)
	}
	return v
}

func s2aJSONArray(t *testing.T, s string) []map[string]any {
	t.Helper()
	var v []map[string]any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("stdout is not a JSON array of objects: %v\n%s", err, s)
	}
	return v
}

// s2aItems decodes a fan-out envelope and returns items, meta and failures.
func s2aItems(t *testing.T, s string) ([]map[string]any, map[string]any, []map[string]any) {
	t.Helper()
	var env struct {
		Meta          map[string]any   `json:"meta"`
		Items         []map[string]any `json:"items"`
		FetchFailures []map[string]any `json:"fetch_failures"`
	}
	if err := json.Unmarshal([]byte(s), &env); err != nil {
		t.Fatalf("stdout is not a {meta, items, fetch_failures} envelope: %v\n%s", err, s)
	}
	raw := map[string]json.RawMessage{}
	_ = json.Unmarshal([]byte(s), &raw)
	for _, k := range []string{"meta", "items", "fetch_failures"} {
		if string(raw[k]) == "" || string(raw[k]) == "null" {
			t.Fatalf("envelope key %q missing or null:\n%s", k, s)
		}
	}
	return env.Items, env.Meta, env.FetchFailures
}

// s2aMutatePage rewrites a page fixture's shelfMapping (absence tests).
func s2aMutatePage(t *testing.T, fixture string, mutate func(sm map[string]any)) []byte {
	t.Helper()
	raw := s2aReadFixture(t, fixture)
	re := regexp.MustCompile(`(?s)(<script type="application/json" id="serialized-server-data">)(.*?)(</script>)`)
	m := re.FindSubmatchIndex(raw)
	if m == nil {
		t.Fatalf("%s has no serialized-server-data", fixture)
	}
	var doc any
	if err := json.Unmarshal(raw[m[4]:m[5]], &doc); err != nil {
		t.Fatalf("decode %s model: %v", fixture, err)
	}
	var entries []any
	switch v := doc.(type) {
	case map[string]any:
		entries, _ = v["data"].([]any)
	case []any:
		entries = v
	}
	if len(entries) == 0 {
		t.Fatalf("%s model has no entries", fixture)
	}
	data, _ := entries[0].(map[string]any)["data"].(map[string]any)
	sm, _ := data["shelfMapping"].(map[string]any)
	if sm == nil {
		t.Fatalf("%s has no shelfMapping", fixture)
	}
	mutate(sm)
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	body := append([]byte{}, raw[:m[4]]...)
	body = append(body, out...)
	return append(body, raw[m[5]:]...)
}

func s2aNum(t *testing.T, v any) float64 {
	t.Helper()
	n, ok := v.(float64)
	if !ok {
		t.Fatalf("value %v (%T) is not a JSON number", v, v)
	}
	return n
}

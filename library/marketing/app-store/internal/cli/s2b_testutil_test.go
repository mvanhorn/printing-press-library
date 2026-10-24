// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Offline harness for the slice-2b command tests: a fake Apple transport
// that serves the sanitized fixtures in internal/appstore/testdata (and
// synthetic Lookup and review-feed bodies), plus a runner that executes the
// real root command against an isolated store. Review text in synthetic
// feeds is placeholder text ("reviewer-N", "Sample title N").

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

// fakeResponse is one canned answer.
type fakeResponse struct {
	Status int
	Body   []byte
	Header http.Header
}

// fakeApple is an http.RoundTripper that records every request and answers
// from a handler.
type fakeApple struct {
	mu       sync.Mutex
	requests []*http.Request
	handler  func(r *http.Request) fakeResponse
}

func (f *fakeApple) RoundTrip(r *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.requests = append(f.requests, r)
	f.mu.Unlock()
	resp := f.handler(r)
	if resp.Status == 0 {
		resp.Status = http.StatusOK
	}
	h := resp.Header
	if h == nil {
		h = http.Header{}
	}
	return &http.Response{
		StatusCode: resp.Status,
		Header:     h,
		Body:       io.NopCloser(bytes.NewReader(resp.Body)),
		Request:    r,
	}, nil
}

// urls returns every requested URL in order.
func (f *fakeApple) urls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.requests))
	for _, r := range f.requests {
		out = append(out, r.URL.String())
	}
	return out
}

// count returns how many requests contained substr in their URL.
func (f *fakeApple) count(substr string) int {
	n := 0
	for _, u := range f.urls() {
		if strings.Contains(u, substr) {
			n++
		}
	}
	return n
}

// s2bEnv is one isolated test environment.
type s2bEnv struct {
	t      *testing.T
	dir    string
	db     string
	home   string
	fake   *fakeApple
	cached bool
}

// newS2bEnv installs a fake-transport client for the slice-2b commands.
// With cached, responses go through a real file cache in the temp dir so
// rerun-from-cache paths can be tested.
func newS2bEnv(t *testing.T, cached bool, handler func(r *http.Request) fakeResponse) *s2bEnv {
	t.Helper()
	dir := t.TempDir()
	env := &s2bEnv{t: t, dir: dir, db: filepath.Join(dir, "store.db"), home: filepath.Join(dir, "home"), fake: &fakeApple{handler: handler}, cached: cached}
	if err := os.MkdirAll(env.home, 0o700); err != nil {
		t.Fatal(err)
	}
	// The shared seam: every client the commands build talks to env.fake.
	// The --home override each run applies is scoped to that run (the root
	// command restores it), so no cleanup of it is needed here.
	useFakeAppStore(t, fakeAppStore{Transport: env.fake, CacheDir: filepath.Join(dir, "cache"), Cached: cached})
	return env
}

// run executes the CLI with the env's store and home appended.
func (e *s2bEnv) run(args ...string) (string, string, error) {
	e.t.Helper()
	root := RootCmd()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(append(append([]string{}, args...), "--db", e.db, "--home", e.home))
	err := root.Execute()
	return out.String(), errb.String(), err
}

// store opens the env's store for assertions.
func (e *s2bEnv) store() *store.Store {
	e.t.Helper()
	st, err := store.OpenWithContext(context.Background(), e.db)
	if err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() { _ = st.Close() })
	return st
}

// fixtureBytes reads a sanitized fixture from internal/appstore/testdata.
func fixtureBytes(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "appstore", "testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func okBody(b []byte) fakeResponse { return fakeResponse{Status: http.StatusOK, Body: b} }

func throttled() fakeResponse {
	return fakeResponse{Status: http.StatusTooManyRequests, Body: []byte("slow down")}
}

func notFoundResp() fakeResponse { return fakeResponse{Status: http.StatusNotFound} }

// synthLookup answers a Lookup request with one synthetic software result
// per requested id (public-catalogue-shaped data, not real apps). mutate may
// adjust each result or return false to leave the id out (a missing id).
func synthLookup(r *http.Request, mutate func(id string, m map[string]any) bool) fakeResponse {
	q := r.URL.Query()
	results := make([]map[string]any, 0)
	ids := strings.Split(q.Get("id"), ",")
	if q.Get("id") == "" {
		ids = nil
	}
	for i, id := range ids {
		n, _ := strconv.ParseInt(id, 10, 64)
		m := map[string]any{
			"wrapperType":       "software",
			"kind":              "software",
			"trackId":           n,
			"trackName":         "Synthetic App " + id,
			"bundleId":          "com.example.app" + id,
			"artistId":          int64(1000 + i),
			"artistName":        "Synthetic Developer " + id,
			"trackViewUrl":      fmt.Sprintf("https://apps.apple.com/%s/app/synthetic/id%s?uo=4", q.Get("country"), id),
			"price":             0.0,
			"formattedPrice":    "Free",
			"currency":          "USD",
			"averageUserRating": 4.5,
			"userRatingCount":   int64(1000 + i),
			"version":           "1.0",
			"primaryGenreId":    6014,
			"primaryGenreName":  "Games",
			"genreIds":          []string{"6014"},
			"genres":            []string{"Games"},
		}
		if mutate != nil && !mutate(id, m) {
			continue
		}
		results = append(results, m)
	}
	b, _ := json.Marshal(map[string]any{"resultCount": len(results), "results": results})
	return okBody(b)
}

// synthReviewFeed builds a customer-reviews RSS page with synthetic reviews
// for the given ids, newest first, one minute apart from base.
func synthReviewFeed(ids []string, base time.Time) []byte {
	entries := make([]map[string]any, 0, len(ids))
	for i, id := range ids {
		at := base.Add(-time.Duration(i) * time.Minute)
		entries = append(entries, map[string]any{
			"id":           map[string]any{"label": id},
			"im:rating":    map[string]any{"label": strconv.Itoa(i%5 + 1)},
			"im:version":   map[string]any{"label": "1.2.3"},
			"title":        map[string]any{"label": "Sample title " + id},
			"content":      map[string]any{"label": "Sample body " + id},
			"author":       map[string]any{"name": map[string]any{"label": "reviewer-" + id}},
			"updated":      map[string]any{"label": at.Format(time.RFC3339)},
			"im:voteSum":   map[string]any{"label": "1"},
			"im:voteCount": map[string]any{"label": "2"},
		})
	}
	feed := map[string]any{"updated": map[string]any{"label": base.Format(time.RFC3339)}}
	if len(entries) > 0 {
		feed["entry"] = entries
	}
	b, _ := json.Marshal(map[string]any{"feed": feed})
	return b
}

// idRange returns ids prefix+from .. prefix+to.
func idRange(prefix string, from, to int) []string {
	out := make([]string, 0, to-from+1)
	for i := from; i <= to; i++ {
		out = append(out, fmt.Sprintf("%s%d", prefix, i))
	}
	return out
}

// pageModelHTML wraps a page model in the serialized-server-data script the
// client extracts.
func pageModelHTML(intent, data map[string]any) []byte {
	b, _ := json.Marshal(map[string]any{"data": []any{map[string]any{"intent": intent, "data": data}}})
	return []byte(`<html><body><script type="application/json" id="serialized-server-data">` + string(b) + `</script></body></html>`)
}

// decodeJSON unmarshals command output or fails the test.
func decodeJSON(t *testing.T, out string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(out), v); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
}

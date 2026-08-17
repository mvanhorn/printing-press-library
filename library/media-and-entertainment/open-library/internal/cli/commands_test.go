// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestBookSearchQueriesOpenLibrarySearch(t *testing.T) {
	var sawSearch bool
	withTestTransport(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search.json" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		sawSearch = true
		if got := r.URL.Query().Get("q"); got != "Designing Data-Intensive Applications" {
			t.Fatalf("q = %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "2" {
			t.Fatalf("limit = %q", got)
		}
		if ua := r.Header.Get("User-Agent"); !strings.Contains(ua, "open-library-tests") || !strings.Contains(ua, "books@example.test") {
			t.Fatalf("User-Agent = %q", ua)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"numFound": 1,
			"docs": [{
				"key": "/works/OL262758W",
				"title": "Designing Data-Intensive Applications",
				"author_name": ["Martin Kleppmann"],
				"author_key": ["OL7443502A"],
				"first_publish_year": 2017,
				"edition_count": 8,
				"isbn": ["1449373321"],
				"language": ["eng"]
			}]
		}`))
	}))

	t.Setenv(baseURLEnv, "https://openlibrary.test")
	t.Setenv(userAgentEnv, "open-library-tests")
	t.Setenv(contactEmailEnv, "books@example.test")

	output := runCLI(t, "book", "Designing Data-Intensive Applications", "--limit", "2", "--agent")

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("unmarshal book output: %v\n%s", err, output)
	}
	if !sawSearch {
		t.Fatalf("search endpoint was not called")
	}
	if result["source"] != "Open Library Search API" {
		t.Fatalf("source = %#v", result["source"])
	}
	results := result["results"].([]any)
	first := results[0].(map[string]any)
	if first["title"] != "Designing Data-Intensive Applications" {
		t.Fatalf("title = %#v", first["title"])
	}
	if !strings.Contains(first["source_url"].(string), "/works/OL262758W") {
		t.Fatalf("source_url = %#v", first["source_url"])
	}
}

func TestAuthorSearchFetchesBoundedWorks(t *testing.T) {
	var sawAuthorSearch, sawWorks bool
	withTestTransport(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/search/authors.json":
			sawAuthorSearch = true
			if got := r.URL.Query().Get("q"); got != "Ursula K. Le Guin" {
				t.Fatalf("author q = %q", got)
			}
			_, _ = w.Write([]byte(`{"numFound":1,"docs":[{"key":"OL31353A","name":"Ursula K. Le Guin","top_work":"A Wizard of Earthsea","work_count":321}]}`))
		case "/authors/OL31353A/works.json":
			sawWorks = true
			if got := r.URL.Query().Get("limit"); got != "3" {
				t.Fatalf("works limit = %q", got)
			}
			_, _ = w.Write([]byte(`{"size":1,"entries":[{"key":"/works/OL64117W","title":"A Wizard of Earthsea","first_publish_date":"1968"}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))

	t.Setenv(baseURLEnv, "https://openlibrary.test")

	output := runCLI(t, "author", "Ursula K. Le Guin", "--limit", "3", "--agent")

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("unmarshal author output: %v\n%s", err, output)
	}
	if !sawAuthorSearch || !sawWorks {
		t.Fatalf("expected author search and works calls, got search=%t works=%t", sawAuthorSearch, sawWorks)
	}
	author := result["author"].(map[string]any)
	if author["name"] != "Ursula K. Le Guin" {
		t.Fatalf("author = %#v", author)
	}
	works := result["works"].([]any)
	if works[0].(map[string]any)["title"] != "A Wizard of Earthsea" {
		t.Fatalf("works = %#v", works)
	}
}

func TestSubjectsSlugAndDetails(t *testing.T) {
	var sawSubject bool
	withTestTransport(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/subjects/distributed_systems.json" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		sawSubject = true
		if got := r.URL.Query().Get("details"); got != "true" {
			t.Fatalf("details = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"key":"/subjects/distributed_systems","name":"distributed systems","work_count":1,"works":[{"key":"/works/OL1W","title":"Distributed Systems","edition_count":2,"authors":[{"name":"Example Author","key":"/authors/OL1A"}]}],"subjects":[{"name":"Computer networks","key":"/subjects/computer_networks","count":4}]}`))
	}))

	t.Setenv(baseURLEnv, "https://openlibrary.test")

	output := runCLI(t, "subjects", "distributed systems", "--details", "--agent")

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("unmarshal subjects output: %v\n%s", err, output)
	}
	if !sawSubject {
		t.Fatalf("subject endpoint was not called")
	}
	if result["source"] != "Open Library Subjects API" {
		t.Fatalf("source = %#v", result["source"])
	}
	if !strings.Contains(result["source_url"].(string), "/subjects/distributed_systems") {
		t.Fatalf("source_url = %#v", result["source_url"])
	}
}

func TestSubjectsOmitFacetsWithoutDetails(t *testing.T) {
	withTestTransport(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/subjects/distributed_systems.json" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("details"); got != "" {
			t.Fatalf("details = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"key":"/subjects/distributed_systems","name":"distributed systems","work_count":1,"works":[]}`))
	}))

	t.Setenv(baseURLEnv, "https://openlibrary.test")

	output := runCLI(t, "subjects", "distributed systems", "--agent")

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("unmarshal subjects output: %v\n%s", err, output)
	}
	if _, ok := result["facets"]; ok {
		t.Fatalf("facets should be omitted without --details: %s", output)
	}
}

func TestSourcesReportsNoAuthAndRatePosture(t *testing.T) {
	t.Setenv(userAgentEnv, "")
	t.Setenv(contactEmailEnv, "")

	output := runCLI(t, "sources", "--agent")

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("unmarshal sources output: %v\n%s", err, output)
	}
	if result["auth"] != "none" {
		t.Fatalf("auth = %#v", result["auth"])
	}
	caveats := result["caveats"].([]any)
	if !containsString(caveats, "bulk") {
		t.Fatalf("caveats did not mention bulk guidance: %#v", caveats)
	}
}

func TestShortenPreservesUTF8(t *testing.T) {
	got := shorten("éclair", 1)
	if got != "é..." {
		t.Fatalf("shorten = %q", got)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("shorten returned invalid UTF-8: %q", got)
	}
}

func runCLI(t *testing.T, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	flags := rootFlags{timeout: time.Second}
	cmd := newRootCmd(&flags)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute %v: %v\nstderr: %s", args, err, stderr.String())
	}
	return stdout.String()
}

func containsString(items []any, needle string) bool {
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.(string)), strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func withTestTransport(t *testing.T, handler http.Handler) {
	t.Helper()
	previous := newHTTPClient
	newHTTPClient = func(timeout time.Duration) *http.Client {
		return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			resp := recorder.Result()
			if resp.Body == nil {
				resp.Body = io.NopCloser(strings.NewReader(""))
			}
			return resp, nil
		})}
	}
	t.Cleanup(func() {
		newHTTPClient = previous
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/seohead"
	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/store"
)

// TestNovelAuditHelpWires smoke-tests that the audit command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelAuditHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"audit", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("audit --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "audit"} {
		if !strings.Contains(help, want) {
			t.Fatalf("audit --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestAnalyzeSEO(t *testing.T) {
	checks := func(names ...string) map[string]bool {
		out := make(map[string]bool, len(names))
		for _, name := range names {
			out[name] = true
		}
		return out
	}
	tests := []struct {
		name   string
		items  []auditItem
		heads  map[string]seohead.Head
		prev   map[string]seohead.Head
		checks map[string]bool
		want   map[string]int
	}{
		{name: "empty", items: []auditItem{}, heads: map[string]seohead.Head{}, prev: map[string]seohead.Head{}, checks: checks(auditChecks...), want: map[string]int{}},
		{name: "duplicate title across three URLs", items: []auditItem{{URL: "https://example.com/a"}, {URL: "https://example.com/b"}, {URL: "https://example.com/c"}}, heads: map[string]seohead.Head{"https://example.com/a": {Title: " Same title "}, "https://example.com/b": {Title: "same   title"}, "https://example.com/c": {Title: "SAME TITLE"}}, prev: map[string]seohead.Head{}, checks: checks("duplicate_title"), want: map[string]int{"duplicate_title": 3}},
		{name: "missing description and noindex", items: []auditItem{{URL: "https://example.com/a"}}, heads: map[string]seohead.Head{"https://example.com/a": {Title: "Title", Noindex: true}}, prev: map[string]seohead.Head{}, checks: checks("missing_description", "noindex"), want: map[string]int{"missing_description": 1, "noindex": 1}},
		{name: "canonical mismatch but trailing slash equal", items: []auditItem{{URL: "https://example.com/a"}, {URL: "https://example.com/b"}}, heads: map[string]seohead.Head{"https://example.com/a": {URL: "https://example.com/a", FinalURL: "https://example.com/a/", Canonical: "https://example.com/a"}, "https://example.com/b": {URL: "https://example.com/b", FinalURL: "https://example.com/b", Canonical: "https://other.example/b"}}, prev: map[string]seohead.Head{}, checks: checks("canonical_mismatch"), want: map[string]int{"canonical_mismatch": 1}},
		{name: "changed title", items: []auditItem{{URL: "https://example.com/a"}}, heads: map[string]seohead.Head{"https://example.com/a": {Title: "New", Robots: "index", Canonical: "https://example.com/a"}}, prev: map[string]seohead.Head{"https://example.com/a": {Title: "Old", Robots: "index", Canonical: "https://example.com/a"}}, checks: checks("changed"), want: map[string]int{"changed": 1}},
		{name: "404 skips missing checks", items: []auditItem{{URL: "https://example.com/a"}}, heads: map[string]seohead.Head{"https://example.com/a": {Status: 404}}, prev: map[string]seohead.Head{}, checks: checks("http_error", "missing_title", "missing_description"), want: map[string]int{"http_error": 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analyzeSEO(tt.items, tt.heads, tt.prev, tt.checks)
			counts := summarizeAudit(got)
			if len(counts) != len(tt.want) {
				t.Fatalf("counts=%v want=%v", counts, tt.want)
			}
			for k, v := range tt.want {
				if counts[k] != v {
					t.Errorf("%s count=%d want=%d", k, counts[k], v)
				}
			}
			if tt.name == "duplicate title across three URLs" {
				for _, issue := range got {
					if len(issue.DuplicateOf) != 2 {
						t.Errorf("duplicate_of for %s = %v", issue.URL, issue.DuplicateOf)
					}
				}
			}
			if tt.name == "empty" && got == nil {
				t.Fatal("empty analysis must return an initialized empty slice")
			}
		})
	}
}

func TestRenderAuditHumanReportsUncheckedURLs(t *testing.T) {
	var out bytes.Buffer
	err := renderAuditHuman(&out, auditResult{SiteItems: 312, ScannedURLs: 200, Fetched: 200, UncheckedURLs: 112, Issues: []auditIssue{}})
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"Checked 200 of 312 URLs", "112 unchecked", "raise --max-fetch", "No on-page SEO issues found among checked URLs"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "No on-page SEO issues found.") {
		t.Fatalf("output contains unqualified no-issues claim:\n%s", got)
	}
}

func TestAuditFetchFailuresExcludeNon404Responses(t *testing.T) {
	var listener net.Listener
	func() {
		defer func() {
			if recover() != nil {
				t.Skip("local listener unavailable in this sandbox")
			}
		}()
		var err error
		listener, err = net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Skipf("loopback listener unavailable in this environment: %v", err)
		}
	}()
	server := &httptest.Server{Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/forbidden":
			w.WriteHeader(http.StatusForbidden)
		case "/missing":
			w.WriteHeader(http.StatusNotFound)
		case "/gone":
			w.WriteHeader(http.StatusGone)
		case "/server-error":
			w.WriteHeader(http.StatusInternalServerError)
		}
	})}, Listener: listener}
	server.Start()
	defer server.Close()

	items := make([]auditItem, 0)
	for _, path := range []string{"/forbidden", "/missing", "/gone", "/server-error"} {
		items = append(items, auditItem{URL: server.URL + path})
	}
	heads, failures := fetchAuditItems(context.Background(), items, 2, 100)
	if len(failures) != 2 || len(heads) != 2 {
		t.Fatalf("heads=%v failures=%v", heads, failures)
	}
	if _, ok := heads[server.URL+"/forbidden"]; ok {
		t.Fatal("403 response was retained for analysis or snapshots")
	}
	if heads[server.URL+"/missing"].Status != http.StatusNotFound || heads[server.URL+"/gone"].Status != http.StatusGone {
		t.Fatalf("404/410 responses were not retained: %v", heads)
	}
}

func TestAuditCommandIntegration(t *testing.T) {
	testenv.Isolate(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/one":
			fmt.Fprint(w, `<html><head><title>Same title</title><meta name="description" content="description"></head></html>`)
		case "/two":
			fmt.Fprint(w, `<html><head><title> Same title </title><meta name="description" content="description"></head></html>`)
		case "/three":
			fmt.Fprint(w, `<html><head><title>Third title</title><meta name="description" content="description"><meta name="robots" content="noindex"></head></html>`)
		default:
			http.NotFound(w, r)
		}
	})
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("loopback listener unavailable in this environment: %v", err)
	}
	server := &httptest.Server{Config: &http.Server{Handler: handler}, Listener: listener}
	server.Start()
	defer server.Close()
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for i, path := range []string{"/one", "/two", "/three"} {
		id := fmt.Sprint(i + 1)
		row, _ := json.Marshal(map[string]any{"id": id, "link": server.URL + path, "status": "publish"})
		if err := db.Upsert("posts", id, row); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	cmd := RootCmd()
	cmd.SetArgs([]string{"audit", "--db", dbPath, "--json", "--rps=100"})
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v; stderr=%s", err, stderr.String())
	}
	var got auditResult
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("JSON output: %v: %s", err, out.String())
	}
	if got.Summary["duplicate_title"] != 2 {
		t.Errorf("duplicate_title count=%d; output=%s", got.Summary["duplicate_title"], out.String())
	}
	if got.Summary["noindex"] != 1 {
		t.Errorf("noindex count=%d; output=%s", got.Summary["noindex"], out.String())
	}
	if got.Summary["missing_title"] != 0 {
		t.Errorf("missing_title count=%d; output=%s", got.Summary["missing_title"], out.String())
	}
}

// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/cliutil/testenv"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceHTMLExportFormatsLimitsAndEmptyArray(t *testing.T) {
	for _, tc := range []struct {
		format, limit string
		empty         bool
		want          int
	}{{"jsonl", "2", false, 2}, {"json", "1", false, 1}, {"json", "2", true, 0}} {
		t.Run(tc.format+tc.limit+map[bool]string{false: "links", true: "empty"}[tc.empty], func(t *testing.T) {
			testenv.Isolate(t)
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.URL.Path != "/park/search.php" || r.Header.Get("X-Printing-Press-HTML-Response") != "" {
					t.Fatal("wrong provider contract")
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				page := `<!doctype html><html><title>Catalog</title><a href="/park/rvpark/1086.html">公園A</a><a href="/park/yypark/213.html">公園B</a><a href="/unrelated">Ignored</a></html>`
				if tc.empty {
					page = `<html><title>Catalog</title></html>`
				}
				_, _ = w.Write([]byte(page))
			}))
			defer srv.Close()
			t.Setenv("KURUMATABI_BASE_URL", srv.URL)
			cmd := RootCmd()
			var out, stderr bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"export", "source", "--format", tc.format, "--limit", tc.limit, "--no-cache", "--no-learn"})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			var links []htmlLink
			if tc.format == "json" {
				if err := json.Unmarshal(out.Bytes(), &links); err != nil {
					t.Fatal(err)
				}
				if links == nil {
					t.Fatal("empty export must be []")
				}
			} else {
				for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
					var link htmlLink
					if err := json.Unmarshal([]byte(line), &link); err != nil {
						t.Fatalf("JSONL object: %v stdout=%s", err, out.String())
					}
					links = append(links, link)
				}
			}
			if requests != 1 || len(links) != tc.want || strings.Contains(out.String(), "<html") {
				t.Fatalf("bounded link contract: requests=%d links=%+v stdout=%s", requests, links, out.String())
			}
			for _, link := range links {
				if !strings.HasPrefix(link.URL, srv.URL+"/park/") || link.Text == "" {
					t.Fatalf("link evidence lost %+v", link)
				}
			}
		})
	}
}

func TestSourceExportValidatesAndReadsBeforeOpeningDestination(t *testing.T) {
	testenv.Isolate(t)
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.WriteHeader(403) }))
	defer srv.Close()
	t.Setenv("KURUMATABI_BASE_URL", srv.URL)
	path := filepath.Join(t.TempDir(), "links.json")
	for _, args := range [][]string{{"export", "source", "--format", "xml"}, {"export", "source", "--limit", "0"}, {"export", "source", "--limit", "201"}, {"export", "source", "extra-id"}} {
		cmd := RootCmd()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(append(args, "--output", path, "--no-learn"))
		if err := cmd.Execute(); err == nil {
			t.Fatalf("invalid export accepted: %v", args)
		}
	}
	if requests != 0 {
		t.Fatal("invalid export fetched provider")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("invalid export created destination")
	}
	if err := os.WriteFile(path, []byte("preserved"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"export", "source", "--output", path, "--no-cache", "--no-learn"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("source failure accepted")
	}
	if requests != 1 {
		t.Fatalf("failure test did not perform one source read: %d", requests)
	}
	body, err := os.ReadFile(path)
	if err != nil || string(body) != "preserved" {
		t.Fatal("failed read truncated destination")
	}
	cmd = RootCmd()
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"export", "source", "--output", path, "--dry-run", "--json", "--no-learn"})
	if err := cmd.Execute(); err != nil || !json.Valid(out.Bytes()) {
		t.Fatalf("dry run err=%v output=%s", err, out.Bytes())
	}
	body, err = os.ReadFile(path)
	if err != nil || string(body) != "preserved" {
		t.Fatal("dry run truncated destination")
	}
}

// Copyright 2026 Vikas and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchLiveParsesShodhgangaHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/simple-search" {
			t.Fatalf("path = %q, want /simple-search", r.URL.Path)
		}
		if got := r.URL.Query().Get("query"); got != "black hole" {
			t.Fatalf("query = %q, want black hole", got)
		}
		_, _ = w.Write([]byte(`<span>Results 1-1 of 1</span><a href="/handle/10603/305247">Black Hole Physics</a>`))
	}))
	defer server.Close()
	t.Setenv("SHODHGANGA_BASE_URL", server.URL)

	cmd := newRootCmd(&rootFlags{})
	cmd.SetArgs([]string{"search", "black hole", "--data-source", "live", "--json", "--no-learn"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("search error = %v", err)
	}
	for _, want := range []string{`"source": "live"`, `"id": "305247"`, `"title": "Black Hole Physics"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %s: %s", want, out.String())
		}
	}
}

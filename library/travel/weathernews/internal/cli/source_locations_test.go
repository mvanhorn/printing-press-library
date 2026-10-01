// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSourceLocationsReadOnlyGetReachesServer(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/onebox/api_search.cgi" {
			http.NotFound(w, r)
			return
		}
		got = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"loc":"京都"}]`))
	}))
	t.Cleanup(srv.Close)

	t.Setenv("WEATHERNEWS_BASE_URL", srv.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "")

	root := RootCmd()
	loc, _, err := root.Find([]string{"source", "locations"})
	if err != nil {
		t.Fatalf("find source locations: %v", err)
	}
	if loc.Annotations["mcp:read-only"] != "true" {
		t.Fatalf("source locations mcp:read-only = %q", loc.Annotations["mcp:read-only"])
	}

	var stdout, stderr strings.Builder
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--json", "source", "locations", "--query", "京都", "--lang", "ja", "--callback", "cb"})
	if err := root.Execute(); err != nil {
		t.Fatalf("source locations: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "__pp_verify_synthetic__") {
		t.Fatalf("verify mode treated location search as a mutation:\n%s", stdout.String())
	}
	if got.Get("query") != "京都" || got.Get("lang") != "ja" || got.Get("callback") != "cb" {
		t.Fatalf("query params = %v", got)
	}
	if !strings.Contains(stdout.String(), "京都") {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

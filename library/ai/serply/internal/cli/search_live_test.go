// Copyright 2026 googio and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/serply/internal/cliutil/testenv"
)

func TestLiveSearchTypeBingUsesBingPath(t *testing.T) {
	testenv.Isolate(t)
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"results":[{"title":"bing hit","link":"https://example.test/b","description":"from bing"}]}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("SERPLY_BASE_URL", srv.URL)
	t.Setenv("SERPLY_API_KEY", "test-key")

	cmd := RootCmd()
	cmd.SetArgs([]string{"search", "serp api", "--type", "bing", "--data-source", "live", "--json", "--no-cache"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("search --type bing: %v\n%s", err, out.String())
	}
	if gotPath != "/v1/b/search" {
		t.Fatalf("live path = %q, want /v1/b/search", gotPath)
	}
	if gotQuery != "serp api" {
		t.Fatalf("q = %q", gotQuery)
	}
	if strings.Contains(gotPath, "/v1/news") || strings.Contains(out.String(), "/v1/news") {
		t.Fatalf("bing search touched news: path %q output %s", gotPath, out.String())
	}
}

// Copyright 2026 Hiten Shah and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-intel/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-intel/internal/config"
)

func TestFetchReleasesSinceUsesFullFetchPageBeforeLimit(t *testing.T) {
	var gotPerPage string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/widget/releases" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		gotPerPage = r.URL.Query().Get("per_page")
		releases := make([]releaseSummary, 0, 10)
		for i := 0; i < 10; i++ {
			releases = append(releases, releaseSummary{
				TagName:     "v1.0." + strconv.Itoa(i),
				PublishedAt: time.Now().Add(-time.Duration(i) * time.Hour).Format(time.RFC3339),
			})
		}
		if err := json.NewEncoder(w).Encode(releases); err != nil {
			t.Fatalf("encoding releases: %v", err)
		}
	}))
	defer server.Close()

	c := client.New(&config.Config{BaseURL: server.URL}, time.Second, 0)
	c.NoCache = true

	releases, err := fetchReleases(t.Context(), c, "acme", "widget", "30d", 5)
	if err != nil {
		t.Fatalf("fetchReleases returned error: %v", err)
	}
	if gotPerPage != "100" {
		t.Fatalf("per_page = %q, want 100", gotPerPage)
	}
	if len(releases) != 5 {
		t.Fatalf("len(releases) = %d, want 5", len(releases))
	}
}

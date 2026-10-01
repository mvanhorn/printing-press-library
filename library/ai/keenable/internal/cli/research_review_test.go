// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/keenable/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/keenable/internal/store"
)

func TestLoadPreviousResearchSnapshotUsesSaveOrderWithLegacyTimestamps(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "research.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	snapshots := []researchSnapshot{
		{ID: "z-oldest", CreatedAt: "now"},
		{ID: "m-middle", CreatedAt: "now"},
		{ID: "a-newest", CreatedAt: "now"},
	}
	for _, snap := range snapshots {
		if err := persistResearchSnapshot(s, snap, nil, nil); err != nil {
			t.Fatalf("persist %s: %v", snap.ID, err)
		}
	}

	latest, err := loadResearchSnapshot(s, "latest")
	if err != nil {
		t.Fatal(err)
	}
	if latest.ID != "a-newest" {
		t.Fatalf("latest ID = %q, want a-newest", latest.ID)
	}

	got, err := loadPreviousResearchSnapshot(s, "a-newest")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "m-middle" {
		t.Fatalf("previous ID = %q, want m-middle", got.ID)
	}
	if _, err := loadPreviousResearchSnapshot(s, "z-oldest"); err == nil || !strings.Contains(err.Error(), "no earlier snapshot") {
		t.Fatalf("oldest snapshot error = %v, want no-earlier-snapshot error", err)
	}
}

func TestSaveLiveSnapshotFailsWhenSelectedPageCannotBeFetched(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/search/public":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"results":[{"title":"Unavailable source","url":%q}]}`, server.URL+"/source")
		case r.Method == http.MethodGet && r.URL.Path == "/v1/fetch/public":
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	restoreHome, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer restoreHome()
	t.Setenv("KEENABLE_BASE_URL", server.URL)

	flags := &rootFlags{timeout: time.Second, noCache: true}
	_, _, _, err = saveLiveSnapshot(context.Background(), flags, researchSearchRequest{
		Query:      "failure test",
		MaxResults: 1,
	}, 1, 1_000, false, "")
	if err == nil {
		t.Fatal("saveLiveSnapshot returned nil error after selected page fetch failed")
	}
	if !strings.Contains(err.Error(), "fetching selected snapshot page") || !strings.Contains(err.Error(), server.URL+"/source") {
		t.Fatalf("error = %q, want selected page URL and fetch context", err)
	}
}

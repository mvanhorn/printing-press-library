// Copyright 2026 Hunter Veltri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/job-boards/lever/internal/client"
	"github.com/mvanhorn/printing-press-library/library/job-boards/lever/internal/config"
)

func TestFetchOpenPostingsSnapshotCollectsEveryOffsetPage(t *testing.T) {
	var offsets []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/postings/acme" {
			t.Errorf("path = %q, want /postings/acme", r.URL.Path)
			http.Error(w, "unexpected path", http.StatusBadRequest)
			return
		}
		if got := r.URL.Query().Get("limit"); got != "100" {
			t.Errorf("limit = %q, want 100", got)
			http.Error(w, "unexpected limit", http.StatusBadRequest)
			return
		}
		if got := r.URL.Query().Get("mode"); got != "json" {
			t.Errorf("mode = %q, want json", got)
			http.Error(w, "unexpected mode", http.StatusBadRequest)
			return
		}
		offset := r.URL.Query().Get("offset")
		offsets = append(offsets, offset)

		count, start := 100, 0
		if offset == "100" {
			count, start = 1, 100
		}
		items := make([]map[string]any, count)
		for i := range items {
			items[i] = map[string]any{"id": fmt.Sprintf("open-%d", start+i)}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(items); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	c := client.New(&config.Config{BaseURL: server.URL}, 5*time.Second, 0)
	data, err := fetchOpenPostingsSnapshot(
		context.Background(), c, &rootFlags{}, "/postings/acme",
	)
	if err != nil {
		t.Fatalf("fetch snapshot: %v", err)
	}
	if len(data) != 101 {
		t.Fatalf("snapshot rows = %d, want 101", len(data))
	}
	if want := []string{"0", "100", "0", "100"}; !reflect.DeepEqual(offsets, want) {
		t.Fatalf("offsets = %v, want %v", offsets, want)
	}
}

func TestFetchOpenPostingsSnapshotRejectsShiftingOffsetPages(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		offset := 0
		if _, err := fmt.Sscan(r.URL.Query().Get("offset"), &offset); err != nil {
			t.Errorf("invalid offset: %v", err)
			http.Error(w, "bad offset", http.StatusBadRequest)
			return
		}
		// One job closes after page one. Offset 100 then skips open-100,
		// even though the API returns a valid short page.
		start := 0
		if requests > 1 {
			start = 1
		}
		items := make([]map[string]any, 0, 100)
		for i := start + offset; i <= 100 && len(items) < 100; i++ {
			items = append(items, map[string]any{"id": fmt.Sprintf("open-%d", i)})
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(items); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	c := client.New(&config.Config{BaseURL: server.URL}, 5*time.Second, 0)
	items, err := fetchOpenPostingsSnapshot(context.Background(), c, &rootFlags{}, "/postings/acme")
	if err == nil || items != nil {
		t.Fatalf("shifting pages returned %d items, err=%v; want no snapshot", len(items), err)
	}
	if requests != 4 {
		t.Fatalf("requests = %d, want two complete scans", requests)
	}
}

func TestFetchOpenPostingsSnapshotRejectsMalformedLaterPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("offset") == "100" {
			fmt.Fprint(w, `null`)
			return
		}
		items := make([]map[string]any, 100)
		for i := range items {
			items[i] = map[string]any{"id": fmt.Sprintf("open-%d", i)}
		}
		json.NewEncoder(w).Encode(items)
	}))
	defer server.Close()

	c := client.New(&config.Config{BaseURL: server.URL}, 5*time.Second, 0)
	items, err := fetchOpenPostingsSnapshot(context.Background(), c, &rootFlags{}, "/postings/acme")
	if err == nil || items != nil {
		t.Fatalf("malformed later page returned %d items, err=%v; want no snapshot", len(items), err)
	}
}

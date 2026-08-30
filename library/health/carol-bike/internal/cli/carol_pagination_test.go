// Copyright 2026 bricenice17 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/health/carol-bike/internal/store"
)

type recordingPageClient struct {
	params []map[string]string
}

func (c *recordingPageClient) GetWithHeaders(_ context.Context, _ string, params map[string]string, _ map[string]string) (json.RawMessage, error) {
	copyParams := make(map[string]string, len(params))
	for key, value := range params {
		copyParams[key] = value
	}
	c.params = append(c.params, copyParams)
	return json.RawMessage(`[]`), nil
}

func TestPaginatedGetPreservesZeroBasedFirstPage(t *testing.T) {
	c := &recordingPageClient{}
	_, err := paginatedGet(context.Background(), c, "/rides", map[string]string{
		"page": "0",
		"size": "40",
	}, nil, false, "page", "page", "size", 40, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.params) != 1 {
		t.Fatalf("request count = %d, want 1", len(c.params))
	}
	if got := c.params[0]["page"]; got != "0" {
		t.Fatalf("page param = %q, want zero-based first page", got)
	}
}

func TestPageCursorAdvancesFromZero(t *testing.T) {
	next, ok := nextClientSidePaginationCursor(map[string]string{"page": "0"}, "page", "page", 40)
	if !ok || next != "1" {
		t.Fatalf("next cursor = %q, %v; want 1, true", next, ok)
	}
}

type rideSyncRequest struct {
	path       string
	page       string
	apiVersion string
}

type recordingRideSyncClient struct {
	requests []rideSyncRequest
}

func (c *recordingRideSyncClient) Get(_ context.Context, path string, params map[string]string) (json.RawMessage, error) {
	page := params["page"]
	c.requests = append(c.requests, rideSyncRequest{path: path, page: page, apiVersion: params["v"]})

	items := []map[string]any{}
	switch path {
	case "/rider/{riderId}/ride/type/REHIT":
		if page == "0" {
			for id := 1; id <= 40; id++ {
				items = append(items, map[string]any{"id": id, "type": "REHIT"})
			}
		} else if page == "1" {
			items = append(items, map[string]any{"id": 41, "type": "REHIT"})
		}
	case "/rider/{riderId}/ride/type/FAT_BURN":
		items = append(items, map[string]any{"id": 100, "type": "FAT_BURN"})
	case "/rider/{riderId}/ride/type/FREE_AND_ZONES_AND_CUSTOM":
		items = append(items,
			map[string]any{"id": 100, "type": "FREE_AND_ZONES_AND_CUSTOM"},
			map[string]any{"id": 200, "type": "FREE_AND_ZONES_AND_CUSTOM"},
		)
	case "/rider/{riderId}/ride/type/FITNESS_TESTS":
		items = append(items, map[string]any{"id": 300, "type": "FITNESS_TESTS"})
	default:
		return nil, fmt.Errorf("unexpected ride sync path %q", path)
	}

	data, err := json.Marshal(map[string]any{"content": items})
	return data, err
}

func (*recordingRideSyncClient) RateLimit() float64 { return 0 }

func TestRideSyncEnumeratesAllFamiliesFromZeroAndDeduplicatesStableIDs(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "rides.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	client := &recordingRideSyncClient{}
	result := syncResource(context.Background(), client, db, "ride", "", false, 0, false, false, &syncUserParams{}, nil)
	if result.Err != nil {
		t.Fatalf("sync ride: %v", result.Err)
	}
	if result.Count != 44 {
		t.Fatalf("synced count = %d, want 44 unique rides", result.Count)
	}

	wantPages := map[string][]string{
		"/rider/{riderId}/ride/type/REHIT":                     {"0", "1"},
		"/rider/{riderId}/ride/type/FAT_BURN":                  {"0"},
		"/rider/{riderId}/ride/type/FREE_AND_ZONES_AND_CUSTOM": {"0"},
		"/rider/{riderId}/ride/type/FITNESS_TESTS":             {"0"},
	}
	gotPages := make(map[string][]string, len(wantPages))
	for _, request := range client.requests {
		gotPages[request.path] = append(gotPages[request.path], request.page)
	}
	for path, want := range wantPages {
		got := gotPages[path]
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("pages for %s = %v, want %v", path, got, want)
		}
	}
	for _, request := range client.requests {
		if request.apiVersion != "3.9.1" {
			t.Errorf("API version for %s page %s = %q, want 3.9.1", request.path, request.page, request.apiVersion)
		}
	}

	var typedCount int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM ride`).Scan(&typedCount); err != nil {
		t.Fatal(err)
	}
	if typedCount != 44 {
		t.Fatalf("typed ride rows = %d, want 44", typedCount)
	}

	client.requests = nil
	result = syncResource(context.Background(), client, db, "ride", "", false, 0, false, false, &syncUserParams{}, nil)
	if result.Err != nil {
		t.Fatalf("repeat sync ride: %v", result.Err)
	}
	if result.Count != 44 {
		t.Fatalf("repeat synced count = %d, want stable 44", result.Count)
	}
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM ride`).Scan(&typedCount); err != nil {
		t.Fatal(err)
	}
	if typedCount != 44 {
		t.Fatalf("typed ride rows after repeat = %d, want stable 44", typedCount)
	}
}

func TestRideSyncExplicitAPIVersionOverridesDefault(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "rides.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	client := &recordingRideSyncClient{}
	params := &syncUserParams{trueGlobal: map[string]string{"v": "test-override"}}
	result := syncResource(context.Background(), client, db, "ride", "", false, 1, false, false, params, nil)
	if result.Err != nil {
		t.Fatalf("sync ride with API-version override: %v", result.Err)
	}
	if len(client.requests) != len(carolRideSyncTargets) {
		t.Fatalf("request count = %d, want %d", len(client.requests), len(carolRideSyncTargets))
	}
	for _, request := range client.requests {
		if request.apiVersion != "test-override" {
			t.Errorf("API version for %s = %q, want explicit override", request.path, request.apiVersion)
		}
	}
}

func TestRideSyncTargetsAreClosedAndConcrete(t *testing.T) {
	want := []rideSyncTarget{
		{path: "/rider/{riderId}/ride/type/REHIT", stateKey: "ride"},
		{path: "/rider/{riderId}/ride/type/FAT_BURN", stateKey: "ride:FAT_BURN"},
		{path: "/rider/{riderId}/ride/type/FREE_AND_ZONES_AND_CUSTOM", stateKey: "ride:FREE_AND_ZONES_AND_CUSTOM"},
		{path: "/rider/{riderId}/ride/type/FITNESS_TESTS", stateKey: "ride:FITNESS_TESTS"},
	}
	if fmt.Sprint(carolRideSyncTargets) != fmt.Sprint(want) {
		t.Fatalf("ride sync targets = %v, want %v", carolRideSyncTargets, want)
	}
	if _, err := syncResourcePath("ride:ARBITRARY"); err == nil {
		t.Fatal("arbitrary ride family unexpectedly accepted as a sync resource")
	}
}

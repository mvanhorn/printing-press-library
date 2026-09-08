// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// recordedCall captures one request the fake Spotify server received.
type recordedCall struct {
	Method string
	Path   string
	Body   map[string]any
}

// newItemsRouteServer serves a one-row /playlists/{id}/items page and
// records every request so tests can assert the HTTP contract of the
// playlist helpers: paths, methods, and body keys.
func newItemsRouteServer(t *testing.T) (*httptest.Server, *[]recordedCall) {
	t.Helper()
	calls := &[]recordedCall{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			_ = json.Unmarshal(raw, &body)
		}
		*calls = append(*calls, recordedCall{Method: r.Method, Path: r.URL.Path, Body: body})
		w.Header().Set("Content-Type", "application/json")
		path := strings.TrimPrefix(r.URL.Path, "/v1")
		switch {
		case r.Method == http.MethodGet && path == "/playlists/PL":
			w.Write([]byte(`{"id":"PL","name":"Fixture","snapshot_id":"snap-1"}`))
		case r.Method == http.MethodGet && path == "/playlists/PL/items":
			// /items rows carry the track under `item`, not `track`.
			w.Write([]byte(`{"items":[{"added_at":"2026-01-01T00:00:00Z","added_by":{"id":"u1"},"item":{"id":"T1","uri":"spotify:track:T1","name":"One","artists":[{"name":"A"}],"external_ids":{"isrc":"ISRC1"}}}],"next":null}`))
		case path == "/playlists/PL/items":
			w.Write([]byte(`{"snapshot_id":"snap-2"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server, calls
}

func TestFetchFullPlaylist_DecodesItemsRoute(t *testing.T) {
	t.Parallel()
	server, calls := newItemsRouteServer(t)
	c := newTestClient(t, server.URL)

	id, name, snapshot, items, err := fetchFullPlaylist(c, "PL")
	if err != nil {
		t.Fatal(err)
	}
	if id != "PL" || name != "Fixture" || snapshot != "snap-1" {
		t.Fatalf("metadata = %q %q %q", id, name, snapshot)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Track.ID != "T1" || items[0].Track.URI != "spotify:track:T1" || items[0].Track.ExternalIDs.ISRC != "ISRC1" {
		t.Fatalf("row decoded from `item` incorrectly: %+v", items[0].Track)
	}
	var gotItems bool
	for _, call := range *calls {
		if call.Method == http.MethodGet && strings.HasSuffix(call.Path, "/playlists/PL/items") {
			gotItems = true
		}
		if strings.HasSuffix(call.Path, "/tracks") {
			t.Fatalf("deprecated /tracks route was called: %s %s", call.Method, call.Path)
		}
	}
	if !gotItems {
		t.Fatalf("GET /playlists/PL/items was never called; calls = %+v", *calls)
	}
}

func TestRemovePlaylistItems_SendsItemsKeyToItemsRoute(t *testing.T) {
	t.Parallel()
	server, calls := newItemsRouteServer(t)
	c := newTestClient(t, server.URL)

	toRemove := []map[string]any{{"uri": "spotify:track:T1", "positions": []int{3}}}
	if err := removePlaylistItems(c, "PL", "snap-1", toRemove); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(*calls))
	}
	call := (*calls)[0]
	if call.Method != http.MethodDelete || !strings.HasSuffix(call.Path, "/playlists/PL/items") {
		t.Fatalf("got %s %s, want DELETE /playlists/PL/items", call.Method, call.Path)
	}
	if _, old := call.Body["tracks"]; old {
		t.Fatalf("body still uses deprecated `tracks` key: %v", call.Body)
	}
	items, ok := call.Body["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("body[items] = %v, want one entry", call.Body["items"])
	}
	if call.Body["snapshot_id"] != "snap-1" {
		t.Fatalf("body[snapshot_id] = %v, want snap-1", call.Body["snapshot_id"])
	}
}

func TestReplacePlaylistItems_PutThenPostInChunks(t *testing.T) {
	t.Parallel()
	server, calls := newItemsRouteServer(t)
	c := newTestClient(t, server.URL)

	uris := make([]string, 0, 150)
	for i := 0; i < 150; i++ {
		uris = append(uris, "spotify:track:X")
	}
	added, err := replacePlaylistItems(c, "PL", uris)
	if err != nil {
		t.Fatal(err)
	}
	if added != 150 {
		t.Fatalf("added = %d, want 150", added)
	}
	if len(*calls) != 2 {
		t.Fatalf("calls = %d, want 2 (PUT then POST)", len(*calls))
	}
	want := []struct {
		method string
		n      int
	}{{http.MethodPut, 100}, {http.MethodPost, 50}}
	for i, w := range want {
		call := (*calls)[i]
		if call.Method != w.method || !strings.HasSuffix(call.Path, "/playlists/PL/items") {
			t.Fatalf("call %d = %s %s, want %s /playlists/PL/items", i, call.Method, call.Path, w.method)
		}
		got, _ := call.Body["uris"].([]any)
		if len(got) != w.n {
			t.Fatalf("call %d uris = %d, want %d", i, len(got), w.n)
		}
	}
}

func TestReplacePlaylistItems_EmptyClearsWithPut(t *testing.T) {
	t.Parallel()
	server, calls := newItemsRouteServer(t)
	c := newTestClient(t, server.URL)

	added, err := replacePlaylistItems(c, "PL", nil)
	if err != nil {
		t.Fatal(err)
	}
	if added != 0 || len(*calls) != 1 {
		t.Fatalf("added = %d, calls = %d; want 0 and 1", added, len(*calls))
	}
	call := (*calls)[0]
	got, _ := call.Body["uris"].([]any)
	if call.Method != http.MethodPut || len(got) != 0 {
		t.Fatalf("got %s with uris=%v, want PUT with empty uris", call.Method, got)
	}
}

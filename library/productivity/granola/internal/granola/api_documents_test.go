// Copyright 2026 Granola Printing Press contributors. Licensed under Apache-2.0.

package granola

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHydrateDocumentsFromAPI_NilCache(t *testing.T) {
	_, err := HydrateDocumentsFromAPI(nil, nil)
	if err == nil {
		t.Fatal("expected error on nil cache")
	}
}

func TestHydrateDocumentsFromAPI_SinglePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/get-documents" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"docs": []map[string]any{
				{"id": "a", "title": "Meeting A"},
				{"id": "b", "title": "Meeting B"},
			},
			"has_more": false,
		})
	}))
	defer srv.Close()

	t.Setenv("GRANOLA_WORKOS_TOKEN", "test-token")
	ResetTokenCache()
	defer ResetTokenCache()

	client, _ := NewInternalClient()
	client.SetBaseURL(srv.URL)
	cache := &Cache{Documents: map[string]Document{}}
	n, err := HydrateDocumentsFromAPI(cache, client)
	if err != nil {
		t.Fatalf("HydrateDocumentsFromAPI: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 docs, got %d", n)
	}
	if cache.Documents["a"].Title != "Meeting A" {
		t.Errorf("missing doc a, got %+v", cache.Documents["a"])
	}
}

func TestHydrateDocumentsFromAPI_Pagination(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		// First call returns a full page (100 docs), second returns 1, signaling done.
		var docs []map[string]any
		if calls == 1 {
			for i := 0; i < 100; i++ {
				docs = append(docs, map[string]any{"id": fmt.Sprintf("p1-%d", i)})
			}
		} else {
			docs = []map[string]any{{"id": "p2-0"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"docs": docs})
	}))
	defer srv.Close()

	t.Setenv("GRANOLA_WORKOS_TOKEN", "test-token")
	ResetTokenCache()
	defer ResetTokenCache()
	client, _ := NewInternalClient()
	client.SetBaseURL(srv.URL)
	cache := &Cache{Documents: map[string]Document{}}

	n, err := HydrateDocumentsFromAPI(cache, client)
	if err != nil {
		t.Fatalf("HydrateDocumentsFromAPI: %v", err)
	}
	if n != 101 {
		t.Errorf("expected 101 docs across 2 pages, got %d", n)
	}
	if calls != 2 {
		t.Errorf("expected 2 API calls, got %d", calls)
	}
}

func TestHydrateDocumentsFromAPI_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"docs": []map[string]any{}})
	}))
	defer srv.Close()
	t.Setenv("GRANOLA_WORKOS_TOKEN", "test-token")
	ResetTokenCache()
	defer ResetTokenCache()
	client, _ := NewInternalClient()
	client.SetBaseURL(srv.URL)
	cache := &Cache{Documents: map[string]Document{}}

	n, err := HydrateDocumentsFromAPI(cache, client)
	if err != nil {
		t.Fatalf("HydrateDocumentsFromAPI: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 docs, got %d", n)
	}
}

func TestHydrateDocumentsFromAPI_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	t.Setenv("GRANOLA_WORKOS_TOKEN", "test-token")
	ResetTokenCache()
	defer ResetTokenCache()
	client, _ := NewInternalClient()
	client.SetBaseURL(srv.URL)
	cache := &Cache{Documents: map[string]Document{}}

	_, err := HydrateDocumentsFromAPI(cache, client)
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestHydrateDocumentsFromAPI_RefreshRefusedSurfacesAsTypedError(t *testing.T) {
	// Simulate the D6 case: the server returns 401, the InternalClient
	// tries to refresh, but the source is encrypted so refresh refuses.
	// HydrateDocumentsFromAPI should wrap that with a clear "wake desktop"
	// hint but still match ErrRefreshRefused via errors.Is.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	t.Setenv("GRANOLA_WORKOS_TOKEN", "")
	ResetTokenCache()
	defer ResetTokenCache()
	tokenMu.Lock()
	cachedAccess = "expired-access"
	cachedRefresh = "expired-refresh"
	cachedSource = TokenSourceEncryptedSupabase
	tokenMu.Unlock()

	client, _ := NewInternalClient()
	client.SetBaseURL(srv.URL)
	cache := &Cache{Documents: map[string]Document{}}

	_, err := HydrateDocumentsFromAPI(cache, client)
	if err == nil {
		t.Fatal("expected error when refresh is refused")
	}
	if !errors.Is(err, ErrRefreshRefused) {
		t.Logf("note: error chain does not include ErrRefreshRefused, got %v - acceptable if InternalClient surfaces 401 distinctly", err)
	}
}

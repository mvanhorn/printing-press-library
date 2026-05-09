// Copyright 2026 nikica-jokic. Licensed under Apache-2.0. See LICENSE.

package gql

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer testkey" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"transcripts":[{"id":"abc","title":"Test Meeting"}]}}`))
	}))
	defer srv.Close()

	c := &Client{
		baseURL:    srv.URL,
		authHeader: "Bearer testkey",
		http:       srv.Client(),
		limiter:    nil,
	}

	data, err := c.Do(t.Context(), `query { transcripts { id title } }`, nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	var result struct {
		Transcripts []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"transcripts"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Transcripts) != 1 || result.Transcripts[0].ID != "abc" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestClientQueryFieldExtraction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"transcript":{"id":"xyz","title":"Demo"}}}`))
	}))
	defer srv.Close()

	c := &Client{
		baseURL:    srv.URL,
		authHeader: "Bearer k",
		http:       srv.Client(),
		limiter:    nil,
	}

	field, err := c.Query(t.Context(), `query { transcript(id: "xyz") { id title } }`, nil, "transcript")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	var item struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(field, &item); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if item.ID != "xyz" {
		t.Errorf("got id %q, want xyz", item.ID)
	}
}

func TestClientRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := &Client{
		baseURL:    srv.URL,
		authHeader: "Bearer k",
		http:       srv.Client(),
		limiter:    nil,
	}

	_, err := c.Do(t.Context(), `query { transcripts { id } }`, nil)
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}
}

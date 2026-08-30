// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.

package graphqlclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/bestfoodtrucks/internal/cliutil"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c := New(5 * time.Second)
	c.endpoint = srv.URL
	return c, srv.Close
}

func TestQuery_Success(t *testing.T) {
	c, closeFn := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"lot":{"id":4702,"name":"Playa District"}}}`))
	})
	defer closeFn()

	var result struct {
		Lot struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"lot"`
	}
	if err := c.Query(context.Background(), "query { lot(seoName: \"playa-district\") { id name } }", nil, &result); err != nil {
		t.Fatalf("Query returned unexpected error: %v", err)
	}
	if result.Lot.ID != 4702 || result.Lot.Name != "Playa District" {
		t.Fatalf("unexpected result: %+v", result.Lot)
	}
}

func TestQuery_GraphQLError(t *testing.T) {
	c, closeFn := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":[{"message":"lot not found"}]}`))
	})
	defer closeFn()

	var result struct{}
	err := c.Query(context.Background(), "query { lot(seoName: \"nope\") { id } }", nil, &result)
	if err == nil {
		t.Fatal("expected an error for a GraphQL errors[] response, got nil")
	}
	if got := err.Error(); got == "" {
		t.Fatal("expected a non-empty error message")
	}
}

func TestQuery_HTTPError(t *testing.T) {
	c, closeFn := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	})
	defer closeFn()

	var result struct{}
	err := c.Query(context.Background(), "query { lot(seoName: \"x\") { id } }", nil, &result)
	if err == nil {
		t.Fatal("expected an error for HTTP 500, got nil")
	}
}

func TestQuery_RateLimited(t *testing.T) {
	c, closeFn := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	defer closeFn()

	var result struct{}
	err := c.Query(context.Background(), "query { lot(seoName: \"x\") { id } }", nil, &result)
	if err == nil {
		t.Fatal("expected a RateLimitError for HTTP 429, got nil")
	}
	var rateErr *cliutil.RateLimitError
	if !errors.As(err, &rateErr) {
		t.Fatalf("expected error to be a *cliutil.RateLimitError, got %T: %v", err, err)
	}
}

func TestQuery_VariablesMarshaled(t *testing.T) {
	var gotBody string
	c, closeFn := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{}}`))
	})
	defer closeFn()

	var result map[string]any
	err := c.Query(context.Background(), "query Q($seoName: String!) { lot(seoName: $seoName) { id } }",
		map[string]any{"seoName": "playa-district"}, &result)
	if err != nil {
		t.Fatalf("Query returned unexpected error: %v", err)
	}
	if gotBody == "" {
		t.Fatal("expected the server to receive a non-empty request body")
	}
}

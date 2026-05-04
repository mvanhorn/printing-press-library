package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

func TestMakeAPIHandlerKeepsQueryParamsForSearchEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/search/movie" {
			t.Fatalf("path = %q, want /search/movie", got)
		}
		if got := r.URL.Query().Get("query"); got != "The Martian" {
			t.Fatalf("query param = %q, want The Martian", got)
		}
		if got := r.URL.Query().Get("year"); got != "2015" {
			t.Fatalf("year param = %q, want 2015", got)
		}
		_, _ = w.Write([]byte(`{"page":1,"results":[{"id":286217,"title":"The Martian"}],"total_pages":1,"total_results":1}`))
	}))
	defer server.Close()

	t.Setenv("MOVIE_GOAT_BASE_URL", server.URL)
	t.Setenv("TMDB_API_KEY", "test-key")

	handler := makeAPIHandler("GET", "/search/movie", []string{"query"})
	result, err := handler(context.Background(), mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Arguments: map[string]any{
				"query": "The Martian",
				"year":  "2015",
			},
		},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result == nil || len(result.Content) != 1 {
		t.Fatalf("unexpected result content: %#v", result)
	}
	text, ok := result.Content[0].(mcplib.TextContent)
	if !ok {
		t.Fatalf("content type = %T, want mcp.TextContent", result.Content[0])
	}

	var payload struct {
		Results []struct {
			ID    int    `json:"id"`
			Title string `json:"title"`
		} `json:"results"`
		TotalResults int `json:"total_results"`
	}
	if err := json.Unmarshal([]byte(text.Text), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.TotalResults != 1 || len(payload.Results) != 1 || payload.Results[0].ID != 286217 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestMakeAPIHandlerExcludesPathParamsFromQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/movie/286217" {
			t.Fatalf("path = %q, want /movie/286217", got)
		}
		if got := r.URL.Query().Get("movieId"); got != "" {
			t.Fatalf("movieId query param = %q, want empty", got)
		}
		if got := r.URL.Query().Get("language"); got != "en-US" {
			t.Fatalf("language param = %q, want en-US", got)
		}
		_, _ = w.Write([]byte(`{"id":286217,"title":"The Martian"}`))
	}))
	defer server.Close()

	t.Setenv("MOVIE_GOAT_BASE_URL", server.URL)
	t.Setenv("TMDB_API_KEY", "test-key")

	handler := makeAPIHandler("GET", "/movie/{movieId}", []string{"movieId"})
	_, err := handler(context.Background(), mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Arguments: map[string]any{
				"movieId":  "286217",
				"language": "en-US",
			},
		},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
}

package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/arcmira/internal/config"
)

func TestTranscriptPollingBypassesResponseCache(t *testing.T) {
	for _, staleCache := range []bool{false, true} {
		t.Run(map[bool]string{false: "pending_then_ready", true: "old_pending_cache"}[staleCache], func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/v1/transcripts/test-video" || r.URL.Query().Get("quality") != "premium" {
					t.Errorf("unexpected request: %s", r.URL)
				}
				w.Header().Set("Content-Type", "application/json")
				if calls == 1 {
					w.WriteHeader(http.StatusAccepted)
					_, _ = w.Write([]byte(`{"status":"processing"}`))
				} else {
					_, _ = w.Write([]byte(`{"status":"ready","text":"Exact words."}`))
				}
			}))
			defer server.Close()
			c := New(&config.Config{BaseURL: server.URL}, time.Second, 0)
			c.cacheDir = t.TempDir()
			path := "/v1/transcripts/test-video"
			params := map[string]string{"quality": "premium"}
			if staleCache {
				c.writeCacheWithHeaders(path, params, nil, json.RawMessage(`{"status":"processing"}`))
			}
			for _, want := range []string{"processing", "ready"} {
				body, err := c.Get(context.Background(), path, params)
				if err != nil {
					t.Fatal(err)
				}
				var got struct {
					Status string `json:"status"`
				}
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatal(err)
				}
				if got.Status != want {
					t.Fatalf("status = %q, want %q", got.Status, want)
				}
			}
			if calls != 2 {
				t.Fatalf("HTTP requests = %d, want 2", calls)
			}
		})
	}
}

func TestOtherReadsStillUseResponseCache(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()
	c := New(&config.Config{BaseURL: server.URL}, time.Second, 0)
	c.cacheDir = t.TempDir()
	for range 2 {
		body, err := c.Get(context.Background(), "/v1/health", nil)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != `{"status":"ok"}` {
			t.Fatalf("unexpected body: %s", body)
		}
	}
	if calls != 1 {
		t.Fatalf("HTTP requests = %d, want 1", calls)
	}
}

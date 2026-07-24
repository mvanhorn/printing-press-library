package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"lichess-pp-cli/internal/config"
)

func TestNDJSONIsNotTreatedAsBinary(t *testing.T) {
	for _, contentType := range []string{"application/x-ndjson", "application/x-ndjson; charset=utf-8", "application/ndjson"} {
		if isBinaryResponseContentType(contentType) {
			t.Fatalf("%q was treated as binary", contentType)
		}
	}
}

func TestPostFormEncodesChallengeFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/challenge/alice" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Fatalf("Content-Type = %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		values, err := url.ParseQuery(string(body))
		if err != nil || values.Get("clock.limit") != "600" || values.Get("clock.increment") != "0" {
			t.Fatalf("form = %q, err = %v", body, err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"challenge-id"}`))
	}))
	defer server.Close()

	c := New(&config.Config{BaseURL: server.URL}, 0, 0)
	if _, status, err := c.PostForm(context.Background(), "/api/challenge/alice", url.Values{"clock.limit": {"600"}, "clock.increment": {"0"}}); err != nil || status != http.StatusOK {
		t.Fatalf("PostForm() status=%d err=%v", status, err)
	}
}

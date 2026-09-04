package client

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/github/internal/config"
)

type statusRoundTripper struct {
	status int
	calls  int
}

func (s *statusRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	s.calls++
	return &http.Response{
		StatusCode: s.status,
		Body:       io.NopCloser(strings.NewReader(`{"message":"boom"}`)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Request:    req,
	}, nil
}

func TestRetryOnServerError_SkipsPostAndPatch(t *testing.T) {
	t.Parallel()
	if retryOnServerError(http.MethodPost) || retryOnServerError(http.MethodPatch) {
		t.Fatal("POST/PATCH 5xx must not retry")
	}
	if !retryOnServerError(http.MethodGet) || !retryOnServerError(http.MethodDelete) {
		t.Fatal("idempotent methods should retry 5xx")
	}
}

func TestDo_DoesNotReplayPostOn500(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "")
	rt := &statusRoundTripper{status: 500}
	cfg := &config.Config{BaseURL: "http://example.test"}
	c := New(cfg, time.Second, 0)
	c.HTTPClient = &http.Client{Transport: rt}
	c.NoCache = true

	_, status, err := c.do(context.Background(), http.MethodPost, "/repos/cli/cli/issues", nil, map[string]any{"title": "x"}, nil)
	if err == nil {
		t.Fatal("expected API error")
	}
	if status != 500 {
		t.Fatalf("status = %d, want 500", status)
	}
	if rt.calls != 1 {
		t.Fatalf("POST 500 retried %d times; want 1 call", rt.calls)
	}
}

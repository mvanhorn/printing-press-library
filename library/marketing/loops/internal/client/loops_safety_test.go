package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/loops/internal/config"
)

func loopsMockClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOOPS_SEND_JOURNAL_DIR", dir)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return New(&config.Config{BaseURL: server.URL, AuthHeaderVal: "Bearer synthetic"}, 3*time.Second, 20)
}

func TestLoopsMutationRequiresIntentAndExactTeam(t *testing.T) {
	var writes atomic.Int32
	c := loopsMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/api-key" {
			_, _ = io.WriteString(w, `{"success":true,"teamName":"Synthetic Team"}`)
			return
		}
		writes.Add(1)
		_, _ = io.WriteString(w, `{"success":true}`)
	})
	if _, _, err := c.Post(context.Background(), "/v1/contacts/create", map[string]any{"synthetic": true}); err == nil {
		t.Fatal("write without intent succeeded")
	}
	if _, _, err := c.Post(WithLoopsMutationIntent(context.Background(), "Wrong Team"), "/v1/contacts/create", nil); err == nil {
		t.Fatal("wrong team succeeded")
	}
	if writes.Load() != 0 {
		t.Fatalf("write count = %d, want zero", writes.Load())
	}
	if _, _, err := c.Post(WithLoopsMutationIntent(context.Background(), "Synthetic Team"), "/v1/contacts/create", nil); err != nil {
		t.Fatal(err)
	}
	if writes.Load() != 1 {
		t.Fatalf("write count = %d, want one", writes.Load())
	}
}

func TestLoopsSendNeedsKeyAndCannotRepeat(t *testing.T) {
	var sends atomic.Int32
	c := loopsMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/api-key" {
			_, _ = io.WriteString(w, `{"success":true,"teamName":"Synthetic Team"}`)
			return
		}
		if got := r.Header.Get("Idempotency-Key"); got != "synthetic-key" {
			t.Errorf("idempotency header = %q", got)
		}
		sends.Add(1)
		_, _ = io.WriteString(w, `{"success":true}`)
	})
	ctx := WithLoopsMutationIntent(context.Background(), "Synthetic Team")
	body := map[string]any{"eventName": "synthetic", "email": "person@example.test"}
	if _, _, err := c.Post(ctx, "/v1/events/send", body); err == nil {
		t.Fatal("send without key succeeded")
	}
	headers := map[string]string{"Idempotency-Key": "synthetic-key"}
	if _, _, err := c.PostWithHeaders(ctx, "/v1/events/send", body, headers); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.PostWithHeaders(ctx, "/v1/events/send", body, headers); err == nil || !strings.Contains(err.Error(), "already attempted") {
		t.Fatalf("duplicate send error = %v", err)
	}
	if sends.Load() != 1 {
		t.Fatalf("send count = %d, want one", sends.Load())
	}
}

func TestLoopsRateLimitReleasesSameKey(t *testing.T) {
	var sends atomic.Int32
	c := loopsMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/api-key" {
			_, _ = io.WriteString(w, `{"success":true,"teamName":"Synthetic Team"}`)
			return
		}
		if sends.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = io.WriteString(w, `{"success":true}`)
	})
	ctx := WithLoopsMutationIntent(context.Background(), "Synthetic Team")
	headers := map[string]string{"Idempotency-Key": "synthetic-key"}
	if _, _, err := c.PostWithHeaders(ctx, "/v1/transactional", map[string]any{"email": "person@example.test"}, headers); err == nil {
		t.Fatal("429 send should fail and require caller retry")
	}
	if _, _, err := c.PostWithHeaders(ctx, "/v1/transactional", map[string]any{"email": "person@example.test"}, headers); err != nil {
		t.Fatal(err)
	}
	if sends.Load() != 2 {
		t.Fatalf("send count = %d, want two caller attempts", sends.Load())
	}
}

func TestLoopsUncertainSendStaysClaimed(t *testing.T) {
	var sends atomic.Int32
	c := loopsMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/api-key" {
			_, _ = io.WriteString(w, `{"success":true,"teamName":"Synthetic Team"}`)
			return
		}
		sends.Add(1)
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("server cannot hijack")
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	})
	ctx := WithLoopsMutationIntent(context.Background(), "Synthetic Team")
	headers := map[string]string{"Idempotency-Key": "synthetic-key"}
	for i := 0; i < 2; i++ {
		if _, _, err := c.PostWithHeaders(ctx, "/v1/events/send", map[string]any{"eventName": "synthetic", "email": "person@example.test"}, headers); err == nil {
			t.Fatal("uncertain or repeated send unexpectedly succeeded")
		}
	}
	if sends.Load() != 1 {
		t.Fatalf("send count = %d, want one", sends.Load())
	}
}

func TestLoopsErrorsOmitContactDataAndUnconfirmedSendStaysClaimed(t *testing.T) {
	c := loopsMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/api-key" {
			_, _ = io.WriteString(w, `{"success":true,"teamName":"Synthetic Team"}`)
			return
		}
		if r.URL.Path == "/v1/events/send" {
			_, _ = io.WriteString(w, `{"success":false}`)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"person@example.test is invalid"}`)
	})
	ctx := WithLoopsMutationIntent(context.Background(), "Synthetic Team")
	if _, _, err := c.Post(ctx, "/v1/contacts/create", map[string]any{"email": "person@example.test"}); err == nil || strings.Contains(err.Error(), "person@example.test") {
		t.Fatalf("API error leaked contact data or was missing: %v", err)
	}
	headers := map[string]string{"Idempotency-Key": "synthetic-key"}
	if _, _, err := c.PostWithHeaders(ctx, "/v1/events/send", map[string]any{"eventName": "synthetic", "email": "person@example.test"}, headers); err == nil || !strings.Contains(err.Error(), "no send confirmation") {
		t.Fatalf("false success error = %v", err)
	}
	if _, _, err := c.PostWithHeaders(ctx, "/v1/events/send", map[string]any{"eventName": "synthetic", "email": "person@example.test"}, headers); err == nil || !strings.Contains(err.Error(), "already attempted") {
		t.Fatalf("unconfirmed send should stay claimed: %v", err)
	}
}

func TestLoopsTransactionalReadDoesNotRequireSendConfirmation(t *testing.T) {
	c := loopsMockClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/transactional" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `[{"id":"synthetic"}]`)
	})
	data, err := c.GetNoCache(context.Background(), "/v1/transactional", nil)
	if err != nil || !strings.Contains(string(data), "synthetic") {
		t.Fatalf("GET listing treated as a send: %v", err)
	}
}

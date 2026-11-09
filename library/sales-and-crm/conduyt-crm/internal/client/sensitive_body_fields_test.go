package client

// PATCH(reprint-637-sensitive-body-fields): Codex reprint637-r1 P1. The Grok Bot webhook
// paste (PUT /chat/agents/{id}/webhook) carries the webhook URL and its crsr_ key under
// field names isSensitiveName reads as ordinary; neither the dry-run preview nor an
// echoed error body may print them.

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const (
	pasteURL = "https://bot.example.test/hooks/agent-77?sig=hookSIG998877"
	pasteKey = "crsr_LIVEkey0123456789abcdef"
)

func webhookBody() map[string]any {
	return map[string]any{
		"paste":   "Webhook URL: " + pasteURL + "\nKey: " + pasteKey,
		"address": pasteURL,
		"key":     pasteKey,
	}
}

func leaks(s string) []string {
	var out []string
	for _, l := range []string{pasteURL, pasteKey, "hookSIG998877", "LIVEkey0123456789", "bot.example.test/hooks"} {
		if strings.Contains(s, l) {
			out = append(out, l)
		}
	}
	return out
}

func TestWebhookPasteIsMaskedInTheDryRunPreview(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&hits, 1) }))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	c.DryRun = true
	out := captureStderr(t, func() {
		if _, _, err := c.do(context.Background(), "PUT", "/chat/agents/550e8400-e29b-41d4-a716-446655440000/webhook", nil, webhookBody(), nil); err != nil {
			t.Errorf("dry run: %v", err)
		}
	})
	if l := leaks(out); len(l) > 0 {
		t.Fatalf("dry-run preview leaked %v:\n%s", l, out)
	}
	if !strings.Contains(out, `"paste": "****"`) || !strings.Contains(out, `"address": "****"`) {
		t.Fatalf("preview should show the masked fields:\n%s", out)
	}
	if atomic.LoadInt32(&hits) != 0 {
		t.Fatalf("dry run sent %d requests", hits)
	}
}

func TestWebhookPasteEchoedInAnErrorBodyIsMasked(t *testing.T) {
	retryHarness(t, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		w.WriteHeader(400)
		// echo the request, and each piece of the paste alone
		_, _ = w.Write([]byte(`{"error":"bad paste","got":` + string(b) + `,"url":"` + pasteURL + `","k":"` + pasteKey + `"}`))
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	_, _, err := c.do(context.Background(), "PUT", "/chat/agents/@grok/webhook", nil, webhookBody(), nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 {
		t.Fatalf("err=%v", err)
	}
	if l := leaks(apiErr.Body + err.Error()); len(l) > 0 {
		t.Fatalf("error body leaked %v: %s", l, apiErr.Body)
	}
}

func TestAddressStaysReadableOffTheWebhookPath(t *testing.T) {
	c := newRetryClient("http://example.invalid", nil)
	c.DryRun = true
	out := captureStderr(t, func() {
		_, _, _ = c.do(context.Background(), "POST", "/email/opt-out-list/check", nil, map[string]any{"address": "person@example.com"}, nil)
	})
	if !strings.Contains(out, "person@example.com") {
		t.Fatalf("an email address off the webhook path must stay readable:\n%s", out)
	}
}

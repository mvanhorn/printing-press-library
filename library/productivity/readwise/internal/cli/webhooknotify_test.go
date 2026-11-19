// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
)

func TestValidWebhookURL(t *testing.T) {
	// allowed
	if err := validWebhookURL("https://hooks.slack.com/x"); err != nil {
		t.Errorf("validWebhookURL(slack) unexpected error: %v", err)
	}

	// denied
	denied := []string{
		"http://x",
		"https://localhost/x",
		"https://127.0.0.1/x",
		"https://10.0.0.1/x",
		"https://192.168.0.1/x",
		"https://169.254.0.1/x",
		"https://[::1]/x",
		"https://printer.local/x",
	}
	for _, u := range denied {
		if err := validWebhookURL(u); err == nil {
			t.Errorf("validWebhookURL(%q) = nil, want error", u)
		}
	}
}

func TestPostWebhookSSRFBlocked(t *testing.T) {
	ctx := context.Background()
	err := postWebhook(ctx, "https://127.0.0.1/ping", "", []byte("{}"))
	if err == nil {
		t.Fatal("postWebhook to 127.0.0.1 should return an error (SSRF blocked)")
	}
}

func TestSignPayload(t *testing.T) {
	// HMAC-SHA256("key", "body") = 515aae133b435d4000956731f68ae5cf5eb85d4f0dc6a546d2bfcd3595ec1ae1
	got := signPayload("key", []byte("body"))
	const want = "515aae133b435d4000956731f68ae5cf5eb85d4f0dc6a546d2bfcd3595ec1ae1"
	if got != want {
		t.Errorf("signPayload = %q, want %q", got, want)
	}
}

func TestBuildDocPayload(t *testing.T) {
	items := []store.DocItem{
		{ID: "1", Title: "T", Author: "A", URL: "https://x.com", Summary: "sum"},
	}

	// includeText=true: summary present, notes absent
	body, err := buildDocPayload("w", items, true)
	if err != nil {
		t.Fatalf("buildDocPayload(true) err: %v", err)
	}
	js := string(body)
	if !strings.Contains(js, `"summary"`) {
		t.Errorf("includeText=true: expected \"summary\" in payload, got: %s", js)
	}
	if strings.Contains(js, `"notes"`) {
		t.Errorf("includeText=true: unexpected \"notes\" in payload: %s", js)
	}

	// includeText=false: summary absent (omitempty on zero value)
	body, err = buildDocPayload("w", items, false)
	if err != nil {
		t.Fatalf("buildDocPayload(false) err: %v", err)
	}
	js = string(body)
	if strings.Contains(js, `"summary"`) {
		t.Errorf("includeText=false: unexpected \"summary\" in payload: %s", js)
	}
}

func TestBuildHLPayload(t *testing.T) {
	items := []store.HLItem{
		{ID: "1", Title: "T", Author: "A", URL: "https://x.com", Text: "txt"},
	}

	// includeText=true: text present
	body, err := buildHLPayload("w", items, true)
	if err != nil {
		t.Fatalf("buildHLPayload(true) err: %v", err)
	}
	if !strings.Contains(string(body), `"text"`) {
		t.Errorf("includeText=true: expected \"text\" in payload, got: %s", body)
	}

	// includeText=false: text absent (omitempty on zero value)
	body, err = buildHLPayload("w", items, false)
	if err != nil {
		t.Fatalf("buildHLPayload(false) err: %v", err)
	}
	if strings.Contains(string(body), `"text"`) {
		t.Errorf("includeText=false: unexpected \"text\" in payload: %s", body)
	}
}

func TestWebhookWantsKind(t *testing.T) {
	cases := []struct {
		filter, kind string
		want         bool
	}{
		{"*", "docs", true},
		{"docs,highlights", "docs", true},
		{"DOCS", "docs", true},
		{"highlights", "docs", false},
		{"", "docs", true},
	}
	for _, c := range cases {
		got := webhookWantsKind(c.filter, c.kind)
		if got != c.want {
			t.Errorf("webhookWantsKind(%q,%q) = %v, want %v", c.filter, c.kind, got, c.want)
		}
	}
}

func TestFireWatchWebhooksVerifySuppression(t *testing.T) {
	t.Setenv(cliutil.VerifyEnvVar, "1")

	// Sentinel server: fails the test if ever called (verify mode should suppress all POSTs).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("webhook server was called in verify mode — network suppression failed")
	}))
	defer srv.Close()

	db, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	// Use a plausible https URL that passes validWebhookURL validation.
	// In verify mode, the URL is printed but never dialed.
	_, err = db.AddWebhook(ctx, "https://hooks.slack.com/test-suppress", "", "*", false)
	if err != nil {
		t.Fatalf("add webhook: %v", err)
	}

	var sb strings.Builder
	count, err := fireWatchWebhooks(ctx, &sb, db, "docs", []byte(`{"kind":"docs"}`), 3)
	if err != nil {
		t.Fatalf("fireWatchWebhooks: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 suppressed webhook, got %d", count)
	}
	out := sb.String()
	if !strings.Contains(out, "hooks.slack.com") {
		t.Errorf("expected URL in output, got: %q", out)
	}
	if !strings.Contains(out, "3") {
		t.Errorf("expected count (3) in output, got: %q", out)
	}
	// The verify output is "would POST to <url> (<n> items)" — no payload content.
	for _, field := range []string{"summary", "author", "title"} {
		if strings.Contains(out, field) {
			t.Errorf("verify suppression output must not contain payload field %q: %q", field, out)
		}
	}
}

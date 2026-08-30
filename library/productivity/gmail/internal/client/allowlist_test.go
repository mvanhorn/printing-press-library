// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Transport-allowlist tests (grill R3-C6 acceptance): forged shapes must be
// refused at the choke point without dialing; each of the 19 permitted
// operations must pass the matcher.

package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/config"
)

// TestTransportAllowlist_RefusesForgedShapes asserts the shapes the grill
// named — message sending, drafts, settings — plus the other destructive
// Gmail operations this binary must be structurally unable to perform.
func TestTransportAllowlist_RefusesForgedShapes(t *testing.T) {
	t.Parallel()
	forged := []struct {
		method, path string
	}{
		{"POST", "/gmail/v1/users/me/messages/send"},
		{"POST", "/gmail/v1/users/me/drafts"},
		{"GET", "/gmail/v1/users/me/drafts"},
		{"GET", "/gmail/v1/users/me/drafts/d1"},
		{"PUT", "/gmail/v1/users/me/drafts/d1"},
		{"POST", "/gmail/v1/users/me/drafts/send"},
		{"GET", "/gmail/v1/users/me/settings/filters"},
		{"POST", "/gmail/v1/users/me/settings/filters"},
		{"PUT", "/gmail/v1/users/me/settings/autoForwarding"},
		{"PUT", "/gmail/v1/users/me/settings/vacation"},
		// Permanent deletion in every form.
		{"DELETE", "/gmail/v1/users/me/messages/m1"},
		{"POST", "/gmail/v1/users/me/messages/batchDelete"},
		{"DELETE", "/gmail/v1/users/me/threads/t1"},
		{"DELETE", "/gmail/v1/users/me/labels/Label_1"},
		// Insertion / import of mail.
		{"POST", "/gmail/v1/users/me/messages"},
		{"POST", "/gmail/v1/users/me/messages/import"},
		{"POST", "/gmail/v1/users/me/messages/insert"},
		// Watch/stop push plumbing.
		{"POST", "/gmail/v1/users/me/watch"},
		{"POST", "/gmail/v1/users/me/stop"},
		// Wrong-method variants of permitted paths.
		{"POST", "/gmail/v1/users/me/history"},
		{"PATCH", "/gmail/v1/users/me/messages/m1"},
		{"PUT", "/gmail/v1/users/me/messages/m1/modify"},
		// Non-Gmail paths entirely.
		{"GET", "/"},
		{"POST", "/anything"},
	}
	for _, f := range forged {
		err := checkTransportAllowlist(f.method, f.path)
		if err == nil {
			t.Fatalf("%s %s: expected refusal, got nil", f.method, f.path)
		}
		var refused *TransportRefusedError
		if !errors.As(err, &refused) {
			t.Fatalf("%s %s: error is %T, want *TransportRefusedError", f.method, f.path, err)
		}
		if !strings.Contains(err.Error(), f.path) {
			t.Fatalf("%s %s: refusal must name the path; got %q", f.method, f.path, err.Error())
		}
	}
}

// TestTransportAllowlist_PermitsThe19Shapes drives one concrete request
// per permitted operation through the matcher — with a resolved segment,
// with the unresolved {userId} template form, and with a query string —
// covering the exact ways the generated commands and the engine build
// paths.
func TestTransportAllowlist_PermitsThe19Shapes(t *testing.T) {
	t.Parallel()
	legit := []struct {
		name, method, path string
	}{
		{"profile.get", "GET", "/gmail/v1/users/me/profile"},
		{"messages.list", "GET", "/gmail/v1/users/me/messages"},
		{"messages.get", "GET", "/gmail/v1/users/me/messages/18c2f9a"},
		{"attachments.get", "GET", "/gmail/v1/users/me/messages/18c2f9a/attachments/ANGjdJ8w"},
		{"messages.modify", "POST", "/gmail/v1/users/me/messages/18c2f9a/modify"},
		{"messages.trash", "POST", "/gmail/v1/users/me/messages/18c2f9a/trash"},
		{"messages.untrash", "POST", "/gmail/v1/users/me/messages/18c2f9a/untrash"},
		{"messages.batchModify", "POST", "/gmail/v1/users/me/messages/batchModify"},
		{"threads.list", "GET", "/gmail/v1/users/me/threads"},
		{"threads.get", "GET", "/gmail/v1/users/me/threads/18c2f9a"},
		{"threads.modify", "POST", "/gmail/v1/users/me/threads/18c2f9a/modify"},
		{"threads.trash", "POST", "/gmail/v1/users/me/threads/18c2f9a/trash"},
		{"threads.untrash", "POST", "/gmail/v1/users/me/threads/18c2f9a/untrash"},
		{"labels.list", "GET", "/gmail/v1/users/me/labels"},
		{"labels.get", "GET", "/gmail/v1/users/me/labels/Label_7"},
		{"labels.create", "POST", "/gmail/v1/users/me/labels"},
		{"labels.update", "PUT", "/gmail/v1/users/me/labels/Label_7"},
		{"labels.patch", "PATCH", "/gmail/v1/users/me/labels/Label_7"},
		{"history.list", "GET", "/gmail/v1/users/me/history"},
	}
	if len(legit) != len(transportAllowlist) {
		t.Fatalf("test covers %d shapes but the allowlist declares %d — keep them in lockstep", len(legit), len(transportAllowlist))
	}
	for _, l := range legit {
		if err := checkTransportAllowlist(l.method, l.path); err != nil {
			t.Fatalf("%s (%s %s): expected pass, got %v", l.name, l.method, l.path, err)
		}
		// Query-string variant (GetWithHeadersValues appends ?k=v).
		if err := checkTransportAllowlist(l.method, l.path+"?maxResults=500&q=older_than%3A1y"); err != nil {
			t.Fatalf("%s with query: expected pass, got %v", l.name, err)
		}
		// Unresolved-template variant (generated commands substitute later).
		tpl := strings.Replace(l.path, "/users/me/", "/users/{userId}/", 1)
		if err := checkTransportAllowlist(l.method, tpl); err != nil {
			t.Fatalf("%s template form (%s): expected pass, got %v", l.name, tpl, err)
		}
	}
}

// TestTransportAllowlist_ChokePointRefusesWithoutDialing drives forged
// requests through the real client entry points against a live httptest
// server and asserts the server never sees a request, while a permitted
// operation on the same client passes through to the wire.
func TestTransportAllowlist_ChokePointRefusesWithoutDialing(t *testing.T) {
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	c := New(&config.Config{BaseURL: server.URL, AuthHeaderVal: "Bearer testtoken"}, time.Second, 0)
	c.HTTPClient = server.Client()
	c.NoCache = true
	ctx := context.Background()

	if _, _, err := c.Post(ctx, "/gmail/v1/users/me/messages/send", map[string]any{"raw": "x"}); err == nil {
		t.Fatal("POST .../messages/send must be refused")
	}
	if _, _, err := c.Post(ctx, "/gmail/v1/users/me/drafts", map[string]any{}); err == nil {
		t.Fatal("POST .../drafts must be refused")
	}
	if _, err := c.Get(ctx, "/gmail/v1/users/me/settings/filters", nil); err == nil {
		t.Fatal("GET .../settings/* must be refused")
	}
	if _, _, err := c.Delete(ctx, "/gmail/v1/users/me/messages/m1"); err == nil {
		t.Fatal("DELETE .../messages/{id} must be refused")
	}
	// Dry-run must not bypass the allowlist either.
	c.DryRun = true
	if _, _, err := c.Post(ctx, "/gmail/v1/users/me/messages/send", nil); err == nil {
		t.Fatal("dry-run POST .../messages/send must be refused")
	}
	c.DryRun = false
	if hits != 0 {
		t.Fatalf("refused requests reached the server %d times; want 0", hits)
	}

	// Control: a permitted operation flows through to the wire.
	if _, err := c.Get(ctx, "/gmail/v1/users/me/profile", nil); err != nil {
		t.Fatalf("permitted GET profile failed: %v", err)
	}
	if hits != 1 {
		t.Fatalf("permitted request should reach the server exactly once; got %d", hits)
	}
}

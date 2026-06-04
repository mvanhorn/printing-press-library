// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"net/http"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/config"
)

func TestClientRedirectDoesNotReAddAuthAfterCrossHostBounce(t *testing.T) {
	client := New(&config.Config{
		BaseURL:       "https://api.everbee.com",
		AuthHeaderVal: "test-token",
	}, time.Second, 0)

	req := mustRedirectRequest(t, "https://api.everbee.com/final")
	via := []*http.Request{
		mustRedirectRequest(t, "https://api.everbee.com/start"),
		mustRedirectRequest(t, "https://attacker.example/redirect"),
	}

	if err := client.HTTPClient.CheckRedirect(req, via); err != nil {
		t.Fatalf("CheckRedirect returned error: %v", err)
	}
	if got := req.Header.Get("x-access-token"); got != "" {
		t.Fatalf("x-access-token = %q, want empty after cross-host bounce", got)
	}
}

func mustRedirectRequest(t *testing.T, rawURL string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatalf("NewRequest(%q): %v", rawURL, err)
	}
	return req
}

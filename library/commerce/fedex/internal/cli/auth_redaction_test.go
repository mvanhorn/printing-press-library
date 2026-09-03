// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOAuthErrorDoesNotExposeUpstreamMessageOrCredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"transactionId":"sentinel-transaction","errors":[{"code":"AUTH.ERROR","message":"sentinel-secret sentinel-recipient"}]}`))
	}))
	t.Cleanup(server.Close)

	_, err := mintFedExToken(nil, server.Client(), server.URL, "sentinel-id", "sentinel-client-secret")
	if err == nil {
		t.Fatal("expected OAuth error")
	}
	for _, forbidden := range []string{"sentinel-secret", "sentinel-recipient", "sentinel-transaction", "sentinel-client-secret"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("OAuth error leaked %q: %s", forbidden, err)
		}
	}
	if !strings.Contains(err.Error(), "AUTH.ERROR") {
		t.Fatalf("OAuth error omitted safe code: %s", err)
	}
}

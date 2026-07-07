// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package firebase

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIKey_EnvOnly(t *testing.T) {
	// No key is bundled: unset env yields empty (auth login/refresh then fail
	// with a clear error; the set-token path does not need a key).
	t.Setenv("FUNCTION_HEALTH_FIREBASE_API_KEY", "")
	if got := APIKey(); got != "" {
		t.Errorf("unset APIKey: want empty, got %q", got)
	}
	t.Setenv("FUNCTION_HEALTH_FIREBASE_API_KEY", "override-key")
	if got := APIKey(); got != "override-key" {
		t.Errorf("override APIKey: want %q, got %q", "override-key", got)
	}
}

func TestSignInWithPassword_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "key=test-key") {
			t.Errorf("expected key query param; got %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"idToken":"id-abc","refreshToken":"rt-xyz","expiresIn":"3600","localId":"u1","email":"u@x.com"}`))
	}))
	defer srv.Close()
	c := &Client{HTTPClient: srv.Client(), APIKey: "test-key"}
	c.HTTPClient.Transport = nil
	// SignInWithPassword hardcodes the Google host, so just test the post() path
	// directly with a stub URL.
	var resp struct {
		IDToken string `json:"idToken"`
		Email   string `json:"email"`
	}
	if err := c.post(context.Background(), srv.URL+"?key=test-key", map[string]any{"email": "u@x.com"}, &resp); err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.IDToken != "id-abc" || resp.Email != "u@x.com" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestSignInWithPassword_ErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":{"code":400,"message":"INVALID_PASSWORD"}}`))
	}))
	defer srv.Close()
	c := &Client{HTTPClient: srv.Client(), APIKey: "k"}
	var resp struct{}
	err := c.post(context.Background(), srv.URL+"?key=k", map[string]any{}, &resp)
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T (%v)", err, err)
	}
	if apiErr.StatusCode != 400 || apiErr.Message != "INVALID_PASSWORD" {
		t.Errorf("unexpected APIError: %+v", apiErr)
	}
	if apiErr.IsRefreshable() {
		t.Errorf("INVALID_PASSWORD should not be IsRefreshable")
	}
}

func TestIsRefreshable(t *testing.T) {
	for _, msg := range []string{"TOKEN_EXPIRED", "API_KEY_INVALID", "USER_DISABLED", "INVALID_REFRESH_TOKEN", "USER_NOT_FOUND"} {
		if !((&APIError{Message: msg}).IsRefreshable()) {
			t.Errorf("%s should be IsRefreshable", msg)
		}
	}
	for _, msg := range []string{"INVALID_PASSWORD", "EMAIL_NOT_FOUND", ""} {
		if (&APIError{Message: msg}).IsRefreshable() {
			t.Errorf("%s should NOT be IsRefreshable", msg)
		}
	}
}

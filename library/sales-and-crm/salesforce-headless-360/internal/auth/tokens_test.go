package auth

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreAndLoadTokenUsesUserPrivateFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	ref := TokenRef{Service: "svc", Key: "prod"}
	expiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	if err := StoreToken(ref, StoredToken{AccessToken: "access", RefreshToken: "refresh", InstanceURL: "https://example", ExpiresAt: expiresAt}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".salesforce-headless-360-pp-cli", "tokens", "svc", "prod.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", info.Mode().Perm())
	}
	token, err := LoadStoredToken(ref)
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "access" || token.RefreshToken != "refresh" || !token.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("unexpected token: %+v", token)
	}
}

func TestRefreshTokenParsesStructuredResponse(t *testing.T) {
	client := &http.Client{Transport: tokenRoundTrip(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			return nil, err
		}
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "refresh" {
			t.Fatalf("unexpected form: %v", r.Form)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"access_token":"new-access","refresh_token":"new-refresh","instance_url":"https://example","expires_in":60}`))}, nil
	})}
	result, err := RefreshToken(context.Background(), client, "https://login.example/token", "client", "secret", "refresh")
	if err != nil {
		t.Fatal(err)
	}
	if result.AccessToken != "new-access" || result.RefreshToken != "new-refresh" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

type tokenRoundTrip func(*http.Request) (*http.Response, error)

func (rt tokenRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt(r)
}

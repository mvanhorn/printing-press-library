// Copyright 2026 cathryn-lavery. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/tiktok-shop/internal/config"
)

func TestSignCanonicalization(t *testing.T) {
	query := url.Values{}
	query.Set("timestamp", "1700000000")
	query.Set("sign", "must-not-be-signed")
	query.Set("shop_cipher", "test_shop_cipher")
	query.Set("access_token", "must-not-be-signed")
	query.Set("page_size", "50")
	query.Set("app_key", "test_app_key")

	got := Sign(
		"/product/202309/products/search",
		query,
		[]byte(`{"status":"ACTIVATE"}`),
		"test_secret",
		ContentTypeJSON,
	)
	const want = "09118b39179acfdb9879512ddc58e8a75459444aca19dcf02f21d8347bf5af8f"
	if got != want {
		t.Fatalf("Sign() = %s, want %s", got, want)
	}
}

func TestSignSkipsMultipartBody(t *testing.T) {
	query := url.Values{}
	query.Set("app_key", "test_app_key")
	query.Set("timestamp", "1700000000")

	withMultipart := Sign("/file/upload", query, []byte("body"), "test_secret", "multipart/form-data")
	withoutBody := Sign("/file/upload", query, nil, "test_secret", "multipart/form-data")
	if withMultipart != withoutBody {
		t.Fatalf("multipart signature included body: got %s, want %s", withMultipart, withoutBody)
	}
}

func TestDryRunRedactsSecrets(t *testing.T) {
	c := &Client{
		Config: &config.Config{
			AppKey:       "plain-app-key",
			AppSecret:    "plain-app-secret",
			AccessToken:  "plain-access-token",
			RefreshToken: "plain-refresh-token",
			ShopCipher:   "plain-shop-cipher",
		},
		DryRun: true,
	}

	openAPI, err := c.DoOpenAPI(context.Background(), "POST", "/product/202309/products/search", url.Values{"page_size": []string{"10"}}, map[string]any{"status": "ACTIVATE"})
	if err != nil {
		t.Fatalf("DoOpenAPI dry run: %v", err)
	}
	refresh, err := c.RefreshToken(context.Background())
	if err != nil {
		t.Fatalf("RefreshToken dry run: %v", err)
	}

	for _, raw := range []json.RawMessage{openAPI, refresh} {
		text := string(raw)
		for _, secret := range []string{
			"plain-app-key",
			"plain-app-secret",
			"plain-access-token",
			"plain-refresh-token",
			"plain-shop-cipher",
		} {
			if strings.Contains(text, secret) {
				t.Fatalf("dry-run output leaked %q in %s", secret, text)
			}
		}
	}

	var payload struct {
		Query map[string]string `json:"query"`
	}
	if err := json.Unmarshal(openAPI, &payload); err != nil {
		t.Fatalf("unmarshal openapi dry run: %v", err)
	}
	for _, key := range []string{"app_key", "shop_cipher", "sign"} {
		if payload.Query[key] != "[redacted]" {
			t.Fatalf("query[%s] = %q, want redacted", key, payload.Query[key])
		}
	}
	if _, ok := payload.Query["access_token"]; ok {
		t.Fatalf("dry-run output should not emit access_token query material")
	}

	if err := json.Unmarshal(refresh, &payload); err != nil {
		t.Fatalf("unmarshal refresh dry run: %v", err)
	}
	for _, key := range []string{"app_key", "app_secret", "refresh_token"} {
		if payload.Query[key] != "[redacted]" {
			t.Fatalf("query[%s] = %q, want redacted", key, payload.Query[key])
		}
	}
}

func TestAuthRequiredErrors(t *testing.T) {
	c := &Client{Config: &config.Config{}}

	err := c.RequireOpenAPIAuth(true)
	authErr, ok := err.(*AuthRequiredError)
	if !ok {
		t.Fatalf("RequireOpenAPIAuth() error = %T, want *AuthRequiredError", err)
	}
	wantMissing := []string{config.EnvAppKey, config.EnvAppSecret, config.EnvAccessToken, config.EnvShopCipher}
	if !equalStrings(authErr.Missing, wantMissing) {
		t.Fatalf("missing = %#v, want %#v", authErr.Missing, wantMissing)
	}

	err = c.RequireTokenRefreshAuth()
	authErr, ok = err.(*AuthRequiredError)
	if !ok {
		t.Fatalf("RequireTokenRefreshAuth() error = %T, want *AuthRequiredError", err)
	}
	wantMissing = []string{config.EnvAppKey, config.EnvAppSecret, config.EnvRefreshToken}
	if !equalStrings(authErr.Missing, wantMissing) {
		t.Fatalf("missing = %#v, want %#v", authErr.Missing, wantMissing)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func makeJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	encode := func(v any) string {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	return strings.Join([]string{encode(header), encode(claims), "signature"}, ".")
}

func TestDecodeJWTClaims_ValidExpiry(t *testing.T) {
	exp := time.Now().Add(45 * time.Minute).Unix()
	token := makeJWT(t, map[string]any{"exp": exp, "iat": time.Now().Unix()})
	claims, ok := DecodeJWTClaims(token)
	if !ok {
		t.Fatalf("expected ok=true for well-formed JWT")
	}
	if claims.ExpiresAt != exp {
		t.Fatalf("exp mismatch: got %d want %d", claims.ExpiresAt, exp)
	}
}

func TestDecodeJWTClaims_BearerPrefix(t *testing.T) {
	exp := time.Now().Add(10 * time.Minute).Unix()
	token := "Bearer " + makeJWT(t, map[string]any{"exp": exp})
	if _, ok := DecodeJWTClaims(token); !ok {
		t.Fatal("expected Bearer-prefixed JWT to decode")
	}
}

func TestDecodeJWTClaims_OpaqueToken(t *testing.T) {
	if _, ok := DecodeJWTClaims("opaque-not-a-jwt"); ok {
		t.Fatal("expected ok=false for opaque token")
	}
	if _, ok := DecodeJWTClaims("Bearer opaque-not-a-jwt"); ok {
		t.Fatal("expected ok=false for Bearer opaque token")
	}
}

func TestDecodeJWTClaims_MissingExp(t *testing.T) {
	token := makeJWT(t, map[string]any{"iat": time.Now().Unix()})
	if _, ok := DecodeJWTClaims(token); ok {
		t.Fatal("expected ok=false when exp claim is missing")
	}
}

func TestJWTExpiry_ExpiredToken(t *testing.T) {
	past := time.Now().Add(-2 * time.Hour).Unix()
	token := makeJWT(t, map[string]any{"exp": past})
	exp, ok := JWTExpiry(token)
	if !ok {
		t.Fatal("expected JWTExpiry to decode an expired-but-well-formed JWT")
	}
	if !exp.Before(time.Now()) {
		t.Fatalf("expected exp in the past, got %s", exp)
	}
}

func TestAuthHeaderExpiry_NilSafe(t *testing.T) {
	var c *Config
	if _, ok := c.AuthHeaderExpiry(); ok {
		t.Fatal("expected ok=false for nil Config")
	}
}

func TestAuthHeaderExpiry_FromAccessToken(t *testing.T) {
	exp := time.Now().Add(30 * time.Minute).Unix()
	token := makeJWT(t, map[string]any{"exp": exp})
	c := &Config{AccessToken: token}
	if _, ok := c.AuthHeaderExpiry(); !ok {
		t.Fatal("expected AuthHeaderExpiry to decode token from AccessToken field")
	}
}

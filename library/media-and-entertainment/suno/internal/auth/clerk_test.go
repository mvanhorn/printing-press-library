// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package auth

import (
	"encoding/base64"
	"strconv"
	"testing"
	"time"
)

// makeJWT builds a syntactically valid (unsigned) JWT whose payload carries the
// given exp claim, so the pure-logic decoder can be exercised without network.
func makeJWT(t *testing.T, payload string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	body := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return header + "." + body + ".sig"
}

func TestJWTExpiry(t *testing.T) {
	cases := []struct {
		name    string
		jwt     string
		wantExp int64
		wantErr bool
	}{
		{
			name:    "valid exp claim",
			jwt:     makeJWT(t, `{"exp":1893456000}`),
			wantExp: 1893456000,
		},
		{
			name: "Bearer prefix is stripped",
			jwt:  "Bearer " + makeJWT(t, `{"exp":1893456000}`),
			// same payload, prefix tolerated
			wantExp: 1893456000,
		},
		{
			name:    "missing exp claim",
			jwt:     makeJWT(t, `{"sub":"user_123"}`),
			wantErr: true,
		},
		{
			name:    "not a JWT (wrong segment count)",
			jwt:     "abc.def",
			wantErr: true,
		},
		{
			name:    "undecodable payload segment",
			jwt:     "aaa.!!!notbase64!!!.ccc",
			wantErr: true,
		},
		{
			name:    "empty string",
			jwt:     "",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := JWTExpiry(tc.jwt)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got exp=%d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantExp {
				t.Fatalf("exp = %d, want %d", got, tc.wantExp)
			}
		})
	}
}

func TestJWTNeedsRefresh(t *testing.T) {
	future := time.Now().Add(2 * time.Hour).Unix()
	soon := time.Now().Add(10 * time.Minute).Unix() // inside the 30m skew window
	past := time.Now().Add(-time.Hour).Unix()

	cases := []struct {
		name string
		jwt  string
		want bool
	}{
		{name: "fresh token (>30m out)", jwt: makeJWT(t, jsonExp(future)), want: false},
		{name: "expiring within skew window", jwt: makeJWT(t, jsonExp(soon)), want: true},
		{name: "already expired", jwt: makeJWT(t, jsonExp(past)), want: true},
		{name: "undecodable token treated as needs-refresh", jwt: "garbage", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := JWTNeedsRefresh(tc.jwt); got != tc.want {
				t.Fatalf("JWTNeedsRefresh = %v, want %v", got, tc.want)
			}
		})
	}
}

func jsonExp(exp int64) string {
	return `{"exp":` + strconv.FormatInt(exp, 10) + `}`
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Fatalf("truncate short = %q, want %q", got, "hello")
	}
	if got := truncate("hello world", 5); got != "hello..." {
		t.Fatalf("truncate long = %q, want %q", got, "hello...")
	}
}

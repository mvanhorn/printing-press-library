// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package dreoauth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogin_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/oauth/login" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		// Verify the canonical Dreo-shaped fields were sent
		if req["grant_type"] != "email-password" {
			t.Errorf("grant_type=%v want email-password", req["grant_type"])
		}
		if req["encrypt"] != "ciphertext" {
			t.Errorf("missing encrypt=ciphertext")
		}
		if req["client_id"] == nil || req["client_secret"] == nil {
			t.Errorf("missing client credentials")
		}
		// Password should be hashed (32 hex chars), not plaintext
		pwd, _ := req["password"].(string)
		if len(pwd) != 32 {
			t.Errorf("password not MD5-hex (len=%d, want 32): %q", len(pwd), pwd)
		}
		if pwd == "plaintext" {
			t.Errorf("password sent in plaintext")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"msg":"OK","data":{"access_token":"tok-abc","region":"NA","expiresIn":3600}}`))
	}))
	defer srv.Close()

	resp, err := Login(context.Background(), srv.URL, "user@example.com", "plaintext")
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	if resp.AccessToken != "tok-abc" {
		t.Errorf("AccessToken=%q want tok-abc", resp.AccessToken)
	}
	if resp.Region != "NA" {
		t.Errorf("Region=%q want NA", resp.Region)
	}
}

func TestLogin_BadCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":4001,"msg":"invalid email or password"}`))
	}))
	defer srv.Close()
	_, err := Login(context.Background(), srv.URL, "bad@example.com", "wrong")
	if err == nil {
		t.Fatal("expected error for bad credentials")
	}
	if !strings.Contains(err.Error(), "invalid email or password") {
		t.Errorf("error missing server msg: %v", err)
	}
}

func TestHostFromBase(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://app-api-us.dreo-tech.com", "app-api-us.dreo-tech.com"},
		{"https://app-api-eu.dreo-tech.com/", "app-api-eu.dreo-tech.com"},
		{"https://app-api-us.dreo-tech.com/api/oauth/login", "app-api-us.dreo-tech.com"},
		{"", ""},
	}
	for _, tc := range cases {
		got := hostFromBase(tc.in)
		if got != tc.want {
			t.Errorf("hostFromBase(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	got := truncate([]byte("hello world"), 5)
	if !strings.HasPrefix(got, "hello") {
		t.Errorf("truncate=%q want hello prefix", got)
	}
	got = truncate([]byte("hi"), 10)
	if got != "hi" {
		t.Errorf("truncate=%q want hi (no overflow)", got)
	}
}

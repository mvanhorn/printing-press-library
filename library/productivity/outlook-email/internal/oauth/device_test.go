package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClientDefaults(t *testing.T) {
	c := NewClient("", "", nil)
	if c.Authority != DefaultAuthority {
		t.Fatalf("authority = %q, want %q", c.Authority, DefaultAuthority)
	}
	if c.ClientID != DefaultClientID {
		t.Fatalf("client_id = %q, want %q", c.ClientID, DefaultClientID)
	}
	if len(c.Scopes) != len(DefaultScopes) {
		t.Fatalf("default scopes lost: %v", c.Scopes)
	}
}

func TestRequestDeviceCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/oauth2/v2.0/devicecode") {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = r.ParseForm()
		if r.Form.Get("client_id") != "abc" {
			t.Fatalf("client_id form value = %q", r.Form.Get("client_id"))
		}
		if !strings.Contains(r.Form.Get("scope"), "Mail.ReadWrite") {
			t.Fatalf("scope missing Mail.ReadWrite: %q", r.Form.Get("scope"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DeviceCodeResponse{
			UserCode:        "ABCD-EFGH",
			DeviceCode:      "device-code-123",
			VerificationURI: "https://microsoft.com/devicelogin",
			ExpiresIn:       900,
			Interval:        0, // exercise the default-fill-in path
			Message:         "go to ...",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "abc", DefaultScopes)
	dc, err := c.RequestDeviceCode(context.Background())
	if err != nil {
		t.Fatalf("RequestDeviceCode: %v", err)
	}
	if dc.UserCode != "ABCD-EFGH" {
		t.Fatalf("user_code = %q", dc.UserCode)
	}
	if dc.Interval != 5 {
		t.Fatalf("expected default interval 5, got %d", dc.Interval)
	}
}

func TestRefreshTokenReusesRefreshIfOmitted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "AT2",
			"expires_in":   3600,
			"scope":        "Mail.ReadWrite User.Read offline_access",
			// no refresh_token in this response
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "abc", DefaultScopes)
	tr, err := c.RefreshToken(context.Background(), "RT-original")
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if tr.AccessToken != "AT2" {
		t.Fatalf("access_token = %q", tr.AccessToken)
	}
	if tr.RefreshToken != "RT-original" {
		t.Fatalf("refresh token should be preserved when omitted, got %q", tr.RefreshToken)
	}
	if tr.ExpiryTime.Before(time.Now()) {
		t.Fatalf("expiry should be in the future")
	}
}

func TestRefreshTokenRejectsEmpty(t *testing.T) {
	c := NewClient("", "", nil)
	if _, err := c.RefreshToken(context.Background(), ""); err == nil {
		t.Fatal("expected error on empty refresh token")
	}
}

func TestRefreshTokenSurfacesAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "AADSTS70008: refresh token expired",
		})
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "abc", DefaultScopes)
	_, err := c.RefreshToken(context.Background(), "stale-refresh")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("error should mention invalid_grant: %v", err)
	}
}

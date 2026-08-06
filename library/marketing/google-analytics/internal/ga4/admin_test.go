package ga4

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestCredentialsKind(t *testing.T) {
	cases := []struct {
		name string
		c    Credentials
		want string
	}{
		{"service_account by key fields", Credentials{PrivateKey: "pk", ClientEmail: "e@x.com"}, "service_account"},
		{"authorized_user by fields", Credentials{ClientID: "c", ClientSecret: "s", RefreshToken: "r"}, "authorized_user"},
		{"empty", Credentials{}, ""},
		{"malformed (client id only)", Credentials{ClientID: "c"}, ""},
		{"malformed (refresh only)", Credentials{RefreshToken: "r"}, ""},
		{"explicit service_account wins", Credentials{Type: "service_account"}, "service_account"},
		{"explicit authorized_user wins", Credentials{Type: "authorized_user"}, "authorized_user"},
		{"unknown type with no fields", Credentials{Type: "weird"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.c.Kind(); got != tc.want {
				t.Fatalf("Kind()=%q want %q", got, tc.want)
			}
		})
	}
}

func TestCredentialsUnmarshalRealisticJSON(t *testing.T) {
	// Service-account key payload. The private key is deliberately fake, not a real PEM.
	sa := `{"type":"service_account","project_id":"analytics-123","private_key":"-----BEGIN PRIVATE KEY-----\nFAKE_KEY_MATERIAL_NOT_A_REAL_KEY\n-----END PRIVATE KEY-----\n","client_email":"svc@analytics-123.iam.gserviceaccount.com","token_uri":"https://oauth2.googleapis.com/token"}`
	var a Credentials
	if err := json.Unmarshal([]byte(sa), &a); err != nil {
		t.Fatal(err)
	}
	if a.Kind() != "service_account" {
		t.Fatalf("service_account Kind()=%q", a.Kind())
	}
	if a.ClientEmail == "" || a.PrivateKey == "" {
		t.Fatalf("service_account fields not decoded: %#v", a)
	}

	// ADC authorized_user payload.
	au := `{"type":"authorized_user","client_id":"123.apps.googleusercontent.com","client_secret":"d-very_secret","refresh_token":"1//fake_refresh_token","quota_project_id":"analytics-123"}`
	var b Credentials
	if err := json.Unmarshal([]byte(au), &b); err != nil {
		t.Fatal(err)
	}
	if b.Kind() != "authorized_user" {
		t.Fatalf("authorized_user Kind()=%q", b.Kind())
	}
	if b.ClientSecret == "" || b.RefreshToken == "" {
		t.Fatalf("authorized_user fields not decoded: %#v", b)
	}
}

func TestAdminURLUpdateMask(t *testing.T) {
	c := NewClient("tok", time.Second)
	c.AdminBase = "https://analyticsadmin.googleapis.com/v1beta"
	c.AdminAlphaBase = "https://analyticsadmin.googleapis.com/v1alpha"

	// No query on the path -> "?" separator.
	if got := c.adminURL("v1beta", "properties/123/keyEvents", "countingMethod"); got != "https://analyticsadmin.googleapis.com/v1beta/properties/123/keyEvents?updateMask=countingMethod" {
		t.Fatalf("no-query case: %q", got)
	}
	// Path already contains "?" -> "&" separator.
	if got := c.adminURL("v1beta", "properties/123/keyEvents?foo=bar", "countingMethod"); got != "https://analyticsadmin.googleapis.com/v1beta/properties/123/keyEvents?foo=bar&updateMask=countingMethod" {
		t.Fatalf("existing-query case: %q", got)
	}
	// Mask is URL-escaped.
	if got := c.adminURL("v1beta", "x", "a.b,c"); got != "https://analyticsadmin.googleapis.com/v1beta/x?updateMask="+url.QueryEscape("a.b,c") {
		t.Fatalf("escape case: %q", got)
	}
	// Empty mask -> omitted entirely.
	if got := c.adminURL("v1beta", "x", ""); got != "https://analyticsadmin.googleapis.com/v1beta/x" {
		t.Fatalf("empty-mask case: %q", got)
	}
	// Leading slash tolerated.
	if got := c.adminURL("v1beta", "/properties/123", ""); got != "https://analyticsadmin.googleapis.com/v1beta/properties/123" {
		t.Fatalf("leading-slash case: %q", got)
	}
	// v1alpha selects the alpha host.
	if got := c.adminURL("v1alpha", "properties/123/accessBindings", ""); got != "https://analyticsadmin.googleapis.com/v1alpha/properties/123/accessBindings" {
		t.Fatalf("v1alpha case: %q", got)
	}
	// Any non-v1alpha value resolves to the beta host via adminBase.
	if got := c.adminURL("v1gamma", "properties/123", ""); got != "https://analyticsadmin.googleapis.com/v1beta/properties/123" {
		t.Fatalf("fallback case: %q", got)
	}
}

func TestValidAdminAPI(t *testing.T) {
	for _, api := range []string{"v1beta", "v1alpha"} {
		if !ValidAdminAPI(api) {
			t.Fatalf("ValidAdminAPI(%q)=false", api)
		}
	}
	for _, api := range []string{"", "v1", "v2beta", "V1BETA", "v1alpha2"} {
		if ValidAdminAPI(api) {
			t.Fatalf("ValidAdminAPI(%q)=true", api)
		}
	}
}

func TestAdminListMergesPages(t *testing.T) {
	seenTokens := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenTokens = append(seenTokens, r.URL.Query().Get("pageToken"))
		if r.URL.Query().Get("pageToken") == "next" {
			_, _ = w.Write([]byte(`{"keyEvents":[{"name":"properties/1/keyEvents/ke2"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"keyEvents":[{"name":"properties/1/keyEvents/ke1"}],"nextPageToken":"next"}`))
	}))
	defer srv.Close()
	c := NewClient("tok", time.Second)
	c.AdminBase = srv.URL
	out, st, err := c.AdminList(context.Background(), "v1beta", "properties/1/keyEvents")
	if err != nil || st != 200 {
		t.Fatalf("%d %v", st, err)
	}
	arr, ok := out["keyEvents"].([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("merged keyEvents=%#v", out["keyEvents"])
	}
	if len(seenTokens) != 2 || seenTokens[1] != "next" {
		t.Fatalf("pagination tokens=%#v", seenTokens)
	}
}

func TestAdminListReturnsEmptyNonNilList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"dataStreams":[]}`))
	}))
	defer srv.Close()
	c := NewClient("tok", time.Second)
	c.AdminBase = srv.URL
	out, _, err := c.AdminList(context.Background(), "v1beta", "properties/1/dataStreams")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := out["dataStreams"].([]any)
	if !ok || arr == nil || len(arr) != 0 {
		t.Fatalf("expected non-nil empty list, got %#v (ok=%v)", out["dataStreams"], ok)
	}
}

func TestAdminListPassesThroughRawPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"weirdField":"value","nested":{"k":1}}`))
	}))
	defer srv.Close()
	c := NewClient("tok", time.Second)
	c.AdminBase = srv.URL
	out, _, err := c.AdminList(context.Background(), "v1beta", "properties/1/somethingUnknown")
	if err != nil {
		t.Fatal(err)
	}
	if out["weirdField"] != "value" {
		t.Fatalf("raw page not passed through: %#v", out)
	}
	// No list key should have been synthesized.
	for _, k := range AdminListKeys {
		if _, ok := out[k]; ok {
			t.Fatalf("unexpected synthesized list key %q in %#v", k, out)
		}
	}
}

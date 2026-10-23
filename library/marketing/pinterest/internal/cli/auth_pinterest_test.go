package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPinterestAuthorizationCodeTokenRequest(t *testing.T) {
	const clientID, secret = "test-client", "test-secret"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, gotSecret, ok := r.BasicAuth()
		if !ok || id != clientID || gotSecret != secret {
			t.Errorf("Basic auth = %q, %q, %v", id, gotSecret, ok)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		if r.Form.Has("client_secret") {
			t.Error("client_secret leaked into token form")
		}
		if r.Form.Get("continuous_refresh") != "true" {
			t.Errorf("continuous_refresh = %q", r.Form.Get("continuous_refresh"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"ok"}`))
	})
	var requestURL string
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requestURL = r.URL.String()
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, r)
		return recorder.Result(), nil
	})}

	got, err := exchangeAuthorizationCode(context.Background(), httpClient, "https://api.example/oauth/token", clientID, secret, url.Values{
		"grant_type": {"authorization_code"}, "continuous_refresh": {"true"},
	})
	if err != nil || got.AccessToken != "ok" {
		t.Fatalf("exchangeAuthorizationCode() = %#v, %v", got, err)
	}
	if requestURL != "https://api.example/oauth/token" {
		t.Fatalf("request URL = %q", requestURL)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPinterestOAuthRedirectURI(t *testing.T) {
	if got := pinterestOAuthRedirectURI(8875); got != "http://localhost:8875/callback" {
		t.Fatalf("redirect URI = %q", got)
	}
}

func TestPinterestLoginScopes(t *testing.T) {
	run := func(args ...string) string {
		cmd := newAuthLoginCmd(&rootFlags{dryRun: true})
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs(append([]string{"--client-id", "x", "--client-secret", "y"}, args...))
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		line := strings.TrimPrefix(strings.TrimSpace(out.String()), "would launch: ")
		u, err := url.Parse(line)
		if err != nil {
			t.Fatal(err)
		}
		return u.Query().Get("scope")
	}
	t.Setenv("PINTEREST_SCOPES", "")
	if got := run(); strings.Contains(got, "msot") || !strings.Contains(got, "pins:read") {
		t.Fatalf("default scopes = %q", got)
	}
	if got := run("--scopes", "pins:read,boards:read"); got != "pins:read boards:read" {
		t.Fatalf("override scopes = %q", got)
	}
	if got := run("--scopes", "basic,msot:write"); strings.Contains(got, "ads:") || !strings.HasSuffix(got, " msot:write") {
		t.Fatalf("basic scopes = %q", got)
	}
	t.Setenv("PINTEREST_SCOPES", "pins:read user_accounts:read")
	if got := run(); got != "pins:read user_accounts:read" {
		t.Fatalf("env scopes = %q", got)
	}
}

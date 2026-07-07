// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestOAuth2AuthorizeURLUsesPKCEAndScopes(t *testing.T) {
	authURL, err := buildOAuth2AuthorizeURL(oauth2AuthorizeOptions{
		ClientID:    "client-id",
		RedirectURI: "http://127.0.0.1:8787/callback",
		Scopes:      []string{"tweet.read", "users.read", "offline.access"},
		State:       "state-123",
		Verifier:    "verifier-123",
	})
	if err != nil {
		t.Fatalf("buildOAuth2AuthorizeURL: %v", err)
	}
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}
	q := u.Query()
	if u.Scheme != "https" || u.Host != "twitter.com" || u.Path != "/i/oauth2/authorize" {
		t.Fatalf("unexpected authorize URL: %s", authURL)
	}
	checks := map[string]string{
		"response_type":         "code",
		"client_id":             "client-id",
		"redirect_uri":          "http://127.0.0.1:8787/callback",
		"scope":                 "tweet.read users.read offline.access",
		"state":                 "state-123",
		"code_challenge":        pkceChallenge("verifier-123"),
		"code_challenge_method": "S256",
	}
	for key, want := range checks {
		if got := q.Get(key); got != want {
			t.Fatalf("%s = %q, want %q in %s", key, got, want, authURL)
		}
	}
}

func TestOAuth2CallbackHandlerValidatesState(t *testing.T) {
	ch := make(chan oauth2CallbackResult, 1)
	h := oauth2CallbackHandler("/callback", "expected-state", ch)
	req := httptest.NewRequest(http.MethodGet, "/callback?code=code-123&state=wrong-state", nil)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadRequest)
	}
	result := <-ch
	if result.Error != "state_mismatch" {
		t.Fatalf("result = %#v", result)
	}
}

func TestAuthOAuth2LoginRunsLoopbackPKCEFlowAndSavesToken(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")

	var sawTokenExchange atomic.Bool
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawTokenExchange.Store(true)
		if r.Method != http.MethodPost {
			t.Fatalf("token method = %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse token form: %v", err)
		}
		want := map[string]string{
			"grant_type":    "authorization_code",
			"client_id":     "client-id",
			"code":          "code-from-x",
			"code_verifier": "test-verifier",
		}
		for key, expected := range want {
			if got := r.Form.Get(key); got != expected {
				t.Fatalf("token form %s = %q, want %q; form=%v", key, got, expected, r.Form)
			}
		}
		if !strings.HasPrefix(r.Form.Get("redirect_uri"), "http://127.0.0.1:") {
			t.Fatalf("redirect_uri = %q", r.Form.Get("redirect_uri"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"token_type":"bearer","expires_in":7200,"access_token":"oauth-access-token","refresh_token":"refresh-token","scope":"tweet.read users.read offline.access"}`)
	}))
	defer tokenServer.Close()

	redirectURI := "http://127.0.0.1:" + freeTCPPort(t) + "/callback"
	oldOpen := oauth2OpenBrowser
	oauth2OpenBrowser = func(authURL string) error {
		u, err := url.Parse(authURL)
		if err != nil {
			return err
		}
		q := u.Query()
		if q.Get("state") != "test-state" {
			t.Fatalf("state = %q", q.Get("state"))
		}
		if q.Get("code_challenge") != pkceChallenge("test-verifier") {
			t.Fatalf("code_challenge = %q", q.Get("code_challenge"))
		}
		callbackURL := q.Get("redirect_uri") + "?code=code-from-x&state=" + url.QueryEscape(q.Get("state"))
		resp, err := http.Get(callbackURL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return io.ErrUnexpectedEOF
		}
		return nil
	}
	defer func() { oauth2OpenBrowser = oldOpen }()

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--config", configPath,
		"auth", "oauth2-login",
		"--client-id", "client-id",
		"--redirect-uri", redirectURI,
		"--scopes", "tweet.read,users.read,offline.access",
		"--token-url", tokenServer.URL,
		"--oauth2-state", "test-state",
		"--pkce-verifier", "test-verifier",
		"--timeout", "5s",
		"--json",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("auth oauth2-login failed: %v\noutput: %s", err, out.String())
	}
	if !sawTokenExchange.Load() {
		t.Fatalf("token endpoint was not called")
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if payload["auth_lane"] != "oauth2_user_context" || payload["saved"] != true || payload["refresh_token_present"] != true {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestAuthOAuth2LoginRejectsJSONNoOpen(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{
		"--config", configPath,
		"auth", "oauth2-login",
		"--client-id", "client-id",
		"--json",
		"--no-open",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("auth oauth2-login --json --no-open succeeded unexpectedly; output: %s", out.String())
	}
	if !strings.Contains(err.Error(), "--json cannot be combined with --no-open") {
		t.Fatalf("error = %q, want --json/--no-open guidance", err.Error())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty stdout on usage error", out.String())
	}
}

func freeTCPPort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free port: %v", err)
	}
	defer ln.Close()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split addr: %v", err)
	}
	return port
}

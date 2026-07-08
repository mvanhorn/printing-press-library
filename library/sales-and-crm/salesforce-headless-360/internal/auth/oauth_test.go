package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestPKCEChallengeS256(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if got := CodeChallengeS256(verifier); got != want {
		t.Fatalf("challenge = %q, want %q", got, want)
	}
	generated, err := GeneratePKCEVerifier()
	if err != nil {
		t.Fatal(err)
	}
	if len(generated) < 43 || len(generated) > 128 {
		t.Fatalf("verifier length = %d", len(generated))
	}
}

func TestOAuthAuthorizeURLAndExchangeHappyPath(t *testing.T) {
	var sawVerifier bool
	client := &http.Client{Transport: oauthRoundTrip(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			return nil, err
		}
		if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "valid-code" {
			t.Fatalf("unexpected token request: %v", r.Form)
		}
		verifier := r.Form.Get("code_verifier")
		if verifier == "" {
			t.Fatal("missing verifier")
		}
		sawVerifier = true
		return oauthJSONResponse(`{"access_token":"web-access","refresh_token":"web-refresh","instance_url":"https://web.example","scope":"refresh_token api","token_type":"Bearer","expires_in":3600}`), nil
	})}

	authorizeURL, err := BuildAuthorizeURL("https://login.example/services/oauth2/authorize", "client-123", "http://127.0.0.1:1234/callback", "verifier-abcdefghijklmnopqrstuvwxyz1234567890", "state")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(authorizeURL)
	q := u.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		t.Fatalf("missing PKCE params: %s", authorizeURL)
	}
	result, err := ExchangeAuthorizationCode(context.Background(), client, "https://login.example/services/oauth2/token", "client-123", "", "http://127.0.0.1:1234/callback", "valid-code", "verifier-abcdefghijklmnopqrstuvwxyz1234567890")
	if err != nil {
		t.Fatal(err)
	}
	if !sawVerifier || result.AccessToken != "web-access" || result.AuthMethod != MethodOAuthWeb {
		t.Fatalf("unexpected result: %+v sawVerifier=%v", result, sawVerifier)
	}
}

func TestExchangeAuthorizationCodeRequiresAPIScope(t *testing.T) {
	client := &http.Client{Transport: oauthRoundTrip(func(r *http.Request) (*http.Response, error) {
		return oauthJSONResponse(`{"access_token":"web-access","scope":"refresh_token","token_type":"Bearer"}`), nil
	})}
	_, err := ExchangeAuthorizationCode(context.Background(), client, "https://login.example/token", "client", "", "http://127.0.0.1/callback", "code", "verifier")
	if err == nil || !strings.Contains(err.Error(), "Connected App missing scope: api") {
		t.Fatalf("expected scope error, got %v", err)
	}
}

func TestBuildAuthorizeURLRequiresVerifier(t *testing.T) {
	_, err := BuildAuthorizeURL("", "client", "http://127.0.0.1/callback", "", "state")
	if err == nil || !strings.Contains(err.Error(), "PKCE verifier missing") {
		t.Fatalf("expected verifier error, got %v", err)
	}
}

type oauthRoundTrip func(*http.Request) (*http.Response, error)

func (rt oauthRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt(r)
}

func oauthJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestCodeChallengeMatchesSHA256(t *testing.T) {
	verifier := "abc123abc123abc123abc123abc123abc123abc123abc"
	sum := sha256.Sum256([]byte(verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if got := CodeChallengeS256(verifier); got != want {
		t.Fatalf("challenge = %q, want %q", got, want)
	}
}

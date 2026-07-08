package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
)

const defaultAuthURL = "https://login.salesforce.com/services/oauth2/authorize"

type OAuthOptions struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
	OpenBrowser  func(string) error
	Loopback     LoopbackOptions
}

func GeneratePKCEVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating PKCE verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func CodeChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating OAuth state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func BuildAuthorizeURL(authURL, clientID, redirectURI, verifier, state string) (string, error) {
	if verifier == "" {
		return "", &Error{Kind: "oauth_pkce", Message: "PKCE verifier missing"}
	}
	if state == "" {
		return "", &Error{Kind: "oauth_state", Message: "state is required"}
	}
	if authURL == "" {
		authURL = defaultAuthURL
	}
	u, err := url.Parse(authURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", "api refresh_token offline_access")
	q.Set("code_challenge", CodeChallengeS256(verifier))
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func LoginOAuthWeb(ctx context.Context, opts OAuthOptions) (*Result, error) {
	if opts.ClientID == "" {
		return nil, &Error{Kind: "oauth_client", Message: "--client-id is required"}
	}
	verifier, err := GeneratePKCEVerifier()
	if err != nil {
		return nil, err
	}
	state, err := GenerateState()
	if err != nil {
		return nil, err
	}
	opts.Loopback.State = state
	server, err := StartLoopback(ctx, opts.Loopback)
	if err != nil {
		return nil, err
	}
	defer server.Close()
	authURL, err := BuildAuthorizeURL(opts.AuthURL, opts.ClientID, server.RedirectURI, verifier, state)
	if err != nil {
		return nil, err
	}
	open := opts.OpenBrowser
	if open == nil {
		open = OpenBrowser
	}
	if err := open(authURL); err != nil {
		return nil, err
	}
	var code string
	select {
	case result := <-server.Results:
		code = result.Code
	case err := <-server.Errors:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if verifier == "" {
		return nil, &Error{Kind: "oauth_pkce", Message: "PKCE verifier missing"}
	}
	tokenURL := opts.TokenURL
	if tokenURL == "" {
		tokenURL = defaultTokenURL
	}
	values := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {server.RedirectURI},
		"client_id":     {opts.ClientID},
		"code_verifier": {verifier},
	}
	if opts.ClientSecret != "" {
		values.Set("client_secret", opts.ClientSecret)
	}
	resp, err := postForm(ctx, opts.HTTPClient, tokenURL, values)
	if err != nil {
		return nil, &Error{Kind: "token_http", Message: "exchanging authorization code", Err: err}
	}
	result, err := parseTokenHTTPResponse(resp, MethodOAuthWeb)
	if err != nil {
		return nil, err
	}
	if !scopeContains(result.Scope, "api") {
		return nil, &Error{Kind: "oauth_scope", Message: "Connected App missing scope: api", Hint: "add the api OAuth scope to the Connected App"}
	}
	return result, nil
}

func ExchangeAuthorizationCode(ctx context.Context, hc *http.Client, tokenURL, clientID, clientSecret, redirectURI, code, verifier string) (*Result, error) {
	if verifier == "" {
		return nil, &Error{Kind: "oauth_pkce", Message: "PKCE verifier missing"}
	}
	values := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"code_verifier": {verifier},
	}
	if clientSecret != "" {
		values.Set("client_secret", clientSecret)
	}
	resp, err := postForm(ctx, hc, tokenURL, values)
	if err != nil {
		return nil, &Error{Kind: "token_http", Message: "exchanging authorization code", Err: err}
	}
	result, err := parseTokenHTTPResponse(resp, MethodOAuthWeb)
	if err != nil {
		return nil, err
	}
	if !scopeContains(result.Scope, "api") {
		return nil, &Error{Kind: "oauth_scope", Message: "Connected App missing scope: api", Hint: "add the api OAuth scope to the Connected App"}
	}
	return result, nil
}

func scopeContains(scope, want string) bool {
	for _, item := range strings.Fields(scope) {
		if item == want {
			return true
		}
	}
	return false
}

func OpenBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "linux":
		cmd = exec.Command("xdg-open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		return nil
	}
	return cmd.Start()
}

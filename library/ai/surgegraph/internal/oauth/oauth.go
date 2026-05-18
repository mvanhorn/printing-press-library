// Package oauth implements the SurgeGraph OAuth 2.1 client used by
// `auth login --browser`: Authorization Code + PKCE + Dynamic Client
// Registration against https://mcp.surgegraph.io. No third-party deps —
// stdlib only, so the binary stays ~50 MB and trivially cross-compiles.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Metadata is the subset of the OAuth 2.1 discovery document we use.
type Metadata struct {
	Issuer                 string   `json:"issuer"`
	AuthorizationEndpoint  string   `json:"authorization_endpoint"`
	TokenEndpoint          string   `json:"token_endpoint"`
	RegistrationEndpoint   string   `json:"registration_endpoint"`
	GrantTypesSupported    []string `json:"grant_types_supported"`
	CodeChallengeMethods   []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthTypes []string `json:"token_endpoint_auth_methods_supported"`
	ScopesSupported        []string `json:"scopes_supported"`
}

// Client is a registered OAuth client (the printed CLI registers itself via DCR).
type Client struct {
	ClientID         string `json:"client_id"`
	ClientSecret     string `json:"client_secret,omitempty"`
	RegistrationDate string `json:"registration_date,omitempty"`
}

// Tokens is the result of a successful token exchange.
type Tokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in,omitempty"`
	Scope        string    `json:"scope,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

// DiscoveryURL builds the well-known URL from a base issuer URL.
func DiscoveryURL(baseURL string) string {
	base := strings.TrimRight(baseURL, "/")
	return base + "/.well-known/oauth-authorization-server"
}

// Discover fetches and parses the OAuth 2.1 metadata document.
func Discover(ctx context.Context, baseURL string) (*Metadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DiscoveryURL(baseURL), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching discovery doc: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("discovery %s: HTTP %d: %s", baseURL, resp.StatusCode, truncate(string(body), 200))
	}
	var m Metadata
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("decoding discovery doc: %w", err)
	}
	if m.AuthorizationEndpoint == "" || m.TokenEndpoint == "" {
		return nil, fmt.Errorf("discovery doc missing authorization_endpoint or token_endpoint")
	}
	return &m, nil
}

// Register performs Dynamic Client Registration (RFC 7591) at the issuer's
// registration_endpoint. redirectURIs is the list the registered client may
// use. clientName is the human-friendly name shown on the consent screen.
func Register(ctx context.Context, m *Metadata, clientName string, redirectURIs []string) (*Client, error) {
	if m.RegistrationEndpoint == "" {
		return nil, errors.New("server has no registration_endpoint; DCR not supported")
	}
	body := map[string]any{
		"client_name":                clientName,
		"redirect_uris":              redirectURIs,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none", // public client (PKCE)
		"application_type":           "native",
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.RegistrationEndpoint, strings.NewReader(string(buf)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("registering client: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registration %s: HTTP %d: %s", m.RegistrationEndpoint, resp.StatusCode, truncate(string(b), 200))
	}
	var c Client
	if err := json.NewDecoder(resp.Body).Decode(&c); err != nil {
		return nil, fmt.Errorf("decoding registration response: %w", err)
	}
	if c.ClientID == "" {
		return nil, errors.New("registration response missing client_id")
	}
	c.RegistrationDate = time.Now().UTC().Format(time.RFC3339)
	return &c, nil
}

// PKCE holds the verifier/challenge pair generated for one login attempt.
type PKCE struct {
	Verifier  string
	Challenge string
	Method    string // "S256"
}

// NewPKCE generates a fresh 32-byte verifier and the SHA-256 challenge.
func NewPKCE() (PKCE, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return PKCE{}, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(verifier))
	return PKCE{
		Verifier:  verifier,
		Challenge: base64.RawURLEncoding.EncodeToString(sum[:]),
		Method:    "S256",
	}, nil
}

// LoginOptions configures the browser flow.
type LoginOptions struct {
	BaseURL    string // OAuth issuer (e.g., https://mcp.surgegraph.io)
	ClientName string // displayed on consent screen
	Scopes     []string
	// Browser opens the given URL when non-nil. Nil disables auto-launch
	// (callers print the URL instead).
	Browser func(string) error
	// Stdout receives short progress lines.
	Stdout io.Writer
	// Timeout for the whole flow. Defaults to 5 minutes.
	Timeout time.Duration
}

// Result is what Login returns to the caller.
type Result struct {
	Metadata *Metadata
	Client   *Client
	Tokens   *Tokens
}

// Login runs the entire Authorization Code + PKCE + DCR flow:
// discovery -> registration -> open browser -> capture code at local callback ->
// exchange for tokens. Caller is responsible for persisting result.Client and
// result.Tokens.
func Login(ctx context.Context, opts LoginOptions) (*Result, error) {
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Minute
	}
	if opts.BaseURL == "" {
		return nil, errors.New("LoginOptions.BaseURL is required")
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	fmt.Fprintln(opts.Stdout, "→ Fetching OAuth discovery doc...")
	meta, err := Discover(ctx, opts.BaseURL)
	if err != nil {
		return nil, err
	}

	// Bind to a fresh ephemeral local port for the callback.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("opening local callback listener: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	clientName := opts.ClientName
	if clientName == "" {
		clientName = "surgegraph-pp-cli"
	}

	fmt.Fprintln(opts.Stdout, "→ Registering OAuth client (Dynamic Client Registration)...")
	registered, err := Register(ctx, meta, clientName, []string{redirectURI})
	if err != nil {
		_ = listener.Close()
		return nil, err
	}

	pkce, err := NewPKCE()
	if err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("generating PKCE: %w", err)
	}

	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		_ = listener.Close()
		return nil, err
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	// Build authorization URL.
	authParams := url.Values{}
	authParams.Set("response_type", "code")
	authParams.Set("client_id", registered.ClientID)
	authParams.Set("redirect_uri", redirectURI)
	authParams.Set("code_challenge", pkce.Challenge)
	authParams.Set("code_challenge_method", pkce.Method)
	authParams.Set("state", state)
	if len(opts.Scopes) > 0 {
		authParams.Set("scope", strings.Join(opts.Scopes, " "))
	}
	authURL := meta.AuthorizationEndpoint + "?" + authParams.Encode()

	// Spin up the callback server with a buffered code receiver.
	type cbResult struct {
		code string
		err  error
	}
	resultCh := make(chan cbResult, 1)
	var once sync.Once
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			msg := q.Get("error_description")
			if msg == "" {
				msg = e
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, "<html><body><h2>SurgeGraph login failed</h2><pre>%s</pre><p>You can close this window.</p></body></html>", msg)
			once.Do(func() { resultCh <- cbResult{err: fmt.Errorf("authorization error: %s: %s", e, msg)} })
			return
		}
		if got := q.Get("state"); got != state {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "state mismatch")
			once.Do(func() { resultCh <- cbResult{err: errors.New("state mismatch (CSRF)")} })
			return
		}
		code := q.Get("code")
		if code == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "missing code")
			once.Do(func() { resultCh <- cbResult{err: errors.New("authorization response missing code")} })
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body style="font-family:system-ui;padding:48px;max-width:480px;margin:auto;text-align:center"><h2 style="color:#2563eb">SurgeGraph login complete</h2><p>You can close this window and return to your terminal.</p></body></html>`))
		once.Do(func() { resultCh <- cbResult{code: code} })
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = server.Serve(listener) }()
	defer func() { _ = server.Shutdown(context.Background()) }()

	fmt.Fprintf(opts.Stdout, "→ Opening browser:\n  %s\n", authURL)
	if opts.Browser != nil {
		if err := opts.Browser(authURL); err != nil {
			fmt.Fprintf(opts.Stdout, "  (could not auto-open browser: %v — paste the URL above)\n", err)
		}
	}

	var code string
	select {
	case r := <-resultCh:
		if r.err != nil {
			return nil, r.err
		}
		code = r.code
	case <-ctx.Done():
		return nil, fmt.Errorf("oauth login timed out (waited %s)", opts.Timeout)
	}

	fmt.Fprintln(opts.Stdout, "→ Exchanging code for tokens...")
	tokens, err := exchangeCode(ctx, meta, registered, code, redirectURI, pkce.Verifier)
	if err != nil {
		return nil, err
	}
	if tokens.ExpiresIn > 0 {
		tokens.ExpiresAt = time.Now().UTC().Add(time.Duration(tokens.ExpiresIn) * time.Second)
	}
	return &Result{Metadata: meta, Client: registered, Tokens: tokens}, nil
}

func exchangeCode(ctx context.Context, m *Metadata, c *Client, code, redirectURI, verifier string) (*Tokens, error) {
	body := url.Values{}
	body.Set("grant_type", "authorization_code")
	body.Set("code", code)
	body.Set("redirect_uri", redirectURI)
	body.Set("client_id", c.ClientID)
	body.Set("code_verifier", verifier)
	if c.ClientSecret != "" {
		body.Set("client_secret", c.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.TokenEndpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token endpoint: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("token exchange: HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}
	var t Tokens
	if err := json.Unmarshal(respBody, &t); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}
	if t.AccessToken == "" {
		return nil, errors.New("token response missing access_token")
	}
	return &t, nil
}

// Refresh exchanges a refresh_token for a fresh access_token.
func Refresh(ctx context.Context, m *Metadata, c *Client, refreshToken string) (*Tokens, error) {
	body := url.Values{}
	body.Set("grant_type", "refresh_token")
	body.Set("refresh_token", refreshToken)
	body.Set("client_id", c.ClientID)
	if c.ClientSecret != "" {
		body.Set("client_secret", c.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.TokenEndpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token endpoint: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("refresh: HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}
	var t Tokens
	if err := json.Unmarshal(respBody, &t); err != nil {
		return nil, fmt.Errorf("decoding refresh response: %w", err)
	}
	if t.ExpiresIn > 0 {
		t.ExpiresAt = time.Now().UTC().Add(time.Duration(t.ExpiresIn) * time.Second)
	}
	return &t, nil
}

// OpenBrowser dials the OS handler. Used as LoginOptions.Browser by `auth login --browser`.
func OpenBrowser(targetURL string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{targetURL}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", targetURL}
	default:
		cmd = "xdg-open"
		args = []string{targetURL}
	}
	return runOSExec(cmd, args...)
}

// truncate keeps error bodies bounded.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ExpiresInDuration converts an Expires_in seconds field to a Duration.
// Centralized so callers don't drift on integer-conversion bugs.
func ExpiresInDuration(seconds int) time.Duration {
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

// PortOf returns the TCP port encoded in a URL. Helper for tests.
func PortOf(rawURL string) (int, bool) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return 0, false
	}
	_, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return 0, false
	}
	return n, true
}

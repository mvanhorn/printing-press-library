// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/config"
	"github.com/spf13/cobra"
)

const (
	xOAuth2AuthorizeURL               = "https://twitter.com/i/oauth2/authorize"
	xOAuth2TokenURL                   = "https://api.twitter.com/2/oauth2/token"
	defaultOAuth2RedirectURI          = "http://127.0.0.1:8787/callback"
	defaultOAuth2Scopes               = "tweet.read,tweet.write,users.read,offline.access,bookmark.read,like.read,like.write,follows.read,follows.write"
	defaultOAuth2TokenExchangeTimeout = 30 * time.Second
)

var oauth2OpenBrowser = openSetupURL

type oauth2LoginOptions struct {
	ClientID          string
	ClientSecret      string
	RedirectURI       string
	Scopes            string
	Timeout           time.Duration
	OpenBrowser       bool
	TokenURL          string
	State             string
	Verifier          string
	PrintAuthorizeURL bool
}

type oauth2CallbackResult struct {
	Code        string
	State       string
	Error       string
	Description string
}

type oauth2TokenResponse struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
}

func newAuthOAuth2LoginCmd(flags *rootFlags) *cobra.Command {
	opts := oauth2LoginOptions{
		RedirectURI: defaultOAuth2RedirectURI,
		Scopes:      defaultOAuth2Scopes,
		Timeout:     5 * time.Minute,
		OpenBrowser: true,
		TokenURL:    xOAuth2TokenURL,
	}
	cmd := &cobra.Command{
		Use:   "oauth2-login --client-id <client-id>",
		Short: "Run OAuth2 authorization-code + PKCE login for user-context API access",
		Long: "Run X OAuth2 authorization-code + PKCE login for user-context API access.\n\n" +
			"The command starts a temporary HTTP listener on the redirect URI, opens the\n" +
			"browser to X's authorization page, validates the callback state, exchanges\n" +
			"the authorization code for tokens, and stores the resulting OAuth2 user-context\n" +
			"token in the same config fields used by auth import-oauth2.\n\n" +
			"Add the redirect URI to your X developer app first. Default: " + defaultOAuth2RedirectURI,
		Example: "  x-twitter-pp-cli auth oauth2-login --client-id YOUR_CLIENT_ID\n" +
			"  x-twitter-pp-cli auth oauth2-login --client-id YOUR_CLIENT_ID --redirect-uri http://127.0.0.1:8787/callback\n" +
			"  x-twitter-pp-cli auth oauth2-login --client-id YOUR_CLIENT_ID --no-open",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ClientID = strings.TrimSpace(opts.ClientID)
			opts.ClientSecret = strings.TrimSpace(opts.ClientSecret)
			opts.RedirectURI = strings.TrimSpace(opts.RedirectURI)
			opts.Scopes = strings.TrimSpace(opts.Scopes)
			opts.PrintAuthorizeURL = !flags.asJSON
			if opts.ClientID == "" {
				return usageErr(fmt.Errorf("--client-id is required"))
			}
			if opts.RedirectURI == "" {
				return usageErr(fmt.Errorf("--redirect-uri is required"))
			}
			if opts.Scopes == "" {
				return usageErr(fmt.Errorf("--scopes must not be empty"))
			}
			if opts.Timeout <= 0 {
				return usageErr(fmt.Errorf("--timeout must be positive"))
			}
			if cliutil.IsVerifyEnv() && opts.OpenBrowser {
				return usageErr(fmt.Errorf("refusing to open browser under PRINTING_PRESS_VERIFY=1; pass --no-open"))
			}
			if !opts.OpenBrowser && !opts.PrintAuthorizeURL {
				return usageErr(fmt.Errorf("--json cannot be combined with --no-open because the authorization URL would be suppressed; omit --json or omit --no-open"))
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			result, token, err := runOAuth2Login(cmd.Context(), cmd.OutOrStdout(), opts)
			if err != nil {
				return err
			}
			scopeList := oauth2ScopesFromToken(token.Scope, opts.Scopes)
			expiry := time.Time{}
			if token.ExpiresIn > 0 {
				expiry = time.Now().UTC().Add(time.Duration(token.ExpiresIn) * time.Second)
			}
			if err := cfg.SaveOAuth2UserContext(opts.ClientID, opts.ClientSecret, token.AccessToken, token.RefreshToken, expiry, scopeList); err != nil {
				return configErr(fmt.Errorf("saving OAuth2 user-context token: %w", err))
			}
			out := map[string]any{
				"saved":                 true,
				"auth_lane":             "oauth2_user_context",
				"config_path":           cfg.Path,
				"redirect_uri":          opts.RedirectURI,
				"scopes":                scopeList,
				"refresh_token_present": strings.TrimSpace(token.RefreshToken) != "",
			}
			if !expiry.IsZero() {
				out["expires_at"] = expiry.UTC().Format(time.RFC3339)
			}
			if result.State != "" {
				out["state_validated"] = true
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "OAuth2 user-context token saved to %s\n", cfg.Path)
			fmt.Fprintf(cmd.OutOrStdout(), "Scopes: %s\n", strings.Join(scopeList, ", "))
			if !expiry.IsZero() {
				fmt.Fprintf(cmd.OutOrStdout(), "Expires: %s\n", expiry.UTC().Format(time.RFC3339))
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Run `x-twitter-pp-cli users get-me --agent` or `x-twitter-pp-cli doctor --json` to verify the token against /2/users/me.")
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.ClientID, "client-id", os.Getenv("X_OAUTH2_CLIENT_ID"), "X OAuth2 client ID; env: X_OAUTH2_CLIENT_ID")
	cmd.Flags().StringVar(&opts.ClientSecret, "client-secret", os.Getenv("X_OAUTH2_CLIENT_SECRET"), "Optional X OAuth2 client secret for confidential clients; env: X_OAUTH2_CLIENT_SECRET")
	cmd.Flags().StringVar(&opts.RedirectURI, "redirect-uri", opts.RedirectURI, "Loopback redirect URI registered in the X developer app")
	cmd.Flags().StringVar(&opts.Scopes, "scopes", opts.Scopes, "Comma-separated or space-separated OAuth2 scopes to request")
	cmd.Flags().DurationVar(&opts.Timeout, "timeout", opts.Timeout, "How long to wait for the browser callback")
	cmd.Flags().BoolVar(&opts.OpenBrowser, "open", opts.OpenBrowser, "Open the authorization URL in the default browser")
	cmd.Flags().BoolVar(&opts.OpenBrowser, "browser", opts.OpenBrowser, "Alias for --open")
	cmd.Flags().Bool("no-open", false, "Print the authorization URL instead of opening a browser")
	cmd.Flags().StringVar(&opts.TokenURL, "token-url", opts.TokenURL, "OAuth2 token endpoint override for tests")
	cmd.Flags().StringVar(&opts.State, "oauth2-state", "", "OAuth2 state override for tests")
	cmd.Flags().StringVar(&opts.Verifier, "pkce-verifier", "", "PKCE verifier override for tests")
	_ = cmd.Flags().MarkHidden("token-url")
	_ = cmd.Flags().MarkHidden("oauth2-state")
	_ = cmd.Flags().MarkHidden("pkce-verifier")
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if changed, _ := cmd.Flags().GetBool("no-open"); changed {
			opts.OpenBrowser = false
		}
		return nil
	}
	return cmd
}

func runOAuth2Login(ctx context.Context, w io.Writer, opts oauth2LoginOptions) (oauth2CallbackResult, oauth2TokenResponse, error) {
	state := opts.State
	if state == "" {
		generated, err := randomURLSafe(32)
		if err != nil {
			return oauth2CallbackResult{}, oauth2TokenResponse{}, fmt.Errorf("generating OAuth2 state: %w", err)
		}
		state = generated
	}
	verifier := opts.Verifier
	if verifier == "" {
		generated, err := randomURLSafe(64)
		if err != nil {
			return oauth2CallbackResult{}, oauth2TokenResponse{}, fmt.Errorf("generating PKCE verifier: %w", err)
		}
		verifier = generated
	}
	listener, callbackPath, err := listenForOAuth2Redirect(opts.RedirectURI)
	if err != nil {
		return oauth2CallbackResult{}, oauth2TokenResponse{}, err
	}
	defer listener.Close()

	resultCh := make(chan oauth2CallbackResult, 1)
	server := &http.Server{Handler: oauth2CallbackHandler(callbackPath, state, resultCh)}
	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Close()

	authorizeURL, err := buildOAuth2AuthorizeURL(oauth2AuthorizeOptions{
		ClientID:    opts.ClientID,
		RedirectURI: opts.RedirectURI,
		Scopes:      oauth2ScopeList(opts.Scopes),
		State:       state,
		Verifier:    verifier,
	})
	if err != nil {
		return oauth2CallbackResult{}, oauth2TokenResponse{}, err
	}
	if opts.PrintAuthorizeURL {
		fmt.Fprintf(w, "Open this URL to authorize X OAuth2 user-context access:\n%s\n", authorizeURL)
	}
	if opts.OpenBrowser {
		if err := oauth2OpenBrowser(authorizeURL); err != nil && opts.PrintAuthorizeURL {
			fmt.Fprintf(w, "Could not open browser automatically: %v\n", err)
		}
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var result oauth2CallbackResult
	select {
	case result = <-resultCh:
	case <-waitCtx.Done():
		return oauth2CallbackResult{}, oauth2TokenResponse{}, fmt.Errorf("timed out waiting for OAuth2 callback at %s", opts.RedirectURI)
	}
	if result.Error != "" {
		return result, oauth2TokenResponse{}, fmt.Errorf("OAuth2 authorization failed: %s %s", result.Error, result.Description)
	}
	if result.Code == "" {
		return result, oauth2TokenResponse{}, fmt.Errorf("OAuth2 callback did not include a code")
	}
	tokenCtx, tokenCancel := context.WithTimeout(ctx, defaultOAuth2TokenExchangeTimeout)
	defer tokenCancel()
	token, err := exchangeOAuth2Code(tokenCtx, opts.TokenURL, opts.ClientID, opts.ClientSecret, opts.RedirectURI, result.Code, verifier)
	if err != nil {
		return result, oauth2TokenResponse{}, err
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return result, oauth2TokenResponse{}, fmt.Errorf("OAuth2 token response did not include access_token")
	}
	return result, token, nil
}

type oauth2AuthorizeOptions struct {
	ClientID    string
	RedirectURI string
	Scopes      []string
	State       string
	Verifier    string
}

func buildOAuth2AuthorizeURL(opts oauth2AuthorizeOptions) (string, error) {
	if strings.TrimSpace(opts.ClientID) == "" {
		return "", fmt.Errorf("client ID is required")
	}
	if strings.TrimSpace(opts.RedirectURI) == "" {
		return "", fmt.Errorf("redirect URI is required")
	}
	if len(opts.Scopes) == 0 {
		return "", fmt.Errorf("at least one OAuth2 scope is required")
	}
	if strings.TrimSpace(opts.State) == "" {
		return "", fmt.Errorf("state is required")
	}
	if strings.TrimSpace(opts.Verifier) == "" {
		return "", fmt.Errorf("PKCE verifier is required")
	}
	u, err := url.Parse(xOAuth2AuthorizeURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", opts.ClientID)
	q.Set("redirect_uri", opts.RedirectURI)
	q.Set("scope", strings.Join(opts.Scopes, " "))
	q.Set("state", opts.State)
	q.Set("code_challenge", pkceChallenge(opts.Verifier))
	q.Set("code_challenge_method", "S256")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func listenForOAuth2Redirect(redirectURI string) (net.Listener, string, error) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		return nil, "", fmt.Errorf("invalid --redirect-uri: %w", err)
	}
	if u.Scheme != "http" {
		return nil, "", fmt.Errorf("redirect URI must use http loopback, got %q", u.Scheme)
	}
	host := u.Hostname()
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return nil, "", fmt.Errorf("redirect URI host must be 127.0.0.1, localhost, or ::1; got %q", host)
	}
	port := u.Port()
	if port == "" {
		return nil, "", fmt.Errorf("redirect URI must include an explicit port")
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(host, port))
	if err != nil {
		return nil, "", fmt.Errorf("listening on %s: %w", redirectURI, err)
	}
	return listener, path, nil
}

func oauth2CallbackHandler(callbackPath, expectedState string, resultCh chan<- oauth2CallbackResult) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != callbackPath {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		result := oauth2CallbackResult{
			Code:        q.Get("code"),
			State:       q.Get("state"),
			Error:       q.Get("error"),
			Description: q.Get("error_description"),
		}
		if result.State != expectedState {
			http.Error(w, "OAuth state mismatch. Return to the CLI and restart login.", http.StatusBadRequest)
			select {
			case resultCh <- oauth2CallbackResult{Error: "state_mismatch", Description: "callback state did not match"}:
			default:
			}
			return
		}
		if result.Error != "" {
			http.Error(w, "OAuth authorization failed. Return to the CLI for details.", http.StatusBadRequest)
			select {
			case resultCh <- result:
			default:
			}
			return
		}
		if result.Code == "" {
			http.Error(w, "OAuth callback missing code. Return to the CLI and restart login.", http.StatusBadRequest)
			select {
			case resultCh <- oauth2CallbackResult{State: result.State, Error: "missing_code", Description: "callback did not include code"}:
			default:
			}
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<html><body><h1>OAuth complete</h1><p>You can close this tab and return to the terminal.</p></body></html>")
		select {
		case resultCh <- result:
		default:
		}
	})
}

func exchangeOAuth2Code(ctx context.Context, tokenURL, clientID, clientSecret, redirectURI, code, verifier string) (oauth2TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("redirect_uri", redirectURI)
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return oauth2TokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if clientSecret != "" {
		req.SetBasicAuth(clientID, clientSecret)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return oauth2TokenResponse{}, fmt.Errorf("exchanging OAuth2 code: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return oauth2TokenResponse{}, fmt.Errorf("OAuth2 token exchange failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var token oauth2TokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return oauth2TokenResponse{}, fmt.Errorf("parsing OAuth2 token response: %w", err)
	}
	return token, nil
}

func oauth2ScopesFromToken(tokenScopes, requested string) []string {
	if strings.TrimSpace(tokenScopes) != "" {
		return oauth2ScopeList(tokenScopes)
	}
	return oauth2ScopeList(requested)
}

func oauth2ScopeList(scopes string) []string {
	scopes = strings.ReplaceAll(scopes, ",", " ")
	parts := strings.Fields(scopes)
	seen := map[string]bool{}
	out := make([]string, 0, len(parts))
	for _, scope := range parts {
		scope = strings.TrimSpace(scope)
		if scope == "" || seen[scope] {
			continue
		}
		seen[scope] = true
		out = append(out, scope)
	}
	return out
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func randomURLSafe(bytesLen int) (string, error) {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

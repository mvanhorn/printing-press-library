// Copyright 2026 omarshahine. Licensed under Apache-2.0. See LICENSE.
// Hand-authored authenticated surface (not generator output).
// PATCH: Blacklane account auth via Auth0 refresh-token grant. Import a
// refresh_token once (from the browser); the CLI mints 24h access tokens itself
// and attaches the discovered header set required by graphql.blacklane.com /
// guest-api.blacklane.com (apollographql-client-name: web, usertype: guest, ...).

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	// Public Auth0 SPA client (no secret) — observed in the web app.
	blAuth0ClientID    = "7eyTSLgxPa6KM585UEJ1ACEN6VjQlj8T"
	blAuth0TokenURL    = "https://login.blacklane.com/oauth/token"
	blAuth0RedirectURI = "https://www.blacklane.com/en/auth/callback"
	blGraphQLURL       = "https://graphql.blacklane.com/graphql"
	blGuestAPIBase     = "https://guest-api.blacklane.com"
)

// authState is the on-disk credential store (mode 0600).
type authState struct {
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
	ExpiresAt    int64  `json:"expires_at"` // unix seconds
}

func authStatePath() string {
	return filepath.Join(filepath.Dir(defaultDBPath("blacklane-pp-cli")), "auth.json")
}

func loadAuthState() (*authState, error) {
	b, err := os.ReadFile(authStatePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not logged in — run 'blacklane-pp-cli auth login' first")
		}
		return nil, err
	}
	var s authState
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("reading auth store: %w", err)
	}
	return &s, nil
}

func saveAuthState(s *authState) error {
	p := authStatePath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	if err := os.WriteFile(p, b, 0o600); err != nil { // secret material — owner-only
		return err
	}
	// Enforce 0600 even if the file already existed with looser permissions
	// (WriteFile keeps the existing mode on overwrite).
	return os.Chmod(p, 0o600)
}

// refreshAccessToken exchanges the stored refresh_token for a fresh access
// token via the public Auth0 client. Auth0 rotates refresh tokens, so a new one
// in the response is persisted.
func refreshAccessToken(s *authState, timeout time.Duration) (*authState, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {blAuth0ClientID},
		"refresh_token": {s.RefreshToken},
		"redirect_uri":  {blAuth0RedirectURI},
	}
	req, _ := http.NewRequest(http.MethodPost, blAuth0TokenURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://www.blacklane.com")
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("token refresh: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed (HTTP %d) — your session may have expired; run 'auth login' again with a fresh refresh token", resp.StatusCode)
	}
	var tr struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("token refresh returned no access token")
	}
	out := &authState{
		RefreshToken: s.RefreshToken,
		AccessToken:  tr.AccessToken,
		ExpiresAt:    time.Now().Unix() + tr.ExpiresIn - 60, // 60s safety margin
	}
	if tr.RefreshToken != "" { // Auth0 rotation
		out.RefreshToken = tr.RefreshToken
	}
	return out, nil
}

// validAccessToken returns a non-expired access token, refreshing if needed and
// persisting any rotated credentials.
func validAccessToken(timeout time.Duration) (string, error) {
	s, err := loadAuthState()
	if err != nil {
		return "", err
	}
	if s.AccessToken != "" && time.Now().Unix() < s.ExpiresAt {
		return s.AccessToken, nil
	}
	if s.RefreshToken == "" {
		// Chrome-imported access token expired; we deliberately don't hold a
		// refresh token in that mode (see importAccessFromChrome).
		return "", fmt.Errorf("session expired — run 'blacklane-pp-cli auth login --chrome' to renew")
	}
	refreshed, err := refreshAccessToken(s, timeout)
	if err != nil {
		return "", err
	}
	if err := saveAuthState(refreshed); err != nil {
		return "", err
	}
	return refreshed.AccessToken, nil
}

// blAuthHeaders is the exact header set graphql.blacklane.com / guest-api accept
// (discovered from the web app; the Bearer token alone is rejected without these).
func blAuthHeaders(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("apollographql-client-name", "web")
	req.Header.Set("apollographql-client-version", "0.0.0")
	req.Header.Set("usertype", "guest")
	req.Header.Set("authheaderpresent", "true")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://www.blacklane.com")
}

// authedGraphQL runs an authenticated GraphQL operation and returns the `data`
// object (or a GraphQL error).
func authedGraphQL(operationName, query string, variables map[string]any, timeout time.Duration) (json.RawMessage, error) {
	token, err := validAccessToken(timeout)
	if err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(map[string]any{
		"operationName": operationName,
		"query":         query,
		"variables":     variables,
	})
	req, _ := http.NewRequest(http.MethodPost, blGraphQLURL, strings.NewReader(string(payload)))
	blAuthHeaders(req, token)
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var env struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("graphql %s: unparseable response (HTTP %d)", operationName, resp.StatusCode)
	}
	if len(env.Errors) > 0 {
		return nil, fmt.Errorf("graphql %s: %s", operationName, env.Errors[0].Message)
	}
	return env.Data, nil
}

// authedGuestGet performs an authenticated GET against guest-api.blacklane.com.
func authedGuestGet(path string, timeout time.Duration) (json.RawMessage, error) {
	token, err := validAccessToken(timeout)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest(http.MethodGet, blGuestAPIBase+path, nil)
	blAuthHeaders(req, token)
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("GET %s: unauthorized (HTTP %d) — try 'auth login' again", path, resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s returned HTTP %d", path, resp.StatusCode)
	}
	return json.RawMessage(body), nil
}

// ---- auth command group ----

func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Blacklane account authentication (login/status/logout)",
		Long: "Authenticate to your Blacklane account so commands like 'me', 'bookings', and 'wallet' work.\n\n" +
			"Auth uses your Auth0 refresh token. Get it once from your logged-in browser:\n" +
			"  DevTools → Application → Local Storage → https://www.blacklane.com →\n" +
			"  the key starting '@@auth0spajs@@…' → expand 'body' → copy 'refresh_token'.\n" +
			"Then: blacklane-pp-cli auth login   (paste the token at the prompt, or pipe it in).\n" +
			"The CLI mints short-lived access tokens itself after that. No booking is made by any auth command.",
	}
	cmd.AddCommand(newAuthLoginCmd(flags))
	cmd.AddCommand(newAuthStatusCmd(flags))
	cmd.AddCommand(newAuthLogoutCmd(flags))
	return cmd
}

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var tokenFile string
	var useChrome bool
	var profile string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to Blacklane (--chrome imports automatically, or paste a refresh token)",
		Long: "Authenticate to your Blacklane account. The easy path imports your current access token\n" +
			"straight from Chrome's local storage (valid ~24h; re-run --chrome to renew). It does not\n" +
			"touch your browser's refresh session:\n\n" +
			"  blacklane-pp-cli auth login --chrome\n\n" +
			"(Log in to blacklane.com in Chrome first. Quitting Chrome first is most reliable.)\n\n" +
			"For a durable, self-refreshing login, paste the refresh_token from DevTools (Local Storage\n" +
			"-> the @@auth0spajs@@... key -> body -> refresh_token) instead:\n\n" +
			"  pbpaste | blacklane-pp-cli auth login\n\n" +
			"The refresh-token path mints short-lived (24h) access tokens itself and auto-refreshes.",
		Example: strings.Trim(`
  blacklane-pp-cli auth login --chrome
  pbpaste | blacklane-pp-cli auth login
  blacklane-pp-cli auth login --token-file ~/bl-refresh.txt`, "\n"),
		Annotations: map[string]string{"mcp:hidden": "true"}, // secret input; not an agent tool
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			confirm := func() {
				data, err := authedGuestGet("/api/v1/users/me", flags.timeout)
				name := ""
				if err == nil {
					var me struct {
						FirstName string `json:"first_name"`
						Email     string `json:"email"`
					}
					_ = json.Unmarshal(data, &me)
					name = strings.TrimSpace(me.FirstName)
					if name == "" {
						name = me.Email
					}
				}
				if name != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s.\n", name)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "Logged in.")
				}
			}

			// --chrome: import the current 24h access token directly. We do NOT
			// take the refresh token — Auth0 rotates it, and reusing the
			// browser's would poison its session. Renew by re-running --chrome.
			if useChrome {
				at, exp, prof, err := importAccessFromChrome(profile)
				if err != nil {
					return err
				}
				if exp < 1_000_000_000 { // looked like a duration, not an absolute time
					exp = time.Now().Unix() + exp
				}
				st := &authState{AccessToken: at, ExpiresAt: exp - 60}
				if err := saveAuthState(st); err != nil {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Imported Blacklane login from Chrome profile %q (valid until %s; re-run --chrome to renew).\n",
					prof, time.Unix(st.ExpiresAt, 0).Format("Jan 2 15:04"))
				confirm()
				return nil
			}

			// Paste / --token-file: store the refresh token for durable auto-refresh.
			var raw string
			if tokenFile != "" {
				b, err := os.ReadFile(tokenFile)
				if err != nil {
					return fmt.Errorf("reading --token-file: %w", err)
				}
				raw = string(b)
			} else {
				if isTerminal(os.Stdin) {
					fmt.Fprint(cmd.ErrOrStderr(), "Paste Blacklane refresh_token, then Enter (or use --chrome): ")
				}
				r := bufio.NewReader(os.Stdin)
				line, _ := r.ReadString('\n')
				raw = line
			}
			rt := strings.TrimSpace(raw)
			if rt == "" {
				return fmt.Errorf("no refresh token provided (use --chrome, pipe it in, or use --token-file)")
			}
			refreshed, err := refreshAccessToken(&authState{RefreshToken: rt}, flags.timeout)
			if err != nil {
				return fmt.Errorf("could not verify refresh token: %w", err)
			}
			if err := saveAuthState(refreshed); err != nil {
				return err
			}
			confirm()
			return nil
		},
	}
	cmd.Flags().BoolVar(&useChrome, "chrome", false, "Import your login automatically from Chrome's storage (24h; re-run to renew)")
	cmd.Flags().StringVar(&profile, "profile-name", "Default", "Chrome profile to read when using --chrome")
	cmd.Flags().StringVar(&tokenFile, "token-file", "", "Read the refresh token from this file instead of stdin")
	return cmd
}

func newAuthStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Show whether you're logged in and the account",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := loadAuthState()
			out := map[string]any{"loggedIn": false}
			if err == nil {
				out["loggedIn"] = true
				out["accessTokenExpiresAt"] = time.Unix(s.ExpiresAt, 0).UTC().Format(time.RFC3339)
				out["accessTokenValid"] = time.Now().Unix() < s.ExpiresAt
				if data, derr := authedGuestGet("/api/v1/users/me", flags.timeout); derr == nil {
					var me struct {
						FirstName string `json:"first_name"`
						LastName  string `json:"last_name"`
						Email     string `json:"email"`
					}
					_ = json.Unmarshal(data, &me)
					out["name"] = strings.TrimSpace(me.FirstName + " " + me.LastName)
					out["email"] = me.Email
				}
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if out["loggedIn"] == true {
				fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %v (%v). Token valid: %v\n", out["name"], out["email"], out["accessTokenValid"])
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Not logged in. Run 'blacklane-pp-cli auth login'.")
			}
			return nil
		},
	}
	return cmd
}

func newAuthLogoutCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "logout",
		Short:       "Delete stored Blacklane credentials",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			err := os.Remove(authStatePath())
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Logged out — stored credentials removed.")
			return nil
		},
	}
}

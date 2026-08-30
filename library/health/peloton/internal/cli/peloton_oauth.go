// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/peloton/internal/client"
	"github.com/mvanhorn/printing-press-library/library/health/peloton/internal/cliutil"
	"github.com/spf13/cobra"
)

const (
	pelotonOAuthTokenURL = "https://auth.onepeloton.com/oauth/token"
	pelotonOAuthClientID = "WVoJxVDdPoFx4RNewvvg6ch2mZ7bwnsM"
	pelotonOAuthAudience = "https://api.onepeloton.com/"
	pelotonOAuthRealm    = "pelo-user-password"
	pelotonOAuthScope    = "openid offline_access peloton-api.members:default"
	pelotonOAuthGrant    = "http://auth0.com/oauth/grant-type/password-realm"
	oauthExpirySkew      = 30 * time.Second
)

type pelotonTokenBundle struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	// SessionID is the peloton_session_id value from Peloton's legacy
	// /auth/login flow. It rides in the same on-disk bundle as the Auth0
	// bearer fields above (both are "the credential this CLI persists"
	// from a user's point of view) but is a distinct credential: Peloton's
	// legacy REST surface (/api/me, performance_graph, ...) checks this
	// cookie, not the bearer token, and Peloton has no expiry contract for
	// it — it is cached until auth logout clears the bundle or a login
	// attempt is needed because none is cached yet.
	SessionID string `json:"session_id"`
}

type pelotonTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// pelotonLoginPath is Peloton's real (non-OAuth) session login endpoint,
// resolved against the client's configured base URL rather than hardcoded
// so PELOTON_BASE_URL overrides (verify mode, mock servers) reach it too.
const pelotonLoginPath = "/auth/login"

type pelotonLoginRequest struct {
	UsernameOrEmail string `json:"username_or_email"`
	Password        string `json:"password"`
}

type pelotonLoginResponse struct {
	SessionID string `json:"session_id"`
}

var (
	oauthNow        = time.Now
	oauthHTTPClient = &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	oauthBundlePath = defaultOAuthBundlePath
	oauthTokenURL   = pelotonOAuthTokenURL
)

func init() {
	registerClientHook(installManagedPelotonBearer)
	registerNovelCommand(configureManagedPelotonAuth)
}

// configureManagedPelotonAuth replaces the generic manual-token helpers with
// commands that describe the managed lifecycle without accepting a bearer.
func configureManagedPelotonAuth(root *cobra.Command, _ *rootFlags) {
	for _, cmd := range root.Commands() {
		if cmd.Name() != "auth" {
			continue
		}
		cmd.Short = "Manage Peloton credentials (auto-login; no OAuth provisioning involved)"
		for _, child := range cmd.Commands() {
			cmd.RemoveCommand(child)
		}
		cmd.AddCommand(newManagedOAuthSetupCmd(), newManagedOAuthStatusCmd(), newManagedOAuthLogoutCmd())
		return
	}
}

func newManagedOAuthSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Show how to supply Peloton login credentials",
		Run: func(cmd *cobra.Command, _ []string) {
			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "Peloton has no OAuth provisioning service to configure — this CLI just needs your Peloton login.")
			fmt.Fprintln(w, "Set PELOTON_OAUTH_USERNAME and PELOTON_OAUTH_PASSWORD to your Peloton email/username and password.")
			fmt.Fprintln(w, "The first live command logs in automatically (both an Auth0 bearer token and a real /auth/login session)")
			fmt.Fprintln(w, "and persists the result to " + oauthBundlePathHint() + "; later commands reuse or refresh it without the env vars set.")
		},
	}
}

func newManagedOAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show persisted Peloton credential status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			bundle, err := loadOAuthBundle()
			if os.IsNotExist(err) {
				fmt.Fprintln(cmd.OutOrStdout(), "No persisted Peloton credentials yet; the next live command will log in automatically.")
				return nil
			}
			if err != nil {
				return authErr(fmt.Errorf("reading persisted Peloton credentials: %w", err))
			}
			bearerState := "expired; the next live request will refresh it"
			if bundle.AccessToken != "" && bundle.ExpiresAt.After(oauthNow().Add(oauthExpirySkew)) {
				bearerState = "available"
			}
			sessionState := "not established; the next live request will log in"
			if bundle.SessionID != "" {
				sessionState = "available"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Bearer token (catalog/list reads): %s.\n", bearerState)
			fmt.Fprintf(cmd.OutOrStdout(), "Session cookie (account/performance reads): %s.\n", sessionState)
			return nil
		},
	}
}

func newManagedOAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove persisted Peloton credentials",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := oauthBundlePath()
			if err != nil {
				return authErr(fmt.Errorf("locating persisted Peloton credentials: %w", err))
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return authErr(fmt.Errorf("removing persisted Peloton credentials: %w", err))
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Persisted Peloton credentials removed.")
			return nil
		},
	}
}

// InstallManagedPelotonBearer applies this CLI's managed Peloton auth (the
// Auth0 bearer token plus, best-effort, the real session cookie) to c. It is
// the exported entry point for internal/mcp: MCP tool calls build their
// client directly via client.New (for MCP-specific NoCache/timeout/rate
// settings) rather than through rootFlags.newClient(), so they never ran the
// clientHooks registry this same auth is registered into for CLI commands —
// MCP tool calls were authenticating with no credential at all until this
// was wired in. Both call sites must invoke the same underlying logic so
// CLI and MCP never diverge in what credential they attach, the same
// principle behind attaching bearer+cookie together in the first place.
func InstallManagedPelotonBearer(c *client.Client) error {
	return installManagedPelotonBearer(c)
}

// installManagedPelotonBearer injects the in-memory managed bearer token
// plus (best-effort) the real Peloton session cookie. It deliberately
// discards any stale persisted Authorization/Cookie header before doing so.
//
// Peloton has no single auth surface: the bearer token above is accepted by
// catalog/list endpoints, but the legacy REST surface (/api/me,
// performance_graph, ...) checks a peloton_session_id cookie instead and
// answers 401 without it regardless of the bearer. Attaching both here means
// sync and single-fetch commands alike carry whatever credential the
// endpoint they hit actually needs, without special-casing individual
// endpoints. Called from two places: the clientHooks registry below (CLI
// commands, via rootFlags.newClient()) and InstallManagedPelotonBearer above
// (MCP tool calls, via internal/mcp's own client construction).
func installManagedPelotonBearer(c *client.Client) error {
	if c == nil || c.Config == nil {
		return authErr(fmt.Errorf("managed Peloton OAuth client is unavailable"))
	}
	token, err := managedPelotonAccessToken()
	if err != nil {
		return authErr(err)
	}
	c.Config.AccessToken = token
	c.Config.RefreshToken = ""
	c.Config.AuthHeaderVal = ""
	for name := range c.Config.Headers {
		if strings.EqualFold(name, "Authorization") || strings.EqualFold(name, "Cookie") {
			delete(c.Config.Headers, name)
		}
	}
	// Best-effort: a session login that can't complete yet (no bootstrap
	// creds cached or supplied) leaves the transport exactly as it behaves
	// today — bearer-only — rather than failing commands that don't need
	// the cookie. Endpoints that do need it will still 401 in that case,
	// same as before this change.
	if sessionID, ok := managedPelotonSessionCookie(c.RequestBaseURL()); ok {
		if c.Config.Headers == nil {
			c.Config.Headers = map[string]string{}
		}
		c.Config.Headers["Cookie"] = "peloton_session_id=" + sessionID
	}
	if c.HTTPClient != nil {
		c.HTTPClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
		c.HTTPClient.Transport = pelotonTwoXXRoundTripper{base: c.HTTPClient.Transport}
	}
	return nil
}

// pelotonTwoXXRoundTripper makes a managed catalog proof fail closed on a
// redirect or other non-2xx response before generated client code can parse it.
type pelotonTwoXXRoundTripper struct{ base http.RoundTripper }

func (t pelotonTwoXXRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err != nil || resp == nil {
		return resp, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("managed Peloton HTTP response must be 2xx")
	}
	return resp, nil
}

func managedPelotonAccessToken() (string, error) {
	bundle, err := loadOAuthBundle()
	if err == nil && bundle.AccessToken != "" && bundle.ExpiresAt.After(oauthNow().Add(oauthExpirySkew)) {
		return bundle.AccessToken, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("loading managed Peloton OAuth token: %w", err)
	}

	var next pelotonTokenResponse
	if err == nil && bundle.RefreshToken != "" {
		next, err = refreshPelotonToken(bundle.RefreshToken)
	} else {
		next, err = bootstrapPelotonToken()
	}
	if err != nil {
		return "", err
	}
	if next.AccessToken == "" || next.ExpiresIn <= 0 {
		return "", fmt.Errorf("managed Peloton OAuth response is incomplete")
	}
	if next.RefreshToken == "" {
		next.RefreshToken = bundle.RefreshToken
	}
	updated := pelotonTokenBundle{
		AccessToken:  next.AccessToken,
		RefreshToken: next.RefreshToken,
		ExpiresAt:    oauthNow().Add(time.Duration(next.ExpiresIn) * time.Second),
		// Preserve whatever session cookie is already cached — this bundle
		// rebuild is for the bearer fields only, and must not silently drop
		// the unrelated session credential on a routine bearer refresh.
		SessionID: bundle.SessionID,
	}
	if updated.RefreshToken == "" {
		return "", fmt.Errorf("managed Peloton OAuth response omitted a refresh token")
	}
	if err := saveOAuthBundle(updated); err != nil {
		return "", fmt.Errorf("saving managed Peloton OAuth token: %w", err)
	}
	return updated.AccessToken, nil
}

func bootstrapPelotonToken() (pelotonTokenResponse, error) {
	username := strings.TrimSpace(os.Getenv("PELOTON_OAUTH_USERNAME"))
	password := os.Getenv("PELOTON_OAUTH_PASSWORD")
	if username == "" || password == "" {
		return pelotonTokenResponse{}, fmt.Errorf("managed Peloton OAuth bootstrap credentials are unavailable")
	}
	return requestPelotonToken(url.Values{
		"grant_type": {pelotonOAuthGrant},
		"client_id":  {oauthClientID()},
		"username":   {username},
		"password":   {password},
		"realm":      {oauthRealm()},
		"scope":      {oauthScope()},
		"audience":   {oauthAudience()},
	})
}

func refreshPelotonToken(refreshToken string) (pelotonTokenResponse, error) {
	return requestPelotonToken(url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {oauthClientID()},
		"refresh_token": {refreshToken},
	})
}

// managedPelotonSessionCookie returns a peloton_session_id value to attach
// to every request, and whether one is available. It is best-effort by
// design: unlike managedPelotonAccessToken, a failure here never fails the
// caller. Peloton sessions carry no expiry contract we can check locally, so
// a cached id is trusted until auth logout clears the bundle or the server
// itself rejects it (surfaced as a live 401, same as any other invalid
// credential).
func managedPelotonSessionCookie(baseURL string) (string, bool) {
	bundle, err := loadOAuthBundle()
	if err == nil && bundle.SessionID != "" {
		return bundle.SessionID, true
	}
	if err != nil && !os.IsNotExist(err) {
		return "", false
	}
	sessionID, loginErr := bootstrapPelotonSession(baseURL)
	if loginErr != nil || sessionID == "" {
		return "", false
	}
	bundle.SessionID = sessionID
	// Best-effort save: a write failure just means the next command
	// re-logs-in rather than reusing this session, which is safe.
	_ = saveOAuthBundle(bundle)
	return sessionID, true
}

// bootstrapPelotonSession performs Peloton's real (non-OAuth) login:
// POST {baseURL}/auth/login with {username_or_email, password}, returning
// the resulting peloton_session_id. Real responses have been observed to
// carry the session id either as a Set-Cookie header or as a session_id
// field in the JSON body; both are checked, Set-Cookie first.
func bootstrapPelotonSession(baseURL string) (string, error) {
	username := strings.TrimSpace(os.Getenv("PELOTON_OAUTH_USERNAME"))
	password := os.Getenv("PELOTON_OAUTH_PASSWORD")
	if username == "" || password == "" {
		return "", fmt.Errorf("Peloton session login credentials are unavailable")
	}
	body, err := json.Marshal(pelotonLoginRequest{UsernameOrEmail: username, Password: password})
	if err != nil {
		return "", fmt.Errorf("encoding Peloton login request: %w", err)
	}
	loginURL := strings.TrimRight(baseURL, "/") + pelotonLoginPath
	req, err := http.NewRequest(http.MethodPost, loginURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating Peloton login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Peloton-Platform", "web")
	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Peloton login request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("Peloton login request failed with HTTP %d", resp.StatusCode)
	}
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "peloton_session_id" && cookie.Value != "" {
			return cookie.Value, nil
		}
	}
	var loginResp pelotonLoginResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&loginResp); err != nil {
		return "", fmt.Errorf("decoding Peloton login response")
	}
	if loginResp.SessionID == "" {
		return "", fmt.Errorf("Peloton login response omitted a session id")
	}
	return loginResp.SessionID, nil
}

func oauthProviderValue(environment, fallback string) string {
	if configured := strings.TrimSpace(os.Getenv(environment)); configured != "" {
		return configured
	}
	return fallback
}

func oauthClientID() string {
	return oauthProviderValue("PELOTON_OAUTH_CLIENT_ID", pelotonOAuthClientID)
}
func oauthRealm() string { return oauthProviderValue("PELOTON_OAUTH_REALM", pelotonOAuthRealm) }
func oauthAudience() string {
	return oauthProviderValue("PELOTON_OAUTH_AUDIENCE", pelotonOAuthAudience)
}
func oauthScope() string { return oauthProviderValue("PELOTON_OAUTH_SCOPE", pelotonOAuthScope) }

func requestPelotonToken(form url.Values) (pelotonTokenResponse, error) {
	if form.Get("client_id") == "" || (form.Get("grant_type") == pelotonOAuthGrant && form.Get("realm") == "") {
		return pelotonTokenResponse{}, fmt.Errorf("managed Peloton OAuth public client configuration is unavailable")
	}
	tokenURL := oauthProviderValue("PELOTON_OAUTH_TOKEN_URL", oauthTokenURL)
	req, err := http.NewRequest(http.MethodPost, tokenURL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return pelotonTokenResponse{}, fmt.Errorf("creating managed Peloton OAuth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return pelotonTokenResponse{}, fmt.Errorf("managed Peloton OAuth request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return pelotonTokenResponse{}, fmt.Errorf("managed Peloton OAuth request failed with HTTP %d", resp.StatusCode)
	}
	var token pelotonTokenResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&token); err != nil {
		return pelotonTokenResponse{}, fmt.Errorf("decoding managed Peloton OAuth response")
	}
	return token, nil
}

func defaultOAuthBundlePath() (string, error) {
	dir, err := cliutil.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "oauth-token.json"), nil
}

// oauthBundlePathHint returns the persisted-credential path for display in
// help text, falling back to a description rather than an error when the
// path can't be resolved (e.g. no home directory) since this only feeds
// human-facing prose.
func oauthBundlePathHint() string {
	if path, err := oauthBundlePath(); err == nil {
		return path
	}
	return "the CLI's config directory"
}

// pelotonPersistedBundleStatus reports whether a usable persisted credential
// bundle exists on disk (bearer access/refresh token or session cookie) and
// where. Used by `doctor` to distinguish "no credentials anywhere" from
// "bootstrap env vars absent but a valid persisted bundle already covers
// this" — cfg.AuthHeader() alone can't see this: it reflects only the
// config file / env-derived fields config.Load() populates, never the
// separate oauth-token.json bundle this CLI's managed auth actually uses.
func pelotonPersistedBundleStatus() (usable bool, path string, err error) {
	path, err = oauthBundlePath()
	if err != nil {
		return false, "", err
	}
	bundle, loadErr := loadOAuthBundle()
	if loadErr != nil {
		if os.IsNotExist(loadErr) {
			return false, path, nil
		}
		return false, path, loadErr
	}
	usable = bundle.AccessToken != "" || bundle.RefreshToken != "" || bundle.SessionID != ""
	return usable, path, nil
}

func loadOAuthBundle() (pelotonTokenBundle, error) {
	path, err := oauthBundlePath()
	if err != nil {
		return pelotonTokenBundle{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return pelotonTokenBundle{}, err
	}
	if info.Mode().Perm()&0o077 != 0 {
		return pelotonTokenBundle{}, fmt.Errorf("managed Peloton OAuth token file permissions are too broad")
	}
	f, err := os.Open(path)
	if err != nil {
		return pelotonTokenBundle{}, err
	}
	defer f.Close()
	var bundle pelotonTokenBundle
	if err := json.NewDecoder(io.LimitReader(f, 64<<10)).Decode(&bundle); err != nil {
		return pelotonTokenBundle{}, fmt.Errorf("managed Peloton OAuth token file is invalid")
	}
	return bundle, nil
}

func saveOAuthBundle(bundle pelotonTokenBundle) error {
	path, err := oauthBundlePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".oauth-token-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if err := json.NewEncoder(f).Encode(bundle); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

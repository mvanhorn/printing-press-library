// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/internal/config"

	"github.com/spf13/cobra"
)

// BMW CarData OAuth2 endpoints (GCDM). These live on customer.bmwgroup.com,
// separate from the REST API host, so the generated client (bound to
// api-cardata.bmwgroup.com) is not used here.
const (
	cardataDeviceCodeURL = "https://customer.bmwgroup.com/gcdm/oauth/device/code"
	// CardataTokenURL is shared with typed MCP handlers so every API surface
	// refreshes the same file-backed OAuth credentials before use.
	CardataTokenURL     = "https://customer.bmwgroup.com/gcdm/oauth/token"
	cardataDefaultScope = "authenticate_user openid cardata:api:read cardata:streaming:read"
)

// ErrCardataLoginRequired means the saved OAuth credential cannot be renewed.
// A timeout or server error is deliberately not classified this way.
var ErrCardataLoginRequired = errors.New("BMW CarData login required")

// ErrCardataRefreshUnavailable marks a retryable network or provider failure.
var ErrCardataRefreshUnavailable = errors.New("BMW CarData refresh temporarily unavailable")

// newAuthLoginCmd implements the OAuth2 Device Authorization Grant with PKCE
// (S256). It is the primary onboarding path: the user generates a client_id
// in the BMW CarData portal, runs this command, opens the printed URL, logs
// in, and the CLI stores the resulting tokens for all later commands.
func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var (
		flagClientID string
		flagScope    string
		flagLaunch   bool
		flagTimeout  int
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate via the BMW CarData OAuth2 device-code flow (PKCE)",
		Long: `Run the BMW CarData OAuth2 Device Authorization Grant to obtain an access token.

Prerequisites (one-time, in the BMW portal):
  1. Open My BMW > BMW CarData > "Create CarData Client" and copy the client id
     (it is hidden on reload, so save it). Tick "Request access to CarData API"
     (and "CarData Stream" if you want live streaming).
  2. Ensure your vehicle is mapped to your account as the PRIMARY user with an
     active ConnectedDrive contract and SIM.

Then run:
  bmw-cardata-pp-cli auth login --client-id <your-client-id>
  # or: export BMW_CARDATA_CLIENT_ID=<your-client-id> && bmw-cardata-pp-cli auth login

The command prints a verification URL. Open it, log in to BMW, and approve;
the CLI polls until the login completes and stores the tokens locally.`,
		Example: "  bmw-cardata-pp-cli auth login --client-id 550e8400-e29b-41d4-a716-446655440000",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would run the BMW CarData OAuth2 device-code login flow")
				return nil
			}
			clientID := flagClientID
			if clientID == "" {
				clientID = os.Getenv("BMW_CARDATA_CLIENT_ID")
			}
			if clientID == "" {
				if cfg, err := config.Load(flags.configPath); err == nil {
					clientID = cfg.ClientID
				}
			}
			if clientID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--client-id is required (or set BMW_CARDATA_CLIENT_ID); generate one in the BMW CarData portal"))
			}
			scope := cardataDefaultScope
			if flagScope != "" {
				scope = flagScope
			}
			pollTimeout := time.Duration(flagTimeout) * time.Second
			if pollTimeout <= 0 {
				pollTimeout = 10 * time.Minute
			}

			verifier, challenge, err := cardataPKCE()
			if err != nil {
				return configErr(fmt.Errorf("generating PKCE pair: %w", err))
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), pollTimeout)
			defer cancel()

			dc, err := cardataRequestDeviceCode(ctx, clientID, scope, challenge)
			if err != nil {
				return apiErr(fmt.Errorf("requesting device code: %w", err))
			}
			verifyURL := dc.VerificationURIComplete
			if verifyURL == "" {
				verifyURL = dc.VerificationURI
			}
			userCode := dc.UserCode

			// Print-by-default + opt-in launch + verify-env short-circuit
			// (per side-effect command convention).
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would launch: %s\n", verifyURL)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Open this URL and approve the login:\n  %s\n", verifyURL)
			if userCode != "" && !strings.Contains(verifyURL, userCode) {
				fmt.Fprintf(cmd.OutOrStdout(), "Or enter code %s at %s\n", userCode, dc.VerificationURI)
			}
			if flagLaunch {
				if err := openBrowser(verifyURL); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not launch browser (--launch): %v\n  open the URL above manually\n", err)
				}
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "(pass --launch to open it automatically)")
			}

			interval := dc.Interval
			if interval <= 0 {
				interval = 5
			}
			deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)
			if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
				deadline = dl
			}

			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					return authErr(fmt.Errorf("login timed out: %w", ctx.Err()))
				case <-time.After(time.Duration(interval) * time.Second):
				}
				tok, err := cardataPollToken(ctx, clientID, dc.DeviceCode, verifier)
				if err != nil {
					// slow_down -> back off; keep polling.
					if strings.Contains(err.Error(), "slow_down") {
						interval += 5
						continue
					}
					if strings.Contains(err.Error(), "authorization_pending") {
						continue
					}
					return authErr(fmt.Errorf("device-code login failed: %w", err))
				}
				// Success: persist tokens.
				cfg, err := config.Load(flags.configPath)
				if err != nil {
					return configErr(fmt.Errorf("loading config to save tokens: %w", err))
				}
				expiry := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
				if err := withCardataRefreshLock(ctx, cfg.Path, func() error {
					fresh, err := config.Load(cfg.Path)
					if err != nil {
						return err
					}
					if err := saveCardataOAuthTokens(fresh, clientID, "", tok, expiry); err != nil {
						return err
					}
					return writeCardataSession(fresh, clientID, tok, expiry)
				}); err != nil {
					return configErr(fmt.Errorf("saving login credentials: %w", err))
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\nLogin successful. Tokens saved to %s\n", cfg.Path)
				fmt.Fprintf(cmd.OutOrStdout(), "Verify with: bmw-cardata-pp-cli doctor\n")
				return nil
			}
			return authErr(fmt.Errorf("device code expired before login completed"))
		},
	}
	cmd.Flags().StringVar(&flagClientID, "client-id", "", "BMW CarData client id (or set BMW_CARDATA_CLIENT_ID)")
	cmd.Flags().StringVar(&flagScope, "scope", "", "OAuth scopes to request (default: api+streaming read)")
	cmd.Flags().BoolVar(&flagLaunch, "launch", false, "Open the verification URL in your browser automatically")
	cmd.Flags().IntVar(&flagTimeout, "timeout", 600, "Maximum seconds to wait for login (default 600)")
	return cmd
}

// ---- OAuth2 device-code helpers ----

type cardataDeviceCode struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type cardataToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	GCID         string `json:"gcid"`
}

func cardataPKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

func cardataPostForm(ctx context.Context, target string, vals url.Values) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(vals.Encode()))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// OAuth form bodies contain reusable credentials. Never replay them to a
	// redirect target, even if the provider returns a 307 or 308 response.
	httpClient := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return body, resp.StatusCode, nil
}

func cardataRequestDeviceCode(ctx context.Context, clientID, scope, challenge string) (*cardataDeviceCode, error) {
	vals := url.Values{
		"client_id":             {clientID},
		"response_type":         {"device_code"},
		"scope":                 {scope},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	body, status, err := cardataPostForm(ctx, cardataDeviceCodeURL, vals)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("device-code endpoint returned HTTP %d", status)
	}
	var dc cardataDeviceCode
	if err := json.Unmarshal(body, &dc); err != nil {
		return nil, fmt.Errorf("parsing device-code response: %w", err)
	}
	if dc.DeviceCode == "" {
		return nil, fmt.Errorf("device-code response missing device_code")
	}
	return &dc, nil
}

func cardataPollToken(ctx context.Context, clientID, deviceCode, verifier string) (*cardataToken, error) {
	vals := url.Values{
		"client_id":     {clientID},
		"grant_type":    {"urn:ietf:params:oauth:grant-type:device_code"},
		"device_code":   {deviceCode},
		"code_verifier": {verifier},
	}
	body, status, err := cardataPostForm(ctx, CardataTokenURL, vals)
	if err != nil {
		return nil, err
	}
	// Error envelope: {"error":"authorization_pending" | "slow_down" | ...}
	var ee struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if json.Unmarshal(body, &ee) == nil {
		switch ee.Error {
		case "authorization_pending", "slow_down":
			return nil, fmt.Errorf("%s (HTTP %d)", ee.Error, status)
		}
	}
	if status != 200 {
		return nil, fmt.Errorf("token endpoint returned HTTP %d", status)
	}
	var tok cardataToken
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("token response missing access_token")
	}
	if tok.GCID == "" {
		tok.GCID = gcidFromJWT(tok.IDToken)
		if tok.GCID == "" {
			tok.GCID = gcidFromJWT(tok.AccessToken)
		}
	}
	return &tok, nil
}

// RefreshCardataAccessTokenIfNeeded refreshes file-backed OAuth credentials
// shortly before expiry. Direct access-token credentials never participate in
// this flow: the CLI must not silently replace a credential supplied through
// BMW_CARDATA_ACCESS_TOKEN or the legacy cardata_access_token config field.
func RefreshCardataAccessTokenIfNeeded(ctx context.Context, cfg *config.Config, now time.Time, target string) error {
	return refreshCardataAccessToken(ctx, cfg, now, target, false)
}

func saveCardataOAuthTokens(cfg *config.Config, clientID, clientSecret string, tok *cardataToken, expiry time.Time) error {
	// The legacy auth_header takes precedence in Config.AuthHeader, so a
	// successful OAuth login must clear it before writing the new token.
	cfg.AuthHeaderVal = ""
	return cfg.SaveTokens(clientID, clientSecret, tok.AccessToken, tok.RefreshToken, expiry)
}

func refreshCardataAccessToken(ctx context.Context, cfg *config.Config, now time.Time, target string, force bool) error {
	if cfg.BmwCardataAccessToken != "" || strings.HasPrefix(cfg.AuthSource, "env:") {
		if force {
			return fmt.Errorf("%w: a direct API token cannot renew the streaming session", ErrCardataLoginRequired)
		}
		return nil
	}
	if !force && (cfg.TokenExpiry.IsZero() || cfg.TokenExpiry.After(now.Add(time.Minute))) {
		return nil
	}
	return withCardataRefreshLock(ctx, cfg.Path, func() error {
		fresh, err := config.Load(cfg.Path)
		if err != nil {
			return fmt.Errorf("reloading OAuth config under lock: %w", err)
		}
		if fresh.BmwCardataAccessToken != "" || strings.HasPrefix(fresh.AuthSource, "env:") {
			*cfg = *fresh
			if force {
				return fmt.Errorf("%w: a direct API token cannot renew the streaming session", ErrCardataLoginRequired)
			}
			return nil
		}
		if err := migrateLegacyCardataSessionLocked(fresh, now); err != nil {
			if force {
				return fmt.Errorf("migrating streaming session: %w", err)
			}
			// An optional old streaming sidecar must not block ordinary API
			// token refresh. Streaming will still refuse unusable sidecars.
			fmt.Fprintln(os.Stderr, "warning: old streaming session could not be migrated; refreshing API credentials anyway")
		}
		if force {
			// Another process may have renewed the streaming identity while
			// this caller waited for the file lock.
			latestSession, _ := loadCardataSession(fresh)
			if validCardataIDToken(latestSession["id_token"], now) &&
				(fresh.TokenExpiry.IsZero() || fresh.TokenExpiry.After(now.Add(time.Minute))) {
				*cfg = *fresh
				return nil
			}
		}
		if !force && (fresh.TokenExpiry.IsZero() || fresh.TokenExpiry.After(now.Add(time.Minute))) {
			*cfg = *fresh
			return nil
		}
		if fresh.ClientID == "" || fresh.RefreshToken == "" {
			return fmt.Errorf("%w: saved OAuth token cannot be refreshed; run 'auth login' again", ErrCardataLoginRequired)
		}
		oldSession, _ := loadCardataSession(fresh)
		protectShared, inspectErr := unprovenExistingCardataSession(fresh)
		if inspectErr != nil {
			// An optional sidecar read error must not disable ordinary API
			// refresh, but the unreadable file must not be overwritten.
			protectShared = true
		}
		tok, err := cardataRefreshToken(ctx, target, fresh.ClientID, fresh.ClientSecret, fresh.RefreshToken)
		if err != nil {
			return fmt.Errorf("refreshing BMW CarData OAuth token: %w", err)
		}
		if tok.RefreshToken == "" {
			tok.RefreshToken = fresh.RefreshToken
		}
		if tok.IDToken == "" && validCardataIDToken(oldSession["id_token"], now) {
			tok.IDToken = oldSession["id_token"]
		}
		if tok.GCID == "" {
			tok.GCID = oldSession["gcid"]
		}
		expiry := now.Add(time.Duration(tok.ExpiresIn) * time.Second)
		if err := saveCardataOAuthTokens(fresh, fresh.ClientID, fresh.ClientSecret, tok, expiry); err != nil {
			return fmt.Errorf("saving refreshed BMW CarData OAuth token: %w", err)
		}
		*cfg = *fresh
		if protectShared {
			fmt.Fprintln(os.Stderr, "warning: existing streaming session could not be linked to this OAuth login; left it in place after API token refresh")
			if force {
				return fmt.Errorf("%w: existing streaming session belongs to another login; run 'auth login' again", ErrCardataLoginRequired)
			}
			return nil
		}
		if err := writeCardataSession(fresh, fresh.ClientID, tok, expiry); err != nil {
			if force {
				return fmt.Errorf("saving refreshed BMW CarData streaming session: %w", err)
			}
			fmt.Fprintf(os.Stderr, "warning: could not update streaming session after token refresh: %v\n", err)
		}
		if force && !validCardataIDToken(tok.IDToken, now) {
			return fmt.Errorf("%w: refreshed OAuth credential has no current streaming ID token; run 'auth login' again", ErrCardataLoginRequired)
		}
		return nil
	})
}

func cardataRefreshToken(ctx context.Context, target, clientID, clientSecret, refreshToken string) (*cardataToken, error) {
	values := url.Values{
		"client_id":     {clientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	if clientSecret != "" {
		values.Set("client_secret", clientSecret)
	}
	body, status, err := cardataPostForm(ctx, target, values)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCardataRefreshUnavailable, err)
	}
	if status != http.StatusOK {
		var response struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &response) == nil {
			switch response.Error {
			case "invalid_grant", "invalid_client", "unauthorized_client":
				return nil, fmt.Errorf("%w: token endpoint rejected the saved credential (HTTP %d)", ErrCardataLoginRequired, status)
			}
		}
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			return nil, fmt.Errorf("%w: token endpoint rejected the saved credential (HTTP %d)", ErrCardataLoginRequired, status)
		}
		if status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= 500 {
			return nil, fmt.Errorf("%w: token endpoint returned HTTP %d", ErrCardataRefreshUnavailable, status)
		}
		return nil, fmt.Errorf("token endpoint returned HTTP %d", status)
	}
	var tok cardataToken
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}
	if tok.AccessToken == "" || tok.ExpiresIn <= 0 {
		return nil, fmt.Errorf("token response missing access_token or expires_in")
	}
	if tok.GCID == "" {
		tok.GCID = gcidFromJWT(tok.IDToken)
		if tok.GCID == "" {
			tok.GCID = gcidFromJWT(tok.AccessToken)
		}
	}
	return &tok, nil
}

// openBrowser opens url in the user's default browser on macOS, Linux, and
// Windows. Returns the underlying error so the caller can warn the user
// instead of silently dropping it.
func openBrowser(rawURL string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name, args = "open", []string{rawURL}
	case "windows":
		// The empty-string arg is required so `start` does not treat the URL
		// as a window title (which breaks for paths with spaces or ampersands).
		name, args = "cmd", []string{"/c", "start", "", rawURL}
	default: // linux, freebsd, openbsd, netbsd, ...
		name, args = "xdg-open", []string{rawURL}
	}
	// PATH lookup: prefer /usr/bin and /usr/local/bin before falling back to
	// exec.LookPath's default $PATH search, so a missing xdg-open on a
	// minimal container returns a clear "not found" rather than a generic
	// "no such file".
	if _, err := exec.LookPath(filepath.Join("/usr/bin", name)); err == nil {
		return exec.Command(filepath.Join("/usr/bin", name), args...).Start()
	}
	if _, err := exec.LookPath(filepath.Join("/usr/local/bin", name)); err == nil {
		return exec.Command(filepath.Join("/usr/local/bin", name), args...).Start()
	}
	return exec.Command(name, args...).Start()
}

// gcidFromJWT extracts a GCID-like subject claim from a JWT payload. Returns
// "" if the token is not a parseable JWT or has no usable claim.
func gcidFromJWT(jwt string) string {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims map[string]any
	if json.Unmarshal(payload, &claims) != nil {
		return ""
	}
	for _, k := range []string{"gcid", "sub", "preferred_username"} {
		if v, ok := claims[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func validCardataIDToken(token string, now time.Time) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Exp <= 0 {
		return false
	}
	return time.Unix(claims.Exp, 0).After(now.Add(time.Minute))
}

// currentCardataStreamSession renews credentials before MQTT connects. The ID
// token has its own expiry, which can differ from the API access token expiry.
func currentCardataStreamSession(ctx context.Context, cfg *config.Config, now time.Time, target string) (map[string]string, error) {
	if cfg.Path != "" {
		fresh, err := config.Load(cfg.Path)
		if err != nil {
			return nil, err
		}
		*cfg = *fresh
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("%w: streaming needs a saved OAuth client ID; run 'auth login'", ErrCardataLoginRequired)
	}
	if cfg.BmwCardataAccessToken != "" || strings.HasPrefix(cfg.AuthSource, "env:") {
		return nil, fmt.Errorf("%w: streaming needs a saved OAuth login, not a direct API token", ErrCardataLoginRequired)
	}
	if err := migrateLegacyCardataSession(ctx, cfg, now); err != nil {
		return nil, err
	}
	session, err := loadCardataSession(cfg)
	needIDToken := err != nil || !validCardataIDToken(session["id_token"], now)
	if err := refreshCardataAccessToken(ctx, cfg, now, target, needIDToken); err != nil {
		return nil, err
	}
	session, err = loadCardataSession(cfg)
	if err != nil || session["gcid"] == "" || !validCardataIDToken(session["id_token"], now) {
		return nil, fmt.Errorf("%w: streaming session needs a current ID token; run 'stream --config <original-alias> <vin>' once if its saved OAuth tokens still match, or run 'auth login' with streaming scope", ErrCardataLoginRequired)
	}
	return session, nil
}

// cardataSessionPath is the pre-migration location beside the selected config
// path. Existing symlink users may have a session here.
func cardataSessionPath(cfg *config.Config) string {
	return filepath.Join(filepath.Dir(cfg.Path), "cardata_session.json")
}

// cardataSharedSessionPath is next to the config's real target. Symlink and
// target users share this file after migration.
func cardataSharedSessionPath(cfg *config.Config) (string, error) {
	target, err := config.CanonicalPath(cfg.Path)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(target), "cardata_session.json"), nil
}

func removeCardataSession(cfg, previous *config.Config) error {
	shared, err := cardataSharedSessionPath(cfg)
	if err != nil {
		return err
	}
	sharedData, readErr := os.ReadFile(shared)
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	sharedSession, sharedOK := parseCardataSession(sharedData)
	sharedOK = sharedOK && savedCardataSessionProof(sharedSession, previous)
	if sharedOK {
		if err := os.Remove(shared); err != nil && !os.IsNotExist(err) {
			return err
		}
	} else if readErr == nil {
		fmt.Fprintln(os.Stderr, "warning: the saved streaming session was left in place because its account could not be verified; review it manually")
	}
	aliases, err := knownCardataLegacySessions(cfg)
	if err != nil {
		return err
	}
	for _, legacy := range aliases {
		// A known alias directory can also hold another config's sidecar.
		// Remove it only when it belongs to the account being cleared.
		data, err := os.ReadFile(legacy)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		legacySession, ok := parseCardataSession(data)
		matchedShared := sharedOK && ok && sameCardataSessionAccount(sharedSession, legacySession)
		matchedSaved := ok && savedCardataSessionProof(legacySession, previous)
		if !matchedShared && !matchedSaved {
			fmt.Fprintln(os.Stderr, "warning: a legacy streaming session beside a config alias was left in place because its account could not be verified; review it manually")
			continue
		}
		if err := os.Remove(legacy); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// Only aliases the user selected or configured can be discovered from a
// target path. Never scan the home directory for arbitrary config symlinks.
func knownCardataLegacySessions(cfg *config.Config) ([]string, error) {
	target, err := config.CanonicalPath(cfg.Path)
	if err != nil {
		return nil, err
	}
	shared := filepath.Join(filepath.Dir(target), "cardata_session.json")
	home, _ := os.UserHomeDir()
	paths := []string{cfg.Path, os.Getenv("BMW_CARDATA_CONFIG")}
	if home != "" {
		paths = append(paths, filepath.Join(home, ".config", "bmw-cardata-pp-cli", "config.toml"))
	}
	seen := map[string]bool{}
	var sessions []string
	for _, configPath := range paths {
		if configPath == "" {
			continue
		}
		resolved, err := config.CanonicalPath(configPath)
		if err != nil || resolved != target {
			continue
		}
		legacy, err := filepath.Abs(filepath.Join(filepath.Dir(configPath), "cardata_session.json"))
		if err != nil {
			return nil, err
		}
		if legacy == shared || seen[legacy] {
			continue
		}
		if canonicalLegacy, err := config.CanonicalPath(legacy); err == nil && canonicalLegacy == shared {
			continue
		}
		seen[legacy] = true
		sessions = append(sessions, legacy)
	}
	sort.Strings(sessions)
	return sessions, nil
}

func parseCardataSession(data []byte) (map[string]string, bool) {
	var session map[string]string
	if json.Unmarshal(data, &session) != nil {
		return nil, false
	}
	return session, true
}

func savedCardataSessionProof(session map[string]string, cfg *config.Config) bool {
	if cfg == nil || cfg.ClientID == "" || session["client_id"] != cfg.ClientID {
		return false
	}
	return (cfg.AccessToken != "" && session["access_token"] != "" && session["access_token"] == cfg.AccessToken) ||
		(cfg.RefreshToken != "" && session["refresh_token"] != "" && session["refresh_token"] == cfg.RefreshToken)
}

func provenCardataSession(session map[string]string, cfg *config.Config) bool {
	return cfg != nil && cfg.BmwCardataAccessToken == "" && !strings.HasPrefix(cfg.AuthSource, "env:") &&
		savedCardataSessionProof(session, cfg)
}

func unprovenExistingCardataSession(cfg *config.Config) (bool, error) {
	shared, err := cardataSharedSessionPath(cfg)
	if err != nil {
		return true, err
	}
	data, err := os.ReadFile(shared)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	session, ok := parseCardataSession(data)
	if !ok {
		return true, nil
	}
	return !savedCardataSessionProof(session, cfg), nil
}

func sameCardataSessionAccount(first, second map[string]string) bool {
	if first["client_id"] == "" || first["client_id"] != second["client_id"] ||
		first["gcid"] == "" || first["gcid"] != second["gcid"] {
		return false
	}
	return (first["access_token"] != "" && first["access_token"] == second["access_token"]) ||
		(first["refresh_token"] != "" && first["refresh_token"] == second["refresh_token"])
}

func retireKnownCardataSessions(cfg *config.Config) error {
	shared, err := cardataSharedSessionPath(cfg)
	if err != nil {
		return err
	}
	sharedData, err := os.ReadFile(shared)
	if err != nil {
		return err
	}
	sharedSession, sharedOK := parseCardataSession(sharedData)
	sharedOK = sharedOK && savedCardataSessionProof(sharedSession, cfg)
	if !sharedOK {
		return fmt.Errorf("saved streaming session does not match the selected OAuth login")
	}
	aliases, err := knownCardataLegacySessions(cfg)
	if err != nil {
		return err
	}
	for _, legacy := range aliases {
		data, err := os.ReadFile(legacy)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		legacySession, ok := parseCardataSession(data)
		if !sharedOK || !ok || !sameCardataSessionAccount(sharedSession, legacySession) {
			fmt.Fprintln(os.Stderr, "warning: a legacy streaming session beside a config alias was left in place because its account could not be verified; review it manually")
			continue
		}
		if err := os.Remove(legacy); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// migrateLegacyCardataSession first checks without a lock, then rechecks and
// copies the legacy sidecar while holding the same lock as token refresh.
func migrateLegacyCardataSession(ctx context.Context, cfg *config.Config, now time.Time) error {
	aliases, err := knownCardataLegacySessions(cfg)
	if err != nil {
		return err
	}
	if len(aliases) == 0 {
		return nil
	}
	found := false
	for _, legacy := range aliases {
		if _, err := os.Stat(legacy); err == nil {
			found = true
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if !found {
		return nil
	}
	return withCardataRefreshLock(ctx, cfg.Path, func() error {
		fresh, err := config.Load(cfg.Path)
		if err != nil {
			return err
		}
		if err := migrateLegacyCardataSessionLocked(fresh, now); err != nil {
			return err
		}
		*cfg = *fresh
		return nil
	})
}

func migrateLegacyCardataSessionLocked(cfg *config.Config, now time.Time) error {
	if cfg.ClientID == "" {
		// A sidecar cannot be linked to an OAuth account without its client ID.
		return nil
	}
	shared, err := cardataSharedSessionPath(cfg)
	if err != nil {
		return err
	}
	aliases, err := knownCardataLegacySessions(cfg)
	if err != nil {
		return err
	}
	selectedLegacy, err := filepath.Abs(cardataSessionPath(cfg))
	if err != nil {
		return err
	}
	sharedData, err := os.ReadFile(shared)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		sharedSession, ok := parseCardataSession(sharedData)
		if !ok || !savedCardataSessionProof(sharedSession, cfg) {
			return fmt.Errorf("%w: existing shared streaming session does not match the selected OAuth login; run 'auth login' again to replace it", ErrCardataLoginRequired)
		}
	}
	if usableCardataSession(sharedData, cfg, now) {
		return retireKnownCardataSessions(cfg)
	}
	var selected []byte
	var selectedIDToken, selectedGCID string
	for _, legacy := range aliases {
		data, err := os.ReadFile(legacy)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !usableCardataSession(data, cfg, now) {
			if legacy == selectedLegacy {
				return fmt.Errorf("%w: streaming session beside the selected config alias is invalid or belongs to another login", ErrCardataLoginRequired)
			}
			continue
		}
		session, _ := parseCardataSession(data)
		if selected != nil && (session["id_token"] != selectedIDToken || session["gcid"] != selectedGCID) {
			return fmt.Errorf("%w: conflicting valid streaming sessions beside config aliases; run 'auth login' again to create one shared session", ErrCardataLoginRequired)
		}
		if selected == nil {
			selected = data
			selectedIDToken = session["id_token"]
			selectedGCID = session["gcid"]
		}
	}
	if selected == nil {
		return nil
	}
	if err := config.WritePrivateFile(shared, selected); err != nil {
		return err
	}
	return retireKnownCardataSessions(cfg)
}

func usableCardataSession(data []byte, cfg *config.Config, now time.Time) bool {
	session, ok := parseCardataSession(data)
	if !ok || !provenCardataSession(session, cfg) {
		return false
	}
	return session["gcid"] != "" && validCardataIDToken(session["id_token"], now)
}

// writeCardataSession persists the streaming credentials (GCID + id_token)
// needed by the MQTT stream alongside the config file.
func writeCardataSession(cfg *config.Config, clientID string, tok *cardataToken, expiry time.Time) error {
	sess := map[string]any{
		"client_id":     clientID,
		"gcid":          tok.GCID,
		"id_token":      tok.IDToken,
		"access_token":  tok.AccessToken,
		"refresh_token": tok.RefreshToken,
		"expires_at":    expiry.Format(time.RFC3339),
	}
	buf, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	shared, err := cardataSharedSessionPath(cfg)
	if err != nil {
		return err
	}
	if err := config.WritePrivateFile(shared, buf); err != nil {
		return err
	}
	return retireKnownCardataSessions(cfg)
}

// loadCardataSession reads only the shared sidecar. Legacy alias data must be
// migrated under the refresh lock before any streaming identity is reused.
func loadCardataSession(cfg *config.Config) (map[string]string, error) {
	shared, err := cardataSharedSessionPath(cfg)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(shared)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for k, v := range raw {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	if !provenCardataSession(out, cfg) {
		return nil, fmt.Errorf("streaming session does not match the saved OAuth login")
	}
	return out, nil
}

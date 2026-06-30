// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	cardataTokenURL      = "https://customer.bmwgroup.com/gcdm/oauth/token"
	cardataDefaultScope  = "authenticate_user openid cardata:api:read cardata:streaming:read"
)

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
				if err := cfg.SaveTokens(clientID, "", tok.AccessToken, tok.RefreshToken, expiry); err != nil {
					return configErr(fmt.Errorf("saving tokens: %w", err))
				}
				if err := writeCardataSession(cfg, clientID, tok, expiry); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not save streaming session: %v\n", err)
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
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
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
		return nil, fmt.Errorf("device-code endpoint returned HTTP %d: %s", status, truncateBody(body))
	}
	var dc cardataDeviceCode
	if err := json.Unmarshal(body, &dc); err != nil {
		return nil, fmt.Errorf("parsing device-code response: %w", err)
	}
	if dc.DeviceCode == "" {
		return nil, fmt.Errorf("device-code response missing device_code: %s", truncateBody(body))
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
	body, status, err := cardataPostForm(ctx, cardataTokenURL, vals)
	if err != nil {
		return nil, err
	}
	// Error envelope: {"error":"authorization_pending" | "slow_down" | ...}
	var ee struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if json.Unmarshal(body, &ee) == nil && ee.Error != "" {
		return nil, fmt.Errorf("%s: %s (HTTP %d)", ee.Error, ee.ErrorDescription, status)
	}
	if status != 200 {
		return nil, fmt.Errorf("token endpoint returned HTTP %d: %s", status, truncateBody(body))
	}
	var tok cardataToken
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("token response missing access_token: %s", truncateBody(body))
	}
	if tok.GCID == "" {
		tok.GCID = gcidFromJWT(tok.IDToken)
		if tok.GCID == "" {
			tok.GCID = gcidFromJWT(tok.AccessToken)
		}
	}
	return &tok, nil
}

func truncateBody(b []byte) string {
	if len(b) > 300 {
		return string(b[:300]) + "..."
	}
	return string(b)
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

// cardataSessionPath returns the sidecar session file path next to the config.
func cardataSessionPath(cfg *config.Config) string {
	return filepath.Join(filepath.Dir(cfg.Path), "cardata_session.json")
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
	return os.WriteFile(cardataSessionPath(cfg), buf, 0o600)
}

// loadCardataSession reads the streaming session sidecar.
func loadCardataSession(cfg *config.Config) (map[string]string, error) {
	data, err := os.ReadFile(cardataSessionPath(cfg))
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
	return out, nil
}

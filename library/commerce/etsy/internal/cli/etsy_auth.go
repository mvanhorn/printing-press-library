package cli

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/config"
)

const etsyAuthorizeURL = "https://www.etsy.com/oauth/connect"

func etsyDefaultScopes() []string {
	return []string{"listings_r", "shops_r", "transactions_r"}
}

func etsyKeystring(apiKey string) string {
	key, _, _ := strings.Cut(apiKey, ":")
	return strings.TrimSpace(key)
}

func etsyDefaultClientID(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return etsyKeystring(cfg.EtsyApiKey)
}

func buildEtsyAuthorizeURL(authBase, clientID, redirectURI, state, codeChallenge string, scopes []string) string {
	if authBase == "" {
		authBase = etsyAuthorizeURL
	}
	params := url.Values{"client_id": {clientID}, "response_type": {"code"}, "state": {state}}
	if codeChallenge != "" {
		params.Set("code_challenge", codeChallenge)
		params.Set("code_challenge_method", "S256")
	}
	etsySetScopeParam(params, scopes)
	if redirectURI != "" {
		params.Set("redirect_uri", redirectURI)
	}
	return authBase + "?" + cliutil.EncodeQuery(params)
}

// Etsy's registered HTTPS redirects cannot be served by the loopback listener.
// Keep this provider-specific print-and-paste protocol in the extension.
type etsyPendingLogin struct {
	State        string    `json:"state"`
	CodeVerifier string    `json:"code_verifier"`
	ClientID     string    `json:"client_id"`
	RedirectURI  string    `json:"redirect_uri"`
	Scopes       []string  `json:"scopes"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type etsyLoginOptions struct {
	PendingPath, RedirectURI, ClientID, TokenURL string
	Scopes                                       []string
	Config                                       *config.Config
	HTTPClient                                   *http.Client
	Now                                          func() time.Time
	DryRun                                       bool
}

type etsyLoginStart struct {
	AuthorizeURL string    `json:"authorize_url"`
	ExpiresAt    time.Time `json:"expires_at"`
	RedirectURI  string    `json:"redirect_uri"`
	Scopes       []string  `json:"scopes"`
	Next         string    `json:"next"`
}

type etsyLoginComplete struct {
	Authenticated bool      `json:"authenticated"`
	ExpiresAt     time.Time `json:"expires_at"`
	Scopes        []string  `json:"scopes"`
}

func validateEtsyRedirect(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return usageErr(fmt.Errorf("invalid redirect URI"))
	}
	if u.Hostname() == "" {
		return usageErr(fmt.Errorf("redirect URI requires a non-empty host"))
	}
	if strings.Contains(raw, "#") {
		return usageErr(fmt.Errorf("redirect URI must not contain a fragment"))
	}
	host := strings.ToLower(u.Hostname())
	if u.Scheme != "https" && !(u.Scheme == "http" && (host == "localhost" || host == "127.0.0.1" || host == "::1")) {
		return usageErr(fmt.Errorf("redirect URI requires https (http is allowed only for loopback hosts)"))
	}
	return nil
}

func etsyStartLogin(o etsyLoginOptions) (*etsyLoginStart, error) {
	if err := validateEtsyRedirect(o.RedirectURI); err != nil {
		return nil, err
	}
	if o.ClientID == "" {
		return nil, usageErr(fmt.Errorf("client id required: set ETSY_API_KEY (keystring) or --client-id"))
	}
	state := make([]byte, 32)
	if _, err := rand.Read(state); err != nil {
		return nil, err
	}
	verifier, err := generatePKCEVerifier()
	if err != nil {
		return nil, err
	}
	now := o.Now().UTC()
	scopes := append(make([]string, 0), o.Scopes...)
	pending := etsyPendingLogin{hex.EncodeToString(state), verifier, o.ClientID, o.RedirectURI, scopes, now, now.Add(10 * time.Minute)}
	if !o.DryRun {
		data, err := json.Marshal(pending)
		if err != nil {
			return nil, err
		}
		if err := cliutil.AtomicWritePrivateFile(o.PendingPath, data, 0600, 0700); err != nil {
			return nil, err
		}
	}
	return &etsyLoginStart{buildEtsyAuthorizeURL(o.Config.AuthorizationURL, o.ClientID, o.RedirectURI, pending.State, pkceCodeChallengeS256(verifier), scopes), pending.ExpiresAt, o.RedirectURI, scopes, `etsy-pp-cli auth login --callback-url "<full redirected URL>"`}, nil
}

func etsyCompleteLogin(ctx context.Context, o etsyLoginOptions, callback string) (*etsyLoginComplete, error) {
	data, err := os.ReadFile(o.PendingPath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("no pending login: run `etsy-pp-cli auth login --redirect-uri URL` first")
	}
	if err != nil {
		return nil, err
	}
	var p etsyPendingLogin
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("invalid pending login; start again")
	}
	if !o.Now().Before(p.ExpiresAt) {
		if err := os.Remove(o.PendingPath); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("pending login expired (10 minute limit); start again")
	}
	if o.RedirectURI != "" && o.RedirectURI != p.RedirectURI {
		return nil, usageErr(fmt.Errorf("redirect URI differs from pending login"))
	}
	u, err := url.Parse(callback)
	if err != nil {
		return nil, usageErr(fmt.Errorf("invalid callback URL"))
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, usageErr(fmt.Errorf("invalid callback query"))
	}
	if q.Has("error") {
		if err := os.Remove(o.PendingPath); err != nil {
			return nil, err
		}
		// Redact known credentials even if the provider reflects them in an error.
		message := etsySanitizeCallbackText(q.Get("error")) + ": " + etsySanitizeCallbackText(q.Get("error_description"))
		for _, secret := range []string{p.CodeVerifier, q.Get("code")} {
			if secret != "" {
				message = strings.ReplaceAll(message, secret, "[redacted]")
			}
		}
		return nil, fmt.Errorf("authorization error: %s", message)
	}
	if p.State == "" || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(p.State)) != 1 {
		return nil, fmt.Errorf("state mismatch: the callback URL does not belong to the pending login (possible CSRF or a stale URL)")
	}
	if q.Get("code") == "" {
		return nil, usageErr(fmt.Errorf("missing code in callback URL"))
	}
	if o.DryRun {
		return &etsyLoginComplete{false, time.Time{}, append(make([]string, 0), p.Scopes...)}, nil
	}
	tokenURL := o.TokenURL
	if tokenURL == "" {
		tokenURL = "https://openapi.etsy.com/v3/public/oauth/token" // #nosec G101 -- public Etsy OAuth token endpoint URL, not a credential.
	}
	exchangeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	token, exchangeErr := exchangeAuthorizationCode(exchangeCtx, o.HTTPClient, tokenURL, url.Values{
		"grant_type": {"authorization_code"}, "code": {q.Get("code")}, "redirect_uri": {p.RedirectURI}, "client_id": {p.ClientID}, "code_verifier": {p.CodeVerifier},
	})
	// Do not relay provider bodies or transport errors that could echo the code.
	removeErr := os.Remove(o.PendingPath)
	if exchangeErr != nil {
		return nil, fmt.Errorf("token exchange failed; authorization codes are single use; start again")
	}
	expiry := time.Time{}
	if token.ExpiresIn > 0 {
		expiry = o.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	}
	o.Config.MarkCredentialsExplicit(false, false)
	if err := o.Config.SaveTokens(p.ClientID, "", token.AccessToken, token.RefreshToken, expiry); err != nil {
		return nil, fmt.Errorf("saving tokens: %w", err)
	}
	if removeErr != nil {
		return nil, removeErr
	}
	return &etsyLoginComplete{true, expiry, append(make([]string, 0), p.Scopes...)}, nil
}

func runEtsyRedirectLogin(cmd *cobra.Command, flags *rootFlags, clientID, redirect, callback string, allScopes bool, scopes []string) (bool, error) {
	if redirect != "" && callback != "" {
		return true, usageErr(fmt.Errorf("--redirect-uri and --callback-url cannot be used together"))
	}
	if callback == "" && redirect == "" {
		redirect = os.Getenv("ETSY_REDIRECT_URI")
	}
	if callback == "" && redirect == "" {
		return false, nil
	}
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return true, err
	}
	if clientID == "" {
		clientID = etsyDefaultClientID(cfg)
	}
	if clientID == "" {
		clientID = cfg.ClientID
	}
	if clientID == "" && requiredInputWaived() {
		clientID = "dry-run"
	}
	scopes = etsyRequestedScopes(allScopes, scopes)
	// Harness callbacks must neither read/delete pending files nor dial.
	if callback != "" && requiredInputWaived() {
		return true, printJSONFiltered(cmd.OutOrStdout(), map[string]any{"authenticated": false, "dry_run": true}, flags)
	}
	dir, err := cliutil.StateDir()
	if err != nil {
		return true, err
	}
	o := etsyLoginOptions{PendingPath: filepath.Join(dir, "etsy-oauth-pending.json"), RedirectURI: redirect, ClientID: clientID, Scopes: scopes, Config: cfg, Now: time.Now, DryRun: dryRunOK(flags) || requiredInputWaived(), TokenURL: cfg.TokenURL}
	if callback != "" {
		o.HTTPClient, err = flags.oauthHTTPClient(cfg)
		if err != nil {
			return true, err
		}
		result, err := etsyCompleteLogin(cmd.Context(), o, callback)
		if err != nil {
			return true, err
		}
		if flags.asJSON {
			return true, printJSONFiltered(cmd.OutOrStdout(), result, flags)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "OK Authentication successful! Token expires at %s\n", result.ExpiresAt.Format(time.RFC3339))
		return true, nil
	}
	result, err := etsyStartLogin(o)
	if err != nil {
		return true, err
	}
	if flags.asJSON {
		return true, printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}
	fmt.Fprintln(cmd.OutOrStdout(), result.AuthorizeURL)
	fmt.Fprintln(cmd.ErrOrStderr(), "Open the URL above, approve access, then run:\n"+result.Next)
	return true, nil
}

// etsySanitizeCallbackText bounds provider-controlled text and removes terminal and bidi controls.
func etsySanitizeCallbackText(text string) string {
	var out strings.Builder
	n := 0
	for _, r := range text {
		if r == '\t' || r == '\n' || r == '\r' {
			r = ' '
		} else if r < 0x20 || (r >= 0x7f && r <= 0x9f) ||
			r == 0x00ad || r == 0x061c || r == 0xfeff ||
			(r >= 0x2060 && r <= 0x2064) ||
			(r >= 0x200b && r <= 0x200f) ||
			(r >= 0x202a && r <= 0x202e) ||
			(r >= 0x2066 && r <= 0x2069) {
			continue
		}
		if n == 200 {
			break
		}
		out.WriteRune(r)
		n++
	}
	return out.String()
}

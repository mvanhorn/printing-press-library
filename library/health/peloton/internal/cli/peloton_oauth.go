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
}

type pelotonTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
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
		cmd.Short = "Manage Peloton's private OAuth token bundle"
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
		Short: "Show managed OAuth bootstrap requirements",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), "The CLI includes Peloton's public OAuth client configuration.")
			fmt.Fprintln(cmd.OutOrStdout(), "Use the environment's private credential wrapper for bootstrap credentials.")
			fmt.Fprintln(cmd.OutOrStdout(), "The first live request creates an owner-only token bundle; later requests reuse or refresh it.")
		},
	}
}

func newManagedOAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show managed OAuth bundle status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			bundle, err := loadOAuthBundle()
			if os.IsNotExist(err) {
				fmt.Fprintln(cmd.OutOrStdout(), "Managed OAuth bundle is not initialized; the next live request will bootstrap it.")
				return nil
			}
			if err != nil {
				return authErr(fmt.Errorf("reading managed Peloton OAuth token: %w", err))
			}
			state := "expired; the next live request will refresh it"
			if bundle.AccessToken != "" && bundle.ExpiresAt.After(oauthNow().Add(oauthExpirySkew)) {
				state = "available"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Managed OAuth bundle is %s.\n", state)
			return nil
		},
	}
}

func newManagedOAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the private managed OAuth token bundle",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := oauthBundlePath()
			if err != nil {
				return authErr(fmt.Errorf("locating managed Peloton OAuth token: %w", err))
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return authErr(fmt.Errorf("removing managed Peloton OAuth token: %w", err))
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Managed OAuth bundle removed.")
			return nil
		},
	}
}

// installManagedPelotonBearer injects only an in-memory managed bearer token.
// It deliberately ignores legacy persisted auth headers and cookie state.
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

// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// OAuth client-credentials support. When no API access token is configured
// but TAILSCALE_OAUTH_CLIENT_ID and TAILSCALE_OAUTH_CLIENT_SECRET are set,
// exchange them at <base>/oauth/token for a short-lived bearer token. The
// token lives in memory for the process only; it is never written to disk.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/client"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/config"
)

type tsOAuthToken struct {
	token  string
	expiry time.Time
}

// tsOAuthState caches minted tokens per token endpoint and credential, so two
// configurations in one process never share a token. The secret is stored
// only as a SHA-256 digest inside the cache key.
var tsOAuthState struct {
	sync.Mutex
	tokens map[string]tsOAuthToken
}

func tsOAuthCacheKey(tokenURL, id, secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return tokenURL + "\x00" + id + "\x00" + hex.EncodeToString(sum[:])
}

func tsOAuthCredentials() (id, secret string) {
	return strings.TrimSpace(cliutil.EnvOverride("TAILSCALE_OAUTH_CLIENT_ID")), strings.TrimSpace(cliutil.EnvOverride("TAILSCALE_OAUTH_CLIENT_SECRET"))
}

// tsEnsureOAuth mints an OAuth token into cfg.AccessToken when the config has
// no API key or stored token and OAuth client credentials are set. hc is the
// generated client's HTTP client so the secret gets its redirect protections.
func tsEnsureOAuth(ctx context.Context, cfg *config.Config, hc *http.Client) error {
	if cfg == nil {
		return nil
	}
	if strings.TrimSpace(cfg.TailscaleApiKey) != "" || strings.TrimSpace(cfg.AccessToken) != "" || strings.TrimSpace(cfg.AuthHeaderVal) != "" {
		return nil
	}
	id, secret := tsOAuthCredentials()
	if id == "" || secret == "" {
		return nil
	}
	tok, err := tsMintOAuthToken(ctx, hc, cfg.BaseURL, id, secret)
	if err != nil {
		return authErr(fmt.Errorf("exchanging TAILSCALE_OAUTH_CLIENT_ID/SECRET for an access token: %w", err))
	}
	cfg.AccessToken = tok
	cfg.AuthSource = "oauth:TAILSCALE_OAUTH_CLIENT_ID"
	return nil
}

func tsMintOAuthToken(ctx context.Context, hc *http.Client, baseURL, id, secret string) (string, error) {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.tailscale.com/api/v2"
	}
	tokenURL := strings.TrimSuffix(baseURL, "/") + "/oauth/token"
	key := tsOAuthCacheKey(tokenURL, id, secret)
	tsOAuthState.Lock()
	defer tsOAuthState.Unlock()
	if cached, ok := tsOAuthState.tokens[key]; ok && time.Now().Before(cached.expiry.Add(-60*time.Second)) {
		return cached.token, nil
	}
	form := url.Values{}
	form.Set("client_id", id)
	form.Set("client_secret", secret)
	form.Set("grant_type", "client_credentials")
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := hc.Do(req) // #nosec G107 -- URL derived from configured API base.
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		_ = json.Unmarshal(body, &e)
		msg := cliutil.SanitizeErrorBody(e.Message)
		if msg == "" {
			msg = cliutil.SanitizeErrorBody(e.Error)
		}
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return "", fmt.Errorf("token endpoint returned HTTP %d: %s", resp.StatusCode, msg)
	}
	var tr struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tr); err != nil || tr.AccessToken == "" {
		return "", fmt.Errorf("token endpoint returned no access_token")
	}
	if tr.ExpiresIn <= 0 {
		tr.ExpiresIn = 3600
	}
	if tsOAuthState.tokens == nil {
		tsOAuthState.tokens = map[string]tsOAuthToken{}
	}
	tsOAuthState.tokens[key] = tsOAuthToken{token: tr.AccessToken, expiry: time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)}
	return tr.AccessToken, nil
}

func init() {
	// Generated endpoint commands build their client through newClient,
	// which runs these hooks. Dry-run and verify runs skip the exchange so
	// they never make a network call.
	registerClientHook(func(c *client.Client) error {
		if c == nil || c.DryRun || cliutil.IsVerifyEnv() {
			return nil
		}
		return tsEnsureOAuth(context.Background(), c.Config, c.HTTPClient)
	})
}

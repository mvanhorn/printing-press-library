// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.
package snap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/durianpay/internal/cliutil"
)

// Client is the SNAP transport: it mints/caches B2B tokens and signs every
// transaction request. It is a sibling hand-written client, so it carries its
// own AdaptiveLimiter and surfaces *cliutil.RateLimitError on throttling.
type Client struct {
	cfg      *Config
	http     *http.Client
	limiter  *cliutil.AdaptiveLimiter
	tokenDir string // cache directory; "" disables caching (tests)
	DryRun   bool
	now      func() time.Time
}

// NewClient builds a SNAP client over the resolved config.
func NewClient(cfg *Config) *Client {
	cacheDir, _ := os.UserConfigDir()
	if cacheDir != "" {
		cacheDir = filepath.Join(cacheDir, "durianpay-pp-cli")
	}
	return &Client{
		cfg:      cfg,
		http:     &http.Client{Timeout: 30 * time.Second},
		limiter:  cliutil.NewAdaptiveLimiter(5),
		tokenDir: cacheDir,
		now:      time.Now,
	}
}

// Config exposes the client's resolved configuration (for debug output).
func (c *Client) Config() *Config { return c.cfg }

// Token is a minted B2B access token with cache metadata.
type Token struct {
	AccessToken string    `json:"access_token"`
	MintedAt    time.Time `json:"minted_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	Env         string    `json:"env"` // base URL it was minted against
}

func (c *Client) tokenCachePath() string {
	if c.tokenDir == "" {
		return ""
	}
	return filepath.Join(c.tokenDir, "snap-token.json")
}

// CachedToken returns the cached token if present, plus validity.
func (c *Client) CachedToken() (*Token, bool) {
	p := c.tokenCachePath()
	if p == "" {
		return nil, false
	}
	// #nosec G304 -- p is the token cache path built by tokenCachePath() from
	// the configured token dir and a fixed filename, not arbitrary user input.
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	var t Token
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, false
	}
	valid := t.Env == c.cfg.BaseURL && c.now().Before(t.ExpiresAt.Add(-30*time.Second))
	return &t, valid
}

func (c *Client) storeToken(t *Token) {
	p := c.tokenCachePath()
	if p == "" {
		return
	}
	_ = os.MkdirAll(c.tokenDir, 0o700)
	// Token cache is written 0600 inside a 0700 dir; no gosec suppression needed.
	data, _ := json.Marshal(t)
	_ = os.WriteFile(p, data, 0o600)
}

// tokenResponse is the /access-token/b2b response envelope.
type tokenResponse struct {
	ResponseCode    string `json:"responseCode"`
	ResponseMessage string `json:"responseMessage"`
	AccessToken     string `json:"accessToken"`
	TokenType       string `json:"tokenType"`
	ExpiresIn       string `json:"expiresIn"`
}

// MintToken signs and posts the B2B access-token request, caching the result.
func (c *Client) MintToken(ctx context.Context) (*Token, error) {
	if missing := c.cfg.MissingCredentials(); len(missing) > 0 {
		return nil, fmt.Errorf("%w: set %s", ErrMissingCredentials, strings.Join(missing, ", "))
	}
	ts := Timestamp(c.now())
	sig, err := SignTokenRequest(c.cfg.PrivateKey, c.cfg.ClientKey, ts)
	if err != nil {
		return nil, err
	}
	body := []byte(`{"grantType":"AUTHORIZATION_CODE"}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/access-token/b2b", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-TIMESTAMP", ts)
	req.Header.Set("X-SIGNATURE", sig)
	req.Header.Set("X-CLIENT-KEY", c.cfg.ClientKey)

	raw, status, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	var tr tokenResponse
	if err := json.Unmarshal(raw, &tr); err != nil {
		return nil, fmt.Errorf("parsing token response (HTTP %d): %w", status, err)
	}
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("token mint failed (HTTP %d, responseCode %s): %s", status, tr.ResponseCode, tr.ResponseMessage)
	}
	minted := c.now()
	expires := minted.Add(900 * time.Second)
	// expiresIn is normally an integer count of seconds (SNAP spec); parse that
	// first. Some environments echo a Go time.String() timestamp instead, so fall
	// back to that layout. Default to 900s when neither parses.
	if n, perr := strconv.Atoi(strings.TrimSpace(tr.ExpiresIn)); perr == nil {
		expires = minted.Add(time.Duration(n) * time.Second)
	} else if t, perr := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", tr.ExpiresIn); perr == nil {
		expires = t
	}
	tok := &Token{AccessToken: tr.AccessToken, MintedAt: minted, ExpiresAt: expires, Env: c.cfg.BaseURL}
	c.storeToken(tok)
	return tok, nil
}

// Token returns a valid access token, minting a fresh one when the cache is
// absent, expired, or minted against a different environment.
func (c *Client) Token(ctx context.Context) (string, error) {
	if t, valid := c.CachedToken(); valid {
		return t.AccessToken, nil
	}
	t, err := c.MintToken(ctx)
	if err != nil {
		return "", err
	}
	return t.AccessToken, nil
}

// SignedRequest captures everything Do computes, for --dry-run and snap sign --debug.
type SignedRequest struct {
	Method       string            `json:"method"`
	URL          string            `json:"url"`
	Path         string            `json:"path"`
	Timestamp    string            `json:"timestamp"`
	MinifiedBody string            `json:"minified_body"`
	BodySHA256   string            `json:"body_sha256"`
	StringToSign string            `json:"string_to_sign"`
	Signature    string            `json:"signature"`
	Headers      map[string]string `json:"headers"`
}

// Prepare builds the fully signed request metadata for a transaction call
// without sending it. path must include the /v1.0 prefix (e.g. /v1.0/balance-inquiry).
func (c *Client) Prepare(ctx context.Context, method, path string, body []byte, externalID string) (*SignedRequest, error) {
	token, err := c.Token(ctx)
	if err != nil {
		return nil, err
	}
	return c.prepareWithToken(method, path, body, externalID, token), nil
}

// PrepareOffline builds signing metadata with a caller-supplied token (no network).
func (c *Client) PrepareOffline(method, path string, body []byte, externalID, token string) *SignedRequest {
	return c.prepareWithToken(method, path, body, externalID, token)
}

func (c *Client) prepareWithToken(method, path string, body []byte, externalID, token string) *SignedRequest {
	ts := Timestamp(c.now())
	minified := MinifyJSON(body)
	bodyHash := BodyHash(body)
	sts := StringToSign(method, path, token, body, ts)
	sig := SignTransaction(c.cfg.ClientSecret, sts)
	if externalID == "" {
		externalID = ExternalID()
	}
	base := strings.TrimSuffix(c.cfg.BaseURL, "/v1.0")
	return &SignedRequest{
		Method:       strings.ToUpper(method),
		URL:          base + path,
		Path:         path,
		Timestamp:    ts,
		MinifiedBody: string(minified),
		BodySHA256:   bodyHash,
		StringToSign: sts,
		Signature:    sig,
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"X-TIMESTAMP":   ts,
			"X-SIGNATURE":   sig,
			"X-PARTNER-ID":  c.cfg.PartnerID,
			"X-EXTERNAL-ID": externalID,
			"CHANNEL-ID":    c.cfg.ChannelID,
			"Authorization": "Bearer " + token,
		},
	}
}

// Do signs and sends a SNAP transaction request. path includes the /v1.0
// prefix. Returns the raw response body. In DryRun mode it prints the would-be
// request and returns the signing metadata instead of calling the API.
// doRequest sends one prepared request through the adaptive limiter (no retry,
// no re-signing). Used by the single-shot token mint.
func (c *Client) doRequest(req *http.Request) (json.RawMessage, int, error) {
	c.limiter.Wait()
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("snap request %s %s: %w", req.Method, displayPath(req.URL), err)
	}
	data, rerr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	_ = resp.Body.Close()
	if rerr != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading snap response: %w", rerr)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return nil, resp.StatusCode, &cliutil.RateLimitError{URL: displayPath(req.URL), RetryAfter: cliutil.RetryAfter(resp), Body: string(data)}
	}
	c.limiter.OnSuccess()
	return data, resp.StatusCode, nil
}

func (c *Client) Do(ctx context.Context, method, path string, body []byte, externalID string) (json.RawMessage, int, error) {
	if missing := c.cfg.MissingCredentials(); len(missing) > 0 {
		return nil, 0, fmt.Errorf("%w: set %s", ErrMissingCredentials, strings.Join(missing, ", "))
	}
	if c.DryRun {
		sr := c.prepareWithToken(method, path, body, externalID, "<token>")
		out, _ := json.MarshalIndent(map[string]any{"dry_run": true, "would_send": sr}, "", "  ")
		return out, 0, nil
	}
	// Re-sign on every attempt so a 429 retry never replays a stale
	// X-TIMESTAMP/X-SIGNATURE: each attempt mints a fresh signature (and a
	// fresh X-EXTERNAL-ID when caller-unset), keeping the request inside the
	// SNAP timestamp tolerance regardless of cumulative backoff.
	minified := MinifyJSON(body)
	for attempt := 0; attempt < 4; attempt++ {
		c.limiter.Wait()
		sr, err := c.Prepare(ctx, method, path, body, externalID)
		if err != nil {
			return nil, 0, err
		}
		req, err := http.NewRequestWithContext(ctx, sr.Method, sr.URL, bytes.NewReader(minified))
		if err != nil {
			return nil, 0, err
		}
		for k, v := range sr.Headers {
			req.Header.Set(k, v)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, 0, fmt.Errorf("snap request %s %s: %w", req.Method, displayPath(req.URL), err)
		}
		data, rerr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		_ = resp.Body.Close()
		if rerr != nil {
			return nil, resp.StatusCode, fmt.Errorf("reading snap response: %w", rerr)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
			if attempt < 3 {
				time.Sleep(cliutil.Backoff(attempt))
				continue
			}
			return nil, resp.StatusCode, &cliutil.RateLimitError{URL: displayPath(req.URL), RetryAfter: cliutil.RetryAfter(resp), Body: string(data)}
		}
		c.limiter.OnSuccess()
		return data, resp.StatusCode, nil
	}
	return nil, 0, &cliutil.RateLimitError{URL: path}
}

func displayPath(u *url.URL) string {
	if u == nil {
		return ""
	}
	return u.Path
}

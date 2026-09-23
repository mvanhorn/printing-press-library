// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.

// Package zimmo holds the hand-written Zimmo domain layer: the anonymous
// token, the search filter builder, the listing flattener and a small typed
// client over Zimmo's JSON micro-services.
package zimmo

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/cliutil"
)

// Service hosts discovered in the site's __ZIMMO_ENV__ config.
const (
	SearchHost = "https://search-api.zimmo.be"
	GeoHost    = "https://geo-api.zimmo.be"
	ScoreHost  = "https://score-api.zimmo.be"
	UserHost   = "https://user-api.zimmo.be"
	SiteHost   = "https://www.zimmo.be"
)

// UserAgent is a desktop Chrome UA; Zimmo's APIs accept Go's TLS stack.
const UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Safari/537.36"

// cachedToken is the on-disk shape of the anonymous token cache.
type cachedToken struct {
	Token     string    `json:"token"`
	DeviceID  string    `json:"device_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

var (
	tokenMu  sync.Mutex
	memToken *cachedToken
	// tokenLimiter paces token mints (one per 24h in practice).
	tokenLimiter = cliutil.NewAdaptiveLimiter(1)
)

// TokenPath is where the anonymous token is cached (0600).
func TokenPath() string {
	dir, err := cliutil.CacheDir()
	if err != nil || dir == "" {
		base, herr := os.UserHomeDir()
		if herr != nil || base == "" {
			base = os.TempDir()
		} else {
			base = filepath.Join(base, ".cache")
		}
		dir = filepath.Join(base, "zimmo-pp-cli")
	}
	return filepath.Join(dir, "anonymous-token.json")
}

// jwtExpiry reads the exp claim without verifying the signature; the
// server is the authority, this only decides when to mint a new one.
func jwtExpiry(tok string) time.Time {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return time.Time{}
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(raw, &claims) != nil || claims.Exp == 0 {
		return time.Time{}
	}
	return time.Unix(claims.Exp, 0)
}

func (t *cachedToken) valid(now time.Time) bool {
	return t != nil && t.Token != "" && now.Add(10*time.Minute).Before(t.ExpiresAt)
}

func newDeviceID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// AnonymousToken returns a valid anonymous JWT, minting one from user-api
// when the cached token is missing or about to expire.
func AnonymousToken(ctx context.Context, hc *http.Client) (string, error) {
	tokenMu.Lock()
	defer tokenMu.Unlock()
	now := time.Now()
	if memToken.valid(now) {
		return memToken.Token, nil
	}
	path := TokenPath()
	if b, err := os.ReadFile(filepath.Clean(path)); err == nil { // #nosec G304 -- fixed cache path built from the user cache dir, not user input.
		var t cachedToken
		if json.Unmarshal(b, &t) == nil && t.valid(now) {
			memToken = &t
			return t.Token, nil
		}
	}
	deviceID := ""
	if memToken != nil {
		deviceID = memToken.DeviceID
	}
	if deviceID == "" {
		deviceID = newDeviceID()
	}
	t, err := mintToken(ctx, hc, deviceID)
	if err != nil {
		return "", err
	}
	memToken = t
	if b, err := json.Marshal(t); err == nil {
		_ = cliutil.AtomicWritePrivateFile(path, b, 0o600, 0o700)
	}
	return t.Token, nil
}

// InvalidateToken drops the cached token so the next call mints a new one.
func InvalidateToken() {
	tokenMu.Lock()
	defer tokenMu.Unlock()
	memToken = nil
	_ = os.Remove(TokenPath())
}

func mintToken(ctx context.Context, hc *http.Client, deviceID string) (*cachedToken, error) {
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	body, _ := json.Marshal(map[string]string{
		"deviceName": "zimmo-pp-cli",
		"deviceId":   deviceID,
		"locale":     "fr",
		"channel":    "web",
	})
	if err := tokenLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, UserHost+"/token/anonymous", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	setCommonHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("minting Zimmo anonymous token: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusTooManyRequests {
		tokenLimiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: UserHost + "/token/anonymous", RetryAfter: cliutil.RetryAfter(resp), Body: truncate(string(raw), 200)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("minting Zimmo anonymous token: HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.Token == "" {
		return nil, fmt.Errorf("minting Zimmo anonymous token: unexpected response")
	}
	exp := jwtExpiry(out.Token)
	if exp.IsZero() {
		exp = time.Now().Add(12 * time.Hour)
	}
	return &cachedToken{Token: out.Token, DeviceID: deviceID, ExpiresAt: exp}, nil
}

func setCommonHeaders(req *http.Request) {
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "fr-BE,fr;q=0.9,nl;q=0.8,en;q=0.7")
	req.Header.Set("Origin", SiteHost)
	req.Header.Set("Referer", SiteHost+"/")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// Spotify Web Player TOTP-signed access-token bootstrap.
//
// Reverse-engineered from open.spotify.com's `web-player.<hash>.js` bundle.
// The secret bytes are extracted from the JS bundle at build time by the
// upstream `CycloneAddons/spotify-token-generator` project; we vendor the
// current latest version below and refresh it via `spotify-cli refresh-secret`
// (v0.2).
//
// Flow:
//   1. GET https://open.spotify.com/api/server-time (no auth) → {serverTime}
//   2. Build TOTP from secret + serverTime
//   3. GET https://open.spotify.com/api/token?reason=init&productType=web-player&totp=...&totpVer=...&totpServer=...
//      with Cookie: sp_dc=<user-cookie> → {accessToken, accessTokenExpirationTimestampMs}
//   4. Use accessToken as `Authorization: Bearer <token>` on spclient.wg.spotify.com.

package spotify

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// totpSecretsDict carries the secret bytes per TOTP version, mirroring
// `secrets/secretDict.json` upstream. We embed the current latest version so
// the adapter works offline; v0.2 will refresh from upstream at startup.
// Source: github.com/CycloneAddons/spotify-token-generator/secrets/secretDict.json
// Last refreshed: 2026-05-17.
var totpSecretsDict = map[int][]int{
	59: {123, 105, 79, 70, 110, 59, 52, 125, 60, 49, 80, 70, 89, 75, 80, 86, 63, 53, 123, 37, 117, 49, 52, 93, 77, 62, 47, 86, 48, 104, 68, 72},
	60: {79, 109, 69, 123, 90, 65, 46, 74, 94, 34, 58, 48, 70, 71, 92, 92, 85, 122, 63, 91, 64, 87, 87},
	61: {44, 55, 47, 42, 70, 40, 34, 114, 76, 74, 50, 111, 120, 97, 75, 76, 94, 102, 43, 69, 49, 120, 118, 80, 64, 78},
}

// latestTOTPVersion returns the highest version present in totpSecretsDict.
func latestTOTPVersion() int {
	latest := 0
	for v := range totpSecretsDict {
		if v > latest {
			latest = v
		}
	}
	return latest
}

// generateTOTP implements the Spotify-flavored TOTP from the upstream
// JavaScript reference verbatim:
//
//	transformed[i] = secret[i] XOR ((i % 33) + 9)
//	joined = decimal-string-concat(transformed)
//	secretBytes = []byte(joined)   // hex round-trip is a no-op
//	counter = timestamp / 30
//	HMAC-SHA1(secretBytes, counter as BE uint64)
//	standard RFC 4226 truncation, mod 10^6
func generateTOTP(timestampSeconds int64, secret []int) int {
	transformed := make([]int, len(secret))
	for i, b := range secret {
		transformed[i] = b ^ ((i % 33) + 9)
	}
	var joined strings.Builder
	for _, n := range transformed {
		joined.WriteString(strconv.Itoa(n))
	}
	secretBytes := []byte(joined.String())

	counter := uint64(timestampSeconds / 30)
	counterBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(counterBuf, counter)

	h := hmac.New(sha1.New, secretBytes)
	h.Write(counterBuf)
	sum := h.Sum(nil)

	offset := int(sum[len(sum)-1] & 0xf)
	code := (uint32(sum[offset]&0x7f) << 24) |
		(uint32(sum[offset+1]&0xff) << 16) |
		(uint32(sum[offset+2]&0xff) << 8) |
		uint32(sum[offset+3]&0xff)
	return int(code) % 1_000_000
}

type serverTimeResp struct {
	ServerTime int64 `json:"serverTime"`
}

type tokenResp struct {
	AccessToken                      string `json:"accessToken"`
	AccessTokenExpirationTimestampMs int64  `json:"accessTokenExpirationTimestampMs"`
	IsAnonymous                      bool   `json:"isAnonymous"`
	ClientID                         string `json:"clientId"`
}

// fetchServerTime returns Spotify's authoritative server time in seconds
// since epoch. The TOTP depends on this exact value — using local time
// drifts past the 30-second window and the upstream rejects the token
// with HTTP 401.
func fetchServerTime(ctx context.Context, hc *http.Client) (int64, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://open.spotify.com/api/server-time", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://open.spotify.com")
	req.Header.Set("Referer", "https://open.spotify.com/")
	resp, err := hc.Do(req)
	if err != nil {
		return 0, fmt.Errorf("spotify server-time: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("spotify server-time HTTP %d: %s", resp.StatusCode, string(body))
	}
	var st serverTimeResp
	if err := json.Unmarshal(body, &st); err != nil {
		return 0, fmt.Errorf("parse server-time: %w (body=%s)", err, string(body))
	}
	if st.ServerTime == 0 {
		// Some deployments wrap it under {data:{serverTime}}, but the
		// open.spotify.com endpoint as of 2026-05 returns the flat shape.
		return 0, fmt.Errorf("spotify server-time: zero value (body=%s)", string(body))
	}
	return st.ServerTime, nil
}

// bootstrapBearer derives a fresh Spotify Web Player access token from a
// sp_dc session cookie. Bearer TTL is roughly 1 hour; callers should cache
// it until accessTokenExpirationTimestampMs minus a safety margin.
func bootstrapBearer(ctx context.Context, hc *http.Client, spDC string) (token string, expiresAtMs int64, err error) {
	if spDC == "" {
		return "", 0, fmt.Errorf("sp_dc cookie required for Spotify bearer bootstrap")
	}
	serverTime, err := fetchServerTime(ctx, hc)
	if err != nil {
		return "", 0, err
	}
	version := latestTOTPVersion()
	secret, ok := totpSecretsDict[version]
	if !ok || len(secret) == 0 {
		return "", 0, fmt.Errorf("no TOTP secret embedded for v0.1; re-run `printing-press` to refresh, or set SPOTIFY_BEARER manually")
	}
	totp := generateTOTP(serverTime, secret)
	totpStr := fmt.Sprintf("%06d", totp)

	url := fmt.Sprintf(
		"https://open.spotify.com/api/token?reason=init&productType=web-player&totp=%s&totpVer=%d&totpServer=%s",
		totpStr, version, totpStr,
	)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://open.spotify.com")
	req.Header.Set("Referer", "https://open.spotify.com/")
	req.Header.Set("Cookie", "sp_dc="+spDC)

	resp, err := hc.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("spotify token bootstrap: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		return "", 0, fmt.Errorf("spotify token HTTP %d: %s", resp.StatusCode, summarize(body))
	}
	var tr tokenResp
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", 0, fmt.Errorf("parse token resp: %w (body=%s)", err, summarize(body))
	}
	if tr.AccessToken == "" {
		return "", 0, fmt.Errorf("spotify token response missing accessToken (body=%s)", summarize(body))
	}
	if tr.IsAnonymous {
		return "", 0, fmt.Errorf("spotify returned an anonymous token — sp_dc rejected; re-run `auth login-service --service spotify`")
	}
	return tr.AccessToken, tr.AccessTokenExpirationTimestampMs, nil
}

func summarize(body []byte) string {
	s := string(body)
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}

// bearerCache holds an in-memory bootstrapped bearer keyed by sp_dc. Surviving
// only for the lifetime of the process — fine for `episode get` batches but
// re-derives at every CLI invocation. A persisted cache lands in v0.2.
type bearerCache struct {
	token     string
	expiresAt time.Time
	spDC      string
}

func (b *bearerCache) valid(spDC string) bool {
	return b.token != "" && b.spDC == spDC && time.Now().Before(b.expiresAt.Add(-90*time.Second))
}

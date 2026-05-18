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
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

// bearerCache holds an in-memory bootstrapped bearer keyed by sp_dc. Survives
// the lifetime of the process. Disk persistence below makes it survive across
// process restarts too — critical for one-shot CLI invocations that would
// otherwise re-bootstrap on every call.
type bearerCache struct {
	token     string
	expiresAt time.Time
	spDC      string
}

func (b *bearerCache) valid(spDC string) bool {
	return b.token != "" && b.spDC == spDC && time.Now().Before(b.expiresAt.Add(-90*time.Second))
}

// diskCacheFile is the JSON sidecar that persists the in-process bearerCache
// across CLI invocations. Lives next to the cookie file. Mode 0600.
type diskCacheFile struct {
	SpDCHash    string `json:"sp_dc_hash"`
	Token       string `json:"token"`
	ExpiresAtMs int64  `json:"expires_at_ms"`
}

// SpDCHash returns the SHA-256 hex of the user's sp_dc cookie. We never write
// the raw sp_dc to the disk cache — the cookies-spotify.json file already
// holds it, and re-encoding here would be a needless duplicate of secret
// material. The hash is used only to detect when sp_dc rotates (user logged
// out + back in) so the cache is invalidated.
func SpDCHash(spDC string) string {
	h := sha256.Sum256([]byte(spDC))
	return hex.EncodeToString(h[:])
}

// DiskCachePath is where the bearer sidecar lives. Override via
// PODCAST_GOAT_BEARER_CACHE for tests.
func DiskCachePath() string {
	if v := os.Getenv("PODCAST_GOAT_BEARER_CACHE"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "podcast-goat", "bearer-cache.json")
	}
	return filepath.Join(home, ".config", "podcast-goat", "bearer-cache.json")
}

// LoadDiskCache reads a cached bearer from disk for the given sp_dc. Returns
// (token, expiresAt, true) when the file exists, parses cleanly, the hash
// matches, and the bearer is still inside its TTL with a 90s safety margin.
// Any other state — missing file, parse error, hash mismatch, expired — is
// treated as a miss (no error; caller bootstraps fresh).
func LoadDiskCache(spDC string) (token string, expiresAt time.Time, hit bool) {
	if spDC == "" {
		return "", time.Time{}, false
	}
	raw, err := os.ReadFile(DiskCachePath())
	if err != nil {
		return "", time.Time{}, false
	}
	var f diskCacheFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return "", time.Time{}, false
	}
	if f.SpDCHash != SpDCHash(spDC) {
		return "", time.Time{}, false
	}
	if f.Token == "" || f.ExpiresAtMs == 0 {
		return "", time.Time{}, false
	}
	exp := time.Unix(0, f.ExpiresAtMs*int64(time.Millisecond))
	if !time.Now().Before(exp.Add(-90 * time.Second)) {
		return "", time.Time{}, false
	}
	return f.Token, exp, true
}

// SaveDiskCache writes the bearer to disk atomically with mode 0600.
// Safe to call concurrently — temp-file + rename gives last-writer-wins
// without partial-file readers.
func SaveDiskCache(spDC, token string, expiresAtMs int64) error {
	if spDC == "" || token == "" {
		return nil
	}
	path := DiskCachePath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	payload := diskCacheFile{
		SpDCHash:    SpDCHash(spDC),
		Token:       token,
		ExpiresAtMs: expiresAtMs,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".bearer-cache.partial-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// ClearDiskCache removes the bearer file. Used by cache-clear --source spotify
// to keep ephemeral state aligned with the user's intent to wipe.
func ClearDiskCache() error {
	err := os.Remove(DiskCachePath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

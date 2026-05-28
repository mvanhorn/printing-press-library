// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// chrome_cookies.go reads Chrome's encrypted cookies database, decrypts values
// with the macOS Keychain "Chrome Safe Storage" key, and exposes a
// session-refresh pipeline that exchanges those cookies for a fresh Superhuman
// JWT via accounts.superhuman.com.
//
// This is the disk-only auth path. No CDP, no MCP, no browser extension, no
// Chrome relaunch. Cookies on disk → CSRF token → Firebase ID token → use it.
// The cookies on accounts.superhuman.com persist for ~10 years from the
// initial browser login, so once the user signs into Superhuman in Chrome
// once, this works indefinitely.

package auth

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/pbkdf2"
	_ "modernc.org/sqlite"
)

// SuperhumanAccountsHost is the Superhuman accounts service that mints JWTs.
const SuperhumanAccountsHost = "https://accounts.superhuman.com"

// SuperhumanBackendVersion is the client version header value Superhuman's
// backend requires. The bundle pins this; older versions get rejected with
// x-superhuman-min-version. Bump periodically as Superhuman ships new builds.
const SuperhumanBackendVersion = "2026-05-12T22:40:08Z"

// CookieAuthResult is the output of a successful session-refresh: a fresh
// Firebase ID token plus the metadata the backend needs in subsequent calls.
type CookieAuthResult struct {
	Email          string
	GoogleID       string // also the cookie name on accounts.superhuman.com
	IDToken        string // Firebase ID token (~936 bytes)
	IDTokenExpires int64  // epoch ms when this token expires
	AccessToken    string // Google OAuth access token (for Gmail passthrough)
	ExternalID     string // user_11SzD... format used as x-superhuman-user-external-id
	Scope          string // OAuth scopes granted
	DeviceID       string // from Chrome localStorage if available
}

// ChromeCookiesPath returns the OS-specific path to Chrome's Cookies database.
// Currently macOS-only; Linux/Windows TBD.
func ChromeCookiesPath() (string, error) {
	dataDir, err := ChromeDataDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("chrome cookies: %s not yet supported (macOS only for now)", runtime.GOOS)
	}
	return filepath.Join(dataDir, "Default", "Cookies"), nil
}

// chromeSafeStorageKey returns the AES-128 key Chrome uses to encrypt cookies
// on macOS. The password lives in the macOS Keychain under the service name
// "Chrome Safe Storage"; the key is derived via PBKDF2-HMAC-SHA1 with the
// fixed salt "saltysalt" and 1003 iterations.
func chromeSafeStorageKey() ([]byte, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("chrome safe storage: %s not yet supported", runtime.GOOS)
	}
	out, err := exec.Command("security", "find-generic-password", "-s", "Chrome Safe Storage", "-w").Output()
	if err != nil {
		return nil, fmt.Errorf("keychain read: %w (you may have denied the prompt; try again and click Always Allow)", err)
	}
	pw := strings.TrimSpace(string(out))
	if pw == "" {
		return nil, fmt.Errorf("keychain returned empty password for Chrome Safe Storage")
	}
	return pbkdf2.Key([]byte(pw), []byte("saltysalt"), 1003, 16, sha1.New), nil
}

// chromeIV is Chrome's hardcoded 16-byte AES-CBC IV (16 space characters).
var chromeIV = bytes.Repeat([]byte{' '}, 16)

// decryptV10Cookie decrypts a v10/v11-prefixed Chrome cookie value.
func decryptV10Cookie(ct, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ct)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext length %d not aligned to AES block size", len(ct))
	}
	pt := make([]byte, len(ct))
	cipher.NewCBCDecrypter(block, chromeIV).CryptBlocks(pt, ct)
	pad := int(pt[len(pt)-1])
	if pad < 1 || pad > aes.BlockSize {
		return pt, nil // unpadding looks wrong; return as-is and let caller decide
	}
	return pt[:len(pt)-pad], nil
}

// DecryptedChromeCookies reads Chrome's Cookies SQLite database, decrypts all
// cookies for the specified host, and returns name→value. The host filter
// matches host_key exactly (e.g., "accounts.superhuman.com" or
// ".superhuman.com" for domain cookies).
func DecryptedChromeCookies(host string) (map[string]string, error) {
	cookiesPath, err := ChromeCookiesPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(cookiesPath); err != nil {
		return nil, fmt.Errorf("chrome cookies: %w", err)
	}

	// Snapshot-copy to avoid contending with Chrome's SQLite writer lock.
	tmp, err := os.MkdirTemp("", "superhuman-pp-cookies-*")
	if err != nil {
		return nil, fmt.Errorf("snapshot mkdir: %w", err)
	}
	defer os.RemoveAll(tmp)
	snap := filepath.Join(tmp, "Cookies")
	if err := copyFileSimple(cookiesPath, snap); err != nil {
		return nil, fmt.Errorf("snapshot copy: %w", err)
	}

	key, err := chromeSafeStorageKey()
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", snap)
	if err != nil {
		return nil, fmt.Errorf("open cookies db: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(
		"SELECT name, encrypted_value FROM cookies WHERE host_key = ?",
		host,
	)
	if err != nil {
		return nil, fmt.Errorf("query cookies: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var name string
		var enc []byte
		if err := rows.Scan(&name, &enc); err != nil {
			continue
		}
		if !bytes.HasPrefix(enc, []byte("v10")) && !bytes.HasPrefix(enc, []byte("v11")) {
			out[name] = string(enc)
			continue
		}
		pt, err := decryptV10Cookie(enc[3:], key)
		if err != nil {
			continue
		}
		// PATCH(host-key-binding-detection): only strip the 32-byte
		// SHA-256(host_key) prefix when it actually matches the
		// computed hash. The previous unconditional strip discarded
		// the first 32 chars of every cookie value (which is every
		// JWT-sized session token), so RefreshFromChromeCookies always
		// sent a corrupted token. The prefix is added by Chrome's
		// M120+ cookie host-binding feature and is absent on older
		// Chrome versions (greptile P1).
		if len(pt) >= 32 {
			sum := sha256.Sum256([]byte(host))
			if bytes.Equal(pt[:32], sum[:]) {
				pt = pt[32:]
			}
		}
		out[name] = string(pt)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no cookies found for host %q (have you logged in to Superhuman in Chrome?)", host)
	}
	return out, nil
}

// RefreshFromChromeCookies is the headline operation: read accounts.superhuman.com
// cookies from Chrome's disk, fetch a CSRF token, then POST sessions.getTokens
// to get a fresh JWT. Returns the authenticated session data ready for use.
//
// On macOS this triggers a Keychain Access prompt on first run (the user
// clicks "Always Allow"); subsequent runs are silent.
//
// Identifies the account to refresh by the GoogleID — which IS the cookie name
// on accounts.superhuman.com (Superhuman uses per-account session cookies
// named by the user's Google ID).
func RefreshFromChromeCookies(ctx context.Context, email, googleID string) (*CookieAuthResult, error) {
	cookies, err := DecryptedChromeCookies("accounts.superhuman.com")
	if err != nil {
		return nil, err
	}
	// Verify we have the per-account session cookie.
	if _, ok := cookies[googleID]; !ok {
		return nil, fmt.Errorf("no session cookie for account %s (googleID=%s); log in to Superhuman in Chrome first", email, googleID)
	}
	if _, ok := cookies["csrf"]; !ok {
		return nil, fmt.Errorf("no CSRF cookie on accounts.superhuman.com; log in to Superhuman in Chrome first")
	}

	jar, _ := cookiejar.New(nil)
	accountsURL, _ := url.Parse(SuperhumanAccountsHost + "/")
	for name, val := range cookies {
		jar.SetCookies(accountsURL, []*http.Cookie{{
			Name: name, Value: val, Path: "/", Secure: true, HttpOnly: true,
		}})
	}
	client := &http.Client{Jar: jar, Timeout: 30 * time.Second}

	// Step 1: get a fresh CSRF token (GET).
	csrfURL := SuperhumanAccountsHost + "/~backend/v3/sessions.getCsrfToken"
	csrfReq, _ := http.NewRequestWithContext(ctx, "GET", csrfURL, nil)
	addBrowserHeaders(csrfReq)
	csrfResp, err := client.Do(csrfReq)
	if err != nil {
		return nil, fmt.Errorf("get csrf: %w", err)
	}
	defer csrfResp.Body.Close()
	if csrfResp.StatusCode != 200 {
		body, _ := io.ReadAll(csrfResp.Body)
		return nil, fmt.Errorf("get csrf: status %d body=%s", csrfResp.StatusCode, string(body))
	}
	var csrfPayload struct {
		CSRFToken string `json:"csrfToken"`
		ExpiresIn int    `json:"expiresIn"`
	}
	if err := json.NewDecoder(csrfResp.Body).Decode(&csrfPayload); err != nil {
		return nil, fmt.Errorf("parse csrf: %w", err)
	}
	if csrfPayload.CSRFToken == "" {
		return nil, fmt.Errorf("get csrf: empty token in response")
	}

	// Step 2: POST sessions.getTokens with body {emailAddress, googleId} and X-CSRF-Token header.
	tokensURL := SuperhumanAccountsHost + "/~backend/v3/sessions.getTokens"
	body, _ := json.Marshal(map[string]string{
		"emailAddress": email,
		"googleId":     googleID,
	})
	tokReq, _ := http.NewRequestWithContext(ctx, "POST", tokensURL, bytes.NewReader(body))
	addBrowserHeaders(tokReq)
	tokReq.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	tokReq.Header.Set("X-CSRF-Token", csrfPayload.CSRFToken)
	tokResp, err := client.Do(tokReq)
	if err != nil {
		return nil, fmt.Errorf("get tokens: %w", err)
	}
	defer tokResp.Body.Close()
	if tokResp.StatusCode != 200 {
		respBody, _ := io.ReadAll(tokResp.Body)
		return nil, fmt.Errorf("get tokens: status %d body=%s", tokResp.StatusCode, string(respBody))
	}

	var payload struct {
		Calendars []any `json:"calendars"`
		Aliases   []any `json:"aliases"`
		AuthData  struct {
			Email       string          `json:"emailAddress"`
			GoogleID    string          `json:"googleId"`
			ExternalID  string          `json:"externalId"`
			IDToken     string          `json:"idToken"`
			AccessToken string          `json:"accessToken"`
			Scope       string          `json:"scope"`
			ExpiresIn   json.RawMessage `json:"expiresIn"` // int or string
		} `json:"authData"`
	}
	if err := json.NewDecoder(tokResp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("parse getTokens: %w", err)
	}
	if payload.AuthData.IDToken == "" {
		return nil, fmt.Errorf("get tokens: no idToken in response")
	}

	expiresSec := 3600
	if raw := payload.AuthData.ExpiresIn; len(raw) > 0 {
		// Try int first, then string-int.
		var asInt int
		if err := json.Unmarshal(raw, &asInt); err == nil && asInt > 0 {
			expiresSec = asInt
		} else {
			var asStr string
			if err := json.Unmarshal(raw, &asStr); err == nil {
				if n, err := fmtParseInt(asStr); err == nil && n > 0 {
					expiresSec = n
				}
			}
		}
	}
	idTokenExpires := time.Now().UnixMilli() + int64(expiresSec)*1000 - 60_000

	// Best-effort device ID from localStorage.
	deviceID, _ := readDeviceIDFromLocalStorage()

	return &CookieAuthResult{
		Email:          payload.AuthData.Email,
		GoogleID:       payload.AuthData.GoogleID,
		IDToken:        payload.AuthData.IDToken,
		IDTokenExpires: idTokenExpires,
		AccessToken:    payload.AuthData.AccessToken,
		ExternalID:     payload.AuthData.ExternalID,
		Scope:          payload.AuthData.Scope,
		DeviceID:       deviceID,
	}, nil
}

// addBrowserHeaders sets the request headers Chrome would send. The Superhuman
// backend doesn't strictly require all of these for getCsrfToken/getTokens,
// but they make the request look authentic and avoid future user-agent gating.
func addBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")
	req.Header.Set("Origin", "https://mail.superhuman.com")
	req.Header.Set("Referer", "https://mail.superhuman.com/")
	req.Header.Set("Accept", "application/json")
}

// ListAccountsOnAccountsHost filters the supplied cookie name set (typically
// the result of DecryptedChromeCookies("accounts.superhuman.com")) down to the
// names that look like Google user IDs — purely-digit strings longer than 15
// characters. Superhuman names one session cookie per logged-in account by the
// Google ID. The `csrf` cookie (and any other non-numeric metadata cookie) is
// skipped. Order is not stable across calls because Go maps don't iterate in
// insertion order; callers that need deterministic order should sort the
// returned slice.
func ListAccountsOnAccountsHost(cookies map[string]string) []string {
	out := make([]string, 0, len(cookies))
	for name := range cookies {
		if name == "csrf" {
			continue
		}
		if !isPureDigits(name) {
			continue
		}
		if len(name) <= 15 {
			continue
		}
		out = append(out, name)
	}
	return out
}

// isPureDigits reports whether s is non-empty and contains only ASCII digits.
// Used to identify googleId-shaped cookie names on accounts.superhuman.com
// without dragging in strconv.Atoi (which would accept leading signs).
func isPureDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// AddSuperhumanBackendHeaders attaches the x-superhuman-* headers every
// authenticated /~backend/v3/* call needs. Exported so the HTTP client wrapper
// can add them uniformly.
func AddSuperhumanBackendHeaders(req *http.Request, result *CookieAuthResult) {
	req.Header.Set("Authorization", "Bearer "+result.IDToken)
	req.Header.Set("x-superhuman-version", SuperhumanBackendVersion)
	req.Header.Set("x-superhuman-user-email", result.Email)
	req.Header.Set("x-superhuman-user-external-id", result.ExternalID)
	req.Header.Set("x-superhuman-session-id", uuid.NewString())
	req.Header.Set("x-superhuman-request-id", uuid.NewString())
	if result.DeviceID != "" {
		req.Header.Set("x-superhuman-device-id", result.DeviceID)
	}
}

// readDeviceIDFromLocalStorage extracts the Superhuman device ID from Chrome's
// localStorage if available. Falls back to empty string on failure.
func readDeviceIDFromLocalStorage() (string, error) {
	dataDir, err := ChromeDataDir()
	if err != nil {
		return "", err
	}
	kv, err := ReadSuperhumanLocalStorage(filepath.Join(dataDir, "Default"))
	if err != nil {
		return "", err
	}
	return kv["deviceId"], nil
}

// fmtParseInt is a small wrapper around strconv.Atoi to avoid pulling in
// strconv just for one use.
func fmtParseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func copyFileSimple(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()
	_, err = io.Copy(d, s)
	return err
}

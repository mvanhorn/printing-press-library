// Copyright 2026 greatclips. Licensed under Apache-2.0. See LICENSE.

//go:build darwin

// Package auth0silent extracts Auth0 session cookies from the user's
// macOS Chrome cookie store and uses them to mint per-audience JWT
// access tokens via Auth0's silent-auth (prompt=none) flow.
package auth0silent

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// AuthCookieNames are the cid.greatclips.com session cookies the SPA
// uses to silently refresh Auth0 access tokens. We extract these four;
// the others on cid.greatclips.com are not part of the auth handshake.
var AuthCookieNames = []string{"auth0", "auth0_compat", "did", "did_compat"}

// CIDHost is the Auth0 tenant host for GreatClips. Cookies are scoped
// here, not on .greatclips.com.
const CIDHost = "cid.greatclips.com"

// ExtractAuth0Cookies returns the decrypted Auth0 session cookies for
// the user's default Chrome profile on macOS. The map is keyed by
// cookie name (e.g. "auth0") and the value is the plaintext cookie
// content the browser would send in the Cookie header.
//
// Reads the Chrome cookie SQLite database in read-only mode against a
// temp copy (Chrome locks the original while running). Decryption
// uses the per-user AES key derived from a Keychain entry Chrome
// also reads ("Chrome Safe Storage").
//
// Returns an error wrapping the first failure: missing Chrome
// install, Keychain denial, SQLite read failure, decrypt failure.
func ExtractAuth0Cookies() (map[string]string, error) {
	dbPath, err := defaultChromeCookiesPath()
	if err != nil {
		return nil, err
	}
	tmpDB, err := copyToTemp(dbPath)
	if err != nil {
		return nil, fmt.Errorf("copying chrome cookies db: %w", err)
	}
	defer os.Remove(tmpDB)

	rows, err := readCookieRows(tmpDB, CIDHost)
	if err != nil {
		return nil, fmt.Errorf("reading cookies for %s: %w", CIDHost, err)
	}

	key, err := chromeKey()
	if err != nil {
		return nil, fmt.Errorf("deriving chrome encryption key: %w", err)
	}

	wanted := make(map[string]struct{}, len(AuthCookieNames))
	for _, n := range AuthCookieNames {
		wanted[n] = struct{}{}
	}
	out := make(map[string]string)
	for _, r := range rows {
		if _, ok := wanted[r.name]; !ok {
			continue
		}
		plain, err := decryptCookie(key, r.encrypted, CIDHost)
		if err != nil {
			return nil, fmt.Errorf("decrypting cookie %q: %w", r.name, err)
		}
		out[r.name] = plain
	}
	for _, n := range AuthCookieNames {
		if _, ok := out[n]; !ok {
			return out, fmt.Errorf("required cookie %q not found in chrome store (is the user logged in at https://app.greatclips.com?)", n)
		}
	}
	return out, nil
}

type cookieRow struct {
	name      string
	encrypted []byte
}

// defaultChromeCookiesPath returns the path to the Default profile's
// Cookies SQLite file. Future work: support multiple profiles.
func defaultChromeCookiesPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "Default", "Cookies")
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("chrome cookies file not found at %s: %w", p, err)
	}
	return p, nil
}

// copyToTemp duplicates a file into /tmp so we can read it without
// fighting Chrome's exclusive lock on the live SQLite file.
func copyToTemp(src string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	tmp, err := os.CreateTemp("", "gc-cookies-*.db")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	// Tighten perms — file holds another process's session cookies.
	_ = os.Chmod(tmp.Name(), 0o600)
	return tmp.Name(), nil
}

// readCookieRows runs the sqlite3 CLI to extract (name, encrypted_value)
// for the given host. macOS ships sqlite3 by default so this avoids a
// CGO dependency. encrypted_value is BLOB; we encode it as hex on the
// SQL side and decode in Go.
func readCookieRows(dbPath, host string) ([]cookieRow, error) {
	// PATCH(cookies-host-allowlist): sqlite3 CLI doesn't expose parameter
	// binding via -separator, so we restrict the host argument to the one
	// caller-side constant the package actually uses. This removes the
	// SQL-injection surface flagged by greptile P1 - any future caller
	// passing user input would hit the allowlist error rather than splicing
	// into the SQL.
	if host != CIDHost {
		return nil, fmt.Errorf("readCookieRows: unexpected host %q (only %q is allowed)", host, CIDHost)
	}
	// hex() returns uppercase hex; safe to round-trip via hex.DecodeString.
	query := fmt.Sprintf("SELECT name, hex(encrypted_value) FROM cookies WHERE host_key = '%s';", host)
	cmd := exec.Command("sqlite3", "-separator", "|", dbPath, query)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("sqlite3 failed: %w (stderr: %s)", err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("sqlite3: %w", err)
	}
	var rows []cookieRow
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		idx := strings.Index(line, "|")
		if idx < 0 {
			continue
		}
		name := line[:idx]
		hexBlob := line[idx+1:]
		enc, err := hex.DecodeString(hexBlob)
		if err != nil {
			return nil, fmt.Errorf("decoding hex for cookie %q: %w", name, err)
		}
		rows = append(rows, cookieRow{name: name, encrypted: enc})
	}
	return rows, nil
}

// chromeKey shells to the macOS `security` CLI to fetch the Chrome
// Safe Storage keychain password, then PBKDF2-HMAC-SHA1-derives the
// 16-byte AES key Chrome uses to encrypt cookie values.
//
// On first invocation the OS will prompt the user for permission to
// access the Chrome keychain item. Subsequent invocations are silent
// for ~10 minutes (Keychain caches the user's approval).
func chromeKey() ([]byte, error) {
	cmd := exec.Command("security", "find-generic-password", "-wa", "Chrome")
	pwBytes, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("keychain access denied or item missing: %w (stderr: %s)", err, string(exitErr.Stderr))
		}
		return nil, err
	}
	pw := strings.TrimRight(string(pwBytes), "\n")
	return pbkdf2.Key(sha1.New, pw, []byte("saltysalt"), 1003, 16)
}

// decryptCookie undoes Chrome's v10 cookie encryption:
//   - strip the 3-byte "v10" prefix
//   - AES-128-CBC with a 16-space IV
//   - PKCS7 unpad
//   - strip the 32-byte SHA-256(host_key) prefix that newer Chrome
//     versions prepend to plaintext (binds the cookie value to its
//     intended host)
//
// PATCH(host-key-binding-detection): the 32-byte SHA-256(host_key) prefix
// is added by Chrome's cookie host-binding feature (M120+) and is NOT
// present in cookies written by older Chrome versions. The previous code
// stripped 32 bytes unconditionally for any plaintext > 32 bytes, so
// older-Chrome cookies (every JWT-sized Auth0 cookie qualifies) were
// silently mangled. We now compute the expected prefix and only strip
// when it matches - if a future Chrome change reshapes the binding we'll
// see the raw cookie value rather than 32 chars of garbage.
func decryptCookie(key, enc []byte, hostKey string) (string, error) {
	if len(enc) < 3 || string(enc[:3]) != "v10" {
		return "", fmt.Errorf("expected v10 prefix, got %q", string(enc[:min(3, len(enc))]))
	}
	ct := enc[3:]
	if len(ct)%aes.BlockSize != 0 {
		return "", fmt.Errorf("ciphertext len %d not aes-block-aligned", len(ct))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	iv := []byte("                ") // 16 spaces — Chrome's v10 IV
	plain := make([]byte, len(ct))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, ct)

	// PKCS7 unpad
	pad := int(plain[len(plain)-1])
	if pad < 1 || pad > aes.BlockSize {
		return "", fmt.Errorf("invalid pkcs7 padding byte %d", pad)
	}
	plain = plain[:len(plain)-pad]

	// Newer Chrome (M120+) prepends 32 bytes of SHA-256(host_key) to
	// the plaintext as a host-binding tag. Strip it only when it matches
	// the computed prefix; older-Chrome cookies have no such prefix and
	// stripping would discard the first 32 chars of the real value.
	if len(plain) >= 32 && hostKey != "" {
		sum := sha256.Sum256([]byte(hostKey))
		if bytes.Equal(plain[:32], sum[:]) {
			plain = plain[32:]
		}
	}
	return string(plain), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

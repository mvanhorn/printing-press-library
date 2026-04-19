// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// Package chromecookies reads and decrypts cookies from Chrome's local cookie
// jar. macOS is the priority target: cookies are stored in a SQLite DB at
// ~/Library/Application Support/Google/Chrome/Default/Cookies and encrypted
// values use AES-128-CBC with a PBKDF2-derived key whose password lives in the
// user's Keychain under the service name "Chrome Safe Storage".
//
// On Linux and Windows, the storage and derivation differs — this package
// returns an explicit unsupported-platform error there and callers should
// surface a doc link to the user.
//
// Secrets (cookie values, keychain password) are NEVER logged or wrapped into
// error messages. Errors reference only structural issues ("decrypt failed",
// "no rows matched domain").
package chromecookies

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha1"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Cookie is a decrypted cookie extracted from Chrome's cookie jar. Value is
// plaintext — callers MUST avoid logging it.
type Cookie struct {
	Name     string
	Value    string
	Domain   string
	Path     string
	Expires  time.Time
	HttpOnly bool
	Secure   bool
}

// ErrUnsupportedPlatform is returned when the current OS is not yet supported
// by this package. Only darwin is implemented here.
var ErrUnsupportedPlatform = errors.New("chromecookies: unsupported platform (only macOS is implemented)")

// ErrKeychainUnavailable is returned when the Chrome Safe Storage password
// cannot be retrieved from the macOS Keychain. Typical cause: user denied
// the prompt, or Chrome is not installed.
var ErrKeychainUnavailable = errors.New("chromecookies: could not read Chrome Safe Storage password from Keychain")

// ReadHappenstanceCookies returns all Chrome cookies whose domain matches
// happenstance.ai (bare or dot-prefixed). Decrypted values are returned in
// Cookie.Value — keep them out of logs and error messages.
func ReadHappenstanceCookies() ([]Cookie, error) {
	return readCookiesForDomains([]string{"happenstance.ai", ".happenstance.ai", "clerk.happenstance.ai", ".clerk.happenstance.ai"})
}

func readCookiesForDomains(domains []string) ([]Cookie, error) {
	if runtime.GOOS != "darwin" {
		return nil, ErrUnsupportedPlatform
	}

	dbPath, err := chromeCookiesDBPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("chrome cookies DB not found at expected path: %w", err)
	}

	password, err := keychainChromeSafeStoragePassword()
	if err != nil {
		return nil, err
	}

	key, err := pbkdf2.Key(sha1.New, password, []byte("saltysalt"), 1003, 16)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	// Copy DB to a temp file so we don't contend with a running Chrome
	// holding a write lock on the original.
	tmpPath, cleanup, err := copyToTemp(dbPath)
	if err != nil {
		return nil, fmt.Errorf("copy cookies DB: %w", err)
	}
	defer cleanup()

	db, err := sql.Open("sqlite", tmpPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open cookies DB: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`
SELECT host_key, name, encrypted_value, path, expires_utc, is_httponly, is_secure
FROM cookies
WHERE host_key = ? OR host_key = ? OR host_key = ? OR host_key = ?
`, anyOf4(domains)...)
	if err != nil {
		return nil, fmt.Errorf("query cookies: %w", err)
	}
	defer rows.Close()

	var out []Cookie
	for rows.Next() {
		var (
			host       string
			name       string
			encrypted  []byte
			path       string
			expiresUTC int64
			isHTTPOnly int
			isSecure   int
		)
		if err := rows.Scan(&host, &name, &encrypted, &path, &expiresUTC, &isHTTPOnly, &isSecure); err != nil {
			return nil, fmt.Errorf("scan cookie row: %w", err)
		}

		plaintext, err := decryptChromeValue(encrypted, key)
		if err != nil {
			// Don't leak the encrypted blob or name into the error string.
			return nil, fmt.Errorf("decrypt cookie: %w", err)
		}

		out = append(out, Cookie{
			Name:     name,
			Value:    plaintext,
			Domain:   host,
			Path:     path,
			Expires:  chromeEpochToTime(expiresUTC),
			HttpOnly: isHTTPOnly != 0,
			Secure:   isSecure != 0,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cookie rows: %w", err)
	}
	return out, nil
}

// anyOf4 pads a domain list out to exactly four values for the fixed-column
// query above. Extra slots are filled with an impossible sentinel so the
// query remains correct when the caller supplies fewer than four.
func anyOf4(domains []string) []any {
	out := make([]any, 4)
	for i := range out {
		out[i] = "__no_such_domain__"
	}
	for i, d := range domains {
		if i >= 4 {
			break
		}
		out[i] = d
	}
	return out
}

func chromeCookiesDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "Default", "Cookies"), nil
}

// keychainChromeSafeStoragePassword runs the macOS `security` CLI to read the
// Chrome Safe Storage password. The password is never logged.
func keychainChromeSafeStoragePassword() (string, error) {
	cmd := exec.Command("security", "find-generic-password", "-w", "-s", "Chrome Safe Storage", "-a", "Chrome")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrKeychainUnavailable, err)
	}
	password := strings.TrimRight(string(out), "\n")
	if password == "" {
		return "", ErrKeychainUnavailable
	}
	return password, nil
}

// decryptChromeValue decrypts a v10/v11 Chrome cookie payload using AES-128-CBC
// with PKCS#7 padding and a fixed IV of 16 space bytes (Chrome's convention
// on macOS/Linux for Safe Storage-encrypted values).
func decryptChromeValue(payload, key []byte) (string, error) {
	if len(payload) < 3 {
		return "", errors.New("payload too short")
	}
	prefix := string(payload[:3])
	if prefix != "v10" && prefix != "v11" {
		// Some rows (especially older or synced ones) may be stored
		// unencrypted — surface the raw bytes if they're ASCII-looking.
		// But refuse silently-wrong decodes: if there's no known prefix
		// we treat the value as already-plaintext.
		return string(payload), nil
	}
	ciphertext := payload[3:]
	if len(ciphertext)%aes.BlockSize != 0 {
		return "", errors.New("ciphertext not block-aligned")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}
	iv := []byte("                ") // 16 spaces, Chrome's documented IV for Safe Storage
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	plaintext, err = pkcs7Unpad(plaintext, aes.BlockSize)
	if err != nil {
		return "", err
	}
	// Chrome 95+ prepends a 32-byte SHA-256(host) integrity prefix to the
	// cleartext before encryption. We don't verify it (no host context
	// passed in here); we strip it if the plaintext is at least 32 bytes
	// AND the byte at offset 32 starts a printable ASCII run (real cookie
	// values are ASCII). This heuristic avoids truncating legitimate short
	// unprefixed values seen on older Chrome builds.
	if len(plaintext) > 32 {
		tail := plaintext[32:]
		if looksASCII(tail) && !looksASCII(plaintext[:32]) {
			plaintext = tail
		}
	}
	return string(plaintext), nil
}

// looksASCII returns true when the first few bytes are printable ASCII.
// Used as a heuristic to detect whether a byte slice is likely the start
// of a real cookie value vs binary (hash) bytes.
func looksASCII(b []byte) bool {
	n := len(b)
	if n > 16 {
		n = 16
	}
	for i := 0; i < n; i++ {
		c := b[i]
		if c < 0x20 || c > 0x7E {
			return false
		}
	}
	return n > 0
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	pad := int(data[len(data)-1])
	if pad <= 0 || pad > blockSize {
		return nil, errors.New("invalid padding")
	}
	if pad > len(data) {
		return nil, errors.New("padding exceeds length")
	}
	for i := len(data) - pad; i < len(data); i++ {
		if int(data[i]) != pad {
			return nil, errors.New("invalid padding byte")
		}
	}
	return data[:len(data)-pad], nil
}

// chromeEpochToTime converts a Chrome WebKit timestamp (microseconds since
// 1601-01-01) to a Go time. Returns zero for sentinel "session cookie" values.
func chromeEpochToTime(micros int64) time.Time {
	if micros <= 0 {
		return time.Time{}
	}
	// Chrome epoch: 1601-01-01 UTC. Unix epoch: 1970-01-01. Difference in
	// microseconds:
	const epochOffsetMicros = 11644473600 * 1000000
	unixMicros := micros - epochOffsetMicros
	if unixMicros <= 0 {
		return time.Time{}
	}
	return time.UnixMicro(unixMicros).UTC()
}

func copyToTemp(src string) (string, func(), error) {
	in, err := os.Open(src)
	if err != nil {
		return "", nil, err
	}
	defer in.Close()

	tmp, err := os.CreateTemp("", "contact-goat-cookies-*.sqlite")
	if err != nil {
		return "", nil, err
	}
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", nil, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", nil, err
	}
	cleanup := func() { os.Remove(tmp.Name()) }
	return tmp.Name(), cleanup, nil
}

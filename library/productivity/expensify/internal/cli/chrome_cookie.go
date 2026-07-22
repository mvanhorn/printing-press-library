// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// Capture the Expensify session authToken from the user's existing Chrome
// session by reading Chrome's cookie store and decrypting the v10 value with
// the "Chrome Safe Storage" key from the macOS Keychain.
//
// Chrome 127+ App-Bound Encryption (v20 values) cannot be decrypted this way;
// callers degrade to the manual paste / `auth login` paths on any error.

package cli

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha1" // #nosec G505 -- SHA1 is mandated by Chrome's cookie encryption scheme (PBKDF2-HMAC-SHA1); required for interop, not used as a security primitive of our own.
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	_ "modernc.org/sqlite"
)

// captureTokenFromChrome returns the Expensify authToken from the user's signed-in
// Chrome. Returns an error (handled by the caller's manual fallback) when the
// token can't be read or decrypted.
func captureTokenFromChrome(debugPort int) (string, string, error) {
	if debugPort > 0 {
		return "", "", fmt.Errorf("--chrome-debug-port (CDP) reading is not implemented yet; omit it to read Chrome's cookie store directly")
	}
	if runtime.GOOS != "darwin" {
		return "", "", fmt.Errorf("reading the Chrome cookie store is only supported on macOS")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	cookiesDB := filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "Default", "Cookies")
	if _, err := os.Stat(cookiesDB); err != nil {
		return "", "", fmt.Errorf("Chrome cookie store not found at %s", cookiesDB)
	}

	enc, err := readEncryptedAuthToken(cookiesDB)
	if err != nil {
		return "", "", err
	}
	key, err := chromeSafeStorageKey()
	if err != nil {
		return "", "", err
	}
	plain, err := decryptChromeCookie(enc, key)
	if err != nil {
		return "", "", err
	}
	token := strings.TrimSpace(plain)
	if token == "" {
		return "", "", fmt.Errorf("decrypted authToken was empty")
	}
	return token, "", nil
}

// readEncryptedAuthToken copies the (locked) Chrome cookie DB to a temp file and
// returns the encrypted_value of the expensify.com authToken cookie.
func readEncryptedAuthToken(dbPath string) ([]byte, error) {
	src, err := os.ReadFile(dbPath) // #nosec G304 -- dbPath resolves to the local user's own Chrome cookie store at the default OS profile location; reading it is the documented purpose of this command.
	if err != nil {
		return nil, fmt.Errorf("reading Chrome cookie store: %w", err)
	}
	// Work in a private temp dir with fixed, constant filenames so SQLite can find
	// the WAL/SHM sidecars next to the DB and there is no caller-controlled path.
	tmpDir, err := os.MkdirTemp("", "exp-cookies")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	tmpPath := filepath.Join(tmpDir, "Cookies")
	if err := os.WriteFile(tmpPath, src, 0o600); err != nil { // #nosec G703 -- tmpPath is filepath.Join of an os.MkdirTemp dir and a constant name; not caller-controlled.
		return nil, err
	}
	// Chrome may keep a freshly-written cookie (e.g. just after login) in the
	// WAL/SHM sidecars before checkpointing it into the main DB. Copy them too so
	// a normal open applies them; otherwise we'd read a stale token. Best-effort;
	// the source paths are siblings of the user's own cookie store.
	for _, ext := range []string{"-wal", "-shm"} {
		sb, rerr := os.ReadFile(dbPath + ext) // #nosec G304 -- sibling of the user's own cookie store
		if rerr != nil {
			continue
		}
		dst := filepath.Join(tmpDir, "Cookies"+ext)
		if werr := os.WriteFile(dst, sb, 0o600); werr != nil { // #nosec G703 -- dst is filepath.Join of an os.MkdirTemp dir and a constant name; not caller-controlled.
			return nil, werr
		}
	}

	db, err := sql.Open("sqlite", tmpPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Prefer the classic www.expensify.com host (the API host the CLI calls), then
	// the most recently updated cookie — a fresh login leaves older/stale authToken
	// rows behind, so recency, not value length, identifies the live token.
	rows, err := db.Query(`SELECT encrypted_value FROM cookies WHERE name = 'authToken' AND host_key LIKE '%expensify.com%' ORDER BY (host_key = 'www.expensify.com') DESC, last_update_utc DESC`)
	if err != nil {
		return nil, fmt.Errorf("querying cookies: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ev []byte
		if err := rows.Scan(&ev); err != nil {
			return nil, err
		}
		if len(ev) > 3 {
			return ev, nil
		}
	}
	return nil, fmt.Errorf("no authToken cookie for expensify.com in Chrome (are you signed in to www.expensify.com?)")
}

// chromeSafeStoragekey derives the AES key from the Keychain "Chrome Safe Storage"
// passphrase using Chrome's macOS parameters (PBKDF2-SHA1, salt "saltysalt",
// 1003 iterations, 16-byte key).
func chromeSafeStorageKey() ([]byte, error) {
	out, err := exec.Command("security", "find-generic-password", "-w", "-s", "Chrome Safe Storage", "-a", "Chrome").Output()
	if err != nil {
		return nil, fmt.Errorf("reading the Chrome Safe Storage key from Keychain (you may be prompted to allow access): %w", err)
	}
	passphrase := strings.TrimSpace(string(out))
	key, err := pbkdf2.Key(sha1.New, passphrase, []byte("saltysalt"), 1003, 16)
	if err != nil {
		return nil, err
	}
	return key, nil
}

// decryptChromeCookie decrypts a Chrome v10 cookie value (AES-128-CBC, IV = 16
// spaces). Returns a helpful error for v20 (App-Bound Encryption).
func decryptChromeCookie(enc, key []byte) (string, error) {
	if len(enc) < 3 {
		return "", fmt.Errorf("cookie value too short")
	}
	switch string(enc[:3]) {
	case "v20":
		return "", fmt.Errorf("cookie uses App-Bound Encryption (Chrome 127+); it can't be read from the cookie store — use `auth login` or paste the token (pbpaste | auth set-token -)")
	case "v10":
		// supported below
	default:
		return "", fmt.Errorf("unexpected cookie encryption version %q", string(enc[:3]))
	}
	ct := enc[3:]
	if len(ct) == 0 || len(ct)%aes.BlockSize != 0 {
		return "", fmt.Errorf("invalid ciphertext length")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	iv := bytes.Repeat([]byte{' '}, aes.BlockSize)
	pt := make([]byte, len(ct))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(pt, ct)
	pt = pkcs7Unpad(pt)
	// Newer Chrome prepends a 32-byte SHA-256 domain hash to the plaintext. Decide
	// by the prefix bytes themselves: a SHA-256 hash is binary (non-printable),
	// while the authToken value is printable ASCII. Strip only when the first 32
	// bytes are non-printable and the remainder is a clean printable token.
	if len(pt) > 32 && !allPrintableASCII(pt[:32]) && allPrintableASCII(pt[32:]) {
		pt = pt[32:]
	}
	return string(pt), nil
}

func pkcs7Unpad(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	pad := int(b[len(b)-1])
	if pad <= 0 || pad > aes.BlockSize || pad > len(b) {
		return b
	}
	// A valid PKCS#7 block has every padding byte equal to pad; if not, the block
	// is malformed (e.g. a corrupted cookie copy) — leave it untouched rather than
	// returning silently-wrong plaintext.
	for _, c := range b[len(b)-pad:] {
		if int(c) != pad {
			return b
		}
	}
	return b[:len(b)-pad]
}

func allPrintableASCII(b []byte) bool {
	for _, c := range b {
		if c < 0x20 || c > 0x7e {
			return false
		}
	}
	return true
}

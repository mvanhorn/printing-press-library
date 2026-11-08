// Package chromecookies reads one site's cookies out of a Chrome profile with no
// Python, no sqlite3 binary, and no manual token copying.
//
// Why this exists: Chrome keeps cookie values in the encrypted_value column and
// leaves value empty. The first version of this CLI read the value column, which
// produced a full list of cookie names with empty values and a false "logged in".
package chromecookies

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha1" // #nosec G505 -- Chrome derives its cookie key with PBKDF2-SHA1; the format fixes this
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure Go driver, shared with the local store
)

// ErrNoCookies means the profile has nothing usable for the site, as opposed to a
// failure reading it.
var ErrNoCookies = errors.New("no cookies for this site in the Chrome profile")

// Key derives the AES key from the Chrome password for this platform.
func Key(password string, iterations int) ([]byte, error) {
	return pbkdf2.Key(sha1.New, password, []byte("saltysalt"), iterations, 16)
}

// SafeStorageKey fetches and derives this machine's cookie key. On macOS the first
// read may show a Keychain prompt, which is the operating system asking the owner,
// so it runs on a timeout rather than hanging forever if nobody answers.
func SafeStorageKey(ctx context.Context) ([]byte, error) {
	switch runtime.GOOS {
	case "darwin":
		ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "security", "find-generic-password", "-w", "-s", "Chrome Safe Storage").Output()
		if err != nil {
			return nil, fmt.Errorf("read Chrome Safe Storage from the Keychain (allow the prompt, or use --cookies-file): %w", err)
		}
		return Key(strings.TrimSpace(string(out)), 1003)
	case "linux":
		// Without a desktop keyring Chrome uses the fixed "peanuts" password.
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(ctx, "secret-tool", "lookup", "application", "chrome").Output(); err == nil && len(strings.TrimSpace(string(out))) > 0 {
			return Key(strings.TrimSpace(string(out)), 1)
		}
		return Key("peanuts", 1)
	default:
		return nil, fmt.Errorf("native Chrome cookie import is not supported on %s; use --cookies-file", runtime.GOOS)
	}
}

// Decrypt reverses Chrome's v10/v11 AES-CBC. stripHostHash handles cookie database
// version 24 and later, which prepends SHA-256 of the host to the plaintext.
func Decrypt(enc, key []byte, stripHostHash bool) (string, error) {
	if len(enc) < 3 || (string(enc[:3]) != "v10" && string(enc[:3]) != "v11") {
		return "", errors.New("unsupported cookie encryption")
	}
	data := enc[3:]
	if len(data) == 0 || len(data)%aes.BlockSize != 0 {
		return "", errors.New("bad ciphertext length")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	plain := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, []byte(strings.Repeat(" ", aes.BlockSize))).CryptBlocks(plain, data)
	pad := int(plain[len(plain)-1])
	if pad < 1 || pad > aes.BlockSize || pad > len(plain) {
		return "", errors.New("bad padding (wrong key?)")
	}
	plain = plain[:len(plain)-pad]
	if stripHostHash {
		if len(plain) < 32 {
			return "", errors.New("plaintext shorter than the host hash")
		}
		plain = plain[32:]
	}
	return string(plain), nil
}

// HostMatches keeps only cookies a browser would send to the apex domain.
// accounts.spothero.com holds host-only Auth0 cookies that spothero.com never
// receives, so a substring match would import cookies the API never sees.
func HostMatches(hostKey, domain string) bool {
	return hostKey == domain || hostKey == "."+domain
}

// Read copies the cookie database so Chrome's lock is not disturbed, then returns
// name to value for the domain. Values are never logged. The copy lives in a 0700
// temp directory because it holds every cookie in the profile.
func Read(ctx context.Context, cookiesDB, domain string, key []byte) (map[string]string, error) {
	tmp, err := os.MkdirTemp("", "spothero-cookies-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	dst := filepath.Join(tmp, "cookies.db")
	if err := copyFile(cookiesDB, dst); err != nil {
		return nil, err
	}
	_ = copyFile(cookiesDB+"-wal", dst+"-wal")
	_ = copyFile(cookiesDB+"-shm", dst+"-shm")
	db, err := sql.Open("sqlite", "file:"+dst)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	version := 0
	_ = db.QueryRowContext(ctx, `SELECT CAST(value AS INTEGER) FROM meta WHERE key='version'`).Scan(&version)
	rows, err := db.QueryContext(ctx, `SELECT host_key, name, value, encrypted_value FROM cookies`)
	if err != nil {
		return nil, fmt.Errorf("read cookie database: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var host, name, value string
		var enc []byte
		if err := rows.Scan(&host, &name, &value, &enc); err != nil {
			return nil, err
		}
		if !HostMatches(host, domain) {
			continue
		}
		if value == "" && len(enc) > 0 {
			if value, err = Decrypt(enc, key, version >= 24); err != nil {
				continue
			}
		}
		// An empty value is not a cookie: storing it is how the original false login
		// happened, so it is dropped here rather than filtered later.
		if value != "" {
			out[name] = value
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, ErrNoCookies
	}
	return out, nil
}

// LocateDB finds a profile's cookie database across Chrome's two layouts.
func LocateDB(profileDir string) (string, bool) {
	for _, p := range []string{filepath.Join(profileDir, "Cookies"), filepath.Join(profileDir, "Network", "Cookies")} {
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src) // #nosec G304 -- src is the local Chrome cookie DB chosen by the owner running auth
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o600) // #nosec G703 -- dst is inside the temp dir made by Read, not user input
}

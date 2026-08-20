package chrome

// PATCH: Chrome cookie reader with macOS/Linux decryption.
//
// On macOS, Chrome stores cookie values in the `encrypted_value` column
// (v10 prefix) encrypted with AES-128-CBC using a key derived from the
// "Chrome" keychain password via PBKDF2 (salt "saltysalt", 1003 iters).
// The previous stub read the plaintext `value` column only, so every cookie
// came back empty on macOS.

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha1"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/browserutils/kooky"
	_ "modernc.org/sqlite"
)

func ReadCookies(ctx context.Context, filename string, filters ...kooky.Filter) ([]*kooky.Cookie, error) {
	// Chrome holds an exclusive lock on the live Cookies DB while running.
	// Copy to a temp file (with WAL/SHM siblings) so we can open it read-only
	// without forcing the user to quit Chrome.
	tmp, cleanup, err := snapshotCookiesDB(filename)
	if err != nil {
		return nil, fmt.Errorf("snapshot Chrome cookies DB: %w", err)
	}
	defer cleanup()

	db, err := sql.Open("sqlite", "file:"+tmp+"?mode=ro&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, `SELECT host_key, name, value, encrypted_value, path, expires_utc, is_secure, is_httponly FROM cookies`)
	if err != nil {
		return nil, fmt.Errorf("query cookies: %w", err)
	}
	defer rows.Close()

	dec := &cookieDecrypter{}
	var cookies []*kooky.Cookie
	for rows.Next() {
		var domain, name, plaintext, path string
		var encrypted []byte
		var expiresUTC int64
		var secure, httpOnly int
		if err := rows.Scan(&domain, &name, &plaintext, &encrypted, &path, &expiresUTC, &secure, &httpOnly); err != nil {
			return nil, err
		}
		value := plaintext
		if value == "" && len(encrypted) > 0 {
			if v, derr := dec.decrypt(encrypted, domain); derr == nil {
				value = v
			}
		}
		cookie := &kooky.Cookie{Cookie: http.Cookie{
			Name:     name,
			Value:    value,
			Domain:   domain,
			Path:     path,
			Secure:   secure != 0,
			HttpOnly: httpOnly != 0,
			Expires:  chromeTime(expiresUTC),
		}}
		if kooky.FilterCookie(cookie, filters...) {
			cookies = append(cookies, cookie)
		}
	}
	return cookies, rows.Err()
}

type cookieDecrypter struct {
	once sync.Once
	key  []byte
	err  error
}

func (d *cookieDecrypter) load() {
	var password []byte
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("security", "find-generic-password", "-wa", "Chrome").Output()
		if err != nil {
			d.err = fmt.Errorf("reading Chrome key from macOS Keychain: %w (grant access when prompted, or run `security find-generic-password -wa Chrome` once to unlock)", err)
			return
		}
		password = []byte(strings.TrimSpace(string(out)))
	case "linux":
		password = []byte("peanuts")
	default:
		d.err = fmt.Errorf("cookie decryption not implemented for %s", runtime.GOOS)
		return
	}
	key, err := pbkdf2.Key(sha1.New, string(password), []byte("saltysalt"), 1003, 16)
	if err != nil {
		d.err = fmt.Errorf("deriving AES key: %w", err)
		return
	}
	d.key = key
}

func (d *cookieDecrypter) decrypt(encrypted []byte, hostKey string) (string, error) {
	d.once.Do(d.load)
	if d.err != nil {
		return "", d.err
	}
	if len(encrypted) < 3 {
		return "", fmt.Errorf("encrypted value too short")
	}
	prefix := string(encrypted[:3])
	// PATCH(chrome-drop-v11-prefix): only `v10` is the legitimate AES-128-CBC
	// prefix on macOS/Linux. `v11` is the Windows App-Bound Encryption marker
	// (AES-256-GCM with a per-cookie nonce + tag — a completely different
	// cipher), so accepting it here and decrypting as v10 would produce
	// garbage. If a future Chrome release ever emits a non-v10 prefix on
	// macOS/Linux we want a clear error, not silent corruption.
	if prefix != "v10" {
		if prefix == "v11" {
			return "", fmt.Errorf("Chrome v11 (App-Bound Encryption) cookies are not decryptable on %s with this AES-128-CBC path; this prefix is Windows-only", runtime.GOOS)
		}
		return "", fmt.Errorf("unsupported encryption version %q", prefix)
	}
	ciphertext := encrypted[3:]
	block, err := aes.NewCipher(d.key)
	if err != nil {
		return "", err
	}
	if len(ciphertext)%block.BlockSize() != 0 {
		return "", fmt.Errorf("ciphertext not a multiple of AES block size")
	}
	iv := bytes.Repeat([]byte{' '}, block.BlockSize())
	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, ciphertext)
	// Strip PKCS#7 padding. Validate every padding byte equals the pad
	// length — a wrong key or corrupted ciphertext often produces a
	// last byte that happens to land in [1, blockSize] but with garbage
	// in the rest of the trailing block, which would silently truncate
	// real cookie data.
	if n := len(plaintext); n > 0 {
		pad := int(plaintext[n-1])
		if pad < 1 || pad > block.BlockSize() || pad > n {
			return "", fmt.Errorf("invalid PKCS#7 pad length %d (likely wrong key)", pad)
		}
		for i := n - pad; i < n; i++ {
			if int(plaintext[i]) != pad {
				return "", fmt.Errorf("invalid PKCS#7 padding byte at %d (likely wrong key)", i)
			}
		}
		plaintext = plaintext[:n-pad]
	}
	// Chrome M120+ prepends SHA-256(host_key) to v10 plaintext for host binding.
	// Older v10 cookies omit this prefix, so check before stripping.
	if len(plaintext) >= 32 {
		hostHash := sha256.Sum256([]byte(hostKey))
		if bytes.Equal(plaintext[:32], hostHash[:]) {
			plaintext = plaintext[32:]
		}
	}
	return string(plaintext), nil
}

// snapshotCookiesDB copies the live Cookies DB plus its WAL/SHM siblings to a
// temp directory so the reader doesn't contend with Chrome's exclusive lock.
func snapshotCookiesDB(src string) (path string, cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "kooky-cookies-")
	if err != nil {
		return "", func() {}, err
	}
	cleanup = func() { os.RemoveAll(dir) }
	dst := dir + "/Cookies"
	if err := copyFile(src, dst); err != nil {
		cleanup()
		return "", func() {}, err
	}
	// WAL/SHM are best-effort; absence is fine for a checkpointed DB.
	_ = copyFile(src+"-wal", dst+"-wal")
	_ = copyFile(src+"-shm", dst+"-shm")
	return dst, cleanup, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func chromeTime(microseconds int64) time.Time {
	if microseconds <= 0 {
		return time.Time{}
	}
	// Chrome timestamps are microseconds since 1601-01-01 UTC. Convert to Unix
	// time directly to avoid int64 nanosecond overflow that occurs with
	// time.Add(Duration(seconds)*time.Second) for far-future dates.
	const microsBetween1601And1970 = 11644473600 * 1_000_000
	unixMicros := microseconds - microsBetween1601And1970
	return time.Unix(unixMicros/1_000_000, (unixMicros%1_000_000)*1000).UTC()
}

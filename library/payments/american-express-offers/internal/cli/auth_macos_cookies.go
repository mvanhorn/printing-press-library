//go:build darwin

package cli

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

// extractViaMacOSChrome reads Chrome cookies on macOS directly from the SQLite
// Cookies database, decrypting values using the Chrome Safe Storage key from Keychain.
// No external tools are required — only sqlite3 shell and the macOS `security` command
// (both ship with every Mac). Chrome must be closed or the DB must be copyable.
func extractViaMacOSChrome(domain string) (string, error) {
	// 1. Get the Chrome Safe Storage key from Keychain.
	keyOut, err := exec.Command("security", "find-generic-password", "-a", "Chrome", "-s", "Chrome Safe Storage", "-w").Output()
	if err != nil {
		return "", fmt.Errorf("getting Chrome Safe Storage key: %w", err)
	}
	rawKey := bytes.TrimSpace(keyOut)
	if len(rawKey) == 0 {
		return "", fmt.Errorf("Chrome Safe Storage key is empty")
	}

	// 2. Derive the 16-byte AES key via PBKDF2-SHA1.
	aesKey := pbkdf2HMACSHA1(rawKey, []byte("saltysalt"), 1003, 16)

	// 3. Locate the Cookies database for the default Chrome profile.
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cookiesDB := filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "Default", "Cookies")
	if _, err := os.Stat(cookiesDB); err != nil {
		return "", fmt.Errorf("Chrome Cookies database not found at %s", cookiesDB)
	}

	// 4. Copy the database to a temp file to avoid Chrome's write-ahead lock.
	tmp, err := os.CreateTemp("", "chrome-cookies-*.db")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)
	defer os.Remove(tmpPath + "-wal")
	defer os.Remove(tmpPath + "-shm")

	if err := copyFileIfExists(cookiesDB, tmpPath); err != nil {
		return "", fmt.Errorf("copying Cookies database: %w", err)
	}
	_ = copyFileIfExists(cookiesDB+"-wal", tmpPath+"-wal")
	_ = copyFileIfExists(cookiesDB+"-shm", tmpPath+"-shm")

	// 5. Query the relevant cookies.
	db, err := sql.Open("sqlite", tmpPath+"?mode=ro&_journal_mode=WAL")
	if err != nil {
		return "", fmt.Errorf("opening Cookies database: %w", err)
	}
	defer db.Close()

	domainPattern := "%" + strings.TrimPrefix(domain, ".") + "%"
	rows, err := db.Query(
		`SELECT name, encrypted_value FROM cookies WHERE host_key LIKE ? ORDER BY name`,
		domainPattern,
	)
	if err != nil {
		return "", fmt.Errorf("querying cookies: %w", err)
	}
	defer rows.Close()

	var parts []string
	for rows.Next() {
		var name string
		var encVal []byte
		if err := rows.Scan(&name, &encVal); err != nil {
			continue
		}
		plaintext, err := decryptChromeCookie(encVal, aesKey)
		if err != nil || plaintext == "" {
			continue
		}
		// Skip cookies with non-UTF-8 or non-printable values — these are
		// binary tracking cookies that would corrupt the TOML config file
		// and aren't needed for API authentication.
		if !utf8.ValidString(plaintext) {
			continue
		}
		parts = append(parts, name+"="+plaintext)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("reading cookies: %w", err)
	}
	return strings.Join(parts, "; "), nil
}

// decryptChromeCookie decrypts a single Chrome cookie value.
// Chrome on macOS uses AES-128-CBC with a 16-space IV and a "v10" prefix.
func decryptChromeCookie(encryptedValue, aesKey []byte) (string, error) {
	if len(encryptedValue) < 3 || string(encryptedValue[:3]) != "v10" {
		// Not encrypted (older Chrome or plaintext). Only return if it's printable UTF-8.
		s := string(encryptedValue)
		if !utf8.ValidString(s) {
			return "", nil
		}
		return s, nil
	}
	ciphertext := encryptedValue[3:]
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return "", fmt.Errorf("invalid ciphertext length %d", len(ciphertext))
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", err
	}
	iv := bytes.Repeat([]byte{' '}, aes.BlockSize)
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// PKCS#5 / PKCS#7 unpad
	if len(plaintext) == 0 {
		return "", fmt.Errorf("empty plaintext")
	}
	padLen := int(plaintext[len(plaintext)-1])
	if padLen == 0 || padLen > aes.BlockSize || padLen > len(plaintext) {
		return "", fmt.Errorf("invalid PKCS padding %d", padLen)
	}
	plaintext = plaintext[:len(plaintext)-padLen]

	// Chrome 127+ prepends a 32-byte internal header to the plaintext before
	// encryption. Skip it to get the actual cookie value.
	const chromeHeaderLen = 32
	if len(plaintext) > chromeHeaderLen {
		plaintext = plaintext[chromeHeaderLen:]
	}

	return string(plaintext), nil
}

// pbkdf2HMACSHA1 derives a key using PBKDF2 with HMAC-SHA1.
// This is exactly what pycookiecheat uses for Chrome's macOS cookie encryption.
func pbkdf2HMACSHA1(password, salt []byte, iterations, keyLen int) []byte {
	prf := hmac.New(sha1.New, password)
	hashLen := prf.Size() // 20 for SHA-1
	numBlocks := (keyLen + hashLen - 1) / hashLen

	dk := make([]byte, 0, keyLen)
	U := make([]byte, hashLen)
	T := make([]byte, hashLen)
	block := make([]byte, 4)

	for blk := 1; blk <= numBlocks; blk++ {
		block[0] = byte(blk >> 24)
		block[1] = byte(blk >> 16)
		block[2] = byte(blk >> 8)
		block[3] = byte(blk)

		prf.Reset()
		prf.Write(salt)
		prf.Write(block)
		copy(U, prf.Sum(nil))
		copy(T, U)

		for n := 2; n <= iterations; n++ {
			prf.Reset()
			prf.Write(U)
			copy(U, prf.Sum(nil))
			for i := range T {
				T[i] ^= U[i]
			}
		}
		dk = append(dk, T[:min(hashLen, keyLen-len(dk))]...)
	}
	return dk
}

// isMacOSChromeAvailable reports whether the macOS native extractor can run.
func isMacOSChromeAvailable() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	// Need the security command (always present on macOS) and the Cookies DB.
	if _, err := exec.LookPath("security"); err != nil {
		return false
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	cookiesDB := filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "Default", "Cookies")
	_, err = os.Stat(cookiesDB)
	return err == nil
}

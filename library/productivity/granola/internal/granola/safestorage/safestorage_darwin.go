// Copyright 2026 Granola Printing Press contributors. Licensed under Apache-2.0.

//go:build darwin

package safestorage

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Layer-1 constants match Electron's safeStorage v10 envelope (Chromium OSCrypt).
const (
	v10Prefix      = "v10"
	pbkdf2Salt     = "saltysalt"
	pbkdf2Iters    = 1003
	pbkdf2KeyLen   = 16
	cbcIVByte      = ' '
	keychainSvc    = "Granola Safe Storage"
	keychainAcct   = "Granola Key"
	expectedB64Len = 24 // 16 raw bytes => 24 chars base64 (with padding)
)

// granolaSupportDir mirrors workos.go's resolver. We duplicate it here
// instead of importing internal/granola to keep safestorage importable
// from anywhere without an import cycle.
func granolaSupportDir() string {
	if v := os.Getenv("GRANOLA_SUPPORT_DIR"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "Granola")
}

func storageDEKPath() string {
	return filepath.Join(granolaSupportDir(), "storage.dek")
}

// loadDEK performs the full two-tier unwrap:
//
//  1. shell out to /usr/bin/security to fetch the Keychain entry
//     (Granola Safe Storage / Granola Key) - a base64 string
//  2. PBKDF2-SHA1 derive a 16-byte AES-128 key with the base64 string
//     as the password and Chromium's "saltysalt" / 1003 iterations
//  3. AES-128-CBC decrypt storage.dek (skipping the "v10" prefix) with
//     a 16-byte ASCII-space IV; strip PKCS7 padding
//  4. base64-decode the plaintext to recover the 32-byte DEK
//
// Returns ErrKeyUnavailable when the Keychain entry is missing or denied,
// or storage.dek is absent. Returns ErrDecryptFailed when the envelope
// is malformed (suggesting Granola changed the scheme).
func loadDEK() ([]byte, error) {
	keychainB64, err := fetchKeychainEntry()
	if err != nil {
		return nil, err
	}

	dekBlob, err := os.ReadFile(storageDEKPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: storage.dek not found at %s (Granola not installed or pre-encryption version)", ErrKeyUnavailable, storageDEKPath())
		}
		return nil, fmt.Errorf("safestorage: read storage.dek: %w", err)
	}

	if !bytes.HasPrefix(dekBlob, []byte(v10Prefix)) {
		return nil, fmt.Errorf("%w: storage.dek does not start with %q (got %q)", ErrDecryptFailed, v10Prefix, dekBlob[:min(3, len(dekBlob))])
	}
	body := dekBlob[len(v10Prefix):]
	if len(body) == 0 || len(body)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("%w: storage.dek body length %d is not a multiple of %d", ErrDecryptFailed, len(body), aes.BlockSize)
	}

	key, err := pbkdf2.Key(sha1.New, keychainB64, []byte(pbkdf2Salt), pbkdf2Iters, pbkdf2KeyLen)
	if err != nil {
		return nil, fmt.Errorf("safestorage: pbkdf2: %w", err)
	}
	defer zero(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("safestorage: aes.NewCipher: %w", err)
	}
	iv := bytes.Repeat([]byte{cbcIVByte}, aes.BlockSize)
	padded := make([]byte, len(body))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(padded, body)
	defer zero(padded)

	plain, err := stripPKCS7(padded, aes.BlockSize)
	if err != nil {
		return nil, fmt.Errorf("%w: storage.dek PKCS7 strip: %v", ErrDecryptFailed, err)
	}
	defer zero(plain)

	dek, err := base64.StdEncoding.DecodeString(string(plain))
	if err != nil {
		return nil, fmt.Errorf("%w: storage.dek plaintext is not valid base64: %v", ErrDecryptFailed, err)
	}
	return dek, nil
}

// fetchKeychainEntry shells out to /usr/bin/security and returns the
// raw base64 string Granola wrote at install time. The base64 string
// itself (not the decoded bytes) is the PBKDF2 password input - this
// matches Chromium OSCrypt's macOS behavior.
//
// Note: the ACL grant on the Keychain entry attaches to /usr/bin/security
// (the requesting binary), not to granola-pp-cli. Once the user clicks
// "Always Allow", any process invoking the same security command on the
// same entry gets silent access. This is a documented trade-off (R8 in
// the plan); the alternative is a CGO Security.framework call where the
// ACL grant attaches to granola-pp-cli specifically.
func fetchKeychainEntry() (string, error) {
	cmd := exec.Command("/usr/bin/security", "find-generic-password",
		"-s", keychainSvc,
		"-a", keychainAcct,
		"-w")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if strings.Contains(msg, "could not be found") || strings.Contains(msg, "errSecItemNotFound") {
			return "", fmt.Errorf("%w: Keychain entry %q / %q not found (sign in to Granola desktop first)", ErrKeyUnavailable, keychainSvc, keychainAcct)
		}
		if strings.Contains(msg, "User interaction is not allowed") || strings.Contains(msg, "errSecAuthFailed") {
			return "", fmt.Errorf("%w: Keychain access denied or user interaction unavailable", ErrKeyUnavailable)
		}
		return "", fmt.Errorf("%w: security: %s (%v)", ErrKeyUnavailable, msg, err)
	}

	b64 := strings.TrimSpace(string(out))
	if len(b64) != expectedB64Len {
		return "", fmt.Errorf("%w: Keychain entry has unexpected length %d (expected %d)", ErrDecryptFailed, len(b64), expectedB64Len)
	}
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(decoded) != 16 {
		return "", fmt.Errorf("%w: Keychain entry is not a valid base64 16-byte value", ErrDecryptFailed)
	}
	return b64, nil
}

// stripPKCS7 removes RFC 5652 PKCS7 padding from a CBC-decrypted block.
func stripPKCS7(b []byte, blockSize int) ([]byte, error) {
	if len(b) == 0 || len(b)%blockSize != 0 {
		return nil, fmt.Errorf("invalid input length %d", len(b))
	}
	pad := int(b[len(b)-1])
	if pad < 1 || pad > blockSize {
		return nil, fmt.Errorf("invalid pad byte %d", pad)
	}
	for i := len(b) - pad; i < len(b); i++ {
		if int(b[i]) != pad {
			return nil, fmt.Errorf("pad byte %d at offset %d does not match expected %d", b[i], i, pad)
		}
	}
	return b[:len(b)-pad], nil
}

// parseKeyOverride parses the GRANOLA_SAFESTORAGE_KEY_OVERRIDE env var.
// Format: base64-encoded 32 raw bytes. Used by tests and CI lanes that
// cannot prompt the Keychain.
func parseKeyOverride(s string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}
	if len(raw) != dekLen {
		return nil, fmt.Errorf("decoded length %d, expected %d", len(raw), dekLen)
	}
	return raw, nil
}

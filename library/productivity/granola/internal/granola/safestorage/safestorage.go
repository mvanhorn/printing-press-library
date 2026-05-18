// Copyright 2026 Granola Printing Press contributors. Licensed under Apache-2.0.

// PATCH(safestorage-package): new package providing two-tier decryption of
// Granola's encrypted local storage (cache-v6.json.enc, supabase.json.enc,
// user-preferences.json.enc) that Granola desktop began writing around May
// 2026. See library/productivity/granola/.printing-press-patches.json
// patches[1] and library/productivity/granola/internal/granola/safestorage/
// testdata/scheme.md for the empirical scheme finding.

// Package safestorage decrypts Granola's encrypted local-storage files.
// Granola desktop (>= ~May 2026) uses a two-tier scheme:
//
//  1. ~/Library/Application Support/Granola/storage.dek holds a 32-byte
//     Data Encryption Key (DEK), itself wrapped by Electron's safeStorage
//     v10 envelope (AES-128-CBC with a Chromium-derived key from the
//     "Granola Safe Storage" macOS Keychain entry).
//  2. cache-v6.json.enc, supabase.json.enc, and user-preferences.json.enc
//     are AES-256-GCM with the DEK as the key.
//
// This package owns the full unwrap. Callers receive plaintext bytes
// from Decrypt and otherwise never touch keys or ciphertext envelopes.
// The empirical scheme finding is documented in testdata/scheme.md.
package safestorage

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"os"
	"sync"
)

// Error sentinels callers can match via errors.Is.
var (
	// ErrUnsupportedPlatform is returned by Key on non-darwin builds where
	// the Keychain integration is not yet implemented.
	ErrUnsupportedPlatform = errors.New("safestorage: unsupported platform")

	// ErrKeyUnavailable means the Keychain entry was missing, the user
	// denied access, or storage.dek does not exist (Granola not installed
	// or pre-encryption version).
	ErrKeyUnavailable = errors.New("safestorage: Keychain key or DEK unavailable")

	// ErrDecryptFailed means the GCM auth tag rejected. This is the
	// scheme-drift signal: the key works for the envelope shape we
	// expect, but the bytes we got do not authenticate.
	ErrDecryptFailed = errors.New("safestorage: GCM authentication failed")
)

const (
	dekLen    = 32
	gcmNonce  = 12
	gcmTagLen = 16
)

// dekCache holds the 32-byte DEK after a successful Key call. We cache
// success only - a denial or missing entry never populates this slot,
// so a retry after fixing the underlying issue (clicking Always Allow,
// signing into Granola) succeeds without process restart. Long-lived
// agents that need to clear a stale DEK call Reset.
var (
	dekMu    sync.Mutex
	dekValue []byte
)

// Key returns the 32-byte DEK used to decrypt Granola's .enc files.
// The first successful call shells out to macOS Keychain (triggering
// the system prompt the first time) and unwraps storage.dek; subsequent
// calls return the cached value within the same process. Returns
// ErrUnsupportedPlatform on non-darwin, ErrKeyUnavailable when the
// Keychain entry is missing or denied, or ErrDecryptFailed if the
// envelope no longer matches the expected shape.
func Key() ([]byte, error) {
	if override := os.Getenv("GRANOLA_SAFESTORAGE_KEY_OVERRIDE"); override != "" {
		dek, err := parseKeyOverride(override)
		if err != nil {
			return nil, fmt.Errorf("safestorage: GRANOLA_SAFESTORAGE_KEY_OVERRIDE: %w", err)
		}
		return dek, nil
	}

	dekMu.Lock()
	defer dekMu.Unlock()
	if dekValue != nil {
		out := make([]byte, len(dekValue))
		copy(out, dekValue)
		return out, nil
	}

	dek, err := loadDEK()
	if err != nil {
		return nil, err
	}
	if len(dek) != dekLen {
		return nil, fmt.Errorf("%w: DEK length %d, expected %d", ErrDecryptFailed, len(dek), dekLen)
	}

	dekValue = make([]byte, len(dek))
	copy(dekValue, dek)
	return dek, nil
}

// Available reports whether Key has succeeded at least once in this
// process. doctor uses this to surface state without itself triggering
// the Keychain prompt.
func Available() bool {
	dekMu.Lock()
	defer dekMu.Unlock()
	return dekValue != nil
}

// Reset clears the in-memory DEK cache. Long-running agents call this
// when a sync attempt has returned ErrKeyUnavailable and the user has
// (e.g.) signed back into Granola so a retry can re-fetch.
func Reset() {
	dekMu.Lock()
	defer dekMu.Unlock()
	zero(dekValue)
	dekValue = nil
}

// Decrypt unwraps an AES-256-GCM blob produced by Granola desktop's
// layer-2 encryption. ciphertext must be at least nonce + tag + 1 bytes;
// the envelope shape is nonce(12) || ciphertext || tag(16) with no AAD.
// Plaintext is returned freshly allocated; callers should ZeroBytes it
// when they are done parsing.
func Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < gcmNonce+gcmTagLen+1 {
		return nil, fmt.Errorf("%w: ciphertext too short (%d bytes)", ErrDecryptFailed, len(ciphertext))
	}
	dek, err := Key()
	if err != nil {
		return nil, err
	}
	defer zero(dek)

	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("safestorage: aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("safestorage: cipher.NewGCM: %w", err)
	}
	nonce := ciphertext[:gcm.NonceSize()]
	body := ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptFailed, err)
	}
	return plaintext, nil
}

// ZeroBytes overwrites the slice with zeros so the Go garbage collector
// is not the only thing keeping decrypted secrets out of swap. Callers
// receiving plaintext from Decrypt should defer ZeroBytes(plaintext)
// once they have parsed it into the destination struct.
func ZeroBytes(b []byte) {
	zero(b)
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

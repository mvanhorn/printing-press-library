// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

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
	// or pre-encryption version). ErrSchemeMigrated is a refinement of
	// this sentinel, not an alternative to it - match the narrower one
	// first when the distinction matters.
	ErrKeyUnavailable = errors.New("safestorage: Keychain key or DEK unavailable")

	// ErrDecryptFailed means the GCM auth tag rejected. This is the
	// scheme-drift signal: the key works for the envelope shape we
	// expect, but the bytes we got do not authenticate.
	ErrDecryptFailed = errors.New("safestorage: GCM authentication failed")

	// PATCH(dek-migration): Granola desktop 7.447.1 moved the 32-byte DEK
	// out of storage.dek and into the macOS data-protection Keychain,
	// service com.granola.app.dek in access group QZ7DHHLN25.granola. That
	// access group is gated by a keychain-access-groups entitlement bound
	// to Granola's Apple Team ID, so no third-party binary can read it.
	// ErrSchemeMigrated is the honest classification for that state: the
	// local key path is closed upstream, not misconfigured locally.
	//
	// Errors carrying this sentinel also report as ErrKeyUnavailable (see
	// migratedSchemeError) - the key genuinely is unavailable, so callers
	// that only know the older sentinel keep their existing fallbacks.
	// Callers that want the precise reason match this one first.
	ErrSchemeMigrated = errors.New("safestorage: Granola moved the DEK to an entitlement-gated Keychain group")
)

const (
	dekLen    = 32
	gcmNonce  = 12
	gcmTagLen = 16
)

// PATCH(dek-migration): migratedSchemeError is the ErrSchemeMigrated
// carrier. It reports as both ErrSchemeMigrated and ErrKeyUnavailable so
// that adding the finer classification never removes a fallback: every
// existing errors.Is(err, ErrKeyUnavailable) branch behaves exactly as it
// did, while callers that branch on ErrSchemeMigrated first get the
// accurate message. Both statements are true - the key is unavailable,
// and it is unavailable because upstream moved it.
type migratedSchemeError struct{ msg string }

func (e *migratedSchemeError) Error() string { return e.msg }

func (e *migratedSchemeError) Is(target error) bool {
	return target == ErrSchemeMigrated || target == ErrKeyUnavailable
}

// newMigratedSchemeError builds the operator-facing migrated-scheme error.
// state describes the observed on-disk/Keychain signature we classified
// from; the shared explanation and remedy are appended so both migrated
// classifications (unlinked storage.dek, stale storage.dek) tell the user
// the same thing about what still works.
func newMigratedSchemeError(state string) error {
	return &migratedSchemeError{msg: fmt.Sprintf("%s: %s. %s", ErrSchemeMigrated.Error(), state, migratedSchemeRemedy)}
}

// migratedSchemeRemedy names the path that still works without sending the
// user somewhere they cannot go. Deliberately absent: any instruction to
// re-run sync or to approve a Keychain prompt. Neither can succeed once
// the key lives in an access group gated by an entitlement bound to
// Granola's own Team ID, and telling the user to try wastes their time.
//
// The override is named because it is a genuine local remedy, not just a
// test seam: the upstream migration imports the existing DEK rather than
// generating a fresh one, so a pre-migration storage.dek recovered from a
// backup still decrypts today's cache-v6.json.enc.
const migratedSchemeRemedy = "Granola desktop now keeps the data encryption key in a Keychain access group gated by an entitlement bound to its own Team ID, so no third-party binary can read it. " +
	"Fetching newly recorded meetings needs a Granola API key, which requires a Business or Enterprise Granola workspace. " +
	"Data already synced to the local store remains readable. " +
	"If you kept a copy of storage.dek from before the migration, base64-encode its 32-byte DEK into GRANOLA_SAFESTORAGE_KEY_OVERRIDE: the migration imported the existing DEK rather than generating a new one, so the old key still decrypts today's files."

// keySource records where the DEK in hand came from. Decrypt uses it to
// tell a stale storage.dek left behind by a failed upstream migration
// apart from a genuine envelope drift - only a key read off disk can be
// stale, an operator-supplied override cannot.
type keySource int

const (
	keySourceOverride keySource = iota
	keySourceStorageDEK
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
	dek, _, err := keyWithSource()
	return dek, err
}

// keyWithSource is Key plus the provenance Decrypt needs to classify a
// GCM failure. The override branch stays ahead of everything else so that
// tests, CI lanes, and any user supplying a recovered DEK out of band are
// unaffected by the scheme classification below it.
func keyWithSource() ([]byte, keySource, error) {
	if override := os.Getenv("GRANOLA_SAFESTORAGE_KEY_OVERRIDE"); override != "" {
		dek, err := parseKeyOverride(override)
		if err != nil {
			return nil, keySourceOverride, fmt.Errorf("safestorage: GRANOLA_SAFESTORAGE_KEY_OVERRIDE: %w", err)
		}
		return dek, keySourceOverride, nil
	}

	dekMu.Lock()
	defer dekMu.Unlock()
	if dekValue != nil {
		out := make([]byte, len(dekValue))
		copy(out, dekValue)
		return out, keySourceStorageDEK, nil
	}

	dek, err := loadDEK()
	if err != nil {
		return nil, keySourceStorageDEK, err
	}
	if len(dek) != dekLen {
		return nil, keySourceStorageDEK, fmt.Errorf("%w: DEK length %d, expected %d", ErrDecryptFailed, len(dek), dekLen)
	}

	dekValue = make([]byte, len(dek))
	copy(dekValue, dek)
	return dek, keySourceStorageDEK, nil
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
	dek, src, err := keyWithSource()
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
		// PATCH(dek-migration): a DEK that unwrapped cleanly out of
		// storage.dek but does not authenticate Granola's own ciphertext
		// is the failed-migration state, not envelope drift. Granola
		// unlinks storage.dek only after a successful import and wraps
		// that import in a five-attempt retry because it can fail at
		// launch, so a failed import leaves the old key file on disk
		// while the app has already rotated to a fresh Keychain DEK.
		// Without this branch such an install reads as a generic decrypt
		// failure and the operator is told to go fix a Keychain prompt
		// that cannot help. Only a key read off disk can be stale, so
		// the override path is never reclassified.
		if src == keySourceStorageDEK {
			if staleErr := classifyStaleDEKFile(); staleErr != nil {
				return nil, staleErr
			}
		}
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

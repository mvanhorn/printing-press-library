// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built: passphrase-sealed vault for ht-ml.app update_keys. The update_key
// is returned only once at creation with no recovery endpoint, so this is the
// only disaster-recovery and cross-machine path. Uses stdlib crypto only
// (pbkdf2 + AES-256-GCM); no third-party dependency. Survives generate --force.

package cli

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// vaultPassphraseEnv is where the passphrase for encrypted export/import is read
// from, so the secret never lands in shell history or process args.
const vaultPassphraseEnv = "HT_ML_VAULT_PASSPHRASE"

const vaultFormat = "ht-ml-vault-v1"

// vaultEnvelope is the on-disk encrypted vault. The plaintext (a JSON array of
// keyExportRecord) is sealed with AES-256-GCM under a pbkdf2-derived key.
type vaultEnvelope struct {
	Format     string `json:"format"`
	KDF        string `json:"kdf"`
	Iterations int    `json:"iterations"`
	Salt       []byte `json:"salt"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

const vaultPBKDF2Iterations = 600000

func deriveVaultKey(passphrase string, salt []byte) ([]byte, error) {
	return pbkdf2.Key(sha256.New, passphrase, salt, vaultPBKDF2Iterations, 32)
}

// encryptVault seals plaintext under passphrase and returns a JSON envelope.
func encryptVault(plaintext []byte, passphrase string) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key, err := deriveVaultKey(passphrase, salt)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	env := vaultEnvelope{
		Format:     vaultFormat,
		KDF:        "pbkdf2-sha256",
		Iterations: vaultPBKDF2Iterations,
		Salt:       salt,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}
	out, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return nil, err
	}
	return out, nil
}

// decryptVault opens a JSON envelope produced by encryptVault.
func decryptVault(data []byte, passphrase string) ([]byte, error) {
	var env vaultEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("not a valid vault file: %w", err)
	}
	if env.Format != vaultFormat {
		return nil, fmt.Errorf("unrecognized vault format %q", env.Format)
	}
	// env.Iterations comes from an untrusted vault file (disaster-recovery and
	// cross-machine workflows accept vaults from third parties). Bound it so a
	// pathological count cannot make pbkdf2.Key spin effectively forever.
	const minIter, maxIter = 100_000, 10_000_000
	if env.Iterations < minIter || env.Iterations > maxIter {
		return nil, fmt.Errorf("vault iteration count %d is outside the accepted range [%d, %d]", env.Iterations, minIter, maxIter)
	}
	key, err := pbkdf2.Key(sha256.New, passphrase, env.Salt, env.Iterations, 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plaintext, err := gcm.Open(nil, env.Nonce, env.Ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (wrong passphrase or corrupt vault)")
	}
	return plaintext, nil
}

// isEncryptedVault reports whether data looks like a vaultEnvelope (vs a
// plaintext export array).
func isEncryptedVault(data []byte) bool {
	trimmed := strings.TrimSpace(string(data))
	if !strings.HasPrefix(trimmed, "{") {
		return false
	}
	var probe struct {
		Format string `json:"format"`
	}
	return json.Unmarshal(data, &probe) == nil && probe.Format == vaultFormat
}

// vaultPassphrase reads the export/import passphrase from the environment.
func vaultPassphrase() (string, error) {
	p := os.Getenv(vaultPassphraseEnv)
	if strings.TrimSpace(p) == "" {
		return "", configErr(fmt.Errorf("set %s to a passphrase for the encrypted vault, or pass --insecure-plaintext to export unencrypted", vaultPassphraseEnv))
	}
	return p, nil
}

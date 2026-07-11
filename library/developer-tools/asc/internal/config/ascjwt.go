// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
//
// App Store Connect auth is NOT a static bearer token. The ASC API requires a
// short-lived (<=20 min) JWT signed with ES256 using your App Store Connect API
// key (.p8 private key + key id + issuer id). This file mints and caches that
// JWT from the ASC_KEY_ID / ASC_ISSUER_ID / ASC_PRIVATE_KEY_PATH env trio.
package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"sync"
	"time"
)

// ascTokenMu serializes JWT minting + cache access. AuthHeader() (hence
// ascBearer) can be reached from concurrent request goroutines if the client is
// ever fanned out; a package-level lock keeps the per-Config token cache safe
// without embedding a lock in the copied Config struct.
var ascTokenMu sync.Mutex

// ascSigningConfigured reports whether the .p8 signing credentials are present.
func (c *Config) ascSigningConfigured() bool {
	return c.ASCKeyID != "" && c.ASCIssuerID != "" && (c.ASCPrivateKeyPath != "" || c.ASCPrivateKey != "")
}

// ascBearer returns a valid "Bearer <jwt>" auth header, minting (and caching)
// the JWT as needed. The token is cached in-process until ~60s before expiry so
// repeated AuthHeader() calls — and the client's response cache key — stay stable.
func (c *Config) ascBearer() (string, error) {
	ascTokenMu.Lock()
	defer ascTokenMu.Unlock()
	if c.ascCachedToken != "" && time.Now().Before(c.ascTokenExp.Add(-60*time.Second)) {
		return "Bearer " + c.ascCachedToken, nil
	}

	pemBytes := []byte(c.ASCPrivateKey)
	if len(pemBytes) == 0 {
		b, err := os.ReadFile(c.ASCPrivateKeyPath)
		if err != nil {
			return "", fmt.Errorf("reading ASC_PRIVATE_KEY_PATH (%s): %w", c.ASCPrivateKeyPath, err)
		}
		pemBytes = b
	}
	key, err := parseECPrivateKey(pemBytes)
	if err != nil {
		return "", err
	}

	now := time.Now()
	exp := now.Add(19 * time.Minute) // Apple rejects exp > 20 min from iat.
	header := map[string]string{"alg": "ES256", "kid": c.ASCKeyID, "typ": "JWT"}
	payload := map[string]any{
		"iss": c.ASCIssuerID,
		"iat": now.Unix(),
		"exp": exp.Unix(),
		"aud": "appstoreconnect-v1",
	}
	hb, _ := json.Marshal(header)
	pb, _ := json.Marshal(payload)
	signingInput := b64url(hb) + "." + b64url(pb)

	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return "", fmt.Errorf("signing ASC JWT: %w", err)
	}
	// JWS ES256 signature is r||s, each left-padded to 32 bytes big-endian.
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])

	token := signingInput + "." + b64url(sig)
	c.ascCachedToken = token
	c.ascTokenExp = exp
	return "Bearer " + token, nil
}

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func parseECPrivateKey(pemBytes []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("ASC private key is not valid PEM (expected a .p8 file)")
	}
	// App Store Connect .p8 keys are PKCS#8; fall back to SEC1 just in case.
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		ec, ok := k.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("ASC private key is not an ECDSA key")
		}
		return checkP256(ec)
	}
	if ec, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return checkP256(ec)
	}
	return nil, fmt.Errorf("parsing ASC private key: unsupported key format")
}

// checkP256 rejects non-P-256 keys before signing — ES256 requires P-256, and a
// P-384/P-521 key would produce r/s > 32 bytes and panic FillBytes.
func checkP256(ec *ecdsa.PrivateKey) (*ecdsa.PrivateKey, error) {
	if ec.Curve != elliptic.P256() {
		return nil, fmt.Errorf("ASC API keys must be ES256/P-256; got curve %s", ec.Curve.Params().Name)
	}
	return ec, nil
}

// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
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
	"math/big"
	"strings"
	"testing"
	"time"
)

// makeP8 generates a P-256 key and returns its PKCS#8 PEM (the .p8 shape Apple
// hands out) plus the public key for signature verification.
func makeP8(t *testing.T) (string, *ecdsa.PublicKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pemStr := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	return pemStr, &key.PublicKey
}

func TestASCBearerSignsValidES256JWT(t *testing.T) {
	pemStr, pub := makeP8(t)
	c := &Config{
		ASCKeyID:      "ABC123DEFG",
		ASCIssuerID:   "69a6de70-1111-2222-3333-444455556666",
		ASCPrivateKey: pemStr,
	}
	if !c.ascSigningConfigured() {
		t.Fatal("ascSigningConfigured = false with full config")
	}

	header, err := c.ascBearer()
	if err != nil {
		t.Fatalf("ascBearer error: %v", err)
	}
	if !strings.HasPrefix(header, "Bearer ") {
		t.Fatalf("missing Bearer prefix: %q", header)
	}
	token := strings.TrimPrefix(header, "Bearer ")
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT should have 3 parts, got %d", len(parts))
	}

	// Header claims.
	var hdr map[string]string
	decodeJSON(t, parts[0], &hdr)
	if hdr["alg"] != "ES256" || hdr["kid"] != c.ASCKeyID || hdr["typ"] != "JWT" {
		t.Fatalf("bad header: %+v", hdr)
	}

	// Payload claims.
	var pl map[string]any
	decodeJSON(t, parts[1], &pl)
	if pl["iss"] != c.ASCIssuerID {
		t.Errorf("iss = %v, want %v", pl["iss"], c.ASCIssuerID)
	}
	if pl["aud"] != "appstoreconnect-v1" {
		t.Errorf("aud = %v, want appstoreconnect-v1", pl["aud"])
	}
	iat := int64(pl["iat"].(float64))
	exp := int64(pl["exp"].(float64))
	if exp <= iat || exp-iat > 1200 {
		t.Errorf("exp-iat = %d, must be >0 and <=1200 (Apple's 20-min cap)", exp-iat)
	}

	// Signature verifies against the public key.
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != 64 {
		t.Fatalf("bad signature encoding (len=%d, err=%v)", len(sig), err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(pub, digest[:], r, s) {
		t.Fatal("ES256 signature does not verify against the public key")
	}
}

func TestASCBearerCachesToken(t *testing.T) {
	pemStr, _ := makeP8(t)
	c := &Config{ASCKeyID: "K", ASCIssuerID: "I", ASCPrivateKey: pemStr}
	first, err := c.ascBearer()
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.ascBearer()
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Error("expected the minted token to be cached (stable across calls for a stable cache key)")
	}
	// Force expiry → new token.
	c.ascTokenExp = time.Now().Add(-time.Hour)
	third, _ := c.ascBearer()
	if third == second {
		t.Error("expected a fresh token after expiry")
	}
}

func TestASCPreflightErrors(t *testing.T) {
	if err := (&Config{}).ASCPreflight(); err == nil {
		t.Error("expected error when signing not configured")
	}
	bad := &Config{ASCKeyID: "K", ASCIssuerID: "I", ASCPrivateKey: "not a pem"}
	if err := bad.ASCPreflight(); err == nil {
		t.Error("expected error on unparseable private key")
	}
}

func decodeJSON(t *testing.T, seg string, v any) {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("json decode: %v", err)
	}
}

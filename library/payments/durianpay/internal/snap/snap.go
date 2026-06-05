// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored SNAP (Standar Nasional Open API Pembayaran) transport for Durianpay.
//
// SNAP requires two signature schemes the generated client cannot produce:
//   - B2B access token: RSA-SHA256 over "clientKey|timestamp" with the merchant's
//     private key (token TTL 900s).
//   - Transactions: HMAC-SHA512 over
//     "METHOD:path:accessToken:lowerhex(sha256(minify(body))):timestamp"
//     with the client secret, plus X-PARTNER-ID / X-EXTERNAL-ID / CHANNEL-ID headers.
package snap

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// ErrMissingCredentials is returned (wrapped) when required SNAP env vars are
// unset, so callers can branch on it with errors.Is.
var ErrMissingCredentials = errors.New("SNAP credentials missing")

// Config holds the SNAP credential set, resolved from environment variables.
type Config struct {
	ClientKey    string // DURIANPAY_SNAP_CLIENT_KEY — also the default X-PARTNER-ID
	ClientSecret string // DURIANPAY_SNAP_CLIENT_SECRET — HMAC key for transaction signatures
	PrivateKey   string // DURIANPAY_SNAP_PRIVATE_KEY — PEM content or path to a .pem file
	PartnerID    string // DURIANPAY_SNAP_PARTNER_ID — defaults to ClientKey
	ChannelID    string // DURIANPAY_SNAP_CHANNEL_ID — numeric channel identifier
	MerchantID   string // DURIANPAY_MERCHANT_ID — used as sourceAccountNo / balance accountNo
	BaseURL      string // derived from the legacy base URL: /v1 -> /v1.0
}

// LoadConfig resolves SNAP credentials from the environment. legacyBaseURL is
// the generated config's BaseURL (e.g. https://api.durianpay.id/v1); the SNAP
// base swaps the version segment to /v1.0 so sandbox/live overrides carry over.
func LoadConfig(legacyBaseURL string) *Config {
	cfg := &Config{
		ClientKey:    os.Getenv("DURIANPAY_SNAP_CLIENT_KEY"),
		ClientSecret: os.Getenv("DURIANPAY_SNAP_CLIENT_SECRET"),
		PrivateKey:   os.Getenv("DURIANPAY_SNAP_PRIVATE_KEY"),
		PartnerID:    os.Getenv("DURIANPAY_SNAP_PARTNER_ID"),
		ChannelID:    os.Getenv("DURIANPAY_SNAP_CHANNEL_ID"),
		MerchantID:   os.Getenv("DURIANPAY_MERCHANT_ID"),
		BaseURL:      SNAPBaseURL(legacyBaseURL),
	}
	if cfg.PartnerID == "" {
		cfg.PartnerID = cfg.ClientKey
	}
	if cfg.ChannelID == "" {
		// 95221 is the ASPI-standard host-to-host (API) channel identifier used
		// throughout Durianpay's SNAP examples; override with DURIANPAY_SNAP_CHANNEL_ID.
		cfg.ChannelID = "95221"
	}
	return cfg
}

// SNAPBaseURL converts a legacy base URL (.../v1) to the SNAP base (.../v1.0).
func SNAPBaseURL(legacyBaseURL string) string {
	base := strings.TrimSuffix(legacyBaseURL, "/")
	if strings.HasSuffix(base, "/v1.0") {
		return base
	}
	if strings.HasSuffix(base, "/v1") {
		return strings.TrimSuffix(base, "/v1") + "/v1.0"
	}
	return base + "/v1.0"
}

// MissingCredentials lists which required SNAP env vars are unset. Empty means ready.
func (c *Config) MissingCredentials() []string {
	var missing []string
	if c.ClientKey == "" {
		missing = append(missing, "DURIANPAY_SNAP_CLIENT_KEY")
	}
	if c.ClientSecret == "" {
		missing = append(missing, "DURIANPAY_SNAP_CLIENT_SECRET")
	}
	if c.PrivateKey == "" {
		missing = append(missing, "DURIANPAY_SNAP_PRIVATE_KEY")
	}
	return missing
}

// Timestamp formats t in the ISO 8601 shape Durianpay's signature scheme
// expects: 2006-01-02T15:04:05.000+07:00 (milliseconds, numeric zone colon-separated).
func Timestamp(t time.Time) string {
	return t.Format("2006-01-02T15:04:05.000-07:00")
}

// loadPrivateKey accepts PEM content or a filesystem path to a PEM file and
// returns the RSA private key (PKCS#1 or PKCS#8).
func loadPrivateKey(keyOrPath string) (*rsa.PrivateKey, error) {
	data := []byte(keyOrPath)
	if !strings.Contains(keyOrPath, "-----BEGIN") {
		// #nosec G304 -- keyOrPath is the private-key path the user explicitly
		// supplied (flag or DURIANPAY_SNAP_PRIVATE_KEY); reading it is the intent.
		b, err := os.ReadFile(keyOrPath)
		if err != nil {
			return nil, fmt.Errorf("reading private key file %q: %w", keyOrPath, err)
		}
		data = b
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("private key is not valid PEM (set DURIANPAY_SNAP_PRIVATE_KEY to PEM content or a .pem path)")
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing private key: %w", err)
	}
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is %T, want RSA", parsed)
	}
	return rsaKey, nil
}

// SignTokenRequest produces the asymmetric X-SIGNATURE for the B2B access-token
// call: base64(RSA-SHA256(clientKey + "|" + timestamp)).
func SignTokenRequest(privateKeyOrPath, clientKey, timestamp string) (string, error) {
	key, err := loadPrivateKey(privateKeyOrPath)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(clientKey + "|" + timestamp))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("signing token request: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// MinifyJSON compacts a JSON body for hashing. Non-JSON or empty input is
// returned as-is (the spec hashes the raw body when it is not a JSON document).
func MinifyJSON(body []byte) []byte {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return []byte("")
	}
	var buf strings.Builder
	dec := json.NewDecoder(strings.NewReader(trimmed))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return []byte(trimmed)
	}
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return []byte(trimmed)
	}
	return []byte(strings.TrimSuffix(buf.String(), "\n"))
}

// BodyHash returns lowerhex(sha256(minify(body))) — the body component of the
// transaction string-to-sign.
func BodyHash(body []byte) string {
	sum := sha256.Sum256(MinifyJSON(body))
	return strings.ToLower(hex.EncodeToString(sum[:]))
}

// StringToSign builds the transaction signing payload:
// METHOD:path:accessToken:lowerhex(sha256(minify(body))):timestamp
func StringToSign(method, path, accessToken string, body []byte, timestamp string) string {
	return strings.ToUpper(method) + ":" + path + ":" + accessToken + ":" + BodyHash(body) + ":" + timestamp
}

// SignTransaction produces the symmetric X-SIGNATURE for transaction calls:
// base64(HMAC-SHA512(stringToSign, clientSecret)).
func SignTransaction(clientSecret, stringToSign string) string {
	mac := hmac.New(sha512.New, []byte(clientSecret))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// ExternalID generates a day-unique X-EXTERNAL-ID (numeric timestamp + random suffix).
func ExternalID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%d%s", time.Now().UnixNano(), hex.EncodeToString(b[:4]))
}

// GenerateKeypair creates an RSA-2048 private/public PEM pair for SNAP onboarding.
func GenerateKeypair() (privatePEM, publicPEM string, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("generating RSA key: %w", err)
	}
	privDER := x509.MarshalPKCS1PrivateKey(key)
	privatePEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privDER}))
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("encoding public key: %w", err)
	}
	publicPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))
	return privatePEM, publicPEM, nil
}

// parsePublicKey parses a PKIX (or PKCS#1) DER public key. Used by webhook
// verification and tests.
func parsePublicKey(der []byte) (*rsa.PublicKey, error) {
	if pub, err := x509.ParsePKIXPublicKey(der); err == nil {
		if rsaPub, ok := pub.(*rsa.PublicKey); ok {
			return rsaPub, nil
		}
		return nil, fmt.Errorf("public key is not RSA")
	}
	return x509.ParsePKCS1PublicKey(der)
}

// VerifyRSASignature verifies a base64 RSA-SHA256 signature over payload with a
// PEM public key (used for SNAP webhook notify verification).
func VerifyRSASignature(publicKeyPEM string, payload []byte, signatureB64 string) error {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return fmt.Errorf("public key is not valid PEM")
	}
	pub, err := parsePublicKey(block.Bytes)
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return fmt.Errorf("signature is not valid base64: %w", err)
	}
	digest := sha256.Sum256(payload)
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig)
}

// LegacyCompletionSignature computes the disbursement completion signature:
// lowerhex(HMAC-SHA256(disbursementID + "|" + amount, apiKey)).
func LegacyCompletionSignature(disbursementID, amount, apiKey string) string {
	mac := hmac.New(sha256.New, []byte(apiKey))
	mac.Write([]byte(disbursementID + "|" + amount))
	return hex.EncodeToString(mac.Sum(nil))
}

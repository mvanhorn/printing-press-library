// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package ga4

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

func MintToken(key ServiceAccountKey) (string, error) {
	if key.TokenURI == "" {
		key.TokenURI = "https://oauth2.googleapis.com/token"
	}
	block, _ := pem.Decode([]byte(key.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("invalid service-account private_key PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}
	pk, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("service-account private key is not RSA")
	}
	now := time.Now().Unix()
	header := b64json(map[string]string{"alg": "RS256", "typ": "JWT"})
	claims := b64json(map[string]any{"iss": key.ClientEmail, "scope": AnalyticsReadonlyScope, "aud": key.TokenURI, "iat": now, "exp": now + 3600})
	unsigned := header + "." + claims
	h := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, pk, crypto.SHA256, h[:])
	if err != nil {
		return "", err
	}
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {unsigned + "." + base64.RawURLEncoding.EncodeToString(sig)}}
	resp, err := http.PostForm(key.TokenURI, form)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", APIError{Status: resp.StatusCode, Body: string(body)}
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("token response missing access_token")
	}
	return out.AccessToken, nil
}
func b64json(v any) string { b, _ := json.Marshal(v); return base64.RawURLEncoding.EncodeToString(b) }

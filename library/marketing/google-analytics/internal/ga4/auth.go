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

func MintToken(creds Credentials, scope string) (string, error) {
	switch creds.Kind() {
	case "service_account":
		return mintServiceAccountToken(creds, scope)
	case "authorized_user":
		return mintRefreshToken(creds)
	default:
		return "", fmt.Errorf("unsupported credential shape: provide a service_account (client_email + private_key) or an authorized_user (client_id + client_secret + refresh_token) JSON")
	}
}

func mintServiceAccountToken(key Credentials, scope string) (string, error) {
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
	claims := b64json(map[string]any{"iss": key.ClientEmail, "scope": scope, "aud": key.TokenURI, "iat": now, "exp": now + 3600})
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

// mintRefreshToken exchanges an authorized_user refresh token for an access
// token. Scope is intentionally not sent: the original grant already fixes it,
// and callers must not assume the result is narrowed to any requested scope.
func mintRefreshToken(creds Credentials) (string, error) {
	uri := creds.TokenURI
	if uri == "" {
		uri = "https://oauth2.googleapis.com/token"
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {creds.ClientID},
		"client_secret": {creds.ClientSecret},
		"refresh_token": {creds.RefreshToken},
	}
	resp, err := http.PostForm(uri, form)
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

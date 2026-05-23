// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// L2 HMAC header construction for authenticated Polymarket CLOB endpoints.
//
// Wire shape (matches py-clob-client / java-clob-client):
//   POLY_ADDRESS:    EOA address (string, EIP-55 checksummed)
//   POLY_API_KEY:    L2 api_key (UUID format)
//   POLY_PASSPHRASE: L2 passphrase (raw string from /auth/derive-api-key)
//   POLY_SIGNATURE:  HMAC-SHA256(base64url-decoded secret, timestamp || method || path || body), base64url-encoded
//   POLY_TIMESTAMP:  unix seconds
//
// Critical: the secret returned by /auth/derive-api-key is base64url-encoded
// and must be DECODED before being used as the HMAC key. The signature output
// is base64url-encoded with padding.
//
// Body normalization: every request body MUST be marshaled with the SAME
// JSON layout used to compute the HMAC. Polymarket's CLOB validates the
// signature against the raw bytes it receives, so a single whitespace
// difference between BuildL2Headers' body and the actual HTTP body causes
// HTTP 401 "invalid signature". Always compute the JSON bytes ONCE and
// reuse them for both HMAC + body.

package clob

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
)

// L2Creds bundles the trio returned by /auth/derive-api-key.
type L2Creds struct {
	APIKey     string
	Secret     string // base64url-encoded; HMAC key is its decoded bytes
	Passphrase string
}

// BuildL2Headers returns the five POLY_* headers ready to ship with any
// L2-authenticated CLOB request. address is the EOA (signer) address.
// method MUST be uppercase (GET, POST, DELETE). path MUST start with "/".
// rawBody is the EXACT bytes to be sent as the HTTP body (use nil for
// requests with no body — NOT an empty []byte{}).
//
// For GETs with query params, Polymarket's HMAC signature path includes
// the query string. Callers should pre-build the path-with-query string
// (e.g. "/orders?market=0x...") and pass it as `path`.
func BuildL2Headers(creds L2Creds, address, method, path string, rawBody []byte, timestamp int64) (map[string]string, error) {
	if creds.APIKey == "" || creds.Secret == "" || creds.Passphrase == "" {
		return nil, fmt.Errorf("L2 creds incomplete (api_key/secret/passphrase): run `auth derive` first")
	}
	if address == "" {
		return nil, fmt.Errorf("address required for POLY_ADDRESS header")
	}
	secretBytes, err := base64.URLEncoding.DecodeString(padBase64(creds.Secret))
	if err != nil {
		// Some Polymarket responses return secret without padding.
		secretBytes, err = base64.RawURLEncoding.DecodeString(creds.Secret)
		if err != nil {
			return nil, fmt.Errorf("decode L2 secret as base64url: %w", err)
		}
	}
	body := ""
	if len(rawBody) > 0 {
		body = string(rawBody)
	}
	ts := fmt.Sprintf("%d", timestamp)
	msg := ts + method + path + body
	mac := hmac.New(sha256.New, secretBytes)
	mac.Write([]byte(msg))
	sig := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	return map[string]string{
		"POLY_ADDRESS":    address,
		"POLY_API_KEY":    creds.APIKey,
		"POLY_PASSPHRASE": creds.Passphrase,
		"POLY_SIGNATURE":  sig,
		"POLY_TIMESTAMP":  ts,
		"Content-Type":    "application/json",
	}, nil
}

// padBase64 right-pads a base64url string with `=` to a multiple of 4. The
// standard base64.URLEncoding decoder requires padding; if Polymarket
// returns the secret unpadded, decoding errors with "illegal base64 data".
func padBase64(s string) string {
	switch len(s) % 4 {
	case 2:
		return s + "=="
	case 3:
		return s + "="
	default:
		return s
	}
}

// QueryString builds a deterministic ?k=v&k=v string suitable for HMAC
// signing. Keys are sorted alphabetically (lexicographic), matching
// py-clob-client's behavior. Empty params returns "".
func QueryString(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	v := url.Values{}
	for k, val := range params {
		v.Set(k, val)
	}
	return "?" + v.Encode()
}

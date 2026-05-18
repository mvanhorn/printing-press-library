// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

// JWTClaims is the minimal claim subset needed to reason about expiry.
type JWTClaims struct {
	IssuedAt  int64 `json:"iat"`
	ExpiresAt int64 `json:"exp"`
}

// DecodeJWTClaims parses the unverified payload of a Clerk-style JWT. The
// signature is not validated — Suno's API enforces signature on every
// request, so the CLI only inspects the payload to decide whether to bother
// sending a token it already knows is expired. Returns ok=false for any
// shape that isn't a parseable three-segment JWT (e.g. opaque tokens or
// plain Bearer strings without a JWT body).
func DecodeJWTClaims(token string) (JWTClaims, bool) {
	t := strings.TrimSpace(token)
	t = strings.TrimPrefix(t, "Bearer ")
	t = strings.TrimPrefix(t, "bearer ")
	parts := strings.Split(t, ".")
	if len(parts) != 3 {
		return JWTClaims{}, false
	}
	// JWT segments are unpadded base64url per RFC 7519, so RawURLEncoding is
	// the correct decoder. Fall back to padded URLEncoding only for tokens
	// from non-conforming issuers that happen to include `=` padding.
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(addBase64Padding(parts[1]))
		if err != nil {
			return JWTClaims{}, false
		}
	}
	var claims JWTClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return JWTClaims{}, false
	}
	if claims.ExpiresAt == 0 {
		return JWTClaims{}, false
	}
	return claims, true
}

// JWTExpiry returns the expiry time of a JWT-shaped bearer token. Returns
// ok=false for opaque tokens or for JWTs that omit the exp claim.
func JWTExpiry(token string) (time.Time, bool) {
	claims, ok := DecodeJWTClaims(token)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(claims.ExpiresAt, 0), true
}

// AuthHeaderExpiry returns the expiry time of the currently configured auth
// header, when that header carries a decodable JWT. Returns ok=false for
// configs with no auth header or with an opaque token.
func (c *Config) AuthHeaderExpiry() (time.Time, bool) {
	if c == nil {
		return time.Time{}, false
	}
	return JWTExpiry(c.AuthHeader())
}

func addBase64Padding(s string) string {
	if pad := len(s) % 4; pad != 0 {
		return s + strings.Repeat("=", 4-pad)
	}
	return s
}

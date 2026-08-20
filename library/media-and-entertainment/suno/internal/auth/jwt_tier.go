// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// Extract the Suno tier id from the Clerk JWT. Suno's /api/generate body
// requires metadata.user_tier to be the account's tier UUID (an empty string
// makes the server return 500). The web app reads it from the JWT's "plan"
// claim, shaped "<tier-uuid>:<period>:", so the CLI derives it the same way
// from the token it already holds.

package auth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

// PlanTier returns the tier UUID from a JWT's "plan" claim (the substring
// before the first ':'), or "" if the token can't be decoded or has no plan.
func PlanTier(jwt string) string {
	jwt = strings.TrimSpace(strings.TrimPrefix(jwt, "Bearer "))
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Some encoders include padding / use std alphabet.
		if payload, err = base64.URLEncoding.DecodeString(parts[1]); err != nil {
			return ""
		}
	}
	var claims struct {
		Plan string `json:"plan"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Plan == "" {
		return ""
	}
	return strings.SplitN(claims.Plan, ":", 2)[0]
}

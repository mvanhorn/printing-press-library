package client

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

// extractUserIDFromJWT best-effort reads the `sub` claim from a JWT.
// Returns "" on any decoding error — the X-User-ID header is optional.
func extractUserIDFromJWT(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Try non-raw URL encoding as well (some signers use padded base64url).
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return claims.Sub
}

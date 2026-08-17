package athlinks

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// RacerIDFromToken extracts the athlinks-racer-id claim from a JWT bearer token
// (with or without a leading "Bearer "). No signature verification — the value
// is read from the payload segment only.
func RacerIDFromToken(token string) (string, error) {
	token = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(token), "Bearer "))
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("athlinks: token is not a JWT")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("athlinks: decode token payload: %w", err)
	}
	var claims struct {
		RacerID json.Number `json:"athlinks-racer-id"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return "", fmt.Errorf("athlinks: parse token claims: %w", err)
	}
	if claims.RacerID == "" {
		return "", fmt.Errorf("athlinks: token has no athlinks-racer-id")
	}
	return claims.RacerID.String(), nil
}

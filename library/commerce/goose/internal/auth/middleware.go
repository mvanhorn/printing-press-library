package auth

import (
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/goose/internal/config"
)

// RefreshIfNeeded checks the config's stored expiry; if the access token
// is within `slack` of expiring (or already expired), call Cognito to mint a
// fresh one and update the config. No-op when there is no refresh token.
//
// Returns an error only on actual refresh failure. If there's no refresh
// token configured (e.g., user is on env-var-only mode), this is silent.
func RefreshIfNeeded(cfg *config.Config, slack time.Duration) error {
	if cfg == nil || cfg.RefreshToken == "" {
		return nil
	}
	if !ExpiryNearOrPast(cfg.TokenExpiry, slack) {
		return nil
	}
	res, err := Refresh(cfg.RefreshToken)
	if err != nil {
		return err
	}
	cfg.AccessToken = res.AccessToken
	cfg.TokenExpiry = res.ExpiresAt
	cfg.AuthHeaderVal = "Bearer " + res.AccessToken
	return config.Save(cfg)
}
